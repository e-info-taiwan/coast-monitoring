# 中央氣象署海象與海溫資料整合及排程（CWA Marine Temperature & Cronjob）

本文件說明如何整合交通部中央氣象署（CWA）海象觀測資料、在建立調查場次時自動帶入海溫，以及三種部署定時排程（Cronjob）的方式。

---

## 1. 資料來源與功能概述

### 1.1 資料來源
- **氣象署海象觀測網頁**：`https://www.cwa.gov.tw/V8/C/M/OBS_Marine.html`
- **測站清單來源**：`https://www.cwa.gov.tw/V8/C/M/MMC_stations.js`
- **海區觀測即時頁面**：`https://www.cwa.gov.tw/V8/C/M/48hrsSeaObs_MOD/OSea{AreaID}.html`
- **測站歷史觀測**：
  - 最近 48 小時：`https://www.cwa.gov.tw/V8/C/M/48hrsSeaObs_MOD/M{StationID}.html`
  - 最近 30 天：`https://www.cwa.gov.tw/V8/C/M/30daysSeaObs_MOD/M{StationID}.html`

### 1.2 系統功能
1. **活動建立時自動帶入水溫**：
   - 管理員在建立 Reef Check 調查場次時，選取樣點會自動帶入推薦之鄰近氣象署測站。
   - 選擇測站與調查日期時間後，系統會即時查詢該測站最接近之觀測海溫，自動填入「當日水溫 (`water_temp_c`)」。
   - 若資料庫尚未收錄該日期之觀測，系統會自動向氣象署拉取過去 30 天紀錄補齊資料庫，再回傳溫度。
2. **自動地理關聯**：
   - 樣點經緯度建立或更新時，資料庫會自動依空間距離比對最近的氣象署海象測站（亦可由管理介面手動指定覆寫）。
3. **海溫排程擷取（Cronjob）**：
   - 定時擷取氣象署全台 70+ 個海象與浮標測站的海溫及波浪觀測數據存入資料庫。

---

## 2. 資料庫模型

資料庫已由 migration `migrations/000007_cwa_marine_temperature.sql` 建立以下表格：

- `cwa_marine_station`：記錄氣象署測站代號、名稱、海區、測站類型、經緯度與啟用狀態。
- `cwa_sea_temperature`：記錄每小時/每次觀測之時間、海溫（°C）、波高、週期、波向與資料來源。
- `site.cwa_station_id`：記錄樣點預設關聯之測站代號。
- `event.cwa_station_id`：記錄調查場次選定之測站代號。
- `transect.water_temp_c`：記錄該穿越線之實際水溫（°C）。

---

## 3. API 端點說明

| 端點 | 方法 | 權限需求 | 說明 |
| --- | --- | --- | --- |
| `/api/admin/cwa-marine/stations` | GET | Admin / Authenticated | 取得所有氣象署海象測站清單（依分區排序） |
| `/api/admin/cwa-marine/sea-temp` | GET | Admin / Authenticated | 查詢指定測站與日期時間最接近之海溫（參數：`station_id`, `date`, `time`） |
| `/api/admin/cwa-marine/sync` | POST | Admin | 手動觸發同步海溫觀測資料 |
| `/api/cron/sync-cwa-marine` | POST / GET | Bearer / Secret Header | 提供排程專用之 Webhook，支援 `X-Cron-Secret` 或 `Authorization: Bearer <CRON_SECRET>` |

---

## 4. 排程部署方式（三選一）

系統提供三種靈活的排程運作方式，可依基礎設施選擇最適合的模式：

### 方式 A（推薦）：Cloud Run + Cloud Scheduler Webhook

在 GCP Cloud Run 環境中，最省資源且無需常駐背景程式的方法是利用 **Cloud Scheduler** 定時呼叫 API Webhook。

#### 1. 設定環境變數
在 Cloud Run 服務中設定：
```env
CRON_SECRET=your-random-high-entropy-secret-string
```

