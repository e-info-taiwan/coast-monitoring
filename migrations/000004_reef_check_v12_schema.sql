DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'taxon_group') THEN
    CREATE TYPE taxon_group AS ENUM ('fish', 'invert', 'rare');
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'impact_group') THEN
    CREATE TYPE impact_group AS ENUM ('coral_damage', 'trash', 'bleaching', 'disease');
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'impact_value_type') THEN
    CREATE TYPE impact_value_type AS ENUM ('level', 'count', 'percent');
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'transect_method') THEN
    CREATE TYPE transect_method AS ENUM ('line', 'belt_fish', 'belt_invert');
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'participant_role') THEN
    CREATE TYPE participant_role AS ENUM ('member', 'team_leader', 'team_scientist');
  END IF;
END $$;

CREATE TABLE IF NOT EXISTS substrate_type (
  code text PRIMARY KEY,
  numeric_code smallint NOT NULL,
  name_zh text NOT NULL,
  name_en text NOT NULL,
  sort_order smallint NOT NULL DEFAULT 0,
  is_active boolean NOT NULL DEFAULT true
);

CREATE TABLE IF NOT EXISTS taxon (
  id serial PRIMARY KEY,
  taxon_group taxon_group NOT NULL,
  name_zh text NOT NULL,
  name_en text,
  size_class text,
  is_aggregate boolean NOT NULL DEFAULT false,
  aggregate_of text,
  sort_order smallint NOT NULL DEFAULT 0,
  is_active boolean NOT NULL DEFAULT true
);

CREATE TABLE IF NOT EXISTS impact_type (
  id serial PRIMARY KEY,
  impact_group impact_group NOT NULL,
  name_zh text NOT NULL,
  name_en text,
  value_type impact_value_type NOT NULL,
  has_raw_count boolean NOT NULL DEFAULT false,
  sort_order smallint NOT NULL DEFAULT 0,
  is_active boolean NOT NULL DEFAULT true
);

CREATE TABLE IF NOT EXISTS diver (
  id serial PRIMARY KEY,
  name_zh text,
  name_en text,
  reef_check_code text,
  is_active boolean NOT NULL DEFAULT true,
  CHECK (name_zh IS NOT NULL OR name_en IS NOT NULL)
);

CREATE TABLE IF NOT EXISTS site (
  id serial PRIMARY KEY,
  region text,
  county text,
  location text,
  name_zh text NOT NULL,
  name_en text,
  latitude numeric,
  longitude numeric,
  is_active boolean NOT NULL DEFAULT true,
  UNIQUE (name_en)
);

CREATE TABLE IF NOT EXISTS survey (
  id serial PRIMARY KEY,
  site_id integer NOT NULL REFERENCES site(id),
  start_date date NOT NULL,
  end_date date NOT NULL,
  label text,
  UNIQUE (site_id, start_date)
);

CREATE TABLE IF NOT EXISTS site_description (
  survey_id integer PRIMARY KEY REFERENCES survey(id) ON DELETE CASCADE,
  state_province text,
  city_town text,
  orientation text,
  temp_air_c numeric,
  temp_surface_c numeric,
  temp_3m_c numeric,
  temp_10m_c numeric,
  visibility_min_m numeric,
  visibility_max_m numeric,
  dist_from_shore_m numeric,
  dist_from_river_km numeric,
  river_mouth_width_m numeric,
  dist_to_pop_center_km numeric,
  population_size_k numeric,
  weather text,
  is_best_reef boolean,
  why_selected text,
  latitude numeric,
  longitude numeric,
  sheltered text,
  major_storms text,
  last_storm text,
  anthropogenic_impact text,
  siltation text,
  blast_fishing text,
  poison_fishing text,
  aquarium_fishing text,
  harvest_inverts_food text,
  harvest_inverts_curio text,
  tourist_diving text,
  sewage_pollution text,
  industrial_pollution text,
  commercial_fishing text,
  live_food_fish_trade text,
  artisinal_recreational text,
  yachts_present text,
  other_impacts text,
  any_protection text,
  protection_enforced text,
  poaching_level text,
  banned_activities text,
  other_comments text,
  submitted_by text,
  email text,
  affiliations text
);

CREATE TABLE IF NOT EXISTS transect (
  id serial PRIMARY KEY,
  survey_id integer NOT NULL REFERENCES survey(id) ON DELETE CASCADE,
  method transect_method NOT NULL,
  depth_m numeric NOT NULL,
  survey_date date,
  start_time text,
  water_temp_c numeric,
  visibility_min_m numeric,
  visibility_max_m numeric,
  comments text,
  rkc_bleaching_note text,
  UNIQUE (survey_id, method, depth_m)
);

