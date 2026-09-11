-- 1. Link diver and transect_participant to users
ALTER TABLE diver ADD COLUMN IF NOT EXISTS user_id uuid REFERENCES users(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS diver_user_id_idx ON diver (user_id);

ALTER TABLE transect_participant ADD COLUMN IF NOT EXISTS user_id uuid REFERENCES users(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS transect_participant_user_id_idx ON transect_participant (user_id);

-- 2. Drop 8 legacy prototype Reef Check tables
DROP TABLE IF EXISTS reef_check_metric_counts CASCADE;
DROP TABLE IF EXISTS substrate_bleaching_counts CASCADE;
DROP TABLE IF EXISTS substrate_points CASCADE;
DROP TABLE IF EXISTS reef_check_segments CASCADE;
DROP TABLE IF EXISTS reef_check_survey_recorders CASCADE;
DROP TABLE IF EXISTS reef_check_surveys CASCADE;
DROP TABLE IF EXISTS reef_check_metrics CASCADE;
DROP TABLE IF EXISTS substrate_codes CASCADE;