#### 2. 建立 Cloud Scheduler 作業
使用 gcloud 建立每小時自動觸發一次的作業：
```bash
PROJECT_ID="your-gcp-project"
REGION="asia-east1"
SERVICE_URL="https://your-service-xyz.run.app"
CRON_SECRET="your-random-high-entropy-secret-string"

gcloud scheduler jobs create http sync-cwa-marine \
  --project="$PROJECT_ID" \
  --location="$REGION" \
  --schedule="0 * * * *" \
  --time-zone="Asia/Taipei" \
  --uri="${SERVICE_URL}/api/cron/sync-cwa-marine" \
  --http-method="POST" \
  --headers="X-Cron-Secret=${CRON_SECRET}" \
  --description="Hourly sync of CWA sea temperatures"
```

---

### 方式 B：Cloud Run Job（獨立執行檔）

若希望將排程運算與 Web 流量完全分開，容器鏡像內已打包獨立執行檔 `/app/sync-cwa-marine`。

#### 1. 建立 Cloud Run Job
```bash
gcloud run jobs create sync-cwa-marine-job \
  --project="$PROJECT_ID" \
  --region="$REGION" \
  --image="asia-east1-docker.pkg.dev/${PROJECT_ID}/coast-monitoring/app:latest" \
  --command="/app/sync-cwa-marine" \
  --args="-latest" \
  --set-env-vars="DATABASE_URL=${DATABASE_URL}"
```

#### 2. 設定 Cloud Scheduler 觸發該 Job
```bash
gcloud scheduler jobs create run sync-cwa-marine-trigger \
  --project="$PROJECT_ID" \
  --location="$REGION" \
  --schedule="0 * * * *" \
  --time-zone="Asia/Taipei" \
  --job="sync-cwa-marine-job"
```

---

### 方式 C：伺服器內建排程器（零額外架構）

若部屬於一般 Linux VM、Docker Compose 或本機環境，可直接開啟內建的背景排程 Goroutine，伺服器會依設定區間定時同步。

#### 設定 `.env` 或環境變數：
```env
# 啟用內建 CWA 海溫同步排程
ENABLE_CWA_CRON=true

# 同步間隔（預設 1h，支援 30m, 1h, 2h 等 Go duration 格式）
CWA_SYNC_INTERVAL=1h
```

伺服器啟動時將自動出現以下日誌並定時執行：
```text
CWA marine sync ticker started (interval: 1h0m0s)
```

---

## 5. CLI 指令手動操作

開發測試或手動資料回填時，可直接在終端機執行 CLI：

```bash
# 查看指令說明
go run ./cmd/sync-cwa-marine -h

# 同步測站清單 + 各海區最新即時觀測
go run ./cmd/sync-cwa-marine

# 僅同步全台測站清單
go run ./cmd/sync-cwa-marine -stations

# 僅同步最新海面觀測
go run ./cmd/sync-cwa-marine -latest

# 同步特定測站（例如富貴角 C4A02）過去 30 天歷史觀測
go run ./cmd/sync-cwa-marine -station C4A02 -days 30

# 設定逾時上限（預設 5m）
go run ./cmd/sync-cwa-marine -timeout 10m
```

在正式 Docker 容器內部：
```bash
docker exec -it <container_id> /app/sync-cwa-marine -latest
```

---

## 6. 前端使用流程

1. **進入後台管理「珊瑚礁體檢管理」**。
2. 點擊「＋ 新增調查場次」。
3. 選擇「樣點 (Site)」：
   - 系統依樣點地理位置自動推薦相鄰的「氣象署海象測站 (CWA Station)」。
   - 例如選擇「綠島柴口」，自動選取「綠島柴口 (C0W150)」。
4. 選擇「調查日期」與「場次時間」：
   - 系統發動即時背景請求檢索當日觀測海溫。
   - 查詢成功後即自動填入「當日水溫 (°C)」，並於下方綠字標記觀測時間與數據。
5. 點擊「建立場次」完成建立，所有建立的穿越線皆會繼承該海溫數值。
