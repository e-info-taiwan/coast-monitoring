-- New tablet entries record bleaching at the same 160-point grain as substrate.
-- NULL deliberately preserves the distinction between legacy/not-applicable and
-- an explicit new-entry answer of false.
ALTER TABLE substrate_point ADD COLUMN IF NOT EXISTS is_bleached boolean;

COMMENT ON COLUMN substrate_point.is_bleached IS
  'Per-point bleaching: NULL=legacy/not applicable/not recorded, false=observed without bleaching, true=bleached';
