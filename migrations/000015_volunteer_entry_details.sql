-- New entry fields are additive. Imported raw bleaching totals remain untouched.
ALTER TABLE transect ADD COLUMN IF NOT EXISTS coral_morphology text
  CHECK (coral_morphology IS NULL OR coral_morphology IN ('general','classified'));
ALTER TABLE transect ADD COLUMN IF NOT EXISTS other_damage_note text;
ALTER TABLE belt_observation ADD COLUMN IF NOT EXISTS observed_size text;
ALTER TABLE belt_observation ADD COLUMN IF NOT EXISTS notes text;
ALTER TABLE belt_observation ALTER COLUMN count DROP NOT NULL;
ALTER TABLE belt_observation ADD COLUMN IF NOT EXISTS record_status text NOT NULL DEFAULT 'recorded'
  CHECK (record_status IN ('recorded','not_recorded'));
ALTER TABLE impact_observation ALTER COLUMN raw_value DROP NOT NULL;
ALTER TABLE impact_observation ADD COLUMN IF NOT EXISTS record_status text NOT NULL DEFAULT 'recorded'
  CHECK (record_status IN ('recorded','not_recorded'));

CREATE TABLE IF NOT EXISTS reef_check_activity (
  id serial PRIMARY KEY,
  name text NOT NULL,
  code_hash text NOT NULL UNIQUE,
  start_date date NOT NULL,
  end_date date NOT NULL CHECK (end_date >= start_date),
  is_active boolean NOT NULL DEFAULT true,
  expires_at timestamptz,
  created_by uuid REFERENCES users(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS reef_check_activity_site (
  activity_id integer NOT NULL REFERENCES reef_check_activity(id) ON DELETE CASCADE,
  site_id integer NOT NULL REFERENCES site(id),
  PRIMARY KEY (activity_id,site_id)
);
CREATE TABLE IF NOT EXISTS reef_check_activity_audit (
  id bigserial PRIMARY KEY,
  activity_id integer NOT NULL REFERENCES reef_check_activity(id),
  actor_id uuid REFERENCES users(id),
  action text NOT NULL CHECK (action IN ('create','disable','rotate')),
  occurred_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS reef_check_entry_metadata (
  event_id integer PRIMARY KEY REFERENCES event(id) ON DELETE CASCADE,
  activity_id integer NOT NULL REFERENCES reef_check_activity(id),
  weather text,
  air_temperature_c numeric,
  latitude numeric,
  longitude numeric,
  CHECK ((latitude IS NULL) = (longitude IS NULL)),
  CHECK (latitude BETWEEN -90 AND 90 AND longitude BETWEEN -180 AND 180)
);

-- This view derives new point counts separately from imported manual totals.
CREATE OR REPLACE VIEW substrate_bleaching_point_summary AS
SELECT transect_id,segment,
  CASE WHEN count(*) FILTER (WHERE substrate_code IN ('HC','HC-a','HC-b','HC-c') AND is_bleached IS NULL)=0
    THEN count(*) FILTER (WHERE substrate_code IN ('HC','HC-a','HC-b','HC-c') AND is_bleached IS TRUE) END AS derived_hc_bleached_point_count,
  CASE WHEN count(*) FILTER (WHERE substrate_code='SC' AND is_bleached IS NULL)=0
    THEN count(*) FILTER (WHERE substrate_code='SC' AND is_bleached IS TRUE) END AS derived_sc_bleached_point_count
FROM substrate_point WHERE substrate_layer='surface'
GROUP BY transect_id,segment;
