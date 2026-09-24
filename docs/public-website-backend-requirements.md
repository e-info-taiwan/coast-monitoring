# 珊瑚觀測公開網站：後台資料與 API 增補規格

版本：2026-09-24  
用途：提供後端工程師依 Figma「珊瑚」、`2026 珊瑚網站時程規劃.md` 第 3 項「網站展示」及現行程式盤點結果實作。

## 1. 盤點來源

- Figma：`t7Re9PhqvfRZ8x5jJOL0Nr`，首頁 intro、首頁地圖、樣區／開發案簡介、行程表、開發案圖層、比較頁面及桌機／手機狀態。
- 規格書：`2026 珊瑚網站時程規劃.md` 的「3. 網站展示」。
- 計算規格：`RC網站規劃細節.md`。
- Reef Check v1.7 schema 與匯入包。
- 現行 API：`docs/api.md`、`internal/http/router.go` 及 v1.7 admin handlers。
- 開發案整理包：`outputs/development_events/2026-09-24_google_mymaps_v1/`。

## 2. 現況結論

### 2.1 已有資料，不需要重新建模

目前 v1.7 已有以下 Reef Check 原始資料結構：

- `site`：區域、縣市、鄉鎮、中文／英文樣點名、經緯度。
- `survey`、`event`：出動、日期、時間、深度。
- `transect`：`line`、`belt_fish`、`belt_invert`。
- `substrate_point`、`substrate_bleaching`。
- `belt_observation`：魚類、無脊椎、罕見生物。
- `impact_observation`：環境衝擊、白化及疾病資料。
- `taxon`、`substrate_type`、`impact_type`、`diver` 及參與者。

已驗證的基準資料量：

| 資料 | 筆數 |
|---|---:|
| 有觀測樣點 | 50 |
| survey | 392 |
| event | 726 |
| transect | 1,985 |
| substrate_point | 104,800 |
| substrate_bleaching | 2,620 |
| belt_observation | 74,780 |
| impact_observation | 22,956 |

### 2.2 現有 API 不能供公開網站使用

目前實際 router 只有管理員用的 v1.7 API：

- `GET /api/admin/reef-check-data/events`
- `GET /api/admin/reef-check-data/events/{id}`
- `GET /api/admin/reef-check-data/sites`
- 其餘編輯與主檔 API

這些端點要求 admin session、cookie 及 CSRF，不得直接提供給公開前台。

`docs/api.md` 所列的 `/api/app/reef-check/config`、`/surveys`、`/report` 目前未在 `internal/http/router.go` 註冊，而且舊 UUID Reef Check tables 已在 migration `000006` 移除。工程師不可把文件中的這組端點視為現有功能；應更新文件，並以 v1.7 單數表為唯一資料來源。

目前沒有任何免登入的 `/api/public/*` Reef Check、首頁內容、行程、開發案或比較 API。

## 3. Figma 與規格書需要的資料

| 前台區塊／互動 | 需要的資料 | 現況 | 後台工作 |
|---|---|---|---|
| 首頁地圖樣點 | 樣點名稱、區域、縣市、座標、事件數、最後調查日 | 原始資料已有；公開 API 無 | 新增公開樣點摘要 API |
| Zoom 群聚泡泡 | 視窗範圍內樣點座標及事件數 | 可由現有資料彙整 | API 回傳點資料；前端群聚，或 API 支援 bbox/zoom |
| 樣區基本資訊 | 中英名稱、位置、最近報告、資料更新時間 | 名稱／位置已有；報告與更新時間不足 | 擴充樣點公開內容欄位 |
| 底質歷年圖表 | 日期、深度、底質覆蓋率、有效點數、SD、SE | 原始逐點資料已有；跨年報表無 | 新增跨年彙整 query/API |
| 物種歷年圖表 | 日期、深度、taxon、平均、SD、SE、單位 | 原始四段資料已有；跨年報表無 | 新增跨年彙整 query/API |
| 活珊瑚覆蓋率 | `HC + SC`、深度、日期、SD、SE | 可計算；公開 API 無 | 納入跨年彙整 API |
| 災害註記 | 颱風、白化或其他事件的日期／期間、說明、來源 | 缺資料表及正式資料 | 新增 annotation table 與後台 CRUD |
| 圖表顯示設定 | 顯示哪些圖表、類別、選項、年份及排序 | 完全缺少 | 新增 chart config 與後台設定 |
| CSV 下載 | 與目前篩選條件一致的底質／物種／活珊瑚資料 | 缺少 | 新增 CSV export API |
| 兩樣區比較 | 兩個樣區相同口徑的時間序列及 metadata | 可沿用跨年 API；目前沒有 | 新增 compare API 或允許一次查兩站 |
| 最新文章 | 日期、標題、摘要、縮圖、外部網址、發布狀態 | 缺資料表／來源串接 | 新增內容來源與公開 API |
| 行程表 | 日期、標題、招募類型、報名網址、狀態 | 缺資料表／來源串接 | 新增活動資料與公開 API |
| 團隊／珊瑚介紹 | 頁面段落、成員、照片、連結、排序 | 規格指定 Google Sheet；尚未結構化 | 建立 Sheet 同步或內容表及公開 API |
| 開發案圖層 | 點／面 GeoJSON、名稱、摘要、進度 | 已整理 25 案匯入包；尚未進正式 DB | 套用新 schema、匯入及發布流程 |
| 開發案 lightbox | 縣市、主管機關、面積、投資額、進度、時間軸、來源、圖片 | 來源多數缺結構化值 | 建立 CRUD；由編輯補資料後發布 |

