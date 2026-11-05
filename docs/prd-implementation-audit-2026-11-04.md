# PRD 0.9.13／Schema 1.16 實作核對

> 此為實作前核對紀錄。2026-11-05 的修正與實測結果請看 [功能補齊與測試結果](prd-implementation-2026-11-05.md)。下列缺漏／失敗不代表修正後狀態。

規格日期：2026-11-04。測試及核對完成：2026-11-05（Asia/Taipei）。

**結論：尚未全部實作，不可用既有回歸測試通過代表新版 PRD 驗收完成。** 本次先建立 [71 項驗收案例](prd-test-cases-2026-11-04.md)，再補可執行測試並核對後端、管理介面、獨立公開網站及 migrations。新增的 10 個頂層測試中，3 個相容行為測試通過、7 個目標需求測試失敗。失敗測試保留原預期，未為配合現況而放寬需求。本次沒有修改產品功能、線上資料或執行部署。

## 1. 核對基準與證據邊界

- 規格及公開網站固定於 `e-info-taiwan/coast-monitoring-website@8503b21ed04c0448767c25ac4a9a63e61c77be83`：[PRD](https://github.com/e-info-taiwan/coast-monitoring-website/blob/8503b21ed04c0448767c25ac4a9a63e61c77be83/prd/珊瑚觀測網站%20PRD.md)、[Schema](https://github.com/e-info-taiwan/coast-monitoring-website/blob/8503b21ed04c0448767c25ac4a9a63e61c77be83/prd/珊瑚礁調查資料庫%20Schema%20規格（Reef%20Check）.md)。以下 `website:` 路徑皆指此版本，非本 repo 的 `web/site`。
- 後端使用本工作目錄當前內容：本機 HEAD 為 `4cd065e74e983534c3c5f4a73d1c10df8a5eff3f`，另有既存工作目錄修改及 `000017` migration。不能只用 HEAD SHA 重現這份後端；本次未覆蓋既有變更，也沒有將工作目錄與線上部署宣稱為完全一致。
- 「具備」表示看得到相應實作且有指定範圍的程式／測試證據；不等於 DB、瀏覽器或實體裝置驗收。「部分」表示已有基礎但缺新版契約。「缺漏」表示已由測試或來源核對發現差異。「未驗證／待交付」不算通過。
- 未連線目標 Cloud SQL、未取得本次 schema-only dump／migration 實際套用清單，未以瀏覽器驗收。之前匯入的成功回執不等於新版 Schema 9.3–9.9 已部署。

## 2. 優先處理的差異

### P1：正式頁面仍可能顯示舊資料

C11、H03 不符合。`website:src/App.tsx:638` 起先抓 `reef-check-summary.json`；API 無篩選且回 `[]` 或出錯後會回退快照（約 695–715 行）。圖表快照也無條件載入。`ReefCharts.tsx:240` 以 `apiEvents.length > 0` 判斷使用 API；成功空集合也會改用快照。`ReefComparison.tsx:35` 雖已保留成功空回應，載入中／錯誤時仍使用快照。開發案 `App.tsx:572` 採 `apiDevelopmentSnapshot ?? developmentSnapshot`，而空正式集合會轉成 null。首頁文章成功空集合時不清除原本七張示範卡片。

因此「DB 已替換」與「畫面一定看到新版資料」不能畫等號。修正應讓未設定 API、載入失敗及成功空集合各有明確狀態；正式環境不可顯示快照／示範資料作為替代。這是來源核對結果，未宣稱本次已重現線上畫面。

### P1：畫面與 CSV 條件不一致

C09 不符合。`website:src/components/ReefCharts.tsx:410` 底質圖含同日多個 event，下載卻只傳 `surveyEvents[0]?.eventId`，會漏掉其他深度／時間場次。物種圖僅取前六系列，下載全部 taxa；罕見生物下載也只傳 `charts=taxa`；環境圖依單位分組，CSV 沒傳該單位。

後端 `PublicReefFilter` 只有單一 event_id、年份、深度、charts、taxon_ids 等；未提供日期 event 集合、taxon_group、unit 篩選。series 及 CSV 缺明確 taxon_id／taxon_group（有 key，但不能替代契約欄位）。新測試已重現欄位缺漏。新增的 JSON/CSV 一致性回歸測試只證明「已支援的相同條件交給同一 service、兩 event 不被 formatter 合併、NULL 不變成 0」，不涵蓋上述缺少條件或 DB 查詢正確性。

### P1：活動、樣點、報告與圖表整合仍是舊模型

- A02–A06：公開行事曆在 public_content，代碼活動在 reef_check_activity，仍有兩個獨立管理入口。缺 activity_content_id、event.activity_id 及 entry_config_id 轉接；代碼驗證回傳的是填報設定 ID。既有代碼安全與範圍驗證不能替代穩定行事曆活動關聯。
- B04–B09：沒有 article category／article_site；仍只有 profile.report_url。未知原文日期會被 Validate 拒絕。樣點基本資料、profile、annotation、chart 是分開入口，尚無同頁三區獨立儲存及跨站影響提示。
- C01–C03、C07–C08：缺 site_chart_setting、shared_chart_group/member、六項 effective_settings 與逐欄繼承。舊 chart JSON 不能替代。公開端 `.find()` 只取第一個共用圖設定，且同組查詢都用單一 coralDepth，無法保存 A5m/A10m/B10m 的系列組合。

### P2：首頁設定、測站追溯及行事曆尚有缺口

- H01–H03：article_limit 的 1–50 整數驗證存在；intro_popup 在服務解析時丟失，管理表單沒有文字欄位，公開彈窗文字寫死在 App.tsx。settings 也未拒絕非 NULL site_id。文章未分類，不能只顯示 news。
- T01–T06：既有站對應、同日期海溫查詢及 UI 缺值提示存在。UpdateSite 已保留未提交的測站欄位；AutoLinkSites 只配對空白站，未覆蓋既有站。但缺 auto/manual/unknown、選站更新人／時間、候選海溫能力／距離、單次海溫來源與取得時間。樣點頁仍是自由文字站號，無完整搜尋／同步入口；新增 event 的站欄目前唯讀，缺新版要求的單次覆寫。海溫查詢回應有 observed_at，不代表 event 已持久保存來源時間。
- A09：`website:src/App.tsx:194–211` 以瀏覽器時區取日期，年份預設取活動年份最大值（可能跳未來年），跨日／跨月只標開始日的日數。需改為 Asia/Taipei 當前年月及完整涵蓋日期。
- C05/C06：物種 `.slice(0,6)`、附加圖 `.slice(0,8)`，沒有讓使用者選取其餘系列的控制。數值表 `lineRows` 還會濾掉 mean=null 的列，缺測資訊不完整。
- B12：多數觀測／主檔異動有前後值稽核；但刪除 participant 的 writeAudit 傳 nil/nil，活動代碼稽核僅記 action/actor/time，仍須補全指定範圍的變更前後值，不能一概標全部完成。

## 3. PRD → 資料結構 → API → 前端 → migration／測試

下表覆蓋所有 71 項案例；相鄰且共用證據的案例合併列示。既有測試通過僅限測試涵蓋範圍。

| 案例 | 現有／目標實體 | API 與前端入口 | migration／測試及判定 |
|---|---|---|---|
| A01 | public_content activity | `/api/public/content/activity`；website CalendarPage | 000009；活動時間／網址／狀態驗證具備，管理新建先 draft；瀏覽器報名狀態未驗收 |
| A02–A04 | 目標 activity_content_id、event.activity_id、entry_config_id | `/api/admin/public-content/activity` 與 `/api/admin/reef-check-data/activities` 仍分開；志工 verify | 000015 為舊設定模型；缺新版 FK、同表單交易與活動歸屬 |
| A05 | reef_check_activity/site | 志工活動碼→允許樣點／日期 | `reef_check_entry_test.go`、志工 app.js 有單項帶入；穩定 calendar activity_id 尚缺，部分 |
| A06 | activity/code hash/audit | 舊代碼 rotate/disable | 舊設定 ID 維持；新活動關聯與名稱／日期同交易同步未實作，部分 |
| A07 | code_hash、scope、expires_at | verify／public survey；admin middleware | `TestPublicActivityVerifyNoCodeLeakAndErrors`、`TestActivityDateSiteAndExpiryScope` 通過；DB 寫入回滾未重跑 |
| A08 | event.publication_status／完整性檢查／audit | 後台新增與調查發布；志工送出 | 000008、000012–16；service/HTTP 回歸具備；DB 發布交易本次未驗證 |
| A09 | activity start_at/end_at | website CalendarPage | 程式核對：時區／預設年份／跨日標示缺漏 |
| B01 | 正式觀測＋統一摘要 | 現為 reports.xlsx/reports.zip | XLSX 模板測試通過；三種指定 CSV 未實作，golden 格式待交付 |
| B02 | 目標 event.activity_id；event 集合 | `/api/admin/reef-check-data/reports.*` | 僅 year/site/method/search；無活動、event 集合、精確日期、深度，未預設過濾 published，部分 |
| B03 | 既有帳號角色 | requireReefDataAdmin；後台報表 | HTTP 權限回歸通過；三種 CSV 邊界待實作。「外部程式不可直呼」的額外技術判準待釐清，不能只靠隱藏連結 |
| B04 | 目標 article.category | content article／文章清單／首頁 | 新測試分類保存及拒絕未知值失敗；缺漏 |
| B05–B06 | 目標 article_site、visible reports/latest_report | 目前 profile.report_url | 無關聯表及完整清單；新測試未知日期失敗；缺漏 |
| B07–B09 | site＋profile/annotation＋報告關聯＋圖表設定 | 現 site、profile、annotation、chart 分開清單 | 三區表單、獨立 dirty state、共用影響聯集未實作 |
| B10 | annotation.site_id/data.date/source_url | content annotation；website SitePanel | 新驗證測試通過；文字清單有實作，ReefCharts 未收到 annotations，垂直註記缺漏 |
| B11 | 現 public_content development；目標 development_case/timeline/media | content development／開發案疊圖 | 幾何／發布／媒體驗證具備；獨立實體表尚缺，且公開快照 fallback 不符；正式資料／DB 未驗收 |
| B12 | audit_logs、public_content_audit、reef_publication_audit、activity_audit | admin mutations／audit | 多數具備；participant delete 前值、活動前後值及全部 UI 影響提示仍有缺口 |
| H01 | settings.data.intro_popup | 現 settings 表單只填 article_limit；website IntroDialog | 新解析保留測試失敗；後端、管理與公開串接皆缺 |
| H02 | 全站單筆 settings | settings GET/POST/PATCH | 000011 唯一索引；整數邊界新測試通過；global site_id 測試失敗，DB singleton/version 本次未驗證 |
| H03 | published settings/article news | website App.tsx content loader | 查詢 published 程式具備；news 分類缺、空集合留示範卡，部分 |
| H04–H05 | sessionStorage；公開無帳號 | SiteHeader／IntroDialog／志工頁 | 現 session 首次及手動開啟、公開 header 無登入具備；文字來源未串接，真實 session/E2E 待驗 |
| C01–C03 | 目標 site_chart_setting/partial unique indexes | 目標六項 effective_settings | 無新 migration/API；目前仍 public_content chart，缺漏 |
| C04 | event＋substrate_summary | series／website ReefCharts | 最新日期／多 event 長條與 mean/SE/n 程式具備；CSV 漏 event 見 C09；DB/E2E 待驗 |
| C05–C06 | taxa/impact/bleaching 摘要 | series／ReefCharts | 5m/最淺預設具備；系列截斷、缺值表格與空／錯誤狀態未全符 |
| C07–C08 | 目標 shared_chart_group/member | 目前 content chart＋site_ids | 單組／單深度舊流程，無群組多深度、全部 groups、本地開關獨立性，缺漏 |
| C09 | 統一摘要＋taxon identity／filters | series/export.csv／圖表下載 | JSON/CSV 已有條件一致性新回歸通過；taxonomy 新測試失敗；前端集合／單位不一致 |
| C10 | 兩個 site 的同條件摘要 | compare/compare.csv／ReefComparison | 現模型測試通過；新 `site_ids=1,01` 測試失敗（HTTP 200），另有 fallback 問題 |
| C11 | published DB 為唯一正式來源 | website App/ReefCharts/ReefComparison | 程式核對確定仍使用快照／示範資料，缺漏 |
| C12 | site coordinates／filters | sites／map/list | 缺座標不造點、空條件與重試控制具備；仍受 C11 fallback 影響，E2E 待驗 |
| C13–C14 | site event_count；development case geometry | website ReefMap、clusterNavigation、developmentMarkers | 群組計數、三次展開、重疊清單、視野歷史、空白縮一級均有實作；模型測試通過，實際互動待驗 |
| T01 | 現 site.cwa_station_id；目標 source/actor/time | `/api/admin/sites/{id}`／樣點基本資料 | UpdateSite 已保留未提交站欄；來源及更新人／時間缺漏 |
| T02 | station active/coordinates/sea-temp capability | AutoLinkSites | 000007；現只看 active/座標，不保證海溫能力；候選距離／狀態契約缺 |
| T03 | station master＋sync status | cwa-marine stations/sync；樣點頁 | 清單與同步 API 存在；樣點頁仍純文字站號，完整搜尋及同步狀態未整合 |
| T04 | event station、transect 溫度；目標 provenance | 後台新增／志工送出 | 可依樣點帶入；新增表單唯讀站欄、無 automatic/manual 及來源時間持久欄位，部分 |
| T05 | 同日 cwa_sea_temperature | cwa-marine sea-temp／新增場次 | SQL 限定 observed_date；UI 不補 0／不找無關站；7 項新增場次既有 Node 測試通過；DB 待驗 |
| T06 | 目標人工來源保護 | 同步／樣點與 event 更新 | AutoLinkSites 不覆蓋已有站，UI 保留手填溫度；尚無持久來源可追溯／驗證完整保護 |
| D01 | integer event.id、text event_id、transect FK | 匯入／survey submit／detail | 000004–5；識別與唯一限制具備，DB RoundTrip 本次未驗證 |
| D02–D03 | canonical substrate＋raw code/version＋160 點 | importer／entry Validate | 000014/17；numeric convention／160位置／複合碼驗證與回歸通過；持久化 DB 待驗 |
| D04–D05 | substrate/bleaching 原始與衍生 views | series／detail／entry | 000014–16；三態與新點驗證具備；歷史總數保護整合測試本次未驗證 |
| D06 | fish_size_mode／taxon(size) | fish_length_mode payload／匯入 mapping | entry及模板回歸通過；不同尺寸資料保留；完整 DB golden 待驗 |
| D07 | belt_summary／aggregates | series／detail／報表 | 有 SQL／單元計算證據；[0,2,NULL,4] DB 統計整合本次未跑，不標 DB 通過 |
| D08–D09 | impact_observation.source_unit／views | series／detail／報表 | 000017 已補逐筆來源，不能沿用舊文件說「完全沒有」；parser／報表／JS 級距回歸通過，DB views 待驗 |
| D10 | value＋record_status upsert | 更新觀測 API | 程式同步更新具備；DB NA↔0 往返本次未驗證 |
| D11 | incidental_sighting | 匯入保存；不併 belt summary | 000017＋replacement importer 具備；本批 CSV 數值解析通過；未新增公開偶見資料端點，DB 排除性待驗 |
| D12 | import provenance／transaction／site master | 匯入工具、replace script | importer/script 回歸通過；本批數值語料另跑通過；本次未重新操作線上匯入 |
| D13 | 統一摘要與 golden | API、公開 CSV、管理模板 | 有模板回歸；不等於三種正式 CSV golden 或完整跨層數值驗收，部分／待資料 |
| D14 | 新活動／報告／chart 遷移與對帳 | 新目標讀寫 API 尚缺 | 無 Schema 9.3–9.5 的遷移，缺漏；不猜舊 ID 或合併同名內容 |
| D15 | 全新／升級 DB、實際 migration 狀態 | isolated DB＋目標環境唯讀檢查 | 未驗證；未取得本次目標 schema，不能以本機 migration 檔替代 |
| V01–V05 | 原始 entry、point flags、notes、missing reason | 志工 app.js／entry Validate | 方法／160點／白化／RKC／其他損害條件與 navigation/entry rules 測試具備；部分 UI 僅來源核對，裝置驗收未跑 |
| V06 | 瀏覽器草稿／回執／debug 隔離 | 志工 submit與 localStorage | 程式存在 DEBUG 不送出、草稿／錯誤保存；真實完整送出流程未跑，跨裝置另待契約 |
| V07–V08 | middleware／scope／URL escaping／CSV | admin/public routes、內容／填報 | 權限、無代碼洩漏、URL／公式及重複拒絕回歸具備；正式權限矩陣與限流門檻待定，不能宣稱完整安全驗收 |
| V09–V10 | responsive UI／載入錯誤狀態 | 全站與志工／後台 | 未做實體裝置、讀屏、Figma、效能驗收；部分錯誤狀態因 fallback 不符 |

## 4. 本次實際測試結果

數量按頂層 Go test 計算，子案例不重複加總。71 項是驗收情境數，與 unit test 數不同。

| 套件／命令 | 結果 | 限制 |
|---|---|---|
| `GOCACHE=/tmp/coast-go-cache go test ./... -skip TestNewGoogleOAuthProviderUsesOIDCDiscovery -json` | 180 通過、7 SKIP、0 失敗 | 6 項 DB 整合缺隔離 DB；1 項 corpus 後續另補跑。OIDC discovery 需本機監聽，本環境限制下明確排除，不能算通過 |
| `REEF_HANDOFF_CSV_DIR=<本批原始 csv 目錄> go test ./cmd/import-reefcheck -run '^TestHandoffNumericCorpus$' -count=1` | 1 通過 | 逐筆解析本批 impact、belt、bleaching、incidental 數值；無 DB。合併前列為 181 個不同頂層回歸測試通過、6 個 DB 未跑 |
| `go test -tags prd_acceptance ./internal/service ./internal/http -run TestPRD20261104 -json` | 3 通過、7 失敗 | 3 通過已含在回歸，不重複加總；此命令目前應以失敗結束 |
| `node --test web/admin/tests/*.test.mjs web/public/reef-check/tests/*.test.cjs web/site/map-clusters.test.mjs` | 16 通過 | 包含既有新增場次／測站／發布核對；web/site 非目前獨立公開網站 |
| 固定版本 website：`node --test --test-skip-pattern='real snapshot' scripts/cluster-navigation.test.mjs scripts/comparison-model.test.mjs scripts/development-markers.test.mjs` | 15 通過 | 2 個依賴完整快照的案例被明確排除；另外 public-proxy 的 3 項因缺 serve-handler 相依未執行；未執行前端完整 build 或瀏覽器 |
| `/opt/homebrew/bin/python3.14 -m unittest discover -s scripts/tests` | 6 通過 | replacement script 的現有單元測試，非線上替換 |

新增測試檔：

- `internal/service/prd_20261104_test.go`：H02 整數上下界／型別；B10 註記日期／來源／站欄。
- `internal/http/public_reef_handlers_test.go`：C09 相同已支援條件、同日多 event、NULL／0 的 JSON 與 CSV 傳輸契約；fake 僅提供資料，不假裝驗證 DB 篩選。
- `internal/service/prd_20261104_acceptance_test.go`、`internal/http/prd_20261104_acceptance_test.go`：以 `prd_acceptance` build tag 獨立執行尚未完成需求，讓舊回歸與目標驗收結果分開記錄。交付時必須執行此套件，不得以預設 `go test` 綠燈跳過。

七個失敗的頂層測試：

| 測試（共同前綴 TestPRD20261104） | 重現結果 |
|---|---|
| H01IntroPopupSurvivesContentParsing | intro_popup 被解析／序列化丟失 |
| B04ArticleCategories | news、observation_report 丟失；未知分類也被接受 |
| B06UnknownReportDate | 未知原文日期被拒絕 |
| H02SettingsAreGlobal | 帶 site_id 的全站設定被接受 |
| C09ExplicitTaxonomyInSeries | taxon_id／taxon_group 不存在於回應型別 |
| C09CSVTaxonomyColumns | CSV 缺 taxon_id／taxon_group |
| C10RejectSameNumericSite | 1 與 01 被當成兩個不同樣點，回 200 |

本機執行紀錄保留於 `.local/prd-audit-20261104/`：`regression-go.jsonl`、`handoff-corpus.jsonl`、`acceptance.jsonl`、`regression-node.tap`、`regression-website.tap`、`regression-python.log`。來源副本在同目錄供重現核對，未將資料快照納入產品。

## 5. 後續實作與驗收順序

1. 先修正式資料 fallback、同日多 event CSV 及篩選／識別契約，避免新 DB 被舊畫面掩蓋或下載漏資料。
2. 補活動、article_site、site_chart_setting、shared_chart_group/member 的遷移與交易 API；附舊 ID 對帳、孤兒資料及不可自動配對清單。
3. 整合單一活動表單及樣點三區；補六項有效圖表設定、共用影響範圍與站別開關；同步公開網站。
4. 補首頁彈窗、文章分類與排序、測站來源／搜尋／單次覆寫、時區與跨日行事曆；逐項讓新增驗收測試轉綠。
5. 收到三種 CSV golden 後驗證正式格式。另在隔離 DB 跑遷移／回滾及原始資料計算，最後才做目標環境 schema、瀏覽器及實體裝置驗收。

測站距離／海域閾值、同步頻率、權限矩陣、跨裝置草稿及冪等契約、效能門檻仍按 PRD 待確認，不擅自補數字或將待交付格式當成已驗收。
