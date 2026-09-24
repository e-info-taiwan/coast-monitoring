package httpx

import (
	"context"
	"net/http"
	"strings"

	"coast-monitoring/internal/service"
)

type AdminCWAMarineService interface {
	ListStations(ctx context.Context) ([]service.CWAMarineStation, error)
	GetTemperatureForEvent(ctx context.Context, stationID, dateStr, timeStr string) (*service.CWATempResult, error)
	SyncAll(ctx context.Context) (*service.CWASyncReport, error)
}

func (h *AdminHandlers) ListCWAMarineStations(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireReefDataAdmin(w, r, h != nil && h.CWAMarine != nil); !ok {
		return
	}
	stations, err := h.CWAMarine.ListStations(r.Context())
	if err != nil {
		writeServiceError(w, err, "無法載入氣象署測站清單")
		return
	}
	writeJSON(w, http.StatusOK, stations)
}

func (h *AdminHandlers) GetCWAMarineSeaTemp(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireReefDataAdmin(w, r, h != nil && h.CWAMarine != nil); !ok {
		return
	}
	stationID := strings.TrimSpace(r.URL.Query().Get("station_id"))
	dateStr := strings.TrimSpace(r.URL.Query().Get("date"))
	timeStr := strings.TrimSpace(r.URL.Query().Get("time"))

	if stationID == "" {
		writeError(w, http.StatusBadRequest, "缺少測站代碼 (station_id)")
		return
	}
	if dateStr == "" {
		writeError(w, http.StatusBadRequest, "缺少調查日期 (date)")
		return
	}

	result, err := h.CWAMarine.GetTemperatureForEvent(r.Context(), stationID, dateStr, timeStr)
	if err != nil {
		writeServiceError(w, err, "查詢氣象署海溫失敗")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *AdminHandlers) SyncCWAMarine(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireReefDataAdmin(w, r, h != nil && h.CWAMarine != nil); !ok {
		return
	}
	report, err := h.CWAMarine.SyncAll(r.Context())
	if err != nil {
		writeServiceError(w, err, "同步氣象署海象資料失敗")
		return
	}
	writeJSON(w, http.StatusOK, report)
}

type CronHandlers struct {
	CWAMarine  AdminCWAMarineService
	CronSecret string
}

func (h *CronHandlers) SyncCWAMarine(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.CWAMarine == nil {
		writeError(w, http.StatusServiceUnavailable, "cron service unavailable")
		return
	}

	if h.CronSecret != "" {
		secretHeader := r.Header.Get("X-Cron-Secret")
		authHeader := r.Header.Get("Authorization")
		bearerToken := ""
		if strings.HasPrefix(authHeader, "Bearer ") {
			bearerToken = strings.TrimPrefix(authHeader, "Bearer ")
		}
		querySecret := r.URL.Query().Get("secret")

		if secretHeader != h.CronSecret && bearerToken != h.CronSecret && querySecret != h.CronSecret {
			writeError(w, http.StatusUnauthorized, "unauthorized cron request")
			return
		}
	}

	report, err := h.CWAMarine.SyncAll(r.Context())
	if err != nil {
		writeServiceError(w, err, "cwa sync failed")
		return
	}
	writeJSON(w, http.StatusOK, report)
}
