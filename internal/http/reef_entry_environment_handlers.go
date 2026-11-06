package httpx

import (
	"coast-monitoring/internal/service"
	"context"
	"net/http"
)

func (h *AppHandlers) ReefEntryEnvironment(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if h == nil {
		writeError(w, http.StatusServiceUnavailable, "環境參考資料暫時無法使用")
		return
	}
	store, ok := h.ReefData.(interface {
		EntryEnvironment(context.Context, service.ReefEntryEnvironmentInput) (service.ReefEntryEnvironment, error)
	})
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "環境參考資料暫時無法使用")
		return
	}
	var input service.ReefEntryEnvironmentInput
	if !decodeAdminJSON(w, r, &input) {
		return
	}
	if err := input.Validate(); err != nil {
		writeServiceError(w, err, "環境查詢條件無效")
		return
	}
	result, err := store.EntryEnvironment(r.Context(), input)
	if err != nil {
		writeServiceError(w, err, "環境參考資料查詢失敗")
		return
	}
	writeJSON(w, http.StatusOK, result)
}
