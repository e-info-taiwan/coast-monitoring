# 志工填報修正與白化核對

## 舊資料證據與本次決策

讀取 2026-09-29 confirmed-only 匯入包的 2,620 筆分段白化紀錄（655 個來源檔），找出 83 筆非零紀錄、共 44 份來源。逐份讀取原 XLS／XLSX 的 DATA、說明、比例公式及表面測點；44 份白化數量均未超過對應 HC／SC 點數。

例如龍洞北 5 m 的 2022-08-18 原檔，DATA!X37 公式以 X35 的 HC 白化數量除以第一子樣區底質 HC 的 COUNTIF。美人洞 2021-10-20 淺場原檔 DATA!X35 為 1、DATA!B42 的 HC 點數為 6，DATA!X37 為 16.6667%；DATA 第 12 列另註記白化以顏色區隔。以上支持本次新填報以「白化測點」定義自動加總。

部分舊檔 SC 白化比例公式在第三、第四子樣區誤用 HC 作為分母。因此只用原始數量與測點核對定義，不照抄舊比例公式；新系統按 HC／SC 各自的測點重算，HC-a／b／c 明確合併到 HC。

完整機器可讀證據：[bleaching_evidence.json](../../outputs/volunteer_entry_2026-10-14/bleaching_evidence.json)。這是定義核對，不是全量數值再轉檔；原始資料與既有匯入包沒有修改。

## 現在的輸入流程

活動代碼驗證 → 選擇探查 → 共用資訊及模式 → 每五格輸入原始數字 → 整張手板核對 → 表格點選白化位置 → 自動加總各區 HC／SC → 補充 RKC／其他損害說明 → 最後原始數字／轉換代碼核對 → 儲存待審核。

- 每五格切換由使用者決定；原始數字不立即轉成英文。
- 左側呈現當前單欄 20 點，右側五格輸入；數字核對、白化與最後核對均依原手板表格排列。
- 新填報用 `substrate_point.is_bleached` 保存每格，`substrate_bleaching_point_summary` 自動加總。歷史 `substrate_bleaching` 原始數量保留，沒有逐格資料的舊紀錄維持 NULL；不重複輸入新總數。
- RKC 以非 NA 有效點為分母，達 10%（含）需填原因；與一般 comments 分欄。
- 其他珊瑚損害任一區大於 0 時需獨立說明。
- 魚類分體長／不分體長切換只輸出選中模式，舊模式值保留於草稿；硬珊瑚型態先選擇是否分類。
- 罕見生物其他列保留名稱、四段數量、選填尺寸與備註。
- 天氣、氣溫及經緯度暫以選填保留；座標需成對。

## 活動代碼與資料庫契約

管理端新增 `/reef-activities.html`，提供建立、停用、重新產生。代碼為 128-bit 隨機值，資料庫僅存 SHA-256；新代碼只回傳一次，操作寫入專用 audit。可限制樣點、日期，失效時間選填。志工端以 POST 驗證，正式送出再次由伺服器檢查範圍；本機 debug 提供測試活動，不會呼叫儲存 API。

新送出格式 `reefcheck-web-v4`，新增 `activity_code`、`entry_metadata`、`rkc_bleaching_note`、`other_damage_note`、`bleaching_mode`、transect `coral_morphology`，及 rare observation `observed_size`／`notes`。後端保存原始欄位、NA 狀態、draft metadata；同一氣瓶的重複新送出不會覆寫既有來源。

本機新增 migration `000009_volunteer_entry_details.sql`，依賴先前 `000008`。此 checkout 與 PRD 引述已驗證的其他後端版本不是同一個部署狀態；本次沒有對既有資料庫套 migration 或部署。正式活動／資料庫往返還需在實際 PostgreSQL 驗證。

## 驗證

前端測試涵蓋五格位置與整張 160 點對應、debug 空白跳關、兩種模式值隔離、各區 HC／SC 自動計數、10% 邊界與 NA 分母、條件式備註、rare metadata、活動樣點範圍。Go 測試涵蓋完整 160 點、段次座標、逐格白化完整性、記錄者與模式、日期／樣點／有效期；另提供資料庫回歸，確認既有手填總數不被逐格計數覆寫。沒有 PostgreSQL 測試連線時，資料庫回歸會明確略過，不能視為資料庫驗證成功。
