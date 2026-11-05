# 公開圖表統計 API（2026-11-05）

所有統計由後端計算，前台直接使用 `value` 繪圖。四個端點共用 `PublicSeries`；比較只能指定兩個不同樣點。

- `GET /api/public/reef-check/sites/{site_id}/series`
- `GET /api/public/reef-check/sites/{site_id}/export.csv`
- `GET /api/public/reef-check/compare?site_ids=102,151`
- `GET /api/public/reef-check/compare.csv?site_ids=102,151`

篩選條件取交集：`charts`（substrate,taxa,live_coral,impact,bleaching）、`from_year`、`to_year`、`depth_m`、`survey_date`、`event_id`、`event_ids`、`category_keys`、`taxon_ids`、`taxon_group`（fish,invert,rare）、`unit`、`size_class`、`fish_size_mode`（split,combined）、`group_id`。列表用逗號分隔。不指定年份時涵蓋目前資料庫完整已發布歷史。共用群組需已發布、樣點及深度屬於群組；仍與使用者要求的圖表／年份取交集。

每列保留樣點、場次、日期、時間、深度、指標／正式物種 ID、體長方式／級距及單位；同日同深度不同場次不合併。只有已發布場次、啟用樣點及有效圖表設定允許的列會公開。

統計定義：

- 底質與活珊瑚：表層資料；HC-a/b/c 歸入 HC，NA 不計分母。`mean` 是有效段覆蓋率平均，`sd` 是段覆蓋率的樣本標準差，`se=sd/sqrt(n)`；`value` 是有效點數加權的整體覆蓋率。百分比均為 0–100；`total` 是符合分類的有效點數。
- 魚類與無脊椎：Reef Check 每段 20×5 m，`mean/value/density_per_100m2` 為有效段件數平均，`total` 為件數合計。稀有物種保留每子樣區尺度。`n` 只計已記錄且有數值的段，不把 NULL 當有效樣本。
- 衝擊：原始件數、百分比、歷史分級與衍生分級分列並保留機器單位；衍生分級不提供無意義的合計。
- 白化：保留 HC／SC 原始白化點數尺度，`n` 只計已記錄且有數值的段。

機器單位為 `percent`、`individuals_per_100m2`、`individuals_per_subsample`、`count`、`historical_level`、`derived_level`、`bleached_points`；另提供 `unit_label`。非物種列的 `taxon_id/size_class/fish_size_mode` 為 null，`taxon_group` 為空字串；未記錄時間為 null。非密度列 `density_per_100m2` 為 null。

有效 0 保留；無有效觀測時數值為 null、`n=0`、`calculation_status=not_calculable` 並提供 `missing_reason`。`n<2` 時 SD／SE 為 null。`data_updated_at` 是資料庫場次修改時間（ISO 8601 UTC），不使用請求當下時間；原始觀測、測線及場次變更會更新它。

單站回應為 `{site_id,filters,availability,series}`；比較為 `{filters,sites:[{site_id,availability,series},...]}`。空資料為 HTTP 200、availability=false、series=[]。CSV 有相同 25 個統計欄位，NULL 以空白儲存，0 仍為 0；帶 UTF-8 BOM。

效能與更新：先將符合條件的 event／transect 限縮，再逐測線計算，避免底質與白化全資料庫彙總。未引入快取或待更新的統計副本；發布、撤回、封存或編輯完成後，下次查詢直接反映資料。四端點均 `Cache-Control: no-store`。SQL statement_timeout=18 秒，單次 API（包含兩站比較）共用 20 秒 deadline，低於前台 30 秒；逾時回傳 HTTP 504 `{"error":"圖表資料查詢逾時"}`，不是成功空集合。

重現實測（包含 HTTP 下載及解析時間，依序跑兩輪）：

```sh
python3 scripts/benchmark-public-reef.py \
  --base https://coast-monitoring-dev-1011078206374.asia-east1.run.app --runs 2
```

測試包含合界 102 全歷年、2024 單年、102/151 比較、單站 CSV、比較 CSV，輸出每項耗時、HTTP 狀態與筆數。實測結果附在本次修正 PR 與交付回覆。

活動驗收：新增活動並啟用填報，儲存後在表單上方顯示代碼與複製按鈕；本次登入期間後續儲存仍顯示該碼。重新整理後原始代碼不能回讀，需使用「重發代碼」（舊碼失效）。觀測資料頁「管理活動與志工填報代碼」會切換至活動行程管理。
