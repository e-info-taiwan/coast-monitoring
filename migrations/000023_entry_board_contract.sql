-- A master catalog includes historical variants, not just the volunteer board.
-- Pin required rows to the submitted template; never synthesize observations.
CREATE TABLE reef_entry_required_taxon (
 template_format text NOT NULL CHECK (template_format='reefcheck-web-v4'),
 taxon_id integer NOT NULL REFERENCES taxon(id),
 row_id text NOT NULL,
 PRIMARY KEY(template_format,taxon_id)
);
CREATE TABLE reef_entry_required_impact (
 template_format text NOT NULL CHECK (template_format='reefcheck-web-v4'),
 impact_type_id integer NOT NULL REFERENCES impact_type(id),
 row_id text NOT NULL,
 PRIMARY KEY(template_format,impact_type_id)
);
INSERT INTO reef_entry_required_taxon(template_format,taxon_id,row_id)
SELECT 'reefcheck-web-v4',x.id,required.row_id FROM taxon x JOIN (VALUES
 ('fish','Butterflyfish','','0'),('fish','Haemulidae','','1'),('fish','Snapper','','2'),
 ('fish','Barramundi cod','','3'),('fish','Humphead wrasse','','4'),('fish','Bumphead parrotfish','','5'),
 ('fish','Parrotfish','>20cm','6'),('fish','Moray eel','','7'),
 ('fish','Grouper','<30cm','8'),('fish','Grouper','30-40 cm','9'),('fish','Grouper','40-50 cm','10'),
 ('fish','Grouper','50-60 cm','11'),('fish','Grouper','>60 cm','12'),('fish','Grouper','','13'),
 ('invert','Banded coral shrimp','','0'),('invert','Diadema','','1'),('invert','Pencil urchin','','2'),
 ('invert','Collector urchin','','3'),('invert','Sea cucumber','','4'),('invert','Crown-of-thorns','','5'),
 ('invert','Triton','','6'),('invert','Lobster','','7'),('invert','Giant clam','<10 cm','8'),
 ('invert','Giant clam','10-20 cm','9'),('invert','Giant clam','20-30 cm','10'),
 ('invert','Giant clam','30-40 cm','11'),('invert','Giant clam','40-50 cm','12'),('invert','Giant clam','>50 cm','13'),
 ('rare','Sharks','','0'),('rare','Turtles','','1'),('rare','Mantas','','2')
) AS required(taxon_group,name_en,size_class,row_id)
ON x.taxon_group::text=required.taxon_group AND lower(x.name_en)=lower(required.name_en)
AND COALESCE(x.size_class,'')=required.size_class AND NOT x.is_aggregate;
INSERT INTO reef_entry_required_impact(template_format,impact_type_id,row_id)
SELECT 'reefcheck-web-v4',x.id,required.row_id FROM impact_type x JOIN (VALUES
 ('coral_damage','Coral damage: boat/anchor','0'),('coral_damage','Coral damage: dynamite','1'),
 ('coral_damage','Coral damage: other','2'),('trash','Trash: fish nets','3'),('trash','Trash: general','4'),
 ('bleaching','Bleaching (% of population)','5'),('bleaching','Bleaching (% of colony)','6'),
 ('disease','Black Band (% colonies)','7'),('disease','White Band (% colonies)','8')
) AS required(impact_group,name_en,row_id)
ON x.impact_group::text=required.impact_group AND lower(x.name_en)=lower(required.name_en);
DO $$ BEGIN
 IF (SELECT count(*) FROM reef_entry_required_taxon)<>31
 OR (SELECT count(*) FROM reef_entry_required_impact)<>9 THEN
  RAISE EXCEPTION 'Volunteer board cannot be mapped to the canonical master catalog';
 END IF;
END $$;

-- All other completeness rules are retained. Historical/non-v4 events retain
-- their existing checks, and no imported values or master rows are changed.
CREATE OR REPLACE FUNCTION reef_event_board_completeness(target text) RETURNS text[] LANGUAGE sql STABLE AS $$
WITH e AS (SELECT e.*,s.site_id FROM event e JOIN survey s ON s.id=e.survey_id WHERE event_id=target),
t AS (SELECT t.*,COALESCE(e.submission_snapshot->>'format','') AS template_format FROM transect t JOIN e ON e.event_id=t.event_id),
missing_taxa AS (
 SELECT x.taxon_group::text AS board,x.name_zh,COALESCE(x.size_class,'') AS size_class,seg
 FROM t JOIN taxon x ON x.is_active AND NOT x.is_aggregate CROSS JOIN generate_series(1,4) seg
 WHERE ((t.method='belt_fish' AND x.taxon_group='fish' AND (lower(x.name_en)<>'grouper' OR (t.fish_size_mode='combined')=(COALESCE(x.size_class,'')='')))
 OR (t.method='belt_invert' AND (x.taxon_group='invert' OR (x.taxon_group='rare' AND lower(COALESCE(x.name_en,''))<>'other'))))
 AND (t.template_format<>'reefcheck-web-v4' OR EXISTS(SELECT 1 FROM reef_entry_required_taxon r WHERE r.template_format=t.template_format AND r.taxon_id=x.id))
 AND NOT EXISTS(SELECT 1 FROM belt_observation b WHERE b.transect_id=t.id AND b.taxon_id=x.id AND b.segment=seg)
), missing_impacts AS (
 SELECT x.name_zh,seg FROM t CROSS JOIN impact_type x CROSS JOIN generate_series(1,4) seg
 WHERE t.method='belt_invert' AND x.is_active
 AND (t.template_format<>'reefcheck-web-v4' OR EXISTS(SELECT 1 FROM reef_entry_required_impact r WHERE r.template_format=t.template_format AND r.impact_type_id=x.id))
 AND NOT EXISTS(SELECT 1 FROM impact_observation o WHERE o.transect_id=t.id AND o.impact_type_id=x.id AND o.segment=seg)
), issues AS (
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
 UNION ALL SELECT format('%s手板缺少「%s%s」：第 %s 段',CASE board WHEN 'fish' THEN '魚類' WHEN 'invert' THEN '無脊椎' ELSE '罕見生物' END,name_zh,CASE WHEN size_class='' THEN '' ELSE ' '||size_class END,string_agg(seg::text,'、' ORDER BY seg))
 FROM missing_taxa GROUP BY board,name_zh,size_class
 UNION ALL SELECT format('環境衝擊手板缺少「%s」：第 %s 段',name_zh,string_agg(seg::text,'、' ORDER BY seg)) FROM missing_impacts GROUP BY name_zh
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