## 4. 必須新增或擴充的資料表

### 4.1 樣點公開內容 `site_public_profile`

避免把編輯內容塞進科學資料主檔；一個 `site` 對應一筆公開設定。

| 欄位 | 型別 | 說明 |
|---|---|---|
| site_id | FK PK | 對應 `site.id` |
| slug | text unique | 公開網址穩定識別 |
| summary | text nullable | 樣區簡介 |
| hero_image_url | text nullable | 樣區圖片 |
| latest_report_title | text nullable | 最新報告名稱 |
| latest_report_url | text nullable | 報告外連或檔案網址 |
| latest_report_date | date nullable | 報告日期 |
| is_published | boolean | 未發布樣點不出現在前台 |
| updated_at | timestamptz | 前台「資料最後更新時間」來源之一 |

### 4.2 樣區事件註記 `site_annotation`

| 欄位 | 型別 | 說明 |
|---|---|---|
| id | bigserial PK | |
| site_id | FK | 可針對單一樣點；若要區域共用，另加 nullable region key |
| event_type | text | `typhoon`／`bleaching`／`development`／`other` |
| date_start / date_end | date | 單日或期間 |
| date_precision | text | `day`／`month`／`year`／`range` |
| title | text | 圖表標記及 tooltip 標題 |
| description | text nullable | 詳細說明 |
| severity | text nullable | 受控詞彙待產品確認 |
| source_url | text nullable | 查證來源 |
| is_published | boolean | 只公開已發布資料 |
| created_at / updated_at | timestamptz | |

### 4.3 圖表設定 `site_chart_config`

每個樣點可覆寫全站預設；沒有樣點設定時使用 `site_id IS NULL` 的全站設定。

| 欄位 | 型別 | 說明 |
|---|---|---|
| id | bigserial PK | |
| site_id | FK nullable | null 代表全站預設 |
| chart_key | text | `substrate`／`taxa`／`live_coral` |
| is_visible | boolean | 是否顯示 |
| default_depths | numeric[] | 預設深度選項 |
| category_keys | text[] | 預設底質 code 或 taxon key |
| year_from / year_to | integer nullable | 顯示年度範圍 |
| sort_order | integer | 圖表排序 |
| updated_at | timestamptz | |

### 4.4 最新文章 `public_article`

| 欄位 | 型別 | 說明 |
|---|---|---|
| id | bigserial PK | |
| title | text | |
| excerpt | text nullable | 卡片摘要 |
| image_url | text nullable | 縮圖 |
| external_url | text | 點擊後外連 |
| published_at | timestamptz | 排序依據 |
| is_published | boolean | |
| sort_order | integer nullable | 可人工置頂 |
| created_at / updated_at | timestamptz | |

若文章實際來源為既有網站，可改用同步表；前台 API 契約不變。

### 4.5 年度行程 `public_activity`

| 欄位 | 型別 | 說明 |
|---|---|---|
| id | bigserial PK | |
| title | text | Figma 行程列主標 |
| start_at / end_at | timestamptz | 支援單日／跨日 |
| recruitment_type | text | `public`／`internal`；只有 public 顯示報名按鈕 |
| registration_url | text nullable | 報名網址 |
| detail_url | text nullable | 詳細活動頁 |
| site_id | FK nullable | 可連到調查樣點 |
| status | text | `scheduled`／`open`／`closed`／`cancelled`／`completed` |
| is_published | boolean | |
| created_at / updated_at | timestamptz | |

