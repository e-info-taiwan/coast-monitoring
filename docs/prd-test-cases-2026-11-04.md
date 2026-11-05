# PRD 0.9.13／Schema 1.16 測試案例

規格基準：2026-11-04；核對完成：2026-11-05（Asia/Taipei）。先依需求建立案例，再執行程式核對；本文件的預期結果不是實作完成聲明。共 71 項驗收案例，並非 71 項皆已自動化；最新結果見 [功能補齊與測試結果](prd-implementation-2026-11-05.md)；[實作前核對](prd-implementation-audit-2026-11-04.md) 保留歷史發現。

## 規格基準與執行範圍

- PRD：[珊瑚觀測網站 PRD](https://github.com/e-info-taiwan/coast-monitoring-website/blob/8503b21ed04c0448767c25ac4a9a63e61c77be83/prd/珊瑚觀測網站%20PRD.md)，0.9.13。
- Schema：[Reef Check Schema](https://github.com/e-info-taiwan/coast-monitoring-website/blob/8503b21ed04c0448767c25ac4a9a63e61c77be83/prd/珊瑚礁調查資料庫%20Schema%20規格（Reef%20Check）.md)，1.16。
- 來源固定於網站 repository commit `8503b21ed04c0448767c25ac4a9a63e61c77be83`，避免核對途中 main 更新造成不同版本混用。
- 下表包含自動化、DB 整合及人工 E2E 案例；執行結果另記於 implementation audit。未跑、被環境阻擋、待資料與失敗不得標成通過。
- 角色 fixture：未登入訪客、僅持活動代碼的志工、非管理員帳號、管理員。資料 fixture：A/B/C 三樣點；A 有 5m/10m，B 僅 10m；同日不同時間兩 event；published/draft/archived 各一組；報告 R 同時關聯 A/B；群組 G 含 A5m/A10m/B10m。
- DB 寫入測試只使用隔離測試庫與交易回滾。不得用剛完成匯入的線上 DB 執行 destructive fixtures。

## A：活動、代碼與填報生命週期

| ID | 規格 | 操作／輸入 | 預期結果 |
|---|---|---|---|
| A01 | A1；3.7 | 建立、發布公開招募且報名中的活動；再測內部、截止、取消、完成 | 只有 published/public/open 且有有效網址的活動提供報名入口 |
| A02 | A2；Schema 9.3 | 同一活動表單不啟用調查填報，不選樣點 | 可以儲存；不建立 entry config、不產碼、不接受該活動觀測 |
| A03 | A2；Schema 9.3 | 同一活動表單啟用填報，指定 A/B 樣點 | 同一交易建立活動及其唯一填報設定、產碼；不用第二頁建立同名活動 |
| A04 | A3；Schema 9.3 | 驗證代碼並送出兩筆不同時間的氣瓶；偽造 payload activity_id | 兩 event 同屬 public_content 的穩定 activity_id；伺服器以代碼重新決定歸屬，不信任偽造 ID |
| A05 | 3.8；Schema 9.3 | 單日單樣點與跨日多樣點活動各驗證一次 | 單一選項自動帶入；多選項供選擇；不猜深度、實際開始時間或記錄者 |
| A06 | 3.9；Schema 9.3 | 已有 event 後重發、停用代碼及修改日期 | 舊碼失效；activity_id 與既有 event 關聯不變；不建立第二筆活動 |
| A07 | 3.8；7.4 | 錯碼、停用碼、過期碼、非允許樣點／日期、重複氣瓶 | 拒絕且不留半套資料；公開回應不含 code_hash／可用代碼；無後台權限 |
| A08 | A4–A5；8.2 | 後台／志工新增 → 不完整發布 → 完整發布 → 封存 | 新增 draft；失敗發布不更動狀態及稽核；公開查詢僅讀 published，封存後消失 |
| A09 | 3.7；8.1 | 台灣時間跨年、跨日、跨月活動；在 UTC 日期不同的時刻開行事曆 | 初始年月採 Asia/Taipei；每月標出實際涵蓋日期，顯示完整起訖時間 |

## B：日常管理與樣點整合

| ID | 規格 | 操作／輸入 | 預期結果 |
|---|---|---|---|
| B01 | B1；3.10 | 提供三種正式 CSV golden 樣本，各匯出單／多場次 | 按正式列粒度、欄位、排序、單位、NA、編碼及檔名比對；XLSX 或公開 CSV 不算完成此項 |
| B02 | B2；3.9 | 依 activity_id、event 集合、日期、樣點、深度、方法匯出 | 條件取交集；預設 published；不同 event 分開，保留時間、NA、n、單位及更新時間 |
| B03 | B1–B2；7.4 | 未登入、志工、非管理員及外部呼叫管理匯出 | 拒絕；管理格式僅允許後台登入流程，不能以公開下載 URL 取代 |
| B04 | B3；Schema 9.4 | 建立 news 與 observation_report 文章；未知分類 | 正式分類會保存並驗證；首頁僅顯示 published news |
| B05 | B3/C3；Schema 9.4 | R 關聯 A/B；在 A 隱藏／解除，然後封存、重發 R | A 的設定不改 B 或 R 內容；全篇封存影響所有站；重發仍保留每站原有開關 |
| B06 | C3；Schema 9.4 | 多篇報告含未知日期、同日期與更新時間 | 回傳完整可見清單；latest_report 依 date DESC NULLS LAST、updated_at DESC、id DESC；不捏造日期 |
| B07 | 3.9；Schema 9.7 | 從調查樣點清單開 A | 同一詳情有基本資料、公開資訊與報告、圖表顯示三區；不另建 profile/chart 清單 |
| B08 | 3.9；Schema 9.7 | 三區皆有未儲存修改，只儲存基本資料，再切換區塊／樣點 | 只寫指定區；其他修改保留；離開樣點提示；不連帶發布 |
| B09 | 3.9；Schema 9.4/9.7 | 修改共用 R、群組 G，將 B 移除並新增 C | 提示共用內容影響；群組提示變更前後樣點聯集 A/B/C，不漏掉被移除站 |
| B10 | 3.3；Schema 9.4 | A 改名；新增有日期及來源的 annotation，測缺日期／來源 | 用 site_id 維持關聯；無效註記拒絕；有效註記出現在文字及圖表日期位置 |
| B11 | B4；3.5/6.1 | 建立 Point/Polygon/MultiPolygon 開發案、時間軸及媒體 | 保留完整幾何及來源；公開只含已發布時間軸及有效、授權、已發布媒體；development_case/timeline/media 獨立存放，與 Reef event 無 FK |
| B12 | 3.9；7.5 | 新增、修改、封存、刪除觀測／主檔／內容 | 留操作者、時間、動作、前後值；刪除二次確認及相依範圍提示，優先封存 |

## H：首頁設定與帳號邊界（B5）

| ID | 規格 | 操作／輸入 | 預期結果 |
|---|---|---|---|
| H01 | B5；Schema 9.9 | settings 寫入 intro_popup 的 kicker/title/body/steps/button_label，讀回 | 所有指定欄位及 body 換行保留；服務解析不丟失新版 intro_popup 欄位；純文字顯示 |
| H02 | B5；Schema 9.9 | article_limit 分別送 1、50、0、51、小數、字串；修改彈窗；傳入 site_id | 只接受 1–50 整數；改彈窗不清空原筆數或其他設定；使用同一 settings ID；全站設定 site_id 必須為 NULL |
| H03 | B5；8.1 | settings 草稿／發布；7 篇 news 設 limit=10 | 公開只讀 published；顯示 7 篇，不填示範卡片、不混入 observation_report |
| H04 | 3.1；4.1 | 同 session 關閉再進首頁、由 Header 重開；新 session | 同 session 不自動重開，可手動開；新 session 顯示最新已發布文字 |
| H05 | 4.1；8.1 | 未登入／後台已登入時查看所有公開頁及填報頁 | 無登入、登出、帳號或會員選單；公開 UI 不因後台 session 改變 |

## C：圖表、比較與正式資料來源

| ID | 規格 | 操作／輸入 | 預期結果 |
|---|---|---|---|
| C01 | C2；Schema 9.5 | 新樣點無任何 chart 設定列，查詢有效設定 | 回傳全部六項 chart_key 及 source=default；不需先建六筆 DB 設定 |
| C02 | C2；Schema 9.5 | 只覆寫 A 的 taxa_density.is_visible=false，再還原 | 逐欄繼承，其餘五圖不變；還原封存差異並回到預設；不改觀測值 |
| C03 | Schema 9.5 | 已發布設定後另存 draft，再發布；並發新增同 key | draft 不覆蓋公開版；交易切版；每 site/key 至多一 published、一 draft |
| C04 | 3.3 A 模式 | 最新底質日期有不同深度與同深度不同時間 event | 初始選最新有底質日期；分組長條含全部 event，呈現 mean±SE、n |
| C05 | 3.3 B 模式 | A 有 5m；B 只有 10m；超過 8 個適用物種 | 初始優先 5m，無則最淺；全部年份；可選其餘物種，不能永久截取前 6/8 個 |
| C06 | 3.3 附加圖 | rare/impact/bleaching 各有資料、空集合與載入錯誤 | 有資料顯示、成功空集合隱藏且保留啟用設定、錯誤提示重試；rare 不混入密度圖 |
| C07 | 3.3 C 模式；Schema 9.5 | 無群組、G=A5/A10/B10、多組 G/H | 無群組畫本站；群組保留每個 site+depth 系列，回傳全部已發布群組；預設不畫活珊瑚誤差棒 |
| C08 | 3.3；Schema 9.5 | A 關閉共用圖，另改 G 的成員／深度 | 本站關閉不刪 G、不影響 B；共用修改影響全部成員並提示 |
| C09 | 3.3；6.7 | API 與 CSV 查同日多 event、taxon IDs／group、size class、unit、深度 | 圖、表、CSV 完全相同集合；明示 taxon_id/taxon_group/size_class/fish_size_mode，不靠名稱推斷 |
| C10 | 3.4 | 選 0/1/2/3 個樣點及重複樣點（含 1,01）；一站無相同條件資料 | 僅兩個不同站可比較；以解析後的 ID 判重；共用尺度／單位／條件，缺資料不補 0；CSV 保留同樣範圍 |
| C11 | 1.4；8.1 | production 缺 API 設定、500、成功 []；然後發布新資料重載 | 三種狀態分開；絕不回退 fixture/snapshot/demo；重載顯示新正式資料 |
| C12 | 3.2 | 樣點無座標、篩選無結果、切圖層及資料載入失敗 | 不放任意預設點；提供清單／空白／重試，保留篩選條件 |
| C13 | 3.2；4.3 | 同群組最多三次展開；觀測／開發案完全重疊 | 觀測以 event 數、開發案以案件數；三次內個別化，重疊用清單選取，不更動真座標 |
| C14 | 3.2；4.3 | 點泡泡、資料圖徵、地圖空白及返回；沒有歷史視野 | 泡泡存視野；圖徵只開資料；空白退一層；返回依歷史，無紀錄停用 |

## T：海溫測站與來源

| ID | 規格 | 操作／輸入 | 預期結果 |
|---|---|---|---|
| T01 | 3.11；Schema 9.8 | 樣點基本資料選站、改名但不傳測站欄位、舊資料來源不明 | 儲存 manual 與更新人／時間；其他欄位 PATCH 不清站；舊來源 unknown 不猜 auto |
| T02 | 3.11；Schema 9.8 | 無對應、有座標；候選含無海溫站；缺座標 | 只從有效且可提供海溫的站建議，回 ID/名稱/距離/可用性；缺條件為 NULL＋原因，不杜撰距離門檻 |
| T03 | 3.11 | 樣點頁搜尋完整清單、同步清單；模擬失敗與清單缺站 | 可選非鄰近站；成功重載清單；顯示最後成功／錯誤，缺站不造假，保留人工對應 |
| T04 | 3.11；Schema 9.8 | 新 event 帶樣點預設，單次改站／手填溫度，再修改 site 預設 | 保存實際站、automatic/manual、來源觀測時間及取得時間；event/site 互不回寫；人工未知時間留 NULL |
| T05 | 3.11 | 未設定、清單失敗、所選日期無海溫、未來日期 | 分別顯示狀態；溫度為 NULL，不用 0 或其他日期替代；仍可存草稿 |
| T06 | 3.11 | 手動值之後執行排程及改樣點座標 | 不覆蓋 manual 站／溫度；重新配對由管理者確認；多樣點活動分別採各站設定 |

## D：觀測、計算、匯入及資料保存

| ID | 規格 | 操作／輸入 | 預期結果 |
|---|---|---|---|
| D01 | Schema 4/9.2 | 含空白／撇號／句點樣點名，同日不同時間／深度 | 正確 event_id；整數內鍵與文字自然鍵不混接；每 event 每方法最多一 transect |
| D02 | 6.2；Schema 3.1 | entry0 的 0/a/b/c、legacy10 的 10、未明示數字來源 | 分別 OT/HC-a/b/c/OT；未明示格式拒絕；保留 raw_input_code 與來源版本 |
| D03 | 6.2；Schema 4.6 | 160 個固定位置、重複／漏點、SI 泥下組合 | 正規化後一位置一筆 surface；SI(SI) 仍 SI；錯誤不靜默略過 |
| D04 | 6.2 | HC/HC-a/b/c/SC/NA 組合與全 NA | subtype 歸 HC 不加分母；NA 排除；活珊瑚 HC+SC；全 NA 為 NULL |
| D05 | 6.2；Schema 8 | 新珊瑚點 true/false/null、非珊瑚點、歷史分段數量 | 新適用點缺布林拒絕；非珊瑚及歷史為 NULL；逐格衍生不覆寫歷史來源總數 |
| D06 | 6.3；Schema 9.2 | fish_recording_mode → fish_length_mode → fish_size_mode；同名多尺寸 | 明確轉接 split/combined；不同 taxon 不混併；combined 不作尺寸分級替代 |
| D07 | 6.3；Schema 5 | [0,2,NULL,4]、單有效段、全 NULL、完整 aggregate／缺成員 | mean=2、n=3、SD=2、SE=2/√3；n<2 的 SD/SE=NULL；合計不接受手填／不把缺成員補 0 |
| D08 | 6.4；Schema 4.9 | count 0/1/2/4/5；level 3；percent 42.5；NULL | 分級 0/1/2/2/3；歷史 3 直通；百分比及 NULL 不產生分級；原值不變 |
| D09 | 6.4/6.7 | 同一方法的 count/level/percent 摘要與報表 | 不混單位計算或繪圖；每筆輸出 unit/unit_label/n/狀態/更新時間，密度欄只用於密度 |
| D10 | 6.5 | 同自然鍵 NA→0→正數→NA | 原值及 record_status 同步更新；NA 為 SQL NULL；摘要立即反映變更 |
| D11 | Schema 4.10 | 有原文但無段次／物種／數量的 incidental_sighting | 保留原文、來源與 NULL，不猜 segment=1，不併入 belt_summary、不重複計數 |
| D12 | 3.10；Schema 7 | confirmed-only CSV、audit 隔離、重跑及錯誤中斷 | 自然鍵無重複；錯誤整批回滾；不以空白覆寫人工座標／行政區；保存來源雜湊、快照、結果 |
| D13 | 6；8.2 | golden 歷史 Excel 與 API／公開 CSV／管理報告 | 同一正式計算定義；原資料修改後一致更新；合成 fixture 不代替客戶 golden |
| D14 | Schema 9.6 | 舊活動／chart／profile.report_url 遷移，含對不上資料 | 不按同名猜活動或把 config ID 當 content ID；保留未知及對帳清單；保留隱藏值、深度、來源與稽核 |
| D15 | 8.4；Schema 9.6 | 隔離 DB 全新遷移、既有資料升級、讀實際 schema/migration 清單 | FK、唯一限制與發布契約符合；程式通過不等於 staging／production 通過 |

## V：志工輸入與安全／裝置驗收

| ID | 規格 | 操作／輸入 | 預期結果 |
|---|---|---|---|
| V01 | 3.8 | 不選方法及選 line/fish/invert 組合 | 至少一方法；共用潛水資料只填一次；invert 自動含 rare/impact |
| V02 | 3.8 | 輸入第五格、下一組按鈕／Enter、點手板返回、切泥下 | 第五格後停留；只明確操作才換組；左右對照保留目前層與位置 |
| V03 | 3.8；6.2 | 修改已核對底質／白化、逐格按鈕操作 | 核對聲明失效；大型圓點有 aria-pressed 與文字；新分段總數唯讀自動計算 |
| V04 | 3.8；6.4 | RKC 低於／等於／高於 10%；其他損害 >0；NA | 門檻含 10%；原因、other_damage_note、comments 分欄；缺條件說明拒絕 |
| V05 | 3.8 | fish/invert/rare 批次補 0，已有 NA／數字；impact 表 | 確認後只補空白；不覆寫 NA/數字；impact 無補 0 功能 |
| V06 | 3.8；5.6 | 重整／失敗／重試、debug=1 空白跳關及嘗試送出 | 保留草稿與備份；真實回執；debug 草稿隔離且不得送 DB；正式不跳驗證 |
| V07 | 2；7.4 | 訪客及志工嘗試後台讀寫，公開 API 查草稿／個資／snapshot | 伺服器拒絕未授權寫入，不洩漏姓名、Email、證號、代碼及來源快照 |
| V08 | 7.4 | 惡意 URL、HTML、CSV 公式、重複按送出 | 安全處理／拒絕；不注入；送出中鎖操作並有伺服器防濫用措施 |
| V09 | 7.1/7.2；8.3 | 實體平板橫直向、手機、鍵盤、讀屏與 Figma 對照 | 主要控制 ≥44px，表格局部捲動、焦點正確、非僅顏色辨識；記錄裝置與截圖 |
| V10 | 5.8；7.3 | 404/403/500、慢 API、地圖資料量 | 明確回復操作；公開無登入提示；不一次載所有原始觀測；效能數值待核定 |

## 待交付／待確認，不能自行補成測試規則

- B01 的三種指定 CSV 已是需求，但 golden 範例／欄位／命名尚待額外交付；現有 Excel 模板不能替代。
- 正式權限矩陣、代碼重用／期限／錯誤次數上限、跨裝置草稿與冪等契約，依 PRD §9 尚待確認。
- 深淺邊界、測站適用距離／海域、同步頻率與保留期、天氣等必填性、疾病新填報契約不得自訂。
- 團隊／體檢介紹內容及來源尚未交付；開發案待查證欄位不由摘要推測。正式效能門檻及法務／授權內容另驗收。

## 自動化命令

既有回歸與新增相容測試：`go test ./...`、`node --test web/admin/tests/*.test.mjs web/site/map-clusters.test.mjs`、`node --test web/public/reef-check/tests/*.cjs`。

新增需求的 7 項失敗案例已修復並移除 opt-in build tag，直接隨 `go test ./...` 執行。完整隔離 SQL／DOM／遷移測試可用 `scripts/test-prd.sh` 重跑。

DB 整合測試需明確設定隔離測試庫 `REEF_DATA_TEST_DATABASE_URL`；環境未提供時所顯示的 SKIP 一律記為未驗證。公開網站使用 `coast-monitoring-website` 的來源版本另行核對，不以本 repository 的 `web/site` 舊版前台測試代替。
