-- Seed Reef Check Taiwan GPS sites from the reviewed GPS CSV.
WITH seed_locations(chinese_name, english_name) AS (
  VALUES
    ('萬里區', '萬里區'),
    ('中正區', '中正區'),
    ('瑞芳區', '瑞芳區'),
    ('貢寮區', '貢寮區'),
    ('恆春鎮', '恆春鎮'),
    ('豐濱鄉', '豐濱鄉'),
    ('成功鎮', '成功鎮'),
    ('卑南鄉', '卑南鄉'),
    ('東河鄉', '東河鄉'),
    ('望安鄉', '望安鄉'),
    ('馬公市', '馬公市'),
    ('琉球鄉', '琉球鄉'),
    ('綠島鄉', '綠島鄉'),
    ('蘭嶼鄉', '蘭嶼鄉')
)
INSERT INTO locations (chinese_name, english_name)
SELECT chinese_name, english_name
FROM seed_locations
WHERE NOT EXISTS (
  SELECT 1 FROM locations WHERE locations.chinese_name = seed_locations.chinese_name
);

WITH seed_sites(location_name, county, site_name, site_english_name, latitude, longitude) AS (
  VALUES
    ('萬里區', '新北市', '野柳', 'Yeliu', 25.211511, 121.698828),
    ('中正區', '基隆市', '潮境保育區', 'Chaojing protected area', 25.145531, 121.806622),
    ('瑞芳區', '新北市', '深澳（番仔澳）', 'Shenao', 25.135556, 121.821944),
    ('瑞芳區', '新北市', '鼻頭角', 'Bitoujiao', 25.126111, 121.914167),
    ('貢寮區', '新北市', '龍洞1.5號', 'Longdong 1.5', 25.115806, 121.917000),
    ('貢寮區', '新北市', '龍洞4號', 'Longdong 4', 25.113803, 121.920169),
    ('中正區', '基隆市', '和平島', 'Heping Island', 25.162602, 121.761922),
    ('恆春鎮', '屏東縣', '合界', 'Hejie', 21.955583, 120.711889),
    ('恆春鎮', '屏東縣', '後壁湖', 'Houbihu', 21.936584, 120.747359),
    ('豐濱鄉', '花蓮縣', '石梯坪', 'Shitiping', 23.484078, 121.513784),
    ('成功鎮', '臺東縣', '基翬', 'Kihaw', 23.116331, 121.396829),
    ('卑南鄉', '臺東縣', '杉原中礁', 'Shanyuan middle reef', 22.829722, 121.188889),
    ('卑南鄉', '臺東縣', '杉原南礁', 'Shanyuan southern reef', 22.825045, 121.194028),
    ('卑南鄉', '臺東縣', '杉原', 'Shanyuan', 22.831238, 121.187613),
    ('東河鄉', '臺東縣', '加母子灣', 'Jiamuzi Bay', 22.863548, 121.209430),
    ('望安鄉', '澎湖縣', '東嶼坪西', 'Dongyuping west', 23.257370, 119.509968),
    ('望安鄉', '澎湖縣', '東嶼坪東', 'Dongyuping east', 23.260306, 119.519556),
    ('望安鄉', '澎湖縣', '東嶼坪南', 'Dongyuping south', 23.254600, 119.512414),
    ('望安鄉', '澎湖縣', '東嶼坪北側', 'North Dongyuping Island', 23.265128, 119.519995),
    ('望安鄉', '澎湖縣', '西嶼坪北側', 'North Xiyuping Island', 23.273670, 119.507600),
    ('馬公市', '澎湖縣', '杭灣', 'HangWan', 23.516289, 119.605964),
    ('馬公市', '澎湖縣', '山水港', 'Shanshui port', 23.511050, 119.598520),
    ('琉球鄉', '屏東縣', '美人洞', 'Beauty Cave', 22.354444, 120.371111),
    ('琉球鄉', '屏東縣', '漁埕尾', 'Yuchengwei', 22.349278, 120.389889),
    ('琉球鄉', '屏東縣', '厚石裙礁', 'Houshi fringing reef', 22.323056, 120.365556),
    ('綠島鄉', '臺東縣', '柴口', 'Chaikou', 22.681389, 121.482778),
    ('綠島鄉', '臺東縣', '公館', 'Gongguan', 22.677221, 121.492581),
    ('綠島鄉', '臺東縣', '將軍岩', 'General rock', 22.677647, 121.498239),
    ('綠島鄉', '臺東縣', '石朗', 'Shihlang', 22.654828, 121.473514),
    ('綠島鄉', '臺東縣', '龜灣', 'Turtle bay', 22.642890, 121.483708),
    ('綠島鄉', '臺東縣', '大白沙', 'Dabaisha', 22.639209, 121.491964),
    ('蘭嶼鄉', '臺東縣', '玉女岩', 'Beauty Rock', 22.082083, 121.516333),
    ('蘭嶼鄉', '臺東縣', '母雞岩', 'Hen Rock', 22.084278, 121.553278),
    ('蘭嶼鄉', '臺東縣', '南獅', 'Lion Couple Rock south', 22.085450, 121.565726),
    ('蘭嶼鄉', '臺東縣', '東清小涼亭', 'DongJing pavilion', 22.070144, 121.568989),
    ('蘭嶼鄉', '臺東縣', '曙光礁', 'Shuguang Reef', 22.067078, 121.569790)
)
INSERT INTO sites (location_id, county, chinese_name, english_name, latitude, longitude)
SELECT locations.id, seed_sites.county, seed_sites.site_name, seed_sites.site_english_name, seed_sites.latitude, seed_sites.longitude
FROM seed_sites
JOIN locations ON locations.chinese_name = seed_sites.location_name
WHERE NOT EXISTS (
  SELECT 1
  FROM sites
  WHERE sites.location_id = locations.id
    AND sites.county = seed_sites.county
    AND sites.chinese_name = seed_sites.site_name
);
