package httpx

import (
	"context"
	"net/http"
	"strconv"

	"coast-monitoring/internal/policy"
	"coast-monitoring/internal/service"
	"github.com/go-chi/chi/v5"
)

type activityStore interface {
	VerifyActivity(context.Context, string) (service.ReefCheckActivity, error)
	ListActivities(context.Context) ([]service.ReefCheckActivity, error)
	CreateActivity(context.Context, service.ReefCheckActivityInput, policy.User) (service.ReefCheckActivity, error)
	ChangeActivity(context.Context, int, string, policy.User) (service.ReefCheckActivity, error)
}

func (h *AppHandlers) VerifyReefActivity(w http.ResponseWriter, r *http.Request) {
	if h == nil {
		writeError(w, http.StatusServiceUnavailable, "活動驗證未啟用")
		return
	}
	store, ok := h.ReefData.(activityStore)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "活動驗證未啟用")
		return
	}
	var input struct {
		Code string `json:"code"`
	}
	if !decodeAdminJSON(w, r, &input) {
		return
	}
	a, err := store.VerifyActivity(r.Context(), input.Code)
	if err != nil {
		writeServiceError(w, err, "活動驗證失敗")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	a.Code = ""
	writeJSON(w, http.StatusOK, a)
}
func (h *AdminHandlers) ReefActivities(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireReefDataAdmin(w, r, h != nil && h.ReefData != nil)
	if !ok {
		return
	}
	store, ok := h.ReefData.(activityStore)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "活動管理未啟用")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	switch r.Method {
	case http.MethodGet:
		list, err := store.ListActivities(r.Context())
		if err != nil {
			writeServiceError(w, err, "活動讀取失敗")
			return
		}
		writeJSON(w, http.StatusOK, list)
	case http.MethodPost:
		var input service.ReefCheckActivityInput
		if !decodeAdminJSON(w, r, &input) {
			return
		}
		a, err := store.CreateActivity(r.Context(), input, actor)
		if err != nil {
			writeServiceError(w, err, "活動建立失敗")
			return
		}
		writeJSON(w, http.StatusCreated, a)
	case http.MethodPatch:
		id, err := strconv.Atoi(chi.URLParam(r, "id"))
		if err != nil || id <= 0 {
			writeError(w, http.StatusBadRequest, "活動識別無效")
			return
		}
		var input struct {
			Action string `json:"action"`
		}
		if !decodeAdminJSON(w, r, &input) {
			return
		}
		a, err := store.ChangeActivity(r.Context(), id, input.Action, actor)
		if err != nil {
			writeServiceError(w, err, "活動更新失敗")
			return
		}
		writeJSON(w, http.StatusOK, a)
	}
}
