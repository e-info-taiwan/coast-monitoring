# Reef Check 報告輸出

後台報告改為填入提供的原始 Excel 範本，保留原有工作表、合併儲存格、欄寬、列高、字型、底色、邊框、數字格式、圖表錨點與列印設定。主要下載不再使用先前的七張正規化資料表版面。

## 後台操作

場次詳情提供四種下載：

- **下載報告套組 ZIP**：依已記錄方法產生 Line、Belt 與 Summary 三類 Excel。
- **底質 Excel**：原始 LineTransect 範本，僅有底質記錄時顯示。
- **魚類／無脊椎 Excel**：原始 BeltTransect 範本，僅有相關記錄時顯示。
- **Summary**：原始統計製圖範本，包含該場次的資料。

列表的 **下載篩選報告 ZIP** 依已套用的年份、樣點、方法、搜尋字串選取全部符合場次，跨分頁；選中場次保留全部方法。一次最多 100 場，超過會要求縮小範圍，不截斷。編輯期間停用下載，輸出只使用已儲存資料。

報告僅供管理員下載，下載不變更資料或發布狀態。包含目前篩選中的草稿、已發布與封存場次。

## 原始範本

| 類型 | 來源檔案 | 保留的主要工作表 |
| --- | --- | --- |
| 底質 | `20260411_合界春_底質_M_LineTransect.xlsx` | DATA、GRAPHS、GRAPHS_SI、EXPORT、使用指引、siteName |
| 魚類／無脊椎 | `20260411_合界春_魚類無脊椎_M_Belt Transect.xlsx` | DATA、GRAPHS、EXPORT、INSTRUCTIONS、FIELD SHEET FISH、FIELD SHEET INVERTS、siteName |
| 製圖與歷年統計 | `合界_202604_製圖＆歷年Summary Data charts_魚類尺寸示範.xlsx` | 本次日期頁、Summary (line)、Summary (fish)、Summary (fish size)、Summary (Invertebrates)、Impacts |

程式內嵌的是清除範例輸入值後的範本；下載不依賴 Drive 或外部試算表服務。原始 OOXML 樣式與圖表結構保留，不使用 openpyxl 重新儲存範本。準備腳本 `scripts/prepare-report-templates.py` 只在更新範本時使用 openpyxl 翻譯共享公式。

依格式說明不輸出泥下模式：Line 保留表層複合代碼與珊瑚型態圖，移除獨立泥下底質圖及其統計區。Summary 範本中舊的兩個隱藏除錯頁及 2020／2021 手工計數頁不輸出。

## 統計與資料完整性

- 底質 DATA 保留四段 × 40 點的原始排列。輸入碼含 0／1～9、a／b／c、91～98、9a／9b／9c。底層 `down` 記錄不輸出。
- 空白／NA 不補為零。覆蓋率使用各段有效點數作分母；Mean 是有效段的平均覆蓋率。SD 使用 n−1，SE=SD/√n；n<2 時 SD／SE 留白。公式亦使用有效段數，避免開啟 Excel 重算後回到固定四段分母。
- 魚類尺寸資料填入原範本各列，再推導整合數字、石斑魚與硨磲貝總數。若資料未分體長，標示未分體長，不把總數推定成某一尺寸；EXPORT 附表保留原始尺寸欄位。
- 長刺海膽以每段 Diadema + Echinothrix 推導，再重新計算 SD／SE；任一成員缺值時該段合計留白。原本分開的海膽欄仍保留。
- 珊瑚損害與垃圾同時保留原始件數及 0～3 衍生級別；原範本的環境衝擊圖使用級別。疾病百分比不推定為逐點疾病數。逐點白化與舊手填總數依現有來源使用，不相加。
- EXPORT 原有欄位位置保留，另外在右側加上 event_id 與 TaiBIF_event_id；魚類／無脊椎每子樣區使用 `_S1`～`_S4`，底質沿用氣瓶 ID。原始底質拆欄、記錄狀態、自訂物種、罕見生物尺寸與備註附在 EXPORT 下方。
- 圖表的範例數值快取全部清除並依新資料重建，標題換成實際樣點、日期、深度。下載後不需先重算才能看到正確圖表。
- 使用者文字寫為文字，不能成為公式。帳號 email、活動代碼不匯出。

## 歷年與跨深度 Summary

Summary 僅使用**這次選取的場次**，不混入範例歷史資料。要下載歷年比較，請在列表選擇樣點與需要的年份後下載篩選套組。

每個樣點依深度排序，沿用原範本每份兩個深度的配置。超過兩個深度時，ZIP 自動分成多份 Summary，不截斷。最新日期頁填入最新日期各深度的資料，該日缺少某深度時留白；不拿其他日期冒充同日資料。歷年頁包含所有選取日期，遇到同日多支氣瓶時保留時間／次序，不相加。圖表來源及誤差棒範圍跟隨實際資料欄數。

## API

```
GET /api/admin/reef-check-data/events/{id}/report.zip
GET /api/admin/reef-check-data/events/{id}/report.xlsx?kind=line
GET /api/admin/reef-check-data/events/{id}/report.xlsx?kind=belt
GET /api/admin/reef-check-data/events/{id}/report.xlsx?kind=summary
GET /api/admin/reef-check-data/reports.zip?year=2026&site=2&method=line&search=合界
```

舊 `.xlsx` 路徑保留，沒有 kind 時預設 Summary；多樣點或超過兩個深度需改用 ZIP。兩者沿用 admin session 權限，回傳 attachment、`Cache-Control: no-store`。無符合場次為 404，無效參數／沒有對應方法／超過數量上限為 400；不回傳部分工作簿。

## 驗證

Go 測試比較範本的 styles.xml、欄寬、合併儲存格、列印設定；檢查範例個資清除、圖表參照、缺值與計算公式、子樣區對應、深度分組、管理員權限及下載類型。輸出亦用獨立的 openpyxl 讀取驗證工作表與圖表物件；不以它重新儲存檔案。

```sh
GOCACHE=/tmp/coast-report-go-cache go test ./internal/report ./internal/http -run 'Test(ReferenceTemplate|Template|SiltMorphology|ReefReport|ReefTemplate)'
node --check web/admin/reef-data.js
```

執行環境的 OAuth discovery 測試因無法監聽本機連接埠需略過；原本完整 CI 仍執行所有測試。尚未以 Excel／LibreOffice 做人工視覺確認。