CREATE TABLE IF NOT EXISTS transect_participant (
  id serial PRIMARY KEY,
  transect_id integer NOT NULL REFERENCES transect(id) ON DELETE CASCADE,
  diver_id integer NOT NULL REFERENCES diver(id),
  role participant_role NOT NULL,
  UNIQUE (transect_id, diver_id, role)
);

CREATE TABLE IF NOT EXISTS substrate_point (
  id bigserial PRIMARY KEY,
  transect_id integer NOT NULL REFERENCES transect(id) ON DELETE CASCADE,
  segment smallint NOT NULL CHECK (segment BETWEEN 1 AND 4),
  position_m numeric NOT NULL,
  substrate_code text NOT NULL REFERENCES substrate_type(code),
  UNIQUE (transect_id, position_m)
);

CREATE TABLE IF NOT EXISTS substrate_bleaching (
  id serial PRIMARY KEY,
  transect_id integer NOT NULL REFERENCES transect(id) ON DELETE CASCADE,
  segment smallint NOT NULL CHECK (segment BETWEEN 1 AND 4),
  hc_bleached_count smallint NOT NULL DEFAULT 0,
  sc_bleached_count smallint NOT NULL DEFAULT 0,
  UNIQUE (transect_id, segment)
);

CREATE TABLE IF NOT EXISTS belt_observation (
  id bigserial PRIMARY KEY,
  transect_id integer NOT NULL REFERENCES transect(id) ON DELETE CASCADE,
  taxon_id integer NOT NULL REFERENCES taxon(id),
  segment smallint NOT NULL CHECK (segment BETWEEN 1 AND 4),
  count integer NOT NULL DEFAULT 0,
  UNIQUE (transect_id, taxon_id, segment)
);

CREATE TABLE IF NOT EXISTS impact_observation (
  id bigserial PRIMARY KEY,
  transect_id integer NOT NULL REFERENCES transect(id) ON DELETE CASCADE,
  impact_type_id integer NOT NULL REFERENCES impact_type(id),
  segment smallint NOT NULL CHECK (segment BETWEEN 1 AND 4),
  raw_value numeric NOT NULL DEFAULT 0,
  UNIQUE (transect_id, impact_type_id, segment)
);

INSERT INTO substrate_type (code, numeric_code, name_zh, name_en, sort_order) VALUES
  ('HC', 1, '硬珊瑚', 'hard coral', 10),
  ('SC', 2, '軟珊瑚', 'soft coral', 20),
  ('RKC', 3, '新死珊瑚', 'recently killed coral', 30),
  ('NIA', 4, '藻類', 'nutrient indicator algae', 40),
  ('SP', 5, '海綿', 'sponge', 50),
  ('RC', 6, '岩石', 'rock', 60),
  ('RB', 7, '碎石', 'rubble', 70),
  ('SD', 8, '沙', 'sand', 80),
  ('SI', 9, '泥沙', 'silt/clay', 90),
  ('OT', 10, '其他', 'other', 100)
ON CONFLICT (code) DO UPDATE SET
  numeric_code = EXCLUDED.numeric_code,
  name_zh = EXCLUDED.name_zh,
  name_en = EXCLUDED.name_en,
  sort_order = EXCLUDED.sort_order,
  is_active = true;

