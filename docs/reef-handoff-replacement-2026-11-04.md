# 2026-11-04 珊瑚觀察資料替換

本次來源為 `coral_data_RD_handoff_2026-11-04_v1.zip`，SHA-256：
`deedd8ea854b7cb246ed8beaf3f5bc2c89339664a79ffd52e74651789feb285a`。
只匯入 `01_reefcheck_observations/csv`；不匯入 audit、開發案或媒體。

## 執行

在一般 Terminal、專案根目錄，使用 Python 3.11 以上：

```bash
/opt/homebrew/bin/python3.14 scripts/replace-reef-observations.py --check-package
/opt/homebrew/bin/python3.14 scripts/replace-reef-observations.py --apply --app-commit DEPLOYED_COMMIT_SHA
```

`DEPLOYED_COMMIT_SHA` 需填入包含本次 source_unit 查詢修正、已部署的 commit SHA；腳本核對該版本已就緒並承接 100% 流量才繼續。
省略 `--apply` 會備份並完成本機資料庫演練，不修改線上觀察資料。
需有可用的 gcloud 使用者登入，以及 `cloud-sql-proxy`、PostgreSQL CLI/server、Go。
PostgreSQL 工具版本須可讀取線上伺服器的備份。腳本不會自動安裝工具或要求重新登入。

目標固定為專案 `tnf-rc`、區域 `asia-east1`、Cloud Run 服務 `coast-monitoring-dev`。
Cloud SQL instance 與 DB 名稱由該服務設定及其 Secret Manager 參照核實；無法對應即停止。
密碼只透過子程序環境傳遞，不寫入檔案或命令列參數。

## 順序與保護

1. 驗證 ZIP 固定雜湊、全部交付檔案雜湊、各 CSV 筆數、自然鍵、FK 與方法。
2. 下載完整 DB 備份，還原至獨立本機 PostgreSQL；不使用現有本機開發 DB。
3. 在本機套用 migration、測試 dry-run 與故意錯誤的匯入，核對原資料指紋未變。
4. 在本機實際提交替換，驗證筆數、NULL、歷史分級，以及使用者／活動／內容設定未變。
5. `--apply` 才套用線上 migration，鎖住觀察相關表，透過 exported snapshot 再備份一次。
6. 備份成功後，在同一 transaction 刪除全部 transect、survey 及其觀察子表，匯入並驗證後提交。

鎖定期間查詢仍可讀取原資料；觀察資料寫入會等待。鎖定失敗、備份失敗、解析失敗或筆數不符均不提交替換。
不重設流水號、不刪除 site/diver/users/activity/public_content 或測站對應。
樣點僅以交付包非空欄位更新行政區與座標，保留 site ID、既有名稱與測站關聯。
原場次的發布稽核與投稿附加資訊會隨場次刪除，保留於完整備份中。
歷史 event 延用既有匯入器的 published 行為；交付包標記為隔離的資料不會匯入。

備份、資料驗證紀錄、來源 CSV 與本機演練 DB 均置於 `.local/backups/reef-replacement-時間/`。
這些資料不加入 Git，也不進 Docker／Cloud Build context。備份含個資，檔案權限限制為目前使用者。
替換前的主要備份為 `online-before-replacement.dump`；`before-migrations.dump` 是演練用來源備份。
若失敗，先確認錯誤階段；交易內的失敗會自動回復。不要直接將完整備份覆寫線上 DB，避免覆蓋其後的帳號或設定異動。

## 預期資料

| 表 | 筆數 |
|---|---:|
| survey | 409 |
| event | 757 |
| transect | 2,041 |
| transect_participant | 269 |
| substrate_point | 110,716 |
| substrate_bleaching | 2,768 |
| belt_observation | 81,912 |
| impact_observation | 24,252 |
| incidental_sighting | 16 |

61 個樣點 lookup 與 65 位人員採保留既有主檔的匯入方式，DB 主檔總數可能較多。
Belt 中 9,626 筆空白保留 SQL NULL；環境衝擊包含 count 5,060 筆、level 8,440 筆、percent 10,752 筆。
來源完整 CSV 會保存於 `reef_import_attempt.source_snapshot`。

後端需同時部署本次 repository 查詢修正，才能讓前台與報表依每筆 `source_unit` 正確解讀原值與歷史分級。
Migration 000017 與舊版寫入相容：未指定 source_unit 的新投稿沿用既有 master 定義。

目前離線已通過檔案／資料檢查及 Go 測試；執行環境禁止 PostgreSQL shared memory，因此尚未在此環境完成 DB 整合測試。
上面的腳本將本機還原演練設為線上替換的必要條件。
