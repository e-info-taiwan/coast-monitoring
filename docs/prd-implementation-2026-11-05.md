# PRD 0.9.13 功能補齊與測試結果

規格固定於網站 `8503b21ed04c0448767c25ac4a9a63e61c77be83`（Schema 1.16）。本文件接續 [71 項驗收案例](prd-test-cases-2026-11-04.md) 與 [實作前核對](prd-implementation-audit-2026-11-04.md)，不將未執行的人工驗收標為通過。

## 已補上的功能

| 案例 | 實作與驗證 |
|---|---|
| A02–A07 | 活動與唯一填報設定在同一表單／交易管理；一般活動不產碼。event 保存穩定 activity_id，舊入口轉入活動頁。重發／停用不改主活動，舊碼失效；前後值稽核不含 code_hash。DB 整合測試通過。 |
| A09 | 台北時區初始年月、跨日／跨月標記與完整活動起訖；日期模型測試通過。 |
| B02–B03 | 管理匯出依活動、event 集合、日期、樣點、深度、方法取交集，預設已發布；方法也限制匯出的 transect。既有管理權限保留。HTTP 回歸測試通過。這仍是 XLSX／ZIP，不是 B01 三種指定 CSV。 |
| B04–B06 | news／observation_report 分類、article_site 每站顯示開關、完整報告清單與 latest_report 排序；共用修改需確認，單站隱藏不影響其他站。DB 測試通過。 |
| B07–B10 | 樣點詳情整合基本資料、公開資訊與報告、圖表；各表單獨立 dirty state／版本，離開提醒。DOM 測試證實儲存一區不清掉其他區；共用群組提示變更前後站點聯集。註記串接文字與圖表。 |
| B11–B12 | development_case／timeline／media 獨立表為讀取來源，公開子項發布與授權規則保留；participant 刪除補前值、活動與顯示設定補稽核。 |
| H01–H04 | 完整 intro_popup、純文字換行、文章筆數及單一全站設定；首頁只顯示 news，沒有示範補卡。原本失敗的驗收測試已納入預設 Go 套件。 |
| C01–C03 | 六項 effective_settings、逐欄繼承、NULL 與空選擇區別、草稿／發布分離、版本衝突及還原封存。DB 測試通過。 |
| C04–C10 | 圖表保留每個 event/time/depth、全部可選物種、rare 分組、單位及 NULL；共用群組保留每個 site+depth，本站關閉不移除他站共用系列。圖、表、CSV 共用條件；兩站比較以解析後 ID 判重。模型、HTTP、DB 測試通過。 |
| C11–C12 | 公開網站正式流程移除 fixture／snapshot 回退；未設定、失敗及空集合分開；缺座標列出名稱而不造點。正式建置通過；線上瀏覽器驗收未執行。 |
| T01–T06 | 測站 manual／auto／unknown、操作者／時間、海溫能力與距離／可用性、同步狀態；新增 event 可明確覆寫本站預設，保存 automatic／manual 與觀測／取得時間。只讀同日資料，人工值不被自動值覆寫。SQL 與前端測站回歸通過。 |
| D14–D15 | 新增 migration 000018；不按同名合併舊活動，保留報告網址、隱藏值、完整來源與遷移對帳表。全新 18 個遷移及帶舊資料升級測試通過。 |

公開網站已保存於 [草稿 PR #1](https://github.com/e-info-taiwan/coast-monitoring-website/pull/1)，commit `f24300d621ff2378d7f1051d97c46f9bc9019e7c`。後端與後台已保存於 [草稿 PR #1](https://github.com/e-info-taiwan/coast-monitoring/pull/1)，功能 commit `7f3fbc72b0b0a88bcab536d14a4f17c905f4c4f8`。

## 實際執行結果

| 執行範圍 | 結果 |
|---|---|
| 完整 Go 套件，含交付 CSV corpus 與隔離 PostgreSQL WASM | 200 個最上層測試通過，0 失敗；1 個舊 v1.7 匯入 DB 案例 SKIP。子案例不重複計數。 |
| 後台／志工／地圖／DOM／遷移 JavaScript | 19 通過，0 失敗、0 SKIP。 |
| 獨立公開網站 `node --test scripts/*.test.mjs` | 23 通過，0 失敗、0 SKIP。保留離線 snapshot 工具的測試，不代表 production 回退快照。 |
| 獨立公開網站 `npm run build` | TypeScript 與 Vite production build 通過；仍有大型 bundle 提醒。 |

原生 PostgreSQL 在此環境受共享記憶體限制，故使用 PostgreSQL WASM，透過 stdio 協定執行相同 repository SQL，不是 SQL stub。每條連線建立新的隔離庫並套用全部 migrations，fixture 以交易回滾。OIDC discovery 測試改用記憶體 HTTP transport，保留真實 discovery 解析／端點與參數斷言，不再排除該測試。

重跑：

```sh
# 如要連同交付 corpus 測試，指向包含 impact_observation.csv 等檔案的 csv 目錄。
export REEF_HANDOFF_CSV_DIR=/absolute/path/to/01_reefcheck_observations/csv
scripts/test-prd.sh
```

`scripts/test-prd.sh` 需要 Node 22.18+ 或 26、Go 及 npm。若未指定 corpus，該案例會明確 SKIP。預設 `go test ./...` 仍可執行單元／HTTP 測試，DB 案例需設定獨立測試庫；不可填線上 DATABASE_URL。另提供 PR CI workflow 使用 PostgreSQL 16 service、執行正式 migration 命令及 Go／JS 測試；[GitHub 原生 PostgreSQL 16 CI](https://github.com/e-info-taiwan/coast-monitoring/actions/runs/41948705064) 已實際通過 migration、完整 Go 套件及 19 項 JS 測試；CI 未提供交付 corpus 與舊 v1.7 fixture，這兩項會 SKIP，本地另有交付 corpus 通過紀錄。

## 尚待交付／環境驗收

- **B01 三種指定管理 CSV 尚未完成**：仍缺正式 golden 樣本、欄位／粒度／命名；不以現有 XLSX 或公開 CSV 替代。
- `TestReefDataConfirmedPackage` 依賴舊 v1.7 confirmed-only 匯入庫，本次沒有該 fixture；新交付 corpus 另有實跑且通過。
- 真實 staging／production migration、既有資料對帳、瀏覽器完整填報與各裝置／Figma 驗收尚未執行。
- 舊多深度 chart 的完整來源保存在 `reef_prd_migration_review`，初始深度採原陣列第一項；舊同名活動／重複報告不猜合併。正式切換前需逐筆檢查對帳清單。
- PRD 未定的正式 CSV、深淺邊界、測站適用距離、同步頻率、效能門檻與權限細節，仍沿用待確認狀態。

## 部署順序

先備份並在 staging 套用 `000018_prd_site_activity.sql`、檢查遷移對帳，再發布配套後端，最後發布網站 PR #1。新後端讀取新表，舊後端仍使用舊 metadata 欄位名稱，兩版本不適合混跑；部署時應協調流量切換。此工作沒有部署或修改線上觀測資料。
