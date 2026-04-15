# Coast Monitoring Admin

這個 repo 目前已經整理成一個 PocketBase + 自訂 admin UI 的起始專案。

## 內容

- `users` auth collection，欄位包含 `email`、`name`、`role`
- `users`、`location`、`species` 都有 `createUser`、`updateUser`、`createDate`、`updateDate` 這些預設欄位
- `location` collection
- `species` collection
- `location` 只有 `admin` 能對它做 read/write
- `species` 預設提供公開 read-only 範例頁，admin 仍可寫入
- `volunteer` 可以有帳號，但不會被允許進入資料管理頁面
- 自訂的管理介面放在 `pb_public/`，不是 PocketBase 原生 dashboard

## 啟動方式

### 方式一: Docker

1. 執行 `docker compose up --build`。
2. 打開 `http://127.0.0.1:8090/`。

### 方式二: 本機 PocketBase binary

1. 安裝 PocketBase 0.36+。
2. 把 PocketBase binary 放在 repo 根目錄，或用一個會和 `pb_public` 同層的執行路徑。
3. 在 repo 根目錄執行 `./pocketbase-local serve --dir /path/to/pb_data`。
4. 打開 `http://127.0.0.1:8090/`。

PocketBase 會自動套用 `pb_migrations/0001_initial.js`，並提供這個 repo 裡的靜態 admin UI。

如果你把 binary 裝在別的地方，請確認它能看到這個 repo 裡的 `pb_public/`、`pb_hooks/` 與 `pb_migrations/`，不然首頁可能會回 404。

如果你用 Docker，`Dockerfile` 會直接把 `pb_migrations`、`pb_hooks` 與 `pb_public` 一起打進容器。

### 本機繞過 Google login

如果你現在只是想先繼續開發，不想先卡在 Google OAuth，可以在 local 先開啟密碼登入。

1. 在 `.env` 或 shell 環境中設定：

```text
PB_DEV_PASSWORD_AUTH=true
```

2. 重啟 PocketBase。
3. 打開 `http://127.0.0.1:8090/`。
4. 你會看到一個本機測試登入表單，直接用預設 superuser 帳號登入：
   - `hcchien@gmail.com`
   - `Test1234!`
5. 這個模式只建議 local 開發使用，正式上線前請關掉，恢復 Google login。

## Google Login

這個專案已經把 `users` collection 的 OAuth2 login 打開，並預留 Google provider。

你還需要在 Google Cloud Console 和 PocketBase 後台把 Google provider 設好。

### 1. 在 Google Cloud Console 建 OAuth 憑證

1. 打開 Google Cloud Console。
2. 選擇或建立一個 project。
3. 到 `APIs & Services -> OAuth consent screen`，先完成 consent screen 設定。
4. 到 `Credentials -> Create Credentials -> OAuth client ID`。
5. 選擇 `Web application`。
6. 加入一個 authorized redirect URI：

```text
http://127.0.0.1:8090/api/oauth2-redirect
```

7. 建立完成後，記下 `Client ID` 和 `Client Secret`。

如果你想讓 PocketBase 在啟動時自動載入 Google provider，可以把這兩個值放進環境變數：

```text
PB_GOOGLE_CLIENT_ID=xxx.apps.googleusercontent.com
PB_GOOGLE_CLIENT_SECRET=xxx
```

### 2. 在 PocketBase 後台設定 Google provider

1. 打開 PocketBase dashboard，預設通常是：

```text
http://127.0.0.1:8090/_/
```

2. 打開 `Collections -> users`。
3. 確認 `users` 是 auth collection，且 OAuth2 已啟用。
4. 到 `Options` 或 `Authentication` 區塊。
5. 在 `OAuth2 providers` 新增 `Google`。
6. 填入 Google Cloud Console 的 `Client ID` 和 `Client Secret`。
7. 檢查 redirect URL 是否是：

```text
http://127.0.0.1:8090/api/oauth2-redirect
```

8. 儲存設定。