### 4.6 靜態內容與團隊

規格要求管理者編輯 Google Spreadsheet，再定期轉為結構資料。後端至少提供：

- `content_page`：`slug`、`title`、`sections jsonb`、`is_published`、`source_updated_at`、`updated_at`。
- `team_member`：姓名、職稱／角色、介紹、照片、外部連結、排序、是否發布。
- 一個具冪等性的 Sheet sync job；來源列刪除時先標記停用，不直接 hard delete。
- sync log：來源版本、同步時間、成功／失敗、筆數、錯誤訊息。

### 4.7 台灣海岸觀光開發案

使用既有交付包中的四表設計：

- `development_status`
- `development_case`
- `development_case_timeline`
- `development_case_media`

幾何優先使用 PostGIS `geometry(Geometry,4326)`，同時支援 Point、Polygon、MultiPolygon，並建立 GiST index。完整欄位、匯入及重跑規則見 `outputs/development_events/2026-09-24_google_mymaps_v1/資料庫異動規格.md`。

## 5. 公開 API 規格

所有下列 GET API 免登入、免 cookie、免 CSRF。只回傳 `published`／`is_published=true` 的資料；不得回傳潛水員、使用者、email、稽核紀錄或未發布備註。

### 5.1 首頁聚合

```http
GET /api/public/home
```

```json
{
  "last_updated_at": "2026-09-24T12:00:00+08:00",
  "articles": [
    {"id": 1, "title": "...", "excerpt": "...", "image_url": "...", "external_url": "...", "published_at": "2026-09-20T00:00:00+08:00"}
  ],
  "next_activities": [
    {"id": 1, "title": "...", "start_at": "...", "end_at": "...", "recruitment_type": "public", "registration_url": "...", "status": "open"}
  ]
}
```

文章預設最多 7 筆；活動預設回傳當年度及下一個仍未結束的活動。亦可分拆成 `/articles` 與 `/activities`，但 `/home` 應避免首頁初次載入多次 round trip。

### 5.2 地圖樣點

```http
GET /api/public/reef-check/sites?bbox=west,south,east,north&from_year=2009&to_year=2026
```

每個樣點至少回傳：

```json
{
  "id": 12,
  "slug": "longdong-4-north",
  "name_zh": "龍洞4號-北",
  "name_en": "LongDong4North",
  "region": "北海岸與東北角",
  "county": "新北市",
  "location": "貢寮區",
  "latitude": 25.1138028,
  "longitude": 121.9201694,
  "survey_count": 19,
  "event_count": 52,
  "first_survey_date": "2009-05-23",
  "latest_survey_date": "2025-08-16",
  "data_updated_at": "2026-09-24T12:00:00+08:00"
}
```

- 預設只回有觀測資料且已發布的樣點。
- 無座標資料不可用猜測位置回傳；可提供 `include_unmapped=true` 給列表頁使用。
- 目前 50 個有觀測樣點中只有 21 個有可用座標；其餘 29 個需客戶補座標或後台人工確認。
- 群聚可由前端 MapLibre cluster 處理；若改成 server clustering，需另定義 cluster id、count、centroid、zoom expansion。

### 5.3 樣點詳情

```http
GET /api/public/reef-check/sites/{slug}
```

回傳樣點摘要、可用深度、資料年份範圍、最新報告、最後更新時間、可用圖表及已發布 annotation。

```json
{
  "site": {"id": 12, "slug": "...", "name_zh": "...", "name_en": "...", "summary": "..."},
  "available_depths_m": [5, 10],
  "year_range": {"from": 2009, "to": 2025},
  "latest_report": {"title": "2025 調查報告", "url": "...", "date": "2025-12-01"},
  "chart_config": [],
  "annotations": [],
  "data_updated_at": "2026-09-24T12:00:00+08:00"
}
```

### 5.4 跨年圖表資料

```http
GET /api/public/reef-check/sites/{slug}/series?depth_m=5&from_year=2009&to_year=2025&charts=substrate,taxa,live_coral&taxon_ids=1,2
```

回傳同一日期／深度口徑的三組 series：

