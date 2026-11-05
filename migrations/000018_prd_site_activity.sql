-- PRD 0.9.13 / Schema 1.16. Additive migration; never match legacy activities
-- or reports by title. Preserve every legacy source for explicit reconciliation.
CREATE TABLE reef_prd_migration_review (
 id bigserial PRIMARY KEY, kind text NOT NULL, source_id bigint NOT NULL,
 target_id bigint, reason text NOT NULL, source_data jsonb NOT NULL,
 resolved_at timestamptz, created_at timestamptz NOT NULL DEFAULT now()
);
ALTER TABLE reef_check_activity ADD COLUMN activity_content_id bigint UNIQUE REFERENCES public_content(id);
ALTER TABLE event ADD COLUMN activity_id bigint REFERENCES public_content(id);
ALTER TABLE reef_check_entry_metadata RENAME COLUMN activity_id TO entry_config_id;
ALTER TABLE reef_check_activity_audit ADD COLUMN before_data jsonb;
ALTER TABLE reef_check_activity_audit ADD COLUMN after_data jsonb;
ALTER TABLE reef_check_activity_audit DROP CONSTRAINT reef_check_activity_audit_action_check;
ALTER TABLE reef_check_activity_audit ADD CHECK(action IN ('create','disable','rotate','update'));
DO $$ DECLARE a record; cid bigint; BEGIN
 FOR a IN SELECT * FROM reef_check_activity ORDER BY id LOOP
  INSERT INTO public_content(kind,data,source_snapshot)
  VALUES('activity', jsonb_build_object('title',a.name,'start_at',a.start_date::text||'T00:00:00+08:00',
   'end_at',a.end_date::text||'T23:59:59+08:00','recruitment_type','internal','status','scheduled',
   'entry_enabled',a.is_active,'expires_at',a.expires_at,
   'site_ids',COALESCE((SELECT jsonb_agg(site_id) FROM reef_check_activity_site WHERE activity_id=a.id),'[]'::jsonb)),
   jsonb_build_object('legacy_entry_config_id',a.id)) RETURNING id INTO cid;
  UPDATE reef_check_activity SET activity_content_id=cid WHERE id=a.id;
  INSERT INTO reef_prd_migration_review(kind,source_id,target_id,reason,source_data)
   VALUES('activity',a.id,cid,'Created separate draft from known entry config; no match to an existing calendar activity was assumed.',to_jsonb(a)-'code_hash');
 END LOOP;
END $$;
ALTER TABLE reef_check_activity ALTER COLUMN activity_content_id SET NOT NULL;
UPDATE event e SET activity_id=a.activity_content_id FROM reef_check_entry_metadata m
 JOIN reef_check_activity a ON a.id=m.entry_config_id WHERE e.id=m.event_id;
CREATE FUNCTION require_calendar_activity() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE cid bigint; BEGIN
 IF TG_TABLE_NAME='event' THEN cid := NEW.activity_id; ELSE cid := NEW.activity_content_id; END IF;
 IF cid IS NOT NULL AND NOT EXISTS(SELECT 1 FROM public_content WHERE id=cid AND kind='activity') THEN
  RAISE EXCEPTION 'parent must be calendar activity' USING ERRCODE='23514';
 END IF; RETURN NEW;
END $$;
CREATE TRIGGER activity_parent BEFORE INSERT OR UPDATE ON reef_check_activity FOR EACH ROW EXECUTE FUNCTION require_calendar_activity();
CREATE TRIGGER event_activity_parent BEFORE INSERT OR UPDATE ON event FOR EACH ROW EXECUTE FUNCTION require_calendar_activity();

