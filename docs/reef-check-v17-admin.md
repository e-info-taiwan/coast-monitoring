# Reef Check v1.7 管理介面與本機驗證

## 資料來源與相容性

2026-09-11 的 confirmed-only 匯入包使用單數表 `site → survey → event → transect`。
舊介面使用 UUID 表 `sites → reef_check_surveys`，所以匯入完成後舊列表仍可能顯示 0。
新版管理員預設開啟「Reef Check 觀測資料」，直接查詢 v1.7 表。
Migration `000006` 移除舊 Reef Check 表，並在 `diver` 與 `transect_participant` 加入 `user_id` 關聯。

目前本機基準套用 migrations `000001`–`000006`。
程式啟動會依 `schema_migrations` 套用尚未執行的 SQL。

## 管理功能

- 按年份、樣點、調查方法或文字篩選場次；每頁 30 筆。
- 顯示出動日期區間、場次時間／深度、樣點座標及各方法明細。
- 逐穿越線編輯開始時間、水溫、能見度、備註及現有觀測值。
- Line 保留每個點的原始位置、layer 與代碼；支援 `NA`、`OT`、`SI(...)`。
- 魚類／無脊椎／罕見生物按 taxon ID 與體長級距呈現四段數量。
- 環境影響保留原始件數；`has_raw_count` 使用 `min(raw_value, 3)` 衍生等級。
- percent 類型使用 0–100，支援小數。
- 參與者按每條穿越線顯示姓名、角色與 Reef Check 證號；缺乏關聯時明確標示。
- 缺少的觀測方法／段落不補成零；座標與數值缺值保持缺值。
- 更新與 audit 在同一 PostgreSQL transaction 中執行，失敗一起 rollback。
- 以穿越線及其明細的 SHA-256 指紋拒絕過期編輯；更新前鎖定 parent transect。

以下觀測編輯 API 限於現有穿越線與觀測列；樣點、人員與字典主檔另由管理介面維護。

## 衍生統計

統計由已讀取的原始值即時計算，儲存後以 API 回傳值重算，不建立衍生資料表。
底質覆蓋率以同一 layer 的非 NA 點為分母，複合底質維持獨立類別。
平均與樣本標準差使用有觀測的段落，少於兩段時 SD 為不可計算。
白化率使用表層 HC／SC 各自的實際點數；沒有該類底質時顯示不可計算。
跨年圖表、aggregate_of 跨體長總數及衍生表持久化仍待後續報表工作。

## Admin API

所有路由要求 active admin session 和 CSRF header，沒有新增 public/volunteer 資料權限。
JSON 使用 v1.7 的 snake_case；數字 ID 是資料表 integer/bigserial ID。

- `GET /api/admin/reef-check-data/events`：輕量場次列表，包含樣點和 methods。
- `GET /api/admin/reef-check-data/codes`：底質代碼主檔。
- `GET /api/admin/reef-check-data/events/{id}`：場次及其所有穿越線、觀測、人員。
- `PATCH /api/admin/reef-check-data/transects/{id}`：版本化的逐列修正。

```json
{
  "version": "value returned by the detail API",
  "changes": [
    {"kind": "point", "id": 123, "code": "SI(HC)"},
    {"kind": "bleaching", "id": 456, "hc": 2, "sc": 0}
  ]
}
```

`kind` 可為 `point`、`bleaching`、`belt`、`impact`；後兩者以 `value` 傳送數值。
省略的觀測列保持原值；不接受新增、刪除或變更自然鍵。每個 ID 必須屬於 URL 指定的穿越線。
可選 `metadata` 是完整六欄快照：`start_time`、`water_temp_c`、`visibility_min_m`、`visibility_max_m`、`comments`、`rkc_bleaching_note`；清空值傳 null。省略整個 metadata 保持原值。
回傳 409 表示資料版本已改變，客戶端保留輸入供核對並提示重新載入。
既有 audit_logs.target_id 為 UUID，新版 transect 使用固定 namespace 的 SHA-1 UUID；實際整數 ID 與原始前後內容保存在 audit snapshots，target_table 為 `transect`。

## 驗證結果（2026-09-11）

日常本機唯一基準 DB：`127.0.0.1:5432/coast_v17`，與 GCP/dev 分離。
使用 Homebrew PostgreSQL 的持久儲存，保留完整匯入資料及 schema。
原 `55439` 驗證資料庫的 32 張 public 表已逐表比對筆數、完整列內容雜湊及 schema 後搬移，再套用 `000006`。
本機設定統一使用 gitignored `.env`；原 `.env.local.v17` 已移除。

```bash
set -a
source .env
set +a
go run ./cmd/migrate
# 需要開啟服務時才執行：
go run ./cmd/server
```

未來 schema 調整新增下一個編號的 migration，對同一個 `coast_v17` 套用並驗證；不要修改已套用的 SQL 或另外手改 schema。
舊本機 `coast_monitoring` 與原 `55439` 的驗證副本已退役；本機 Web server 保持停止。
已確認目前 schema 與空資料庫執行全部 `000001`–`000006` migrations 的結果完全一致；
升級後亦逐表核對保留欄位與完整資料內容。Go 全套（含 PostgreSQL 整合測試）及前端統計測試通過。
搬移前完整備份保存在 gitignored `.local/backups/coast_v17-before-consolidation.dump`；
舊 DB 的封存備份為 `.local/backups/coast_monitoring-retired.dump`。
GCP 的資料庫名稱與連線設定不受本機整理影響。

| 資料 | 資料包／本機／明細 API repository 核對 |
| --- | ---: |
| survey | 392 |
| event | 726 |
| transect | 1,985 |
| substrate_point | 104,800 |
| substrate_bleaching | 2,620 |
| belt_observation | 74,780 |
| impact_observation | 22,956 |
| transect_participant | 279 |

全部 726 個場次的明細查詢已逐一驗證；有觀測的樣點共 50 個。
匯入包的 61 列樣點 lookup 不等於有觀測的樣點數；migration 舊種子亦可能保留無觀測的別名，未擅自合併或刪除。

```bash
go test ./...
node --test web/admin/tests/*.test.mjs
REEF_DATA_TEST_DATABASE_URL="$DATABASE_URL" go test ./internal/repository -run TestReefDataPostgresRoundTrip -v
REEF_DATA_TEST_DATABASE_URL="$DATABASE_URL" REEF_DATA_CONFIRMED_PACKAGE=1 go test ./internal/repository -run TestReefDataConfirmedPackage -v
```

整合測試會 rollback 測試觀測，不建立管理員帳號。
瀏覽器已檢查搜尋、同日不同時間／深度、缺漏方法、NA 無法計算、魚類體長、編輯失敗保留輸入與取消，以及 390px 手機版無整頁橫向溢出。
瀏覽器 UI 驗證使用同一 repository 輸出的本機資料快照；預覽停用寫入，並非登入後 API 儲存流程的端到端驗證。

Cloud Build trigger `coast-monitoring-dev` 監聽 `main`，採用 `cloudbuild.yaml`。
本次加入前端語法／統計測試步驟。推送 main 後由既有 trigger 執行測試、建置與 dev 部署；實際部署結果以 Cloud Build 與 Cloud Run revision 為準。
