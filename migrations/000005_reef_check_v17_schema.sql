INSERT INTO substrate_type (code, numeric_code, name_zh, name_en, sort_order) VALUES
  ('NA', 0, '未記錄／未知', 'not recorded / unknown', 0),
  ('SI(HC)', 91, '泥沙（硬珊瑚）', 'silt/clay over hard coral', 91),
  ('SI(SC)', 92, '泥沙（軟珊瑚）', 'silt/clay over soft coral', 92),
  ('SI(RKC)', 93, '泥沙（新死珊瑚）', 'silt/clay over recently killed coral', 93),
  ('SI(NIA)', 94, '泥沙（藻類）', 'silt/clay over nutrient indicator algae', 94),
  ('SI(SP)', 95, '泥沙（海綿）', 'silt/clay over sponge', 95),
  ('SI(RC)', 96, '泥沙（岩石）', 'silt/clay over rock', 96),
  ('SI(RB)', 97, '泥沙（碎石）', 'silt/clay over rubble', 97),
  ('SI(SD)', 98, '泥沙（沙）', 'silt/clay over sand', 98)
ON CONFLICT (code) DO UPDATE SET
  numeric_code = EXCLUDED.numeric_code,
  name_zh = EXCLUDED.name_zh,
  name_en = EXCLUDED.name_en,
  sort_order = EXCLUDED.sort_order,
  is_active = true;

CREATE UNIQUE INDEX IF NOT EXISTS taxon_import_lookup_unique
  ON taxon (taxon_group, name_en, COALESCE(size_class, ''));

UPDATE impact_type SET has_raw_count = true WHERE impact_group = 'coral_damage';

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'impact_type_impact_group_name_en_key'
  ) THEN
    ALTER TABLE impact_type ADD CONSTRAINT impact_type_impact_group_name_en_key UNIQUE (impact_group, name_en);
  END IF;
END $$;

ALTER TABLE diver ADD COLUMN IF NOT EXISTS diver_key text;
CREATE UNIQUE INDEX IF NOT EXISTS diver_diver_key_idx ON diver (diver_key);

CREATE TABLE IF NOT EXISTS event (
  id serial PRIMARY KEY,
  survey_id integer NOT NULL REFERENCES survey(id) ON DELETE CASCADE,
  event_id text NOT NULL UNIQUE,
  survey_date date NOT NULL,
  event_time text NOT NULL DEFAULT 'na',
  depth_m numeric NOT NULL,
  UNIQUE (survey_id, survey_date, event_time, depth_m)
);

ALTER TABLE transect ADD COLUMN IF NOT EXISTS event_id text REFERENCES event(event_id) ON DELETE CASCADE;
ALTER TABLE transect ALTER COLUMN survey_id DROP NOT NULL;
ALTER TABLE transect ALTER COLUMN depth_m DROP NOT NULL;
ALTER TABLE transect DROP CONSTRAINT IF EXISTS transect_survey_id_method_depth_m_key;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'transect_event_id_method_key'
  ) THEN
    ALTER TABLE transect ADD CONSTRAINT transect_event_id_method_key UNIQUE (event_id, method);
  END IF;
END $$;

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'substrate_layer') THEN
    CREATE TYPE substrate_layer AS ENUM ('surface', 'down');
  END IF;
END $$;

ALTER TABLE substrate_point ADD COLUMN IF NOT EXISTS substrate_layer substrate_layer NOT NULL DEFAULT 'surface';
ALTER TABLE substrate_point DROP CONSTRAINT IF EXISTS substrate_point_transect_id_position_m_key;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'substrate_point_transect_id_position_m_substrate_layer_key'
  ) THEN
    ALTER TABLE substrate_point ADD CONSTRAINT substrate_point_transect_id_position_m_substrate_layer_key UNIQUE (transect_id, position_m, substrate_layer);
  END IF;
END $$;