CREATE TABLE article_site (
 article_id bigint NOT NULL REFERENCES public_content(id), site_id integer NOT NULL REFERENCES site(id),
 is_visible boolean NOT NULL DEFAULT true, sort_order integer NOT NULL DEFAULT 0,
 updated_at timestamptz NOT NULL DEFAULT now(), updated_by uuid REFERENCES users(id),
 PRIMARY KEY(article_id,site_id)
);
UPDATE public_content SET data=jsonb_set(data,'{category}','"news"') WHERE kind='article' AND NOT data ? 'category';
CREATE FUNCTION require_observation_report() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM public_content WHERE id=NEW.article_id AND kind='article' AND data->>'category'='observation_report') THEN
  RAISE EXCEPTION 'relation requires observation report' USING ERRCODE='23514';
 END IF; RETURN NEW; END $$;
CREATE TRIGGER article_site_kind BEFORE INSERT OR UPDATE ON article_site FOR EACH ROW EXECUTE FUNCTION require_observation_report();
CREATE FUNCTION protect_public_content_kind() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF NEW.kind<>OLD.kind THEN RAISE EXCEPTION 'content kind is immutable' USING ERRCODE='23514'; END IF;
 IF EXISTS(SELECT 1 FROM article_site WHERE article_id=NEW.id) AND NEW.data->>'category' IS DISTINCT FROM 'observation_report' THEN
  RAISE EXCEPTION 'unlink reports before changing category' USING ERRCODE='23514'; END IF;
 RETURN NEW; END $$;
CREATE TRIGGER public_content_kind_guard BEFORE UPDATE ON public_content FOR EACH ROW EXECUTE FUNCTION protect_public_content_kind();
ALTER TABLE public_content ADD CONSTRAINT settings_global CHECK(kind<>'settings' OR site_id IS NULL);
DO $$ DECLARE p record; cid bigint; BEGIN
 FOR p IN SELECT * FROM public_content WHERE kind='profile' AND site_id IS NOT NULL AND COALESCE(data->>'report_url','')<>'' LOOP
  INSERT INTO public_content(kind,publication_status,data,source_snapshot)
  VALUES('article',p.publication_status,jsonb_build_object('title',p.data->>'title','category','observation_report',
   'external_url',p.data->>'report_url','date',NULLIF(p.data->>'date',''),'summary',p.data->>'summary','sort_order',0),
   jsonb_build_object('legacy_profile_id',p.id,'profile',to_jsonb(p))) RETURNING id INTO cid;
  INSERT INTO article_site(article_id,site_id) VALUES(cid,p.site_id);
  INSERT INTO reef_prd_migration_review(kind,source_id,target_id,reason,source_data)
   VALUES('report',p.id,cid,'Preserved exact profile URL; potential shared articles require explicit review.',to_jsonb(p));
 END LOOP;
END $$;

-- Keep all earlier profiles and record why only the latest remains published.
WITH ranked AS (SELECT id,row_number() OVER(PARTITION BY site_id ORDER BY updated_at DESC,id DESC) AS rank FROM public_content WHERE kind='profile' AND publication_status='published' AND site_id IS NOT NULL)
INSERT INTO reef_prd_migration_review(kind,source_id,reason,source_data)
SELECT 'profile',c.id,'Older published profile retained as archive; latest profile remains public.',to_jsonb(c) FROM public_content c JOIN ranked r ON r.id=c.id WHERE r.rank>1;
UPDATE public_content SET publication_status='archived' WHERE id IN(SELECT source_id FROM reef_prd_migration_review WHERE kind='profile');
CREATE UNIQUE INDEX public_content_one_site_profile ON public_content(site_id) WHERE kind='profile' AND publication_status='published';

