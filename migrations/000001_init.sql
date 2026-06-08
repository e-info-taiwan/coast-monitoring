CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS citext;

CREATE TYPE user_role AS ENUM ('admin', 'volunteer');
CREATE TYPE user_status AS ENUM ('active', 'disabled');
CREATE TYPE audit_action AS ENUM ('create', 'update', 'delete');

CREATE TABLE users (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  email citext NOT NULL UNIQUE,
  name text NOT NULL,
  role user_role NOT NULL DEFAULT 'volunteer',
  status user_status NOT NULL DEFAULT 'active',
  google_sub text UNIQUE,
  password_hash text,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE sessions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token_hash bytea NOT NULL UNIQUE,
  csrf_token_hash bytea NOT NULL UNIQUE,
  user_agent text NOT NULL DEFAULT '',
  ip inet,
  expires_at timestamptz NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  revoked_at timestamptz
);

CREATE TABLE oauth_states (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  state_hash bytea NOT NULL UNIQUE,
  redirect_path text NOT NULL DEFAULT '/',
  expires_at timestamptz NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  consumed_at timestamptz
);

CREATE TABLE locations (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  chinese_name text NOT NULL,
  english_name text NOT NULL,
  created_by uuid REFERENCES users(id) ON DELETE SET NULL,
  updated_by uuid REFERENCES users(id) ON DELETE SET NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE sites (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  location_id uuid NOT NULL REFERENCES locations(id) ON DELETE RESTRICT,
  county text NOT NULL,
  chinese_name text NOT NULL,
  english_name text NOT NULL,
  latitude numeric(9,6) NOT NULL,
  longitude numeric(9,6) NOT NULL,
  created_by uuid REFERENCES users(id) ON DELETE SET NULL,
  updated_by uuid REFERENCES users(id) ON DELETE SET NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE species (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  chinese_name text NOT NULL,
  english_name text NOT NULL,
  created_by uuid REFERENCES users(id) ON DELETE SET NULL,
  updated_by uuid REFERENCES users(id) ON DELETE SET NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

INSERT INTO species (chinese_name, english_name) VALUES
  ('蝶魚', 'Butterflyfish'),
  ('蝶魚 <5', 'Butterflyfish <5'),
  ('石鱸', 'Sweetlips'),
  ('石鱸 juvenile', 'Sweetlips juvenile'),
  ('笛鯛', 'Snapper'),
  ('笛鯛 <20', 'Snapper <20'),
  ('老鼠斑', 'Barramundi cod'),
  ('蘇眉', 'Humphead wrasse'),
  ('隆頭鸚哥', 'Bumphead parrotfish'),
  ('鸚哥魚', 'Other parrotfish'),
  ('鸚哥魚 <20', 'Other parrotfish <20'),
  ('裸胸鯙', 'Moray eel'),
  ('石斑魚 <30', 'Grouper <30'),
  ('石斑魚 30-40', 'Grouper 30-40'),
  ('石斑魚 40-50', 'Grouper 40-50'),
  ('石斑魚 50-60', 'Grouper 50-60'),
  ('石斑魚 >60', 'Grouper >60'),
  ('其他魚類', 'Other fish'),
  ('珊瑚蝦', 'Banded coral shrimp'),
  ('魔鬼海膽 Diadema', 'Diadema'),
  ('刺冠海膽 Echinothrix', 'Echinothrix'),
  ('鉛筆海膽', 'Pencil urchin'),
  ('收集海膽', 'Collector urchin'),
  ('海參', 'Sea cucumber'),
  ('棘冠海星', 'Crown of thorns'),
  ('法螺', 'Triton'),
  ('龍蝦', 'Lobster'),
  ('硨磲貝 <10', 'Giant clam <10'),
  ('硨磲貝 10-20', 'Giant clam 10-20'),
  ('硨磲貝 20-30', 'Giant clam 20-30'),
  ('硨磲貝 30-40', 'Giant clam 30-40'),
  ('硨磲貝 40-50', 'Giant clam 40-50'),
  ('硨磲貝 >50', 'Giant clam >50'),
  ('鯊魚', 'Sharks'),
  ('海龜', 'Turtles'),
  ('鬼蝠魟', 'Mantas'),
  ('其他罕見生物', 'Other rare organism');

CREATE TABLE reef_check_surveys (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  survey_date date NOT NULL,
  site_id uuid NOT NULL REFERENCES sites(id) ON DELETE RESTRICT,
  depth_m integer NOT NULL CHECK (depth_m > 0),
  general_comments text NOT NULL DEFAULT '',
  rkc_reason text NOT NULL DEFAULT '',
  fish_length_mode text NOT NULL CHECK (fish_length_mode IN ('separate', 'combined')),
  created_by uuid REFERENCES users(id) ON DELETE SET NULL,
  updated_by uuid REFERENCES users(id) ON DELETE SET NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE reef_check_survey_recorders (
  survey_id uuid NOT NULL REFERENCES reef_check_surveys(id) ON DELETE CASCADE,
  role text NOT NULL CHECK (role IN ('benthos', 'fish', 'invertebrate')),
  user_id uuid REFERENCES users(id) ON DELETE SET NULL,
  recorder_name text NOT NULL DEFAULT '',
  PRIMARY KEY (survey_id, role)
);

CREATE TABLE reef_check_segments (
  survey_id uuid NOT NULL REFERENCES reef_check_surveys(id) ON DELETE CASCADE,
  segment_index integer NOT NULL CHECK (segment_index BETWEEN 1 AND 4),
  label text NOT NULL,
  start_m integer NOT NULL,
  end_m integer NOT NULL CHECK (end_m > start_m),
  PRIMARY KEY (survey_id, segment_index)
);

CREATE TABLE substrate_codes (
  code text PRIMARY KEY,
  display_name text NOT NULL,
  normalized_category text NOT NULL CHECK (normalized_category IN ('HC', 'SC', 'RKC', 'NIA', 'SP', 'RC', 'RB', 'SD', 'SI', 'OT'))
);

INSERT INTO substrate_codes (code, display_name, normalized_category) VALUES
  ('1', '硬珊瑚', 'HC'),
  ('2', '軟珊瑚', 'SC'),
  ('3', '新死珊瑚', 'RKC'),
  ('4', '藻類', 'NIA'),
  ('5', '海綿', 'SP'),
  ('6', '岩石', 'RC'),
  ('7', '碎石', 'RB'),
  ('8', '沙', 'SD'),
  ('9', '泥沙', 'SI'),
  ('0', '其他', 'OT'),
  ('a', '硬珊瑚 -a', 'HC'),
  ('b', '硬珊瑚 -b', 'HC'),
  ('c', '硬珊瑚 -c', 'HC'),
  ('91', '泥沙 (硬珊瑚)', 'HC'),
  ('92', '泥沙 (軟珊瑚)', 'SC'),
  ('93', '泥沙 (新死珊瑚)', 'RKC'),
  ('94', '泥沙 (藻類)', 'NIA'),
  ('95', '泥沙 (海綿)', 'SP'),
  ('96', '泥沙 (岩石)', 'RC'),
  ('97', '泥沙 (碎石)', 'RB'),
  ('98', '泥沙 (沙)', 'SD');

CREATE TABLE substrate_points (
  survey_id uuid NOT NULL,
  segment_index integer NOT NULL CHECK (segment_index BETWEEN 1 AND 4),
  point_index integer NOT NULL CHECK (point_index BETWEEN 1 AND 40),
  transect_m numeric(4,1) NOT NULL,
  substrate_code text NOT NULL REFERENCES substrate_codes(code) ON DELETE RESTRICT,
  PRIMARY KEY (survey_id, segment_index, point_index),
  UNIQUE (survey_id, segment_index, point_index),
  UNIQUE (survey_id, transect_m),
  FOREIGN KEY (survey_id, segment_index) REFERENCES reef_check_segments(survey_id, segment_index) ON DELETE CASCADE
);

CREATE TABLE substrate_bleaching_counts (
  survey_id uuid NOT NULL,
  segment_index integer NOT NULL CHECK (segment_index BETWEEN 1 AND 4),
  hc_bleached_count integer NOT NULL CHECK (hc_bleached_count >= 0),
  sc_bleached_count integer NOT NULL CHECK (sc_bleached_count >= 0),
  PRIMARY KEY (survey_id, segment_index),
  FOREIGN KEY (survey_id, segment_index) REFERENCES reef_check_segments(survey_id, segment_index) ON DELETE CASCADE
);

CREATE TABLE reef_check_metrics (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  module text NOT NULL CHECK (module IN ('fish', 'invertebrate', 'impact', 'rare_organism')),
  key text NOT NULL,
  chinese_name text NOT NULL,
  english_name text NOT NULL DEFAULT '',
  size_class text NOT NULL DEFAULT '',
  sort_order integer NOT NULL DEFAULT 0,
  active boolean NOT NULL DEFAULT true,
  UNIQUE (module, key)
);

INSERT INTO reef_check_metrics (module, key, chinese_name, english_name, size_class, sort_order) VALUES
  ('fish', 'butterflyfish', '蝶魚', 'Butterflyfish', '', 10),
  ('fish', 'butterflyfish_less5', '蝶魚 <5', 'Butterflyfish', '<5', 20),
  ('fish', 'sweetlips', '石鱸', 'Sweetlips', '', 30),
  ('fish', 'sweetlips_juv', '石鱸 juvenile', 'Sweetlips juvenile', 'juvenile', 40),
  ('fish', 'snapper', '笛鯛', 'Snapper', '', 50),
  ('fish', 'snapper_less20', '笛鯛 <20', 'Snapper', '<20', 60),
  ('fish', 'barramundi_cod', '老鼠斑', 'Barramundi cod', '', 70),
  ('fish', 'humphead_wrasse', '蘇眉', 'Humphead wrasse', '', 80),
  ('fish', 'bumphead_parrotfish', '隆頭鸚哥', 'Bumphead parrotfish', '', 90),
  ('fish', 'other_parrotfish', '鸚哥魚', 'Other parrotfish', '', 100),
  ('fish', 'other_parrotfish_less20', '鸚哥魚 <20', 'Other parrotfish', '<20', 110),
  ('fish', 'moray_eel', '裸胸鯙', 'Moray eel', '', 120),
  ('fish', 'grouper_less30', '石斑魚 <30', 'Grouper', '<30', 130),
  ('fish', 'grouper_30_40', '石斑魚 30-40', 'Grouper', '30-40', 140),
  ('fish', 'grouper_40_50', '石斑魚 40-50', 'Grouper', '40-50', 150),
  ('fish', 'grouper_50_60', '石斑魚 50-60', 'Grouper', '50-60', 160),
  ('fish', 'grouper_60', '石斑魚 >60', 'Grouper', '>60', 170),
  ('fish', 'others', '其他魚類', 'Other fish', '', 180),
  ('invertebrate', 'banded_coral_shrimp', '珊瑚蝦', 'Banded coral shrimp', '', 10),
  ('invertebrate', 'diadema', '魔鬼海膽 Diadema', 'Diadema', '', 20),
  ('invertebrate', 'echinothrix', '刺冠海膽 Echinothrix', 'Echinothrix', '', 30),
  ('invertebrate', 'pencil_urchin', '鉛筆海膽', 'Pencil urchin', '', 40),
  ('invertebrate', 'collector_urchin', '收集海膽', 'Collector urchin', '', 50),
  ('invertebrate', 'seacucumber', '海參', 'Sea cucumber', '', 60),
  ('invertebrate', 'crown_of_thorns', '棘冠海星', 'Crown of thorns', '', 70),
  ('invertebrate', 'triton', '法螺', 'Triton', '', 80),
  ('invertebrate', 'lobster', '龍蝦', 'Lobster', '', 90),
  ('invertebrate', 'giantclam_less10', '硨磲貝 <10', 'Giant clam', '<10', 100),
  ('invertebrate', 'giantclam_10_20', '硨磲貝 10-20', 'Giant clam', '10-20', 110),
  ('invertebrate', 'giantclam_20_30', '硨磲貝 20-30', 'Giant clam', '20-30', 120),
  ('invertebrate', 'giantclam_30_40', '硨磲貝 30-40', 'Giant clam', '30-40', 130),
  ('invertebrate', 'giantclam_40_50', '硨磲貝 40-50', 'Giant clam', '40-50', 140),
  ('invertebrate', 'giantclam_50', '硨磲貝 >50', 'Giant clam', '>50', 150),
  ('impact', 'boat_anchor', '船錨', 'Boat anchor', '', 10),
  ('impact', 'dynamite', '炸魚', 'Dynamite', '', 20),
  ('impact', 'other_coral_damage', '其他珊瑚損害', 'Other coral damage', '', 30),
  ('impact', 'fishnet', '漁網', 'Fishnet', '', 40),
  ('impact', 'trash', '垃圾', 'Trash', '', 50),
  ('impact', 'general_trash', '一般垃圾', 'General trash', '', 60),
  ('impact', 'bleaching_population_percent', '白化比例 population', 'Bleaching population percent', '', 70),
  ('impact', 'bleaching_colony_percent', '白化比例 colony', 'Bleaching colony percent', '', 80),
  ('impact', 'disease_coral_black_band', '黑帶病', 'Black band disease', '', 90),
  ('impact', 'disease_coral_white_band', '白帶病', 'White band disease', '', 100),
  ('rare_organism', 'sharks', '鯊魚', 'Sharks', '', 10),
  ('rare_organism', 'turtles', '海龜', 'Turtles', '', 20),
  ('rare_organism', 'mantas', '鬼蝠魟', 'Mantas', '', 30),
  ('rare_organism', 'other', '其他罕見生物', 'Other rare organism', '', 40);

CREATE TABLE reef_check_metric_counts (
  survey_id uuid NOT NULL,
  segment_index integer NOT NULL CHECK (segment_index BETWEEN 1 AND 4),
  metric_id uuid NOT NULL REFERENCES reef_check_metrics(id) ON DELETE RESTRICT,
  count integer NOT NULL CHECK (count >= 0),
  comment text NOT NULL DEFAULT '',
  PRIMARY KEY (survey_id, segment_index, metric_id),
  FOREIGN KEY (survey_id, segment_index) REFERENCES reef_check_segments(survey_id, segment_index) ON DELETE CASCADE
);

CREATE TABLE observations (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  observed_on date NOT NULL,
  location_id uuid NOT NULL REFERENCES locations(id) ON DELETE RESTRICT,
  species_id uuid NOT NULL REFERENCES species(id) ON DELETE RESTRICT,
  observer_id uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
  count integer NOT NULL CHECK (count >= 0),
  notes text NOT NULL DEFAULT '',
  created_by uuid REFERENCES users(id) ON DELETE SET NULL,
  updated_by uuid REFERENCES users(id) ON DELETE SET NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE audit_logs (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  action audit_action NOT NULL,
  target_table text NOT NULL,
  target_id uuid NOT NULL,
  actor_user_id uuid REFERENCES users(id) ON DELETE SET NULL,
  actor_email text NOT NULL DEFAULT '',
  before_data jsonb,
  after_data jsonb,
  method text NOT NULL DEFAULT '',
  path text NOT NULL DEFAULT '',
  ip inet,
  user_agent text NOT NULL DEFAULT '',
  logged_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE login_attempts (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  email text NOT NULL DEFAULT '',
  ip inet,
  success boolean NOT NULL,
  attempted_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX sessions_user_id_idx ON sessions(user_id);
CREATE INDEX sessions_expires_at_idx ON sessions(expires_at);
CREATE INDEX oauth_states_expires_at_idx ON oauth_states(expires_at);
CREATE INDEX sites_location_id_idx ON sites(location_id);
CREATE INDEX reef_check_surveys_site_id_idx ON reef_check_surveys(site_id);
CREATE INDEX reef_check_surveys_survey_date_idx ON reef_check_surveys(survey_date);
CREATE INDEX reef_check_metric_counts_metric_id_idx ON reef_check_metric_counts(metric_id);
CREATE INDEX observations_location_id_idx ON observations(location_id);
CREATE INDEX observations_species_id_idx ON observations(species_id);
CREATE INDEX observations_observer_id_idx ON observations(observer_id);
CREATE INDEX observations_observed_on_idx ON observations(observed_on);
CREATE INDEX audit_logs_target_idx ON audit_logs(target_table, target_id);
CREATE INDEX audit_logs_logged_at_idx ON audit_logs(logged_at DESC);
CREATE INDEX login_attempts_email_time_idx ON login_attempts(email, attempted_at DESC);
CREATE INDEX login_attempts_ip_time_idx ON login_attempts(ip, attempted_at DESC);