- `substrate`：每個 substrate code 的 coverage percent、SD、SE、有效點數。
- `taxa`：taxon/size class 的四段平均、SD、SE、有效段數。
- `live_coral`：`HC + SC` 的 coverage percent、SD、SE。
- `annotations`：落在查詢期間內的已發布圖表事件。

每個資料點必須帶：`survey_date`、`depth_m`、`mean` 或 `coverage_percent`、`sd`、`se`、`n`、`missing_reason`。不可把缺測補成 0。

建議回傳格式：

```json
{
  "site": {"id": 12, "slug": "..."},
  "filters": {"depth_m": 5, "from_year": 2009, "to_year": 2025},
  "substrate": [{"date": "2025-08-16", "code": "HC", "coverage_percent": 36.875, "sd": 4.1, "se": 1.025, "n": 4}],
  "taxa": [{"date": "2025-08-16", "taxon_id": 1, "name_zh": "蝴蝶魚", "size_class": null, "mean": 10.75, "sd": 2.5, "se": 1.25, "n": 4}],
  "live_coral": [{"date": "2025-08-16", "coverage_percent": 37.5, "sd": 4.0, "se": 1.0, "n": 4}],
  "annotations": []
}
```

同站、同日、同深度若有多個不同時間 event，API 不得默默挑一筆。應依確認的產品規則回傳獨立點，或合併並在 `sample_count` 說明；此規則需在上線前固定並加測試。

### 5.5 CSV 匯出

```http
GET /api/public/reef-check/sites/{slug}/export.csv?depth_m=5&from_year=2009&to_year=2025&charts=substrate,taxa,live_coral&taxon_ids=1,2
```

- 查詢條件與 series API 完全一致。
- UTF-8 BOM、RFC 4180、`Content-Type: text/csv; charset=utf-8`。
- `Content-Disposition` 帶可辨識檔名。
- 每列含樣點、日期、時間（若適用）、深度、圖表類型、指標 code/id、中英名稱、size class、值、SD、SE、n、資料更新時間。

### 5.6 樣區比較

```http
GET /api/public/reef-check/compare?site_slugs=a,b&depth_m=5&from_year=2009&to_year=2025&charts=substrate,taxa,live_coral
```

- `site_slugs` 必須剛好 2 個，符合 Figma 的比較流程。
- 兩站使用相同計算口徑與 filter metadata。
- 若某站沒有指定深度或年份資料，回傳空 series 及 `availability`，不要用 0 補齊。
- 亦可讓前端並行呼叫兩次 series API，但若資料量大，建議提供 compare endpoint 以確保口徑與快取一致。

### 5.7 開發案

```http
GET /api/public/development-cases?bbox=west,south,east,north&status=...
GET /api/public/development-cases/{id}
```

列表回傳 GeoJSON geometry、名稱、狀態、摘要；詳情另回縣市、主管機關、面積、投資金額、進度原文、detail/source URL、已發布圖片及已發布時間軸。

### 5.8 靜態頁

```http
GET /api/public/pages/reef-check-intro
GET /api/public/team
GET /api/public/activities?year=2026
GET /api/public/articles?limit=7
```

## 6. 計算規則

### 6.1 底質

- 每 event／depth／segment 各自以有效、非 `NA` 點為分母。
- `coverage_percent = category_count / valid_point_count * 100`。
- 四段 coverage 計算樣本標準差 `SD = STDEV.S(segment coverages)`。
- `SE = SD / sqrt(n)`；`n` 使用實際有資料段數，不固定假設為 4。
- 少於 2 段時 `sd`、`se` 為 null，並回傳 `missing_reason`。
- 活珊瑚覆蓋率依規格為 `HC + SC`。
- `SI(HC)` 等複合代碼目前 schema 視為獨立 surface category；不得未經確認自動算入 HC／SC。若前台要把所有 `SI(*)` 合併為 SI，需明訂 display mapping，但保留原 code。

### 6.2 魚類、無脊椎與罕見生物

- 同一 event／depth／taxon／size class 的四段值計算 mean、樣本 SD、SE。
- 缺段不是 0；只有資料明確記錄 0 才是 0。
- aggregate taxon（例如石斑魚總數）由 `aggregate_of` 對 size classes 加總，不重複計入原始輸入。
- 「密度」若要換算成每 100 m² 或其他面積，必須先由客戶確認 belt 寬度與單位；未確認前 API 名稱使用 `mean_count`，不要誤標 `density`。

