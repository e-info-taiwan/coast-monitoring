ALTER TABLE reef_check_surveys
  ADD COLUMN IF NOT EXISTS country_island text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS team_leader text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS survey_time text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS visibility text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS temperature text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS substrate_comments text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS rkc_bleaching_percent numeric(5,2),
  ADD CONSTRAINT reef_check_surveys_rkc_bleaching_percent_check
    CHECK (rkc_bleaching_percent IS NULL OR (rkc_bleaching_percent >= 0 AND rkc_bleaching_percent <= 100));