如果你已經在 migration 裡用 `PB_GOOGLE_CLIENT_ID` / `PB_GOOGLE_CLIENT_SECRET` 做了初始化，那這一步通常只需要檢查，不一定要手動再建一次。

### 3. 第一次登入時會發生什麼事

1. 使用者從這個 repo 的自訂 admin page 點 `Sign in with Google`。
2. Google 驗證成功後，PocketBase 會建立 `users` record。
3. `pb_hooks/01_oauth_defaults.pb.js` 會幫新帳號補上預設 `name` 與 `role=volunteer`。
4. 之後你可以在 `users` admin page 把該帳號改成 `admin`，讓他進入後台。

### 4. 第一個 admin bootstrap 流程

如果這是你第一次上線，建議先用環境變數指定第一個 admin 的 Google email。

1. 在啟動 PocketBase 前設定：

```text
PB_BOOTSTRAP_ADMIN_EMAIL=your.name@example.com
PB_BOOTSTRAP_ADMIN_NAME=Your Name
```

如果你用的是 Docker，可以直接複製 `.env.example` 成 `.env` 再修改內容，`docker-compose.yml` 會把這兩個值傳進容器。

2. 啟動 PocketBase。
3. 用 `PB_BOOTSTRAP_ADMIN_EMAIL` 對應的 Google 帳號登入一次。
4. 因為目前還沒有任何 admin，PocketBase 會把這個第一個使用者建立成 `role=admin`。
5. 請確認登入時的 Google email 和 `PB_BOOTSTRAP_ADMIN_EMAIL` 完全一致，否則這個帳號會以 `volunteer` 建立。
6. 登入後到 `users` admin page 檢查這筆帳號的 role 是否已經是 `admin`。
7. 建議在確認完成後，把 `PB_BOOTSTRAP_ADMIN_EMAIL` 從正式環境移除，讓之後的新帳號都先以 `volunteer` 建立。

如果未來你把所有 admin 都刪掉了，可以再暫時把 `PB_BOOTSTRAP_ADMIN_EMAIL` 設回你自己的 email，重新登入一次做救援。

### 4.1 更嚴格的一次性版本

如果你希望這個 bootstrap 真的只在第一次啟動時可用，建議不要把它放進正式環境的長期 `.env`，而是改成一次性的啟動參數。

1. 另外建立一份只給 bootstrap 使用的環境檔，例如 `.env.bootstrap`。
2. 只在第一次啟動時使用它：

```text
docker compose --env-file .env.bootstrap up --build
```

3. 用指定的 Google 帳號完成第一次登入，確認它被建立成 `admin`。
4. 把 `.env.bootstrap` 刪掉，或至少把 `PB_BOOTSTRAP_ADMIN_EMAIL` / `PB_BOOTSTRAP_ADMIN_NAME` 移除。
5. 重新啟動服務，讓正式環境只保留一般登入，不再保留 bootstrap 條件。

這個做法比單純把 env 放在正式 `.env` 裡更安全，因為就算之後有人重新部署，也不會無意間再啟用 bootstrap。

如果你的部署流程比較嚴謹，也可以把 bootstrap 變成 CI/CD 的一次性手動步驟，只有第一次上線時才注入這組 env。

### 5. 常見注意事項

- 如果 redirect URI 不一致，Google 會直接拒絕登入。
- 如果 `users` collection 沒有打開 OAuth2，前端會看不到可登入的 provider。
- 如果你有開 `PB_DEV_PASSWORD_AUTH=true`，前端會顯示本機 superuser email/password 登入，不需要 Google。
- 本機登入請直接使用 `hcchien@gmail.com` / `Test1234!`，不要再走建立帳號的流程。
- 如果你把 PocketBase 部署到正式網域，redirect URI 要改成正式網域對應的 `/api/oauth2-redirect`。
- 如果你想看不需要登入的物種清單，請打開 `/species.html`。

## 正式上線時要改哪些 URL / 變數

正式部署時，最重要的是把所有還寫死在本機的網址改成正式網域，避免 Google login 和前端 API 呼叫失效。

