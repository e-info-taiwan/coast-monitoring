-- Existing imports have not been reviewed for publication. Never publish implicitly.
ALTER TABLE event ADD COLUMN publication_status text NOT NULL DEFAULT 'draft'
  CHECK (publication_status IN ('draft','published','archived'));
ALTER TABLE event ADD COLUMN updated_at timestamptz NOT NULL DEFAULT now();
ALTER TABLE event ADD COLUMN missing_reason text NOT NULL DEFAULT '';
ALTER TABLE event ADD COLUMN submission_snapshot jsonb;
ALTER TABLE transect ADD COLUMN fish_size_mode text
  CHECK (fish_size_mode IN ('split','combined'));
ALTER TABLE transect ADD COLUMN classify_hard_coral boolean NOT NULL DEFAULT false;
ALTER TABLE belt_observation ADD COLUMN record_status text NOT NULL DEFAULT 'recorded'
  CHECK (record_status IN ('recorded','not_recorded'));
ALTER TABLE impact_observation ADD COLUMN record_status text NOT NULL DEFAULT 'recorded'
  CHECK (record_status IN ('recorded','not_recorded'));
ALTER TABLE substrate_bleaching ADD COLUMN hc_record_status text NOT NULL DEFAULT 'recorded'
  CHECK (hc_record_status IN ('recorded','not_recorded'));
ALTER TABLE substrate_bleaching ADD COLUMN sc_record_status text NOT NULL DEFAULT 'recorded'
  CHECK (sc_record_status IN ('recorded','not_recorded'));
-- numeric_code=0 previously meant NA. Codes remain explicit and historical data is untouched.
UPDATE substrate_type SET numeric_code=-1 WHERE code='NA';
UPDATE substrate_type SET numeric_code=0 WHERE code='OT';
INSERT INTO substrate_type(code,numeric_code,name_zh,name_en,sort_order) VALUES
 ('HC-a',11,'硬珊瑚 a','hard coral a',11),('HC-b',12,'硬珊瑚 b','hard coral b',12),('HC-c',13,'硬珊瑚 c','hard coral c',13);
INSERT INTO taxon(taxon_group,name_zh,name_en,size_class,sort_order) VALUES
 ('fish','石斑魚（不分體長）','Grouper',NULL,135)
 ON CONFLICT DO NOTHING;
UPDATE impact_type SET has_raw_count=true WHERE impact_group IN ('coral_damage','trash');
CREATE OR REPLACE VIEW impact_observation_with_level AS
 SELECT o.*, CASE WHEN o.record_status='not_recorded' THEN NULL
 WHEN t.has_raw_count THEN CASE WHEN o.raw_value=0 THEN 0 WHEN o.raw_value=1 THEN 1 WHEN o.raw_value<=4 THEN 2 ELSE 3 END
 ELSE NULL END AS derived_level
 FROM impact_observation o JOIN impact_type t ON t.id=o.impact_type_id;
-- Live queries rather than cached summaries: edits are reflected immediately.
CREATE VIEW belt_summary AS
 SELECT o.transect_id,o.taxon_id,count(*) FILTER (WHERE record_status='recorded')::int AS n,
 sum(count) FILTER (WHERE record_status='recorded') AS total,
 avg(count) FILTER (WHERE record_status='recorded') AS mean,
 stddev_samp(count) FILTER (WHERE record_status='recorded') AS sd,
 stddev_samp(count) FILTER (WHERE record_status='recorded') / sqrt(NULLIF(count(*) FILTER (WHERE record_status='recorded'),0)) AS se
 FROM belt_observation o GROUP BY o.transect_id,o.taxon_id;
CREATE VIEW impact_summary AS
 SELECT o.transect_id,o.impact_type_id,count(*) FILTER (WHERE record_status='recorded')::int AS n,
 sum(raw_value) FILTER (WHERE record_status='recorded') AS total,
 avg(raw_value) FILTER (WHERE record_status='recorded') AS mean,
 stddev_samp(raw_value) FILTER (WHERE record_status='recorded') AS sd,
 stddev_samp(raw_value) FILTER (WHERE record_status='recorded') / sqrt(NULLIF(count(*) FILTER (WHERE record_status='recorded'),0)) AS se,
 avg(derived_level) AS level_mean, stddev_samp(derived_level) AS level_sd,
 stddev_samp(derived_level)/sqrt(NULLIF(count(derived_level),0)) AS level_se
 FROM impact_observation_with_level o GROUP BY o.transect_id,o.impact_type_id;
CREATE VIEW substrate_segment_summary AS
 WITH points AS (
 SELECT transect_id,segment,CASE WHEN substrate_code IN ('HC-a','HC-b','HC-c') THEN 'HC' ELSE substrate_code END AS code
 FROM substrate_point WHERE substrate_layer='surface'),
 categories AS (SELECT code FROM substrate_type WHERE code NOT IN ('NA','HC-a','HC-b','HC-c') UNION SELECT 'live_coral'),
 segments AS (SELECT transect_id,segment,count(*) FILTER (WHERE code<>'NA') AS valid_points FROM points GROUP BY transect_id,segment)
 SELECT s.transect_id,s.segment,c.code,s.valid_points,
 count(p.code) FILTER (WHERE p.code=c.code OR (c.code='live_coral' AND p.code IN ('HC','SC'))) AS category_points,
 100.0*count(p.code) FILTER (WHERE p.code=c.code OR (c.code='live_coral' AND p.code IN ('HC','SC')))/NULLIF(s.valid_points,0) AS coverage_percent
 FROM segments s CROSS JOIN categories c LEFT JOIN points p ON p.transect_id=s.transect_id AND p.segment=s.segment
 GROUP BY s.transect_id,s.segment,c.code,s.valid_points;
CREATE VIEW substrate_summary AS
 SELECT transect_id,code,sum(valid_points) AS valid_points,sum(category_points) AS total,
 100.0*sum(category_points)/NULLIF(sum(valid_points),0) AS coverage_percent,
 count(coverage_percent)::int AS n,avg(coverage_percent) AS mean,stddev_samp(coverage_percent) AS sd,
 stddev_samp(coverage_percent)/sqrt(NULLIF(count(coverage_percent),0)) AS se
 FROM substrate_segment_summary GROUP BY transect_id,code;
CREATE VIEW substrate_bleaching_summary AS
 WITH segments AS (
 SELECT b.transect_id,b.segment,c.code,
 CASE WHEN c.code='HC' AND b.hc_record_status='recorded' THEN b.hc_bleached_count
 WHEN c.code='SC' AND b.sc_record_status='recorded' THEN b.sc_bleached_count END AS bleached,
 s.category_points AS denominator
 FROM substrate_bleaching b CROSS JOIN (VALUES ('HC'),('SC')) c(code)
 LEFT JOIN substrate_segment_summary s ON s.transect_id=b.transect_id AND s.segment=b.segment AND s.code=c.code)
 SELECT transect_id,code,count(bleached)::int AS n,sum(bleached) AS total,avg(bleached) AS mean,
 stddev_samp(bleached) AS sd,stddev_samp(bleached)/sqrt(NULLIF(count(bleached),0)) AS se,
 100.0*sum(bleached)/NULLIF(sum(denominator) FILTER (WHERE bleached IS NOT NULL),0) AS bleaching_percent
 FROM segments GROUP BY transect_id,code;
CREATE TABLE reef_publication_audit (
 id bigserial PRIMARY KEY,event_id integer NOT NULL REFERENCES event(id) ON DELETE CASCADE,
 old_status text NOT NULL,new_status text NOT NULL,created_at timestamptz NOT NULL DEFAULT now()
);
