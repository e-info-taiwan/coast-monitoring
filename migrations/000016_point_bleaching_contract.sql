-- Reuse the established catalog/board checks. Point-mode entries do not also
-- write manual totals: imported substrate_bleaching records remain unchanged.
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
 WHERE t.point_mode AND p.substrate_layer='surface' AND p.substrate_code IN('HC','HC-a','HC-b','HC-c','SC') AND p.is_bleached IS NULL
 UNION ALL SELECT '非表面珊瑚點不可標記白化' FROM t JOIN substrate_point p ON p.transect_id=t.id
 WHERE p.is_bleached IS NOT NULL AND (p.substrate_layer<>'surface' OR p.substrate_code NOT IN('HC','HC-a','HC-b','HC-c','SC'))
 UNION ALL SELECT '白化點數超過對應底質點數' FROM t JOIN substrate_bleaching b ON b.transect_id=t.id
 WHERE (b.hc_record_status='recorded' AND (b.hc_bleached_count<0 OR b.hc_bleached_count>(SELECT count(*) FROM substrate_point p WHERE p.transect_id=t.id AND p.segment=b.segment AND p.substrate_layer='surface' AND p.substrate_code IN('HC','HC-a','HC-b','HC-c'))))
 OR (b.sc_record_status='recorded' AND (b.sc_bleached_count<0 OR b.sc_bleached_count>(SELECT count(*) FROM substrate_point p WHERE p.transect_id=t.id AND p.segment=b.segment AND p.substrate_layer='surface' AND p.substrate_code='SC')))
 UNION ALL SELECT '生物觀測方法或體長模式不符，合計列不可手填' FROM t JOIN belt_observation b ON b.transect_id=t.id JOIN taxon x ON x.id=b.taxon_id
 WHERE x.is_aggregate OR (x.taxon_group='fish' AND t.method<>'belt_fish') OR (x.taxon_group<>'fish' AND t.method<>'belt_invert')
 OR (x.taxon_group='fish' AND lower(x.name_en)='grouper' AND (t.fish_size_mode='combined')<>(COALESCE(x.size_class,'')=''))
 UNION ALL SELECT '環境衝擊調查方法不符' FROM t JOIN impact_observation o ON o.transect_id=t.id WHERE t.method<>'belt_invert'
) SELECT COALESCE(array_agg(DISTINCT issue),'{}'::text[]) FROM issues;
$$;

-- The public summary contract is unchanged; choose point-derived values only
-- for explicitly identified v4 entries, otherwise use the historical raw count.
CREATE OR REPLACE VIEW substrate_bleaching_summary AS
WITH point_mode AS (
 SELECT t.id FROM transect t JOIN event e ON e.event_id=t.event_id
 WHERE e.submission_snapshot->>'format'='reefcheck-web-v4'
 AND e.submission_snapshot->>'bleaching_mode'='point_marks'
), counts AS (
 SELECT b.transect_id,b.segment,'HC'::text AS code,
 CASE WHEN b.hc_record_status='recorded' THEN b.hc_bleached_count END::numeric AS bleached
 FROM substrate_bleaching b WHERE NOT EXISTS(SELECT 1 FROM point_mode m WHERE m.id=b.transect_id)
 UNION ALL SELECT b.transect_id,b.segment,'SC',CASE WHEN b.sc_record_status='recorded' THEN b.sc_bleached_count END::numeric
 FROM substrate_bleaching b WHERE NOT EXISTS(SELECT 1 FROM point_mode m WHERE m.id=b.transect_id)
 UNION ALL SELECT p.transect_id,p.segment,'HC',p.derived_hc_bleached_point_count::numeric FROM substrate_bleaching_point_summary p JOIN point_mode m ON m.id=p.transect_id
 UNION ALL SELECT p.transect_id,p.segment,'SC',p.derived_sc_bleached_point_count::numeric FROM substrate_bleaching_point_summary p JOIN point_mode m ON m.id=p.transect_id
), segments AS (
 SELECT c.*,s.category_points AS denominator FROM counts c LEFT JOIN substrate_segment_summary s
 ON s.transect_id=c.transect_id AND s.segment=c.segment AND s.code=c.code
)
SELECT transect_id,code,count(bleached)::int AS n,sum(bleached)::bigint AS total,avg(bleached) AS mean,
stddev_samp(bleached) AS sd,stddev_samp(bleached)/sqrt(NULLIF(count(bleached),0)) AS se,
100.0*sum(bleached)/NULLIF(sum(denominator) FILTER (WHERE bleached IS NOT NULL),0) AS bleaching_percent
FROM segments GROUP BY transect_id,code;
