-- Keep each historical impact's unit; NULL retains the existing web-entry contract.
ALTER TABLE impact_observation ADD COLUMN source_unit text
  CHECK (source_unit IN ('count','level','percent'));
ALTER TABLE impact_observation ALTER COLUMN raw_value DROP DEFAULT;
ALTER TABLE belt_observation ALTER COLUMN count DROP DEFAULT;
ALTER TABLE substrate_bleaching ALTER COLUMN hc_bleached_count DROP NOT NULL;
ALTER TABLE substrate_bleaching ALTER COLUMN sc_bleached_count DROP NOT NULL;
ALTER TABLE substrate_bleaching ALTER COLUMN hc_bleached_count DROP DEFAULT;
ALTER TABLE substrate_bleaching ALTER COLUMN sc_bleached_count DROP DEFAULT;
ALTER TABLE impact_observation ADD CONSTRAINT impact_source_range CHECK (
  source_unit IS NULL OR raw_value IS NULL OR
  (raw_value >= 0 AND CASE source_unit
    WHEN 'count' THEN raw_value = trunc(raw_value)
    WHEN 'level' THEN raw_value = trunc(raw_value) AND raw_value <= 3
    WHEN 'percent' THEN raw_value <= 100 END));

ALTER TABLE substrate_point ADD COLUMN raw_input_code text;
ALTER TABLE substrate_point ADD COLUMN source_template_or_version text;

INSERT INTO taxon(taxon_group,name_zh,name_en,size_class,sort_order)
 VALUES ('fish','鸚哥魚（大於等於 20cm）','Parrotfish','>=20cm',71),
 ('fish','鸚哥魚','Parrotfish','<20cm',72),
 ('fish','鸚哥魚','Parrotfish','20-30cm',73),
 ('fish','鸚哥魚','Parrotfish','>30cm',74),
 ('fish','蝴蝶魚','Butterflyfish','<5cm',11),
 ('fish','蝴蝶魚','Butterflyfish','>=5cm',12),
 ('fish','石鱸幼魚','Haemulidae','juvenile',21),
 ('fish','笛鯛','Snapper','<20cm',31),
 ('fish','笛鯛','Snapper','>=20cm',32),
 ('fish','笛鯛','Snapper','20-30cm',33),
 ('fish','笛鯛','Snapper','>30cm',34),
 ('fish','老鼠斑','Barramundi cod','<30cm',41),
 ('fish','老鼠斑','Barramundi cod','30-40 cm',42),
 ('fish','老鼠斑','Barramundi cod','40-50 cm',43),
 ('fish','老鼠斑','Barramundi cod','50-60 cm',44),
 ('fish','老鼠斑','Barramundi cod','>60 cm',45),
 ('invert','刺棘海膽','Echinothrix',NULL,165)
 ON CONFLICT DO NOTHING;
INSERT INTO impact_type(impact_group,name_zh,name_en,value_type,has_raw_count,sort_order)
 VALUES ('disease','珊瑚疾病總百分比（未分病種）','Coral disease (unspecified)','percent',false,100)
 ON CONFLICT (impact_group,name_en) DO NOTHING;

CREATE TABLE incidental_sighting (
 id bigserial PRIMARY KEY,
 transect_id integer NOT NULL REFERENCES transect(id) ON DELETE CASCADE,
 observation_scope text NOT NULL CHECK (observation_scope IN ('unspecified','event','segment')),
 segment smallint CHECK (segment BETWEEN 1 AND 4),
 taxon_id integer REFERENCES taxon(id),
 count integer CHECK (count >= 0),
 raw_text text NOT NULL,
 source_file text NOT NULL, source_sheet text NOT NULL, source_cell text NOT NULL,
 CHECK ((observation_scope='segment' AND segment IS NOT NULL)
     OR (observation_scope IN ('unspecified','event') AND segment IS NULL)),
 UNIQUE (transect_id,source_file,source_sheet,source_cell)
);

-- Preserve the original view's column order/types for its dependent summaries.
CREATE OR REPLACE VIEW impact_observation_with_level AS
 SELECT o.id,o.transect_id,o.impact_type_id,o.segment,o.raw_value,o.record_status,
 CASE WHEN o.record_status='not_recorded' OR o.raw_value IS NULL THEN NULL
 WHEN o.source_unit='level' THEN o.raw_value::integer
 WHEN o.source_unit='percent' THEN NULL
 WHEN o.source_unit='count' OR (o.source_unit IS NULL AND t.has_raw_count)
 THEN CASE WHEN o.raw_value=0 THEN 0 WHEN o.raw_value=1 THEN 1 WHEN o.raw_value<=4 THEN 2 ELSE 3 END
 ELSE NULL END AS derived_level, o.source_unit
 FROM impact_observation o JOIN impact_type t ON t.id=o.impact_type_id;

CREATE OR REPLACE VIEW impact_summary AS
 SELECT o.transect_id,o.impact_type_id,count(*) FILTER (WHERE record_status='recorded')::int AS n,
 sum(raw_value) FILTER (WHERE record_status='recorded') AS total,
 avg(raw_value) FILTER (WHERE record_status='recorded') AS mean,
 stddev_samp(raw_value) FILTER (WHERE record_status='recorded') AS sd,
 stddev_samp(raw_value) FILTER (WHERE record_status='recorded') / sqrt(NULLIF(count(*) FILTER (WHERE record_status='recorded'),0)) AS se,
 avg(derived_level) AS level_mean,stddev_samp(derived_level) AS level_sd,
 stddev_samp(derived_level)/sqrt(NULLIF(count(derived_level),0)) AS level_se,
 o.source_unit
 FROM impact_observation_with_level o GROUP BY o.transect_id,o.impact_type_id,o.source_unit;