CREATE TABLE site_chart_setting (
 id bigserial PRIMARY KEY, site_id integer NOT NULL REFERENCES site(id),
 chart_key text NOT NULL CHECK(chart_key IN ('substrate_depth','taxa_density','rare_history','live_coral_history','impact_history','bleaching_history')),
 is_visible boolean, sort_order integer, year_from smallint, year_to smallint,
 default_depth_m numeric CHECK(default_depth_m>0), category_keys text[], taxon_ids integer[],
 publication_status text NOT NULL DEFAULT 'draft' CHECK(publication_status IN ('draft','published','archived')),
 updated_at timestamptz NOT NULL DEFAULT now(), updated_by uuid REFERENCES users(id),
 CHECK(year_from IS NULL OR year_to IS NULL OR year_from<=year_to),
 CHECK(year_from BETWEEN 1900 AND 2100), CHECK(year_to BETWEEN 1900 AND 2100)
);
CREATE UNIQUE INDEX site_chart_one_live ON site_chart_setting(site_id,chart_key) WHERE publication_status='published';
CREATE UNIQUE INDEX site_chart_one_draft ON site_chart_setting(site_id,chart_key) WHERE publication_status='draft';
CREATE TABLE shared_chart_group (
 id bigserial PRIMARY KEY,title text NOT NULL,chart_key text NOT NULL DEFAULT 'live_coral_history' CHECK(chart_key='live_coral_history'),
 year_from smallint,year_to smallint,publication_status text NOT NULL DEFAULT 'draft' CHECK(publication_status IN ('draft','published','archived')),
 updated_at timestamptz NOT NULL DEFAULT now(),updated_by uuid REFERENCES users(id),
 CHECK(year_from IS NULL OR year_to IS NULL OR year_from<=year_to)
);
CREATE TABLE shared_chart_member (
 group_id bigint NOT NULL REFERENCES shared_chart_group(id) ON DELETE CASCADE,
 site_id integer NOT NULL REFERENCES site(id),depth_m numeric NOT NULL CHECK(depth_m>0),sort_order integer NOT NULL DEFAULT 0,
 PRIMARY KEY(group_id,site_id,depth_m)
);
CREATE TABLE reef_site_display_audit (
 id bigserial PRIMARY KEY,entity text NOT NULL,entity_id bigint NOT NULL,actor_id uuid NOT NULL REFERENCES users(id),
 before_data jsonb,after_data jsonb,occurred_at timestamptz NOT NULL DEFAULT now()
);
-- Old charts remain archived with full provenance. Ambiguous multi-depth site
-- settings are recorded for review; group depths are copied without collapsing.
DO $$ DECLARE c record; k text; gid bigint; BEGIN
 FOR c IN SELECT * FROM public_content WHERE kind='chart' ORDER BY updated_at,id LOOP
  k := CASE c.data->>'chart' WHEN 'substrate' THEN 'substrate_depth' WHEN 'taxa' THEN 'taxa_density'
   WHEN 'live_coral' THEN 'live_coral_history' WHEN 'impact' THEN 'impact_history' WHEN 'bleaching' THEN 'bleaching_history' END;
  IF c.site_id IS NOT NULL AND k IS NOT NULL THEN
   UPDATE site_chart_setting SET publication_status='archived' WHERE site_id=c.site_id AND chart_key=k AND publication_status=c.publication_status AND publication_status<>'archived';
   INSERT INTO site_chart_setting(site_id,chart_key,is_visible,sort_order,year_from,year_to,default_depth_m,category_keys,publication_status)
   VALUES(c.site_id,k,(c.data->>'is_visible')::boolean,(c.data->>'sort_order')::integer,(c.data->>'year_from')::smallint,
    (c.data->>'year_to')::smallint,(c.data->'depths_m'->>0)::numeric,
    CASE WHEN jsonb_array_length(COALESCE(NULLIF(c.data->'category_keys','null'::jsonb),'[]'))>0 THEN ARRAY(SELECT jsonb_array_elements_text(c.data->'category_keys')) END,c.publication_status);
   -- Legacy taxa settings covered rare observations too. Preserve that scope
   -- when splitting the display into density and rare charts.
   IF k='taxa_density' THEN
    UPDATE site_chart_setting SET publication_status='archived' WHERE site_id=c.site_id AND chart_key='rare_history' AND publication_status=c.publication_status AND publication_status<>'archived';
    INSERT INTO site_chart_setting(site_id,chart_key,is_visible,sort_order,year_from,year_to,default_depth_m,category_keys,publication_status)
    SELECT site_id,'rare_history',is_visible,sort_order,year_from,year_to,default_depth_m,category_keys,publication_status FROM site_chart_setting WHERE site_id=c.site_id AND chart_key=k ORDER BY id DESC LIMIT 1;
   END IF;
  ELSIF k='live_coral_history' AND jsonb_array_length(COALESCE(NULLIF(c.data->'site_ids','null'::jsonb),'[]'))>0 AND jsonb_array_length(COALESCE(NULLIF(c.data->'depths_m','null'::jsonb),'[]'))>0 THEN
   INSERT INTO shared_chart_group(title,year_from,year_to,publication_status) VALUES(c.data->>'title',(c.data->>'year_from')::smallint,(c.data->>'year_to')::smallint,
    CASE WHEN COALESCE((c.data->>'is_visible')::boolean,false) THEN c.publication_status ELSE 'archived' END) RETURNING id INTO gid;
   INSERT INTO shared_chart_member(group_id,site_id,depth_m) SELECT gid,s::integer,d::numeric FROM jsonb_array_elements_text(c.data->'site_ids') s CROSS JOIN jsonb_array_elements_text(c.data->'depths_m') d ON CONFLICT DO NOTHING;
  END IF;
  INSERT INTO reef_prd_migration_review(kind,source_id,target_id,reason,source_data) VALUES('chart',c.id,NULL,'Review copied settings and all legacy ranges before accepting migration.',to_jsonb(c));
 END LOOP;
