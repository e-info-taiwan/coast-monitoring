package httpx

import (
	"bytes"
	"coast-monitoring/internal/policy"
	"coast-monitoring/internal/repository"
	"coast-monitoring/internal/service"
	"context"
	"encoding/json"
	"errors"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"io"
	"net/http"
	"strconv"
)

type AdminReefDataService interface {
	ListEvents(context.Context) ([]service.ReefDataEvent, error)
	Codes(context.Context) ([]service.ReefDataCode, error)
	Event(context.Context, int) (service.ReefDataDetail, error)
	Transect(context.Context, int, bool) (service.ReefDataTransect, error)
	Update(context.Context, int, service.ReefDataUpdate) (service.ReefDataTransect, error)
}

func (h *AdminHandlers) ListReefDataEvents(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireReefDataAdmin(w, r, h != nil && h.ReefData != nil); !ok {
		return
	}
	data, err := h.ReefData.ListEvents(r.Context())
	if err != nil {
		writeServiceError(w, err, "無法載入 Reef Check 場次")
		return
	}
	writeJSON(w, http.StatusOK, data)
}
func (h *AdminHandlers) ReefDataCodes(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireReefDataAdmin(w, r, h != nil && h.ReefData != nil); !ok {
		return
	}
	data, err := h.ReefData.Codes(r.Context())
	if err != nil {
		writeServiceError(w, err, "無法載入底質代碼")
		return
	}
	writeJSON(w, http.StatusOK, data)
}
func (h *AdminHandlers) GetReefDataEvent(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireReefDataAdmin(w, r, h != nil && h.ReefData != nil); !ok {
		return
	}
	id, ok := reefDataID(w, r)
	if !ok {
		return
	}
	data, err := h.ReefData.Event(r.Context(), id)
	if err != nil {
		writeServiceError(w, err, "無法載入調查詳情")
		return
	}
	writeJSON(w, http.StatusOK, data)
}
func (h *AdminHandlers) UpdateReefDataTransect(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireReefDataAdmin(w, r, h != nil && h.Mutations != nil)
	if !ok {
		return
	}
	id, ok := reefDataID(w, r)
	if !ok {
		return
	}
	var input service.ReefDataUpdate
	r.Body = http.MaxBytesReader(w, r.Body, maxAdminRequestBodyBytes)
	var raw json.RawMessage
	bodyDecoder := json.NewDecoder(r.Body)
	if err := bodyDecoder.Decode(&raw); err != nil {
		writeError(w, http.StatusBadRequest, "無效的修改內容")
		return
	}
	if err := bodyDecoder.Decode(new(any)); err != io.EOF {
		writeError(w, http.StatusBadRequest, "只接受一份 JSON 修改內容")
		return
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "無效的修改內容")
		return
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		writeError(w, http.StatusBadRequest, "只接受一份 JSON 修改內容")
		return
	}
	if input.Metadata != nil {
		var envelope struct {
			Metadata map[string]json.RawMessage `json:"metadata"`
		}
		if err := json.Unmarshal(raw, &envelope); err != nil {
			writeError(w, http.StatusBadRequest, "無效的基本資料")
			return
		}
		for _, key := range []string{"start_time", "water_temp_c", "visibility_min_m", "visibility_max_m", "comments", "rkc_bleaching_note"} {
			if _, present := envelope.Metadata[key]; !present {
				writeError(w, http.StatusBadRequest, "metadata 必須包含全部六個欄位，清空值請使用 null")
				return
			}
		}
	}
	var result service.ReefDataTransect
	err := h.Mutations.RunAdminMutation(r.Context(), func(s AdminMutationServices) error {
		if s.ReefData == nil || s.AuditLogs == nil {
			return errAdminMutationUnavailable
		}
		before, err := s.ReefData.Transect(r.Context(), id, true)
		if err != nil {
			return err
		}
		codes, err := s.ReefData.Codes(r.Context())
		if err != nil {
			return err
		}
		if err = input.Validate(before, codes); err != nil {
			return err
		}
		result, err = s.ReefData.Update(r.Context(), id, input)
		if err != nil {
			return err
		}
		// Existing audit target IDs are UUIDs; use a stable namespaced key and retain the real integer ID in both snapshots.
		target := uuid.NewSHA1(uuid.NameSpaceURL, []byte("coast-monitoring:transect:"+strconv.Itoa(id)))
		return writeAudit(r, s.AuditLogs, actor, repository.AuditActionUpdate, "transect", target, before, result)
	})
	if errors.Is(err, service.ErrConflict) {
		writeError(w, http.StatusConflict, "資料已被其他人或匯入程序更新，請重新載入後再修改。")
		return
	}
	if err != nil {
		writeServiceError(w, err, "儲存失敗，修改未寫入")
		return
	}
	writeJSON(w, http.StatusOK, result)
}
func reefDataID(w http.ResponseWriter, r *http.Request) (int, bool) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "無效編號")
		return 0, false
	}
	return id, true
}
func requireReefDataAdmin(w http.ResponseWriter, r *http.Request, configured bool) (policy.User, bool) {
	actor, ok := requireAdminHandlerService(w, r, configured)
	if !ok {
		return actor, false
	}
	if !policy.CanUseAdminAPI(actor) {
		writeError(w, http.StatusForbidden, "forbidden")
		return actor, false
	}
	return actor, true
}
