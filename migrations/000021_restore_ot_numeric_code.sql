-- Historical handboards use 10 for OT. The volunteer input alias 0 is handled at entry.
UPDATE substrate_type SET numeric_code = 10 WHERE code = 'OT' AND numeric_code <> 10;
