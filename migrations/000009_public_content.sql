ALTER TABLE reef_publication_audit ADD COLUMN IF NOT EXISTS actor_id uuid REFERENCES users(id);
CREATE TABLE public_content (
 id bigserial PRIMARY KEY,
 kind text NOT NULL CHECK (kind IN ('article','activity','development','profile','annotation','chart')),
 publication_status text NOT NULL DEFAULT 'draft' CHECK (publication_status IN ('draft','published','archived')),
 site_id integer REFERENCES site(id),
 data jsonb NOT NULL,
 source_snapshot jsonb,
 updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX public_content_kind_status ON public_content(kind,publication_status);
CREATE TABLE public_content_audit (
 id bigserial PRIMARY KEY,content_id bigint NOT NULL REFERENCES public_content(id),actor_id uuid NOT NULL REFERENCES users(id),
 before_data jsonb,after_data jsonb,created_at timestamptz NOT NULL DEFAULT now()
);
CREATE FUNCTION touch_reef_event() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE tid integer;
BEGIN
 tid := CASE WHEN TG_OP='DELETE' THEN OLD.transect_id ELSE NEW.transect_id END;
 UPDATE event SET updated_at=clock_timestamp() WHERE event_id=(SELECT event_id FROM transect WHERE id=tid);
 RETURN NULL;
END $$;
CREATE TRIGGER substrate_point_touch AFTER INSERT OR UPDATE OR DELETE ON substrate_point FOR EACH ROW EXECUTE FUNCTION touch_reef_event();
CREATE TRIGGER bleaching_touch AFTER INSERT OR UPDATE OR DELETE ON substrate_bleaching FOR EACH ROW EXECUTE FUNCTION touch_reef_event();
CREATE TRIGGER belt_touch AFTER INSERT OR UPDATE OR DELETE ON belt_observation FOR EACH ROW EXECUTE FUNCTION touch_reef_event();
CREATE TRIGGER impact_touch AFTER INSERT OR UPDATE OR DELETE ON impact_observation FOR EACH ROW EXECUTE FUNCTION touch_reef_event();
