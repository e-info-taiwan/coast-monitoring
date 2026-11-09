-- 000021 restores the OT master number; health status is a separate migration.
-- Store the explicit point-level answer. Not bleached does not mean healthy.
-- Preserve legacy NULLs; do not infer point positions from historical totals.
ALTER TABLE substrate_point ADD COLUMN health_status text
  CHECK (health_status IN ('bleached','not_bleached'));
UPDATE substrate_point SET health_status=CASE WHEN is_bleached THEN 'bleached' ELSE 'not_bleached' END
WHERE is_bleached IS NOT NULL;
COMMENT ON COLUMN substrate_point.health_status IS
  'Point bleaching answer: bleached / confirmed not_bleached; NULL=unreviewed, legacy unknown or not applicable. Not a general health diagnosis.';

-- Keep old readers/writers compatible without allowing two contradictory facts.
CREATE FUNCTION sync_substrate_point_health() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE expected boolean;
BEGIN
  IF TG_OP='UPDATE' THEN
    IF NEW.health_status IS DISTINCT FROM OLD.health_status THEN
      expected:=CASE NEW.health_status WHEN 'bleached' THEN true WHEN 'not_bleached' THEN false ELSE NULL END;
      IF NEW.is_bleached IS DISTINCT FROM OLD.is_bleached AND NEW.is_bleached IS DISTINCT FROM expected THEN
        RAISE EXCEPTION 'health_status and is_bleached disagree' USING ERRCODE='23514';
      END IF;
      NEW.is_bleached:=expected;
    ELSIF NEW.is_bleached IS DISTINCT FROM OLD.is_bleached THEN
      NEW.health_status:=CASE WHEN NEW.is_bleached IS NULL THEN NULL WHEN NEW.is_bleached THEN 'bleached' ELSE 'not_bleached' END;
    END IF;
  ELSE
    IF NEW.health_status IS NULL AND NEW.is_bleached IS NOT NULL THEN
      NEW.health_status:=CASE WHEN NEW.is_bleached THEN 'bleached' ELSE 'not_bleached' END;
    ELSIF NEW.health_status IS NOT NULL AND NEW.is_bleached IS NULL THEN
      NEW.is_bleached:=CASE NEW.health_status WHEN 'bleached' THEN true WHEN 'not_bleached' THEN false ELSE NULL END;
    END IF;
  END IF;
  RETURN NEW;
END;
$$;
CREATE TRIGGER substrate_point_health_sync BEFORE INSERT OR UPDATE ON substrate_point
FOR EACH ROW EXECUTE FUNCTION sync_substrate_point_health();
ALTER TABLE substrate_point ADD CONSTRAINT substrate_point_health_consistent CHECK (
  is_bleached IS NOT DISTINCT FROM CASE health_status WHEN 'bleached' THEN true WHEN 'not_bleached' THEN false ELSE NULL END
);
COMMENT ON COLUMN substrate_point.is_bleached IS
  'Legacy compatibility mirror of health_status; true=bleached, false=not_bleached, NULL=no point answer. Kept synchronized.';

CREATE OR REPLACE VIEW substrate_bleaching_point_summary AS
SELECT transect_id,segment,
  CASE WHEN count(*) FILTER (WHERE substrate_code IN ('HC','HC-a','HC-b','HC-c') AND health_status IS NULL)=0
    THEN count(*) FILTER (WHERE substrate_code IN ('HC','HC-a','HC-b','HC-c') AND health_status='bleached') END AS derived_hc_bleached_point_count,
  CASE WHEN count(*) FILTER (WHERE substrate_code='SC' AND health_status IS NULL)=0
    THEN count(*) FILTER (WHERE substrate_code='SC' AND health_status='bleached') END AS derived_sc_bleached_point_count
FROM substrate_point WHERE substrate_layer='surface'
GROUP BY transect_id,segment;

CREATE OR REPLACE FUNCTION reef_event_completeness(target text) RETURNS text[] LANGUAGE sql STABLE AS $$
WITH t AS (
 SELECT t.*,COALESCE(e.submission_snapshot->>'bleaching_mode','')='point_marks'
 AND e.submission_snapshot->>'format'='reefcheck-web-v4' AS point_mode
 FROM transect t JOIN event e ON e.event_id=t.event_id WHERE t.event_id=target
), issues AS (
 SELECT unnest(reef_event_board_completeness(target)) AS issue
 UNION ALL SELECT '底質白化附表需完整四段，未記錄請標記 NA' FROM t CROSS JOIN generate_series(1,4) seg
 WHERE t.method='line' AND NOT t.point_mode AND NOT EXISTS(SELECT 1 FROM substrate_bleaching b WHERE b.transect_id=t.id AND b.segment=seg)
 UNION ALL SELECT '逐格珊瑚白化尚未完成核對' FROM t JOIN substrate_point p ON p.transect_id=t.id
 WHERE t.point_mode AND p.substrate_layer='surface' AND p.substrate_code IN('HC','HC-a','HC-b','HC-c','SC') AND p.health_status IS NULL
 UNION ALL SELECT '非表面珊瑚點不可標記白化' FROM t JOIN substrate_point p ON p.transect_id=t.id
 WHERE p.health_status IS NOT NULL AND (p.substrate_layer<>'surface' OR p.substrate_code NOT IN('HC','HC-a','HC-b','HC-c','SC'))
 UNION ALL SELECT '白化點數超過對應底質點數' FROM t JOIN substrate_bleaching b ON b.transect_id=t.id
 WHERE (b.hc_record_status='recorded' AND (b.hc_bleached_count<0 OR b.hc_bleached_count>(SELECT count(*) FROM substrate_point p WHERE p.transect_id=t.id AND p.segment=b.segment AND p.substrate_layer='surface' AND p.substrate_code IN('HC','HC-a','HC-b','HC-c'))))
 OR (b.sc_record_status='recorded' AND (b.sc_bleached_count<0 OR b.sc_bleached_count>(SELECT count(*) FROM substrate_point p WHERE p.transect_id=t.id AND p.segment=b.segment AND p.substrate_layer='surface' AND p.substrate_code='SC')))
 UNION ALL SELECT '生物觀測方法或體長模式不符，合計列不可手填' FROM t JOIN belt_observation b ON b.transect_id=t.id JOIN taxon x ON x.id=b.taxon_id
 WHERE x.is_aggregate OR (x.taxon_group='fish' AND t.method<>'belt_fish') OR (x.taxon_group<>'fish' AND t.method<>'belt_invert')
 OR (x.taxon_group='fish' AND lower(x.name_en)='grouper' AND (t.fish_size_mode='combined')<>(COALESCE(x.size_class,'')=''))
 UNION ALL SELECT '環境衝擊調查方法不符' FROM t JOIN impact_observation o ON o.transect_id=t.id WHERE t.method<>'belt_invert'
) SELECT COALESCE(array_agg(DISTINCT issue),'{}'::text[]) FROM issues;
$$;