INSERT INTO taxon (taxon_group, name_zh, name_en, size_class, is_aggregate, aggregate_of, sort_order) VALUES
  ('fish', '蝴蝶魚', 'Butterflyfish', NULL, false, NULL, 10),
  ('fish', '石鱸', 'Haemulidae', NULL, false, NULL, 20),
  ('fish', '笛鯛', 'Snapper', NULL, false, NULL, 30),
  ('fish', '老鼠斑', 'Barramundi cod', NULL, false, NULL, 40),
  ('fish', '蘇眉', 'Humphead wrasse', NULL, false, NULL, 50),
  ('fish', '龍頭鸚哥', 'Bumphead parrotfish', NULL, false, NULL, 60),
  ('fish', '鸚哥魚', 'Parrotfish', '>20cm', false, NULL, 70),
  ('fish', '裸胸鯙', 'Moray eel', NULL, false, NULL, 80),
  ('fish', '石斑魚', 'Grouper', '<30cm', false, 'grouper', 90),
  ('fish', '石斑魚', 'Grouper', '30-40 cm', false, 'grouper', 100),
  ('fish', '石斑魚', 'Grouper', '40-50 cm', false, 'grouper', 110),
  ('fish', '石斑魚', 'Grouper', '50-60 cm', false, 'grouper', 120),
  ('fish', '石斑魚', 'Grouper', '>60 cm', false, 'grouper', 130),
  ('fish', '石斑魚總數', 'Grouper total', NULL, true, 'grouper', 140),
  ('invert', '櫻花蝦', 'Banded coral shrimp', NULL, false, NULL, 150),
  ('invert', '魔鬼海膽', 'Diadema', NULL, false, NULL, 160),
  ('invert', '鉛筆海膽', 'Pencil urchin', NULL, false, NULL, 170),
  ('invert', '馬糞海膽', 'Collector urchin', NULL, false, NULL, 180),
  ('invert', '海參', 'Sea cucumber', NULL, false, NULL, 190),
  ('invert', '棘冠海星', 'Crown-of-thorns', NULL, false, NULL, 200),
  ('invert', '大法螺', 'Triton', NULL, false, NULL, 210),
  ('invert', '龍蝦', 'Lobster', NULL, false, NULL, 220),
  ('invert', '硨磲貝', 'Giant clam', '<10 cm', false, 'giant_clam', 230),
  ('invert', '硨磲貝', 'Giant clam', '10-20 cm', false, 'giant_clam', 240),
  ('invert', '硨磲貝', 'Giant clam', '20-30 cm', false, 'giant_clam', 250),
  ('invert', '硨磲貝', 'Giant clam', '30-40 cm', false, 'giant_clam', 260),
  ('invert', '硨磲貝', 'Giant clam', '40-50 cm', false, 'giant_clam', 270),
  ('invert', '硨磲貝', 'Giant clam', '>50 cm', false, 'giant_clam', 280),
  ('invert', '硨磲貝總數', 'Giant clam total', NULL, true, 'giant_clam', 290),
  ('rare', '鯊魚', 'Sharks', NULL, false, NULL, 300),
  ('rare', '海龜', 'Turtles', NULL, false, NULL, 310),
  ('rare', '魟', 'Mantas', NULL, false, NULL, 320),
  ('rare', '其他', 'Other', NULL, false, NULL, 330)
ON CONFLICT DO NOTHING;

INSERT INTO impact_type (impact_group, name_zh, name_en, value_type, has_raw_count, sort_order) VALUES
  ('coral_damage', '珊瑚損害：船錨', 'Coral damage: boat/anchor', 'level', false, 10),
  ('coral_damage', '珊瑚損害：炸魚', 'Coral damage: dynamite', 'level', false, 20),
  ('coral_damage', '珊瑚損害：其他', 'Coral damage: other', 'level', false, 30),
  ('trash', '漁業垃圾（漁網）', 'Trash: fish nets', 'level', true, 40),
  ('trash', '一般垃圾', 'Trash: general', 'level', true, 50),
  ('bleaching', '珊瑚白化佔總體百分比', 'Bleaching (% of population)', 'percent', false, 60),
  ('bleaching', '珊瑚白化佔群體百分比', 'Bleaching (% of colony)', 'percent', false, 70),
  ('disease', '黑帶病佔珊瑚百分比', 'Black Band (% colonies)', 'percent', false, 80),
  ('disease', '白帶病佔珊瑚百分比', 'White Band (% colonies)', 'percent', false, 90)
ON CONFLICT DO NOTHING;

