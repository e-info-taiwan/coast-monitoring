-- Compute only the selected transect. PL/pgSQL keeps the scoped query boundary:
-- PostgreSQL cannot inline the all-history aggregation ahead of the site filter.
CREATE FUNCTION reef_transect_series(p_transect_id integer, p_charts text[])
RETURNS TABLE(chart text,key text,name text,size_class text,unit text,total double precision,
 mean double precision,value double precision,sd double precision,se double precision,n integer,
 taxon_id integer,taxon_group text)
LANGUAGE plpgsql STABLE AS $function$
BEGIN
 IF p_charts && ARRAY['substrate','live_coral'] THEN
  RETURN QUERY WITH points AS MATERIALIZED (
   SELECT o.segment,CASE WHEN o.substrate_code IN ('HC-a','HC-b','HC-c') THEN 'HC' ELSE o.substrate_code END code
   FROM substrate_point o WHERE o.transect_id=p_transect_id AND o.substrate_layer='surface'
  ), categories AS (
   SELECT s.code,s.name_zh FROM substrate_type s WHERE s.code NOT IN ('NA','HC-a','HC-b','HC-c') AND 'substrate'=ANY(p_charts)
   UNION ALL SELECT 'live_coral','活珊瑚覆蓋率' WHERE 'live_coral'=ANY(p_charts)
  ), segments AS (
   SELECT p.segment,count(*) FILTER(WHERE p.code<>'NA') valid FROM points p GROUP BY p.segment
  ), samples AS (
   SELECT s.segment,c.code,c.name_zh,s.valid,
    count(p.code) FILTER(WHERE p.code=c.code OR (c.code='live_coral' AND p.code IN ('HC','SC'))) hits
   FROM segments s CROSS JOIN categories c LEFT JOIN points p ON p.segment=s.segment GROUP BY s.segment,c.code,c.name_zh,s.valid
  ), totals AS (
   SELECT s.code,s.name_zh,sum(s.hits) FILTER(WHERE s.valid>0) total,
    100.0*sum(s.hits)/NULLIF(sum(s.valid),0) value,
    avg(100.0*s.hits/NULLIF(s.valid,0)) mean,stddev_samp(100.0*s.hits/NULLIF(s.valid,0)) sd,
    count(*) FILTER(WHERE s.valid>0)::int n FROM samples s GROUP BY s.code,s.name_zh
  ) SELECT CASE WHEN t.code='live_coral' THEN 'live_coral' ELSE 'substrate' END,t.code,t.name_zh,NULL::text,'percent',
    t.total::float8,t.mean::float8,t.value::float8,t.sd::float8,(t.sd/sqrt(NULLIF(t.n,0)))::float8,t.n,NULL::int,''::text FROM totals t;
 END IF;
 IF 'taxa'=ANY(p_charts) THEN
  RETURN QUERY WITH samples AS (
   SELECT o.taxon_id,CASE WHEN o.record_status='recorded' THEN o.count END value FROM belt_observation o WHERE o.transect_id=p_transect_id
  ), totals AS (
   SELECT s.taxon_id,sum(s.value)::float8 total,avg(s.value)::float8 mean,stddev_samp(s.value)::float8 sd,count(s.value)::int n FROM samples s GROUP BY s.taxon_id
  ) SELECT 'taxa',x.id::text,x.name_zh,NULLIF(x.size_class,''),
   CASE WHEN x.taxon_group='rare' THEN 'individuals_per_subsample' ELSE 'individuals_per_100m2' END,
   t.total,t.mean,t.mean,t.sd,t.sd/sqrt(NULLIF(t.n,0)),t.n,x.id,x.taxon_group::text FROM totals t JOIN taxon x ON x.id=t.taxon_id;
 END IF;
 IF 'impact'=ANY(p_charts) THEN
  RETURN QUERY WITH samples AS (
   SELECT o.impact_type_id,x.name_zh,o.source_unit,
    CASE WHEN o.source_unit='percent' THEN 'percent' WHEN o.source_unit='level' THEN 'historical_level'
     WHEN o.source_unit='count' OR x.has_raw_count OR x.value_type='count' THEN 'count' WHEN x.value_type='percent' THEN 'percent' ELSE 'historical_level' END unit,
    CASE WHEN o.record_status='recorded' THEN o.raw_value END raw,
    o.derived_level,
    (o.source_unit IN ('count','level') OR (o.source_unit IS NULL AND x.has_raw_count)) include_level
   FROM impact_observation_with_level o JOIN impact_type x ON x.id=o.impact_type_id WHERE o.transect_id=p_transect_id
  ), totals AS (
   SELECT s.impact_type_id::text key,s.name_zh,s.unit,sum(s.raw)::float8 total,avg(s.raw)::float8 mean,stddev_samp(s.raw)::float8 sd,count(s.raw)::int n
   FROM samples s GROUP BY s.impact_type_id,s.name_zh,s.unit
   UNION ALL SELECT s.impact_type_id::text||':level',s.name_zh||'（分級）',CASE WHEN s.source_unit='level' THEN 'historical_level' ELSE 'derived_level' END,
    NULL::float8,avg(s.derived_level)::float8,stddev_samp(s.derived_level)::float8,count(s.derived_level)::int
   FROM samples s WHERE s.include_level GROUP BY s.impact_type_id,s.name_zh,CASE WHEN s.source_unit='level' THEN 'historical_level' ELSE 'derived_level' END
  ) SELECT 'impact',t.key,t.name_zh,NULL::text,t.unit,t.total,t.mean,t.mean,t.sd,t.sd/sqrt(NULLIF(t.n,0)),t.n,NULL::int,''::text FROM totals t;
 END IF;
 IF 'bleaching'=ANY(p_charts) THEN
  RETURN QUERY WITH samples AS (
   SELECT c.code,CASE WHEN c.code='HC' AND b.hc_record_status='recorded' THEN b.hc_bleached_count
    WHEN c.code='SC' AND b.sc_record_status='recorded' THEN b.sc_bleached_count END value
   FROM substrate_bleaching b CROSS JOIN (VALUES('HC'),('SC')) c(code) WHERE b.transect_id=p_transect_id
  ), totals AS (
   SELECT s.code,sum(s.value)::float8 total,avg(s.value)::float8 mean,stddev_samp(s.value)::float8 sd,count(s.value)::int n FROM samples s GROUP BY s.code
  ) SELECT 'bleaching',t.code,x.name_zh,NULL::text,'bleached_points',t.total,t.mean,t.mean,t.sd,t.sd/sqrt(NULLIF(t.n,0)),t.n,NULL::int,''::text FROM totals t JOIN substrate_type x ON x.code=t.code;
 END IF;
END $function$;

CREATE INDEX IF NOT EXISTS event_public_site_filter ON event(survey_id,survey_date,depth_m) WHERE publication_status='published';
CREATE INDEX IF NOT EXISTS survey_site_filter ON survey(site_id);

CREATE FUNCTION reef_event_modified() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 NEW.updated_at=clock_timestamp(); RETURN NEW;
END $$;
CREATE TRIGGER reef_event_modified BEFORE UPDATE ON event FOR EACH ROW EXECUTE FUNCTION reef_event_modified();
CREATE FUNCTION reef_transect_modified() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF TG_OP<>'INSERT' THEN UPDATE event SET updated_at=clock_timestamp() WHERE event_id=OLD.event_id; END IF;
 IF TG_OP<>'DELETE' THEN UPDATE event SET updated_at=clock_timestamp() WHERE event_id=NEW.event_id; END IF;
 RETURN NULL;
END $$;
CREATE TRIGGER reef_transect_modified AFTER INSERT OR UPDATE OR DELETE ON transect FOR EACH ROW EXECUTE FUNCTION reef_transect_modified();
