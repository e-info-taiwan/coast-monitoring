-- A shared database check for submitted drafts and explicit publication.
-- Only methods belonging to this event are required; missing observations are
-- explicit NA with a reason, never inferred zeroes.
CREATE FUNCTION reef_event_completeness(target text) RETURNS text[] LANGUAGE sql STABLE AS $$
WITH e AS (SELECT e.*,s.site_id FROM event e JOIN survey s ON s.id=e.survey_id WHERE event_id=target),
t AS (SELECT * FROM transect WHERE event_id=target),
issues AS (
 SELECT '缺少正式樣點、日期、時間或深度' AS issue FROM e
 WHERE NOT EXISTS(SELECT 1 FROM site WHERE id=e.site_id AND is_active)
 OR event_time !~ '^([01][0-9]|2[0-3])[:-][0-5][0-9]$' OR depth_m<0
 UNION ALL SELECT '至少需要一個調查方法' WHERE NOT EXISTS(SELECT 1 FROM t)
 UNION ALL SELECT '每個方法需有記錄者' FROM t WHERE NOT EXISTS(SELECT 1 FROM transect_participant p WHERE p.transect_id=t.id)
 UNION ALL SELECT '魚類體長模式未指定' FROM t WHERE method='belt_fish' AND (fish_size_mode IS NULL OR fish_size_mode NOT IN ('split','combined'))
 UNION ALL SELECT '底質需完整 160 點，四段各 40 個指定位置' FROM t
 WHERE method='line' AND (
 EXISTS(SELECT 1 FROM generate_series(1,4) seg CROSS JOIN generate_series(0,39) pos
 WHERE NOT EXISTS(SELECT 1 FROM substrate_point p WHERE p.transect_id=t.id AND p.substrate_layer='surface' AND p.segment=seg AND p.position_m=(seg-1)*25+pos*0.5))
 OR (SELECT count(*) FROM substrate_point p WHERE p.transect_id=t.id AND p.substrate_layer='surface')<>160)
 UNION ALL SELECT '物種手板缺少應填項目或段次' FROM t JOIN taxon x ON x.is_active AND NOT x.is_aggregate
 CROSS JOIN generate_series(1,4) seg
 WHERE ((t.method='belt_fish' AND x.taxon_group='fish' AND (lower(x.name_en)<>'grouper' OR (t.fish_size_mode='combined')=(COALESCE(x.size_class,'')='')))
 OR (t.method='belt_invert' AND (x.taxon_group='invert' OR (x.taxon_group='rare' AND lower(COALESCE(x.name_en,''))<>'other'))))
 AND NOT EXISTS(SELECT 1 FROM belt_observation b WHERE b.transect_id=t.id AND b.taxon_id=x.id AND b.segment=seg)
 UNION ALL SELECT '環境衝擊手板缺少應填項目或段次' FROM t CROSS JOIN impact_type x CROSS JOIN generate_series(1,4) seg
 WHERE t.method='belt_invert' AND x.is_active AND NOT EXISTS(SELECT 1 FROM impact_observation o WHERE o.transect_id=t.id AND o.impact_type_id=x.id AND o.segment=seg)
 UNION ALL SELECT '觀測值或 NA 狀態無效' FROM t WHERE
 EXISTS(SELECT 1 FROM belt_observation b WHERE b.transect_id=t.id AND (b.segment NOT BETWEEN 1 AND 4 OR b.count<0 OR b.record_status NOT IN ('recorded','not_recorded')))
 OR EXISTS(SELECT 1 FROM impact_observation o JOIN impact_type x ON x.id=o.impact_type_id WHERE o.transect_id=t.id AND (o.segment NOT BETWEEN 1 AND 4 OR o.raw_value<0 OR o.record_status NOT IN ('recorded','not_recorded') OR (o.record_status='recorded' AND ((x.value_type='percent' AND o.raw_value>100) OR (x.has_raw_count AND o.raw_value<>trunc(o.raw_value))))))
 UNION ALL SELECT '含 NA 需填未記錄原因' FROM e WHERE trim(COALESCE(missing_reason,''))='' AND EXISTS(SELECT 1 FROM t WHERE
 EXISTS(SELECT 1 FROM substrate_point p WHERE p.transect_id=t.id AND p.substrate_code='NA')
 OR EXISTS(SELECT 1 FROM belt_observation b WHERE b.transect_id=t.id AND b.record_status='not_recorded')
 OR EXISTS(SELECT 1 FROM impact_observation o WHERE o.transect_id=t.id AND o.record_status='not_recorded')
 OR EXISTS(SELECT 1 FROM substrate_bleaching b WHERE b.transect_id=t.id AND (b.hc_record_status='not_recorded' OR b.sc_record_status='not_recorded')))
) SELECT COALESCE(array_agg(DISTINCT issue),'{}'::text[]) FROM issues;
$$;