INSERT INTO site (region, county, location, name_zh, name_en, latitude, longitude) VALUES
  ('北海岸與東北角', '新北市', '萬里區', '野柳', 'Yeliu', 25.21151111, 121.6988278),
  ('北海岸與東北角', '基隆市', '中正區', '潮境保育區', 'Chaojing protected area', 25.145531, 121.806622),
  ('北海岸與東北角', '新北市', '瑞芳區', '深澳（番仔澳）', 'Shenao', 25.13555556, 121.8219444),
  ('北海岸與東北角', '新北市', '瑞芳區', '鼻頭角', 'Bitoujiao', 25.12611111, 121.9141667),
  ('北海岸與東北角', '新北市', '貢寮區', '龍洞1.5號', 'Longdong 1.5', 25.115806, 121.917),
  ('北海岸與東北角', '新北市', '貢寮區', '龍洞4號', 'Longdong 4', 25.11380278, 121.9201694),
  ('北海岸與東北角', '基隆市', '中正區', '和平島', 'Heping Island', 25.162602, 121.761922),
  ('墾丁', '屏東縣', '恆春鎮', '合界', 'Hejie', 21.955583, 120.711889),
  ('墾丁', '屏東縣', '恆春鎮', '後壁湖', 'Houbihu', 21.936584, 120.747359),
  ('東海岸', '花蓮縣', '豐濱鄉', '石梯坪', 'Shitiping', 23.484078, 121.513784),
  ('東海岸', '臺東縣', '成功鎮', '基翬', 'Kihaw', 23.116331, 121.396829),
  ('東海岸', '臺東縣', '卑南鄉', '杉原中礁', 'Shanyuan middle reef', 22.82972222, 121.1888889),
  ('東海岸', '臺東縣', '卑南鄉', '杉原南礁', 'Shanyuan southern reef', 22.825045, 121.194028),
  ('東海岸', '臺東縣', '卑南鄉', '杉原', 'Shanyuan', 22.831238, 121.187613),
  ('東海岸', '臺東縣', '東河鄉', '加母子灣', 'Jiamuzi Bay', 22.863548, 121.20943),
  ('澎湖嶼坪', '澎湖縣', '望安鄉', '東嶼坪西', 'Dongyuping west', 23.25737, 119.509968),
  ('澎湖嶼坪', '澎湖縣', '望安鄉', '東嶼坪東', 'Dongyuping east', 23.260306, 119.519556),
  ('澎湖嶼坪', '澎湖縣', '望安鄉', '東嶼坪南', 'Dongyuping south', 23.2546, 119.512414),
  ('澎湖嶼坪', '澎湖縣', '望安鄉', '東嶼坪北側', 'North Dongyuping Island', 23.265128, 119.519995),
  ('澎湖嶼坪', '澎湖縣', '望安鄉', '西嶼坪北側', 'North Xiyuping Island', 23.27367, 119.5076),
  ('澎湖嶼坪', '澎湖縣', '馬公市', '杭灣', 'HangWan', 23.516289, 119.605964),
  ('澎湖嶼坪', '澎湖縣', '馬公市', '山水港', 'Shanshui port', 23.51105, 119.59852),
  ('小琉球', '屏東縣', '琉球鄉', '美人洞', 'Beauty Cave', 22.35444444, 120.3711111),
  ('小琉球', '屏東縣', '琉球鄉', '漁埕尾', 'Yuchengwei', 22.349278, 120.389889),
  ('小琉球', '屏東縣', '琉球鄉', '厚石裙礁', 'Houshi fringing reef', 22.32305556, 120.3655556),
  ('綠島', '臺東縣', '綠島鄉', '柴口', 'Chaikou', 22.68138889, 121.4827778),
  ('綠島', '臺東縣', '綠島鄉', '公館', 'Gongguan', 22.677221, 121.492581),
  ('綠島', '臺東縣', '綠島鄉', '將軍岩', 'General rock', 22.67764722, 121.4982389),
  ('綠島', '臺東縣', '綠島鄉', '石朗', 'Shihlang', 22.65482778, 121.4735139),
  ('綠島', '臺東縣', '綠島鄉', '龜灣', 'Turtle bay', 22.64289, 121.483708),
  ('綠島', '臺東縣', '綠島鄉', '大白沙', 'Dabaisha', 22.639209, 121.491964),
  ('蘭嶼', '臺東縣', '蘭嶼鄉', '玉女岩', 'Beauty Rock', 22.082083, 121.516333),
  ('蘭嶼', '臺東縣', '蘭嶼鄉', '母雞岩', 'Hen Rock', 22.084278, 121.553278),
  ('蘭嶼', '臺東縣', '蘭嶼鄉', '南獅', 'Lion Couple Rock south', 22.08545, 121.565726),
  ('蘭嶼', '臺東縣', '蘭嶼鄉', '東清小涼亭', 'DongJing pavilion', 22.070144, 121.568989),
  ('蘭嶼', '臺東縣', '蘭嶼鄉', '曙光礁', 'Shuguang Reef', 22.067078, 121.56979)
ON CONFLICT (name_en) DO UPDATE SET
  region = EXCLUDED.region,
  county = EXCLUDED.county,
  location = EXCLUDED.location,
  name_zh = EXCLUDED.name_zh,
  latitude = EXCLUDED.latitude,
  longitude = EXCLUDED.longitude,
  is_active = true;

CREATE INDEX IF NOT EXISTS survey_site_id_idx ON survey(site_id);
CREATE INDEX IF NOT EXISTS transect_survey_id_idx ON transect(survey_id);
CREATE INDEX IF NOT EXISTS substrate_point_transect_id_idx ON substrate_point(transect_id);
CREATE INDEX IF NOT EXISTS belt_observation_transect_id_idx ON belt_observation(transect_id);
CREATE INDEX IF NOT EXISTS belt_observation_taxon_id_idx ON belt_observation(taxon_id);
CREATE INDEX IF NOT EXISTS impact_observation_transect_id_idx ON impact_observation(transect_id);
CREATE INDEX IF NOT EXISTS impact_observation_impact_type_id_idx ON impact_observation(impact_type_id);