### 6.3 環境衝擊

- 件數分級：0 → 0、1 → 1、2–4 → 2、5+ → 3。
- percent 一律 0–100。
- 平均、SD、SE 使用實際有資料段數。
- 原始件數與衍生 grade 均保留並可輸出。

## 7. 後台管理 API／介面

除了公開 GET，管理後台需有下列 CRUD，全部沿用 admin session、CSRF、audit log 及刪除二次確認：

- 樣點公開 profile、最新報告、發布狀態。
- site annotations。
- chart config。
- 最新文章及排序／發布。
- 年度行程、招募類型、報名網址及發布。
- 靜態頁與團隊內容；若由 Google Sheet 同步，提供最後同步結果及手動重跑。
- development status、case、timeline、media 及發布。

所有更新至少記錄：actor、時間、target table/id、before、after、method、path。

## 8. 公開 API 的 CORS、快取與時間

- 將 `http://localhost:5173`、`http://localhost:5174` 及正式前台網域加入 public CORS；公開 API 不使用 credentials。
- `Access-Control-Allow-Origin` 可依 allowlist 回應；不要沿用 admin cookie CORS middleware。
- API 日期時間使用 ISO 8601；資料庫 timestamptz，輸出含時區。
- 公開資料支援 `ETag` 或 `Last-Modified`。
- 地圖／series 可設短期 CDN cache；發布或資料更新後需可失效。
- `data_updated_at` 是該公開資料集最後有效更新時間，不是伺服器每次 request 的現在時間。

## 9. 工程優先順序

### P0：讓現有前台不再依賴本機 snapshot

1. 建立 `/api/public/reef-check/sites`。
2. 建立 site detail 及跨年 series。
3. 補公開 CORS、快取、測試與 API 文件。
4. 補齊或確認 29 個有觀測但無座標樣點；未補前可正常排除於地圖。
5. 新增 `site_public_profile`，至少支援發布狀態、最新報告及更新時間。

### P1：完成規格書主要互動

1. CSV export。
2. 兩樣區 compare。
3. annotations 及圖表顯示設定。
4. 最新文章、行程表、團隊與介紹頁資料。
5. 開發案 schema、25 筆匯入、公開 GeoJSON 及 lightbox detail。

### P2：編輯效率與長期維運

1. Google Sheet 定期同步、sync log、錯誤通知。
2. chart query materialized view／快取（確定實際流量後再做）。
3. 開發案來源重抓、差異比對及 missing 標記流程。

## 10. 目前確定仍缺客戶或編輯提供的內容

這些不是工程師可由既有觀測資料自行推導的值：

- 29 個有觀測樣點的正式經緯度。
- 各樣點簡介、代表圖片、最新報告名稱／網址／日期。
- 颱風、白化或其他災害事件的正式日期、影響樣點、說明及來源。
- 最新文章的來源清單或同步規則。
- 年度行程、對外招募判定及報名網址。
- 團隊成員及珊瑚體檢介紹頁的 Google Sheet 欄位與內容。
- 25 個開發案的縣市、主管機關、正式面積、投資金額、標準化進度及逐筆時間軸。
- 開發案圖片的授權、alt text 及長期託管位置。
- 物種圖表是否顯示「平均數量」或需換算成指定面積的「密度」。
- 同站、同日、同深度多個 event 在歷年圖表的合併規則。

## 11. 驗收條件

- 未登入瀏覽器可從 localhost 與正式前台讀取 `/api/public/*`，不需 CSRF/cookie。
- 公開 API 不含個資、參與者、未發布內容或 admin-only 欄位。
- 50 個有觀測樣點均可由 API 列出；有座標者可上圖，無座標者不製造假座標。
- 以既有範例「墾丁合界 5m」抽樣，比對底質、活珊瑚及 taxa 歷年值與規格表一致。
- 缺段、NA、少於 2 段的 SD/SE 均正確處理，不補 0。
- compare 與單站 series 使用完全相同的計算結果。
- CSV 與畫面篩選結果一致，Excel 開啟中文正常。
- 開發案只回 `published`；Point 與 Polygon 均能在 GeoJSON 正確顯示。
- admin 內容修改有 audit log，發布狀態與公開 API 可在合理快取時間內同步。
- `docs/api.md` 更新為實際 router，刪除或標註失效的舊 `/api/app/reef-check/*` 說明。
