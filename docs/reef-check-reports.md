# Reef Check 報告輸出

管理後台「Reef Check 觀測資料」提供：

- 場次詳情的「下載 Excel 報告」。編輯期間停用，輸出已儲存資料。
- 列表的「下載篩選場次 Excel」：依已套用的年份、樣點、方法、搜尋字串選取全部符合場次，跨分頁；每個選中場次保留全部方法。一次最多 100 場，超過時要求縮小範圍，不會截斷。

報告只供管理員下載，包含草稿、已發布、封存場次，並顯示發布狀態。下載不變更觀測、發布狀態或原始 event ID。

## Excel 結構

| 工作表 | 內容 |
| --- | --- |
| 說明 | 計算規則、缺值解釋、有效底質點數不足 40 的警示 |
| Site Form | 每場次／方法的已存地點、日期、深度、水溫、能見度、記錄者、隊長、科導、天氣、備註 |
| 底質逐點 | surface 原始代碼及 Base_Substrate / Morphology / Under_Substrate / Health_Status |
| 魚類與無脊椎 | 子樣區、物種、尺寸、數量、記錄狀態、罕見生物備註、推導總數標記 |
| 環境影響 | 原始數值、值類型、衍生級別、記錄狀態 |
| 白化 | 手填白化記錄與逐點推導結果分開標示，不相加 |
| Summary | 每場次指標的四段數值、Total、Mean、SD、SE、n、底質 Coverage；右側是帶 SE 誤差棒的圖表 |

資料與樣式以「海洋網站-數據格式輸出說明」、20260411 合界 Line/Belt 及歷年 Summary 檔為依據，重新整理為可篩選的資料表，**不是既有 Excel 範本的逐格複製**。Site Form 只輸出系統已有欄位，不重現尚未提供的完整 Site Form 範本與底色預設規則。

## 格式與計算

- `event_id` 保留資料庫氣瓶 ID。魚類與無脊椎另加 `TaiBIF_event_id = event_id + _S1` 至 `_S4`。這是子樣區識別欄位，尚非完整 TaiBIF / Darwin Core 交換封裝。
- 不輸出 `down` 底質層。泥沙複合代碼仍由 surface 記錄拆解，例如 `SI(HC-a)` → Base_Substrate=`SI`、Morphology=`a`、Under_Substrate=`HC`；總部代碼為 `SI`。
- Health_Status：逐點 true → Bleached，false → Normal，未記錄留白；不以環境影響疾病百分比推定逐點 Disease。
- NA 與缺值不補成零。覆蓋率使用各段有效點作分母，Mean 是有效段覆蓋率平均，Coverage 是全場次有效點加權覆蓋率，Total 是點數。
- SD 使用 n−1，SE=SD/√n；n<2 時 SD/SE 留白。有效點數不足 40 時在說明中警示。
- 指定的聚合指標由有效物種目錄的成員明細推導，不採舊手填 aggregate 數值；缺少任一成員時該段總數留白。單一尺寸的原始記錄仍保留。
- 長刺海膽按各段 Diadema + Echinothrix 後重新計算統計，不相加兩者 SE；任一缺值則該段留白。
- 環境影響依原始個數、級別、百分比分開作圖，不共用不同單位的軸。Summary 跨場次保留日期及深度，可用 Excel 篩選比較；尚未產生範本中的跨深度比較或歷年折線圖。
- 使用原生數值儲存；文字以 inline string 寫入，避免使用者輸入被當作公式。工作表凍結標題列、提供篩選與列印方向；帳號 email 與活動代碼不匯出。

## API

```
GET /api/admin/reef-check-data/events/{id}/report.xlsx
GET /api/admin/reef-check-data/reports.xlsx?year=2026&site=2&method=line&search=合界
```

兩者沿用 admin session 權限，回傳 `application/vnd.openxmlformats-officedocument.spreadsheetml.sheet`、attachment 及 `Cache-Control: no-store`。無符合場次回傳 404，無效篩選或超過 100 場回傳 400。資料載入或工作簿建立失敗時回傳一般 JSON 錯誤，不傳送部分工作簿。

## 驗證

報告單元測試檢查 XML/ZIP、合界範例硬珊瑚覆蓋率 21.875%、長刺海膽分段加總、缺值、aggregate 排除及 SE chart，HTTP 測試檢查管理員權限、篩選、錯誤與數量上限。

```sh
GOCACHE=/tmp/coast-report-go-cache go test ./internal/report ./internal/http -run 'Test(ReefReport|ReefWorkbook|SubstrateMapping|DerivedAggregate|XLSX)'
node --check web/admin/app.js
node --check web/admin/reef-data.js
```

此執行環境無 Excel / LibreOffice，尚未以實際試算表軟體驗證開啟與圖表顯示。PostgreSQL opt-in integration tests 需設定測試資料庫；本次沒有進行資料庫寫入或部署。
