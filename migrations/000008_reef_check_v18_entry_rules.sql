-- v1.8 keeps legacy observations untouched while defining the contract for new entry.

ALTER TABLE transect ADD COLUMN IF NOT EXISTS fish_length_mode text;

-- New tablet entries record bleaching at the same 160-point grain as substrate.
-- NULL deliberately preserves the distinction between legacy/not-applicable and
-- an explicit new-entry answer of false.
ALTER TABLE substrate_point ADD COLUMN IF NOT EXISTS is_bleached boolean;

COMMENT ON COLUMN substrate_point.is_bleached IS
  'Per-point bleaching: NULL=legacy/not applicable/not recorded, false=observed without bleaching, true=bleached';

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'transect_fish_length_mode_check'
  ) THEN
    ALTER TABLE transect ADD CONSTRAINT transect_fish_length_mode_check
      CHECK (fish_length_mode IS NULL OR fish_length_mode IN ('split', 'combined'));
  END IF;
END $$;

-- Letter codes are canonical codes. numeric_code remains the HC family number for
-- compatibility; public input aliases are normalized before they reach the database.
INSERT INTO substrate_type (code, numeric_code, name_zh, name_en, sort_order) VALUES
  ('HC-a', 1, '硬珊瑚 a 型', 'hard coral morphology a', 11),
  ('HC-b', 1, '硬珊瑚 b 型', 'hard coral morphology b', 12),
  ('HC-c', 1, '硬珊瑚 c 型', 'hard coral morphology c', 13)
ON CONFLICT (code) DO UPDATE SET
  name_zh = EXCLUDED.name_zh,
  name_en = EXCLUDED.name_en,
  sort_order = EXCLUDED.sort_order,
  is_active = true;

UPDATE impact_type
SET has_raw_count = true
WHERE impact_group IN ('coral_damage', 'trash');

CREATE OR REPLACE VIEW impact_observation_with_level AS
SELECT
  i.*,
  CASE
    WHEN t.has_raw_count IS NOT TRUE THEN NULL
    WHEN i.raw_value = 0 THEN 0
    WHEN i.raw_value = 1 THEN 1
    WHEN i.raw_value BETWEEN 2 AND 4 THEN 2
    WHEN i.raw_value >= 5 THEN 3
    ELSE NULL
  END::smallint AS derived_level
FROM impact_observation i
JOIN impact_type t ON t.id = i.impact_type_id;
