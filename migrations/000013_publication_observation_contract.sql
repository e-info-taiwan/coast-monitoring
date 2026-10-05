ALTER FUNCTION reef_event_completeness(text) RENAME TO reef_event_board_completeness;
CREATE FUNCTION reef_event_completeness(target text) RETURNS text[] LANGUAGE sql STABLE AS $$
WITH t AS(SELECT * FROM transect WHERE event_id=target), issues AS (
 SELECT unnest(reef_event_board_completeness(target)) AS issue
 UNION ALL SELECT '底質白化附表需完整四段，未記錄請標記 NA' FROM t CROSS JOIN generate_series(1,4) seg
 WHERE t.method='line' AND NOT EXISTS(SELECT 1 FROM substrate_bleaching b WHERE b.transect_id=t.id AND b.segment=seg)
 UNION ALL SELECT '白化點數超過對應底質點數' FROM t JOIN substrate_bleaching b ON b.transect_id=t.id
 WHERE (b.hc_record_status='recorded' AND (b.hc_bleached_count<0 OR b.hc_bleached_count>(SELECT count(*) FROM substrate_point p WHERE p.transect_id=t.id AND p.segment=b.segment AND p.substrate_layer='surface' AND p.substrate_code IN('HC','HC-a','HC-b','HC-c'))))
 OR (b.sc_record_status='recorded' AND (b.sc_bleached_count<0 OR b.sc_bleached_count>(SELECT count(*) FROM substrate_point p WHERE p.transect_id=t.id AND p.segment=b.segment AND p.substrate_layer='surface' AND p.substrate_code='SC')))
 UNION ALL SELECT '生物觀測方法或體長模式不符，合計列不可手填' FROM t JOIN belt_observation b ON b.transect_id=t.id JOIN taxon x ON x.id=b.taxon_id
 WHERE x.is_aggregate OR (x.taxon_group='fish' AND t.method<>'belt_fish') OR (x.taxon_group<>'fish' AND t.method<>'belt_invert')
 OR (x.taxon_group='fish' AND lower(x.name_en)='grouper' AND (t.fish_size_mode='combined')<>(COALESCE(x.size_class,'')=''))
 UNION ALL SELECT '環境衝擊調查方法不符' FROM t JOIN impact_observation o ON o.transect_id=t.id WHERE t.method<>'belt_invert'
) SELECT COALESCE(array_agg(DISTINCT issue),'{}'::text[]) FROM issues;
$$;