END $$;
UPDATE public_content SET publication_status='archived' WHERE kind='chart';

ALTER TABLE site ADD COLUMN cwa_station_source text NOT NULL DEFAULT 'unknown' CHECK(cwa_station_source IN ('auto','manual','unknown'));
ALTER TABLE site ADD COLUMN cwa_station_updated_at timestamptz;
ALTER TABLE site ADD COLUMN cwa_station_updated_by uuid REFERENCES users(id);
ALTER TABLE event ADD COLUMN sea_temp_source text CHECK(sea_temp_source IN ('automatic','manual'));
ALTER TABLE event ADD COLUMN sea_temp_observed_at timestamptz;
ALTER TABLE event ADD COLUMN sea_temp_fetched_at timestamptz;
ALTER TABLE event ADD COLUMN sea_temp_c numeric;
CREATE TABLE cwa_sync_run (
 id bigserial PRIMARY KEY,started_at timestamptz NOT NULL DEFAULT now(),finished_at timestamptz,
 success boolean NOT NULL DEFAULT false,error_message text,range_from date,range_to date
);

-- Retain the content ID/API while putting development data in independent tables.
CREATE TABLE development_case (
 id bigint PRIMARY KEY REFERENCES public_content(id), data jsonb NOT NULL,
 updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE development_case_timeline (
 case_id bigint NOT NULL REFERENCES development_case(id) ON DELETE CASCADE,sort_order integer NOT NULL,data jsonb NOT NULL,PRIMARY KEY(case_id,sort_order)
);
CREATE TABLE development_case_media (
 case_id bigint NOT NULL REFERENCES development_case(id) ON DELETE CASCADE,sort_order integer NOT NULL,data jsonb NOT NULL,PRIMARY KEY(case_id,sort_order)
);
INSERT INTO development_case SELECT id,data-'timeline'-'media',updated_at FROM public_content WHERE kind='development';
INSERT INTO development_case_timeline SELECT c.id,(t.n-1)::int,t.data FROM public_content c CROSS JOIN LATERAL jsonb_array_elements(COALESCE(NULLIF(c.data->'timeline','null'::jsonb),'[]')) WITH ORDINALITY t(data,n) WHERE c.kind='development';
INSERT INTO development_case_media SELECT c.id,(t.n-1)::int,t.data FROM public_content c CROSS JOIN LATERAL jsonb_array_elements(COALESCE(NULLIF(c.data->'media','null'::jsonb),'[]')) WITH ORDINALITY t(data,n) WHERE c.kind='development';