### 必改項目

- `PocketBase base URL`
  - 如果前端是和 PocketBase 同網域同站台部署，`pb_public/app.js` 裡的 `window.location.origin` 可以直接沿用。
  - 如果前端和 PocketBase 分開部署，請把前端改成指向正式的 PocketBase 網址，例如 `https://api.example.com`。
- `PB_BOOTSTRAP_ADMIN_EMAIL`
  - 只在第一次建立 admin 時使用。
  - 正式上線後，建議把它從環境變數中移除，避免未來 admin 被刪光時又自動重建。
- `PB_BOOTSTRAP_ADMIN_NAME`
  - 可選。
  - 預設第一個 admin 建議先用 Google login 搭配 bootstrap。
  - 用來指定第一個 admin 的顯示名稱。
- `PB_GOOGLE_CLIENT_ID`
  - Google OAuth client ID。
  - 留空時，`users` 會沒有 OAuth2 provider，登入按鈕不會出現。
- `PB_GOOGLE_CLIENT_SECRET`
  - Google OAuth client secret。
  - 必須和 `PB_GOOGLE_CLIENT_ID` 一起設定。
- `Google OAuth redirect URI`
  - 把 Google Cloud Console 內的 redirect URI 改成正式網域。
  - 例如：

```text
https://api.example.com/api/oauth2-redirect
```

- `PocketBase dashboard URL`
  - 正式環境通常會改成內部管理用網址，不要公開給一般使用者。
  - 例如：`https://api.example.com/_/`
- `Public admin UI URL`
  - 這份 repo 的自訂 admin UI 預設是由 PocketBase 的 `pb_public/` 提供。
  - 如果你改成獨立前端站，請確認它的 `origin` 與 API URL 一致，或有正確設定 CORS。

### 建議你一起確認的環境設定

- `PocketBase public URL`
  - 任何會被瀏覽器打到的地方，都要用正式 HTTPS 網址。
- `Google Cloud OAuth client`
  - Production 用的 client ID / secret 不要和本機測試混在一起。
- `CORS`
  - 如果前端和 API 分開網域，PocketBase 要允許前端來源。
- `Cookie / HTTPS`
  - 正式站務必用 HTTPS，避免 OAuth callback 與登入狀態被瀏覽器擋掉。
- `DNS / reverse proxy`
  - 如果你用 Nginx、Traefik、Caddy 或 Cloudflare，確認 `X-Forwarded-Proto` 和原始 Host 會被正確傳遞。

### 一個實際例子

假設正式站是：

- 前端：admin.example.com
- PocketBase API：api.example.com

那你要改成：

- Google redirect URI：`https://api.example.com/api/oauth2-redirect`
- 前端 PocketBase 連線 URL：`https://api.example.com`
- PocketBase dashboard：`https://api.example.com/_/`

如果前端和 PocketBase 是同一個站台部署，那就維持同一個 origin，設定會簡單很多。

## 公開物種頁

如果你只想讓一般訪客查看物種清單，可以直接用這個範例頁：

- `http://127.0.0.1:8090/species.html`

這個頁面不需要登入，會直接讀取 `species` collection 的公開 read 權限。

## 權限設計

- `users`
  - `createRule` 允許 OAuth2 新建 record
  - `list/view/update/delete/manage` 都只允許 `admin`
- `location`
  - 全部 CRUD 都只允許 `admin`
- `species`
  - 全部 CRUD 都只允許 `admin`

## 我這次假設的資料欄位

- `users`
  - `name`
  - `role`，預設 `volunteer`
  - `createUser` / `updateUser`
  - `createDate` / `updateDate`
- `location`
  - `chineseName`
  - `englishName`
  - `createUser` / `updateUser`
  - `createDate` / `updateDate`
- `species`
  - `chineseName`
  - `englishName`
  - `createUser` / `updateUser`
  - `createDate` / `updateDate`

如果你想要 `location` 或 `species` 再加更多欄位，我可以直接幫你擴成你實際要用的 schema。
