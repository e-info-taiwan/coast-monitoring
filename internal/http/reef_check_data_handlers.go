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
	"strings"
)

type AdminReefDataService interface {
	ListEvents(context.Context) ([]service.ReefDataEvent, error)
	Codes(context.Context) ([]service.ReefDataCode, error)
	Event(context.Context, int) (service.ReefDataDetail, error)
	Transect(context.Context, int, bool) (service.ReefDataTransect, error)
	Update(context.Context, int, service.ReefDataUpdate) (service.ReefDataTransect, error)
	CreateEvent(context.Context, service.ReefDataCreateInput) (service.ReefDataDetail, error)
	DeleteEvent(context.Context, int) (service.ReefDataDetail, error)
	Sites(context.Context) ([]service.ReefDataSite, error)
	Users(context.Context) ([]service.ReefDataUser, error)
	Divers(context.Context) ([]service.ReefDataDiver, error)
	AddParticipant(context.Context, int, service.ReefDataParticipantInput) error
	RemoveParticipant(context.Context, int) error

	ListDivers(context.Context) ([]service.ReefDataDiver, error)
	GetDiver(context.Context, int) (service.ReefDataDiver, error)
	CreateDiver(context.Context, service.ReefDataDiverInput) (service.ReefDataDiver, error)
	UpdateDiver(context.Context, int, service.ReefDataDiverInput) (service.ReefDataDiver, error)
	DeleteDiver(context.Context, int) (service.ReefDataDiver, error)

	ListSites(context.Context) ([]service.ReefDataSite, error)
	GetSite(context.Context, int) (service.ReefDataSite, error)
	CreateSite(context.Context, service.ReefDataSiteInput) (service.ReefDataSite, error)
	UpdateSite(context.Context, int, service.ReefDataSiteInput) (service.ReefDataSite, error)
	DeleteSite(context.Context, int) (service.ReefDataSite, error)

	ListTaxa(context.Context) ([]service.ReefDataTaxon, error)
	GetTaxon(context.Context, int) (service.ReefDataTaxon, error)
	CreateTaxon(context.Context, service.ReefDataTaxonInput) (service.ReefDataTaxon, error)
	UpdateTaxon(context.Context, int, service.ReefDataTaxonInput) (service.ReefDataTaxon, error)
	DeleteTaxon(context.Context, int) (service.ReefDataTaxon, error)

	ListSubstrateTypes(context.Context) ([]service.ReefDataSubstrateType, error)
	GetSubstrateType(context.Context, string) (service.ReefDataSubstrateType, error)
	CreateSubstrateType(context.Context, service.ReefDataSubstrateTypeInput) (service.ReefDataSubstrateType, error)
	UpdateSubstrateType(context.Context, string, service.ReefDataSubstrateTypeInput) (service.ReefDataSubstrateType, error)
	DeleteSubstrateType(context.Context, string) (service.ReefDataSubstrateType, error)

	ListImpactTypes(context.Context) ([]service.ReefDataImpactType, error)
	GetImpactType(context.Context, int) (service.ReefDataImpactType, error)
	CreateImpactType(context.Context, service.ReefDataImpactTypeInput) (service.ReefDataImpactType, error)
	UpdateImpactType(context.Context, int, service.ReefDataImpactTypeInput) (service.ReefDataImpactType, error)
	DeleteImpactType(context.Context, int) (service.ReefDataImpactType, error)
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

func (h *AdminHandlers) CreateReefDataEvent(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireReefDataAdmin(w, r, h != nil && h.Mutations != nil && h.ReefData != nil)
	if !ok {
		return
	}
	var input service.ReefDataCreateInput
	r.Body = http.MaxBytesReader(w, r.Body, maxAdminRequestBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "無效的建立內容")
		return
	}
	sites, err := h.ReefData.Sites(r.Context())
	if err != nil {
		writeServiceError(w, err, "無法驗證樣點")
		return
	}
	validSites := make(map[int]bool, len(sites))
	for _, s := range sites {
		validSites[s.ID] = true
	}
	if err := input.Validate(validSites); err != nil {
		writeServiceError(w, err, "驗證失敗")
		return
	}

	var result service.ReefDataDetail
	err = h.Mutations.RunAdminMutation(r.Context(), func(s AdminMutationServices) error {
		if s.ReefData == nil || s.AuditLogs == nil {
			return errAdminMutationUnavailable
		}
		var createErr error
		result, createErr = s.ReefData.CreateEvent(r.Context(), input)
		if createErr != nil {
			return createErr
		}
		target := uuid.NewSHA1(uuid.NameSpaceURL, []byte("coast-monitoring:event:"+strconv.Itoa(result.Event.ID)))
		return writeAudit(r, s.AuditLogs, actor, repository.AuditActionCreate, "event", target, nil, result)
	})
	if err != nil {
		writeServiceError(w, err, "建立失敗")
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (h *AdminHandlers) DeleteReefDataEvent(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireReefDataAdmin(w, r, h != nil && h.Mutations != nil && h.ReefData != nil)
	if !ok {
		return
	}
	id, ok := reefDataID(w, r)
	if !ok {
		return
	}
	err := h.Mutations.RunAdminMutation(r.Context(), func(s AdminMutationServices) error {
		if s.ReefData == nil || s.AuditLogs == nil {
			return errAdminMutationUnavailable
		}
		deleted, err := s.ReefData.DeleteEvent(r.Context(), id)
		if err != nil {
			return err
		}
		target := uuid.NewSHA1(uuid.NameSpaceURL, []byte("coast-monitoring:event:"+strconv.Itoa(id)))
		return writeAudit(r, s.AuditLogs, actor, repository.AuditActionDelete, "event", target, deleted, nil)
	})
	if err != nil {
		writeServiceError(w, err, "刪除失敗")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *AdminHandlers) ReefDataSites(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireReefDataAdmin(w, r, h != nil && h.ReefData != nil); !ok {
		return
	}
	data, err := h.ReefData.Sites(r.Context())
	if err != nil {
		writeServiceError(w, err, "無法載入樣點清單")
		return
	}
	writeJSON(w, http.StatusOK, data)
}

func (h *AdminHandlers) ReefDataUsers(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireReefDataAdmin(w, r, h != nil && h.ReefData != nil); !ok {
		return
	}
	data, err := h.ReefData.Users(r.Context())
	if err != nil {
		writeServiceError(w, err, "無法載入使用者清單")
		return
	}
	writeJSON(w, http.StatusOK, data)
}

func (h *AdminHandlers) ReefDataDivers(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireReefDataAdmin(w, r, h != nil && h.ReefData != nil); !ok {
		return
	}
	data, err := h.ReefData.Divers(r.Context())
	if err != nil {
		writeServiceError(w, err, "無法載入潛水員清單")
		return
	}
	writeJSON(w, http.StatusOK, data)
}

func (h *AdminHandlers) AddReefDataParticipant(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireReefDataAdmin(w, r, h != nil && h.Mutations != nil && h.ReefData != nil)
	if !ok {
		return
	}
	transectID, ok := reefDataID(w, r)
	if !ok {
		return
	}
	var input service.ReefDataParticipantInput
	r.Body = http.MaxBytesReader(w, r.Body, maxAdminRequestBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "無效的參與人員資料")
		return
	}
	var result service.ReefDataTransect
	err := h.Mutations.RunAdminMutation(r.Context(), func(s AdminMutationServices) error {
		if s.ReefData == nil || s.AuditLogs == nil {
			return errAdminMutationUnavailable
		}
		before, err := s.ReefData.Transect(r.Context(), transectID, true)
		if err != nil {
			return err
		}
		if err := s.ReefData.AddParticipant(r.Context(), transectID, input); err != nil {
			return err
		}
		result, err = s.ReefData.Transect(r.Context(), transectID, false)
		if err != nil {
			return err
		}
		target := uuid.NewSHA1(uuid.NameSpaceURL, []byte("coast-monitoring:transect:"+strconv.Itoa(transectID)))
		return writeAudit(r, s.AuditLogs, actor, repository.AuditActionUpdate, "transect_participant", target, before.Participants, result.Participants)
	})
	if err != nil {
		writeServiceError(w, err, "新增參與人員失敗")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *AdminHandlers) RemoveReefDataParticipant(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireReefDataAdmin(w, r, h != nil && h.Mutations != nil && h.ReefData != nil)
	if !ok {
		return
	}
	participantID, ok := reefDataID(w, r)
	if !ok {
		return
	}
	err := h.Mutations.RunAdminMutation(r.Context(), func(s AdminMutationServices) error {
		if s.ReefData == nil || s.AuditLogs == nil {
			return errAdminMutationUnavailable
		}
		if err := s.ReefData.RemoveParticipant(r.Context(), participantID); err != nil {
			return err
		}
		target := uuid.NewSHA1(uuid.NameSpaceURL, []byte("coast-monitoring:participant:"+strconv.Itoa(participantID)))
		return writeAudit(r, s.AuditLogs, actor, repository.AuditActionDelete, "transect_participant", target, nil, nil)
	})
	if err != nil {
		writeServiceError(w, err, "移除參與人員失敗")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func reefDataID(w http.ResponseWriter, r *http.Request) (int, bool) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "無效編號")
		return 0, false
	}
	return id, true
}

func reefDataCodeParam(w http.ResponseWriter, r *http.Request) (string, bool) {
	code := strings.TrimSpace(chi.URLParam(r, "code"))
	if code == "" {
		writeError(w, http.StatusBadRequest, "無效代碼")
		return "", false
	}
	return code, true
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

// Divers
func (h *AdminHandlers) ListDivers(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireReefDataAdmin(w, r, h != nil && h.ReefData != nil); !ok {
		return
	}
	data, err := h.ReefData.ListDivers(r.Context())
	if err != nil {
		writeServiceError(w, err, "無法載入潛水員清單")
		return
	}
	writeJSON(w, http.StatusOK, data)
}

func (h *AdminHandlers) CreateDiver(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireReefDataAdmin(w, r, h != nil && h.Mutations != nil && h.ReefData != nil)
	if !ok {
		return
	}
	var input service.ReefDataDiverInput
	r.Body = http.MaxBytesReader(w, r.Body, maxAdminRequestBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "無效的建立內容")
		return
	}
	var result service.ReefDataDiver
	err := h.Mutations.RunAdminMutation(r.Context(), func(s AdminMutationServices) error {
		if s.ReefData == nil || s.AuditLogs == nil {
			return errAdminMutationUnavailable
		}
		var createErr error
		result, createErr = s.ReefData.CreateDiver(r.Context(), input)
		if createErr != nil {
			return createErr
		}
		target := uuid.NewSHA1(uuid.NameSpaceURL, []byte("coast-monitoring:diver:"+strconv.Itoa(result.ID)))
		return writeAudit(r, s.AuditLogs, actor, repository.AuditActionCreate, "diver", target, nil, result)
	})
	if err != nil {
		writeServiceError(w, err, "建立潛水員失敗")
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (h *AdminHandlers) UpdateDiver(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireReefDataAdmin(w, r, h != nil && h.Mutations != nil && h.ReefData != nil)
	if !ok {
		return
	}
	id, ok := reefDataID(w, r)
	if !ok {
		return
	}
	var input service.ReefDataDiverInput
	r.Body = http.MaxBytesReader(w, r.Body, maxAdminRequestBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "無效的修改內容")
		return
	}
	var result service.ReefDataDiver
	err := h.Mutations.RunAdminMutation(r.Context(), func(s AdminMutationServices) error {
		if s.ReefData == nil || s.AuditLogs == nil {
			return errAdminMutationUnavailable
		}
		before, err := s.ReefData.GetDiver(r.Context(), id)
		if err != nil {
			return err
		}
		result, err = s.ReefData.UpdateDiver(r.Context(), id, input)
		if err != nil {
			return err
		}
		target := uuid.NewSHA1(uuid.NameSpaceURL, []byte("coast-monitoring:diver:"+strconv.Itoa(id)))
		return writeAudit(r, s.AuditLogs, actor, repository.AuditActionUpdate, "diver", target, before, result)
	})
	if err != nil {
		writeServiceError(w, err, "更新潛水員失敗")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *AdminHandlers) DeleteDiver(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireReefDataAdmin(w, r, h != nil && h.Mutations != nil && h.ReefData != nil)
	if !ok {
		return
	}
	id, ok := reefDataID(w, r)
	if !ok {
		return
	}
	err := h.Mutations.RunAdminMutation(r.Context(), func(s AdminMutationServices) error {
		if s.ReefData == nil || s.AuditLogs == nil {
			return errAdminMutationUnavailable
		}
		deleted, err := s.ReefData.DeleteDiver(r.Context(), id)
		if err != nil {
			return err
		}
		target := uuid.NewSHA1(uuid.NameSpaceURL, []byte("coast-monitoring:diver:"+strconv.Itoa(id)))
		return writeAudit(r, s.AuditLogs, actor, repository.AuditActionDelete, "diver", target, deleted, nil)
	})
	if err != nil {
		writeServiceError(w, err, "刪除潛水員失敗")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Sites
func (h *AdminHandlers) ListSites(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireReefDataAdmin(w, r, h != nil && h.ReefData != nil); !ok {
		return
	}
	data, err := h.ReefData.ListSites(r.Context())
	if err != nil {
		writeServiceError(w, err, "無法載入樣點清單")
		return
	}
	writeJSON(w, http.StatusOK, data)
}

func (h *AdminHandlers) CreateSite(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireReefDataAdmin(w, r, h != nil && h.Mutations != nil && h.ReefData != nil)
	if !ok {
		return
	}
	var input service.ReefDataSiteInput
	r.Body = http.MaxBytesReader(w, r.Body, maxAdminRequestBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "無效的建立內容")
		return
	}
	var result service.ReefDataSite
	err := h.Mutations.RunAdminMutation(r.Context(), func(s AdminMutationServices) error {
		if s.ReefData == nil || s.AuditLogs == nil {
			return errAdminMutationUnavailable
		}
		var createErr error
		result, createErr = s.ReefData.CreateSite(r.Context(), input)
		if createErr != nil {
			return createErr
		}
		target := uuid.NewSHA1(uuid.NameSpaceURL, []byte("coast-monitoring:site:"+strconv.Itoa(result.ID)))
		return writeAudit(r, s.AuditLogs, actor, repository.AuditActionCreate, "site", target, nil, result)
	})
	if err != nil {
		writeServiceError(w, err, "建立樣點失敗")
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (h *AdminHandlers) UpdateSite(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireReefDataAdmin(w, r, h != nil && h.Mutations != nil && h.ReefData != nil)
	if !ok {
		return
	}
	id, ok := reefDataID(w, r)
	if !ok {
		return
	}
	var input service.ReefDataSiteInput
	r.Body = http.MaxBytesReader(w, r.Body, maxAdminRequestBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "無效的修改內容")
		return
	}
	var result service.ReefDataSite
	err := h.Mutations.RunAdminMutation(r.Context(), func(s AdminMutationServices) error {
		if s.ReefData == nil || s.AuditLogs == nil {
			return errAdminMutationUnavailable
		}
		before, err := s.ReefData.GetSite(r.Context(), id)
		if err != nil {
			return err
		}
		result, err = s.ReefData.UpdateSite(r.Context(), id, input)
		if err != nil {
			return err
		}
		target := uuid.NewSHA1(uuid.NameSpaceURL, []byte("coast-monitoring:site:"+strconv.Itoa(id)))
		return writeAudit(r, s.AuditLogs, actor, repository.AuditActionUpdate, "site", target, before, result)
	})
	if err != nil {
		writeServiceError(w, err, "更新樣點失敗")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *AdminHandlers) DeleteSite(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireReefDataAdmin(w, r, h != nil && h.Mutations != nil && h.ReefData != nil)
	if !ok {
		return
	}
	id, ok := reefDataID(w, r)
	if !ok {
		return
	}
	err := h.Mutations.RunAdminMutation(r.Context(), func(s AdminMutationServices) error {
		if s.ReefData == nil || s.AuditLogs == nil {
			return errAdminMutationUnavailable
		}
		deleted, err := s.ReefData.DeleteSite(r.Context(), id)
		if err != nil {
			return err
		}
		target := uuid.NewSHA1(uuid.NameSpaceURL, []byte("coast-monitoring:site:"+strconv.Itoa(id)))
		return writeAudit(r, s.AuditLogs, actor, repository.AuditActionDelete, "site", target, deleted, nil)
	})
	if err != nil {
		writeServiceError(w, err, "刪除樣點失敗")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Taxa
func (h *AdminHandlers) ListTaxa(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireReefDataAdmin(w, r, h != nil && h.ReefData != nil); !ok {
		return
	}
	data, err := h.ReefData.ListTaxa(r.Context())
	if err != nil {
		writeServiceError(w, err, "無法載入物種指標清單")
		return
	}
	writeJSON(w, http.StatusOK, data)
}

func (h *AdminHandlers) CreateTaxon(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireReefDataAdmin(w, r, h != nil && h.Mutations != nil && h.ReefData != nil)
	if !ok {
		return
	}
	var input service.ReefDataTaxonInput
	r.Body = http.MaxBytesReader(w, r.Body, maxAdminRequestBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "無效的建立內容")
		return
	}
	var result service.ReefDataTaxon
	err := h.Mutations.RunAdminMutation(r.Context(), func(s AdminMutationServices) error {
		if s.ReefData == nil || s.AuditLogs == nil {
			return errAdminMutationUnavailable
		}
		var createErr error
		result, createErr = s.ReefData.CreateTaxon(r.Context(), input)
		if createErr != nil {
			return createErr
		}
		target := uuid.NewSHA1(uuid.NameSpaceURL, []byte("coast-monitoring:taxon:"+strconv.Itoa(result.ID)))
		return writeAudit(r, s.AuditLogs, actor, repository.AuditActionCreate, "taxon", target, nil, result)
	})
	if err != nil {
		writeServiceError(w, err, "建立物種指標失敗")
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (h *AdminHandlers) UpdateTaxon(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireReefDataAdmin(w, r, h != nil && h.Mutations != nil && h.ReefData != nil)
	if !ok {
		return
	}
	id, ok := reefDataID(w, r)
	if !ok {
		return
	}
	var input service.ReefDataTaxonInput
	r.Body = http.MaxBytesReader(w, r.Body, maxAdminRequestBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "無效的修改內容")
		return
	}
	var result service.ReefDataTaxon
	err := h.Mutations.RunAdminMutation(r.Context(), func(s AdminMutationServices) error {
		if s.ReefData == nil || s.AuditLogs == nil {
			return errAdminMutationUnavailable
		}
		before, err := s.ReefData.GetTaxon(r.Context(), id)
		if err != nil {
			return err
		}
		result, err = s.ReefData.UpdateTaxon(r.Context(), id, input)
		if err != nil {
			return err
		}
		target := uuid.NewSHA1(uuid.NameSpaceURL, []byte("coast-monitoring:taxon:"+strconv.Itoa(id)))
		return writeAudit(r, s.AuditLogs, actor, repository.AuditActionUpdate, "taxon", target, before, result)
	})
	if err != nil {
		writeServiceError(w, err, "更新物種指標失敗")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *AdminHandlers) DeleteTaxon(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireReefDataAdmin(w, r, h != nil && h.Mutations != nil && h.ReefData != nil)
	if !ok {
		return
	}
	id, ok := reefDataID(w, r)
	if !ok {
		return
	}
	err := h.Mutations.RunAdminMutation(r.Context(), func(s AdminMutationServices) error {
		if s.ReefData == nil || s.AuditLogs == nil {
			return errAdminMutationUnavailable
		}
		deleted, err := s.ReefData.DeleteTaxon(r.Context(), id)
		if err != nil {
			return err
		}
		target := uuid.NewSHA1(uuid.NameSpaceURL, []byte("coast-monitoring:taxon:"+strconv.Itoa(id)))
		return writeAudit(r, s.AuditLogs, actor, repository.AuditActionDelete, "taxon", target, deleted, nil)
	})
	if err != nil {
		writeServiceError(w, err, "刪除物種指標失敗")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// SubstrateTypes
func (h *AdminHandlers) ListSubstrateTypes(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireReefDataAdmin(w, r, h != nil && h.ReefData != nil); !ok {
		return
	}
	data, err := h.ReefData.ListSubstrateTypes(r.Context())
	if err != nil {
		writeServiceError(w, err, "無法載入底質代碼清單")
		return
	}
	writeJSON(w, http.StatusOK, data)
}

func (h *AdminHandlers) CreateSubstrateType(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireReefDataAdmin(w, r, h != nil && h.Mutations != nil && h.ReefData != nil)
	if !ok {
		return
	}
	var input service.ReefDataSubstrateTypeInput
	r.Body = http.MaxBytesReader(w, r.Body, maxAdminRequestBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "無效的建立內容")
		return
	}
	var result service.ReefDataSubstrateType
	err := h.Mutations.RunAdminMutation(r.Context(), func(s AdminMutationServices) error {
		if s.ReefData == nil || s.AuditLogs == nil {
			return errAdminMutationUnavailable
		}
		var createErr error
		result, createErr = s.ReefData.CreateSubstrateType(r.Context(), input)
		if createErr != nil {
			return createErr
		}
		target := uuid.NewSHA1(uuid.NameSpaceURL, []byte("coast-monitoring:substrate_type:"+result.Code))
		return writeAudit(r, s.AuditLogs, actor, repository.AuditActionCreate, "substrate_type", target, nil, result)
	})
	if err != nil {
		writeServiceError(w, err, "建立底質代碼失敗")
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (h *AdminHandlers) UpdateSubstrateType(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireReefDataAdmin(w, r, h != nil && h.Mutations != nil && h.ReefData != nil)
	if !ok {
		return
	}
	code, ok := reefDataCodeParam(w, r)
	if !ok {
		return
	}
	var input service.ReefDataSubstrateTypeInput
	r.Body = http.MaxBytesReader(w, r.Body, maxAdminRequestBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "無效的修改內容")
		return
	}
	var result service.ReefDataSubstrateType
	err := h.Mutations.RunAdminMutation(r.Context(), func(s AdminMutationServices) error {
		if s.ReefData == nil || s.AuditLogs == nil {
			return errAdminMutationUnavailable
		}
		before, err := s.ReefData.GetSubstrateType(r.Context(), code)
		if err != nil {
			return err
		}
		result, err = s.ReefData.UpdateSubstrateType(r.Context(), code, input)
		if err != nil {
			return err
		}
		target := uuid.NewSHA1(uuid.NameSpaceURL, []byte("coast-monitoring:substrate_type:"+code))
		return writeAudit(r, s.AuditLogs, actor, repository.AuditActionUpdate, "substrate_type", target, before, result)
	})
	if err != nil {
		writeServiceError(w, err, "更新底質代碼失敗")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *AdminHandlers) DeleteSubstrateType(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireReefDataAdmin(w, r, h != nil && h.Mutations != nil && h.ReefData != nil)
	if !ok {
		return
	}
	code, ok := reefDataCodeParam(w, r)
	if !ok {
		return
	}
	err := h.Mutations.RunAdminMutation(r.Context(), func(s AdminMutationServices) error {
		if s.ReefData == nil || s.AuditLogs == nil {
			return errAdminMutationUnavailable
		}
		deleted, err := s.ReefData.DeleteSubstrateType(r.Context(), code)
		if err != nil {
			return err
		}
		target := uuid.NewSHA1(uuid.NameSpaceURL, []byte("coast-monitoring:substrate_type:"+code))
		return writeAudit(r, s.AuditLogs, actor, repository.AuditActionDelete, "substrate_type", target, deleted, nil)
	})
	if err != nil {
		writeServiceError(w, err, "刪除底質代碼失敗")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ImpactTypes
func (h *AdminHandlers) ListImpactTypes(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireReefDataAdmin(w, r, h != nil && h.ReefData != nil); !ok {
		return
	}
	data, err := h.ReefData.ListImpactTypes(r.Context())
	if err != nil {
		writeServiceError(w, err, "無法載入環境影響指標清單")
		return
	}
	writeJSON(w, http.StatusOK, data)
}

func (h *AdminHandlers) CreateImpactType(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireReefDataAdmin(w, r, h != nil && h.Mutations != nil && h.ReefData != nil)
	if !ok {
		return
	}
	var input service.ReefDataImpactTypeInput
	r.Body = http.MaxBytesReader(w, r.Body, maxAdminRequestBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "無效的建立內容")
		return
	}
	var result service.ReefDataImpactType
	err := h.Mutations.RunAdminMutation(r.Context(), func(s AdminMutationServices) error {
		if s.ReefData == nil || s.AuditLogs == nil {
			return errAdminMutationUnavailable
		}
		var createErr error
		result, createErr = s.ReefData.CreateImpactType(r.Context(), input)
		if createErr != nil {
			return createErr
		}
		target := uuid.NewSHA1(uuid.NameSpaceURL, []byte("coast-monitoring:impact_type:"+strconv.Itoa(result.ID)))
		return writeAudit(r, s.AuditLogs, actor, repository.AuditActionCreate, "impact_type", target, nil, result)
	})
	if err != nil {
		writeServiceError(w, err, "建立環境影響指標失敗")
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (h *AdminHandlers) UpdateImpactType(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireReefDataAdmin(w, r, h != nil && h.Mutations != nil && h.ReefData != nil)
	if !ok {
		return
	}
	id, ok := reefDataID(w, r)
	if !ok {
		return
	}
	var input service.ReefDataImpactTypeInput
	r.Body = http.MaxBytesReader(w, r.Body, maxAdminRequestBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "無效的修改內容")
		return
	}
	var result service.ReefDataImpactType
	err := h.Mutations.RunAdminMutation(r.Context(), func(s AdminMutationServices) error {
		if s.ReefData == nil || s.AuditLogs == nil {
			return errAdminMutationUnavailable
		}
		before, err := s.ReefData.GetImpactType(r.Context(), id)
		if err != nil {
			return err
		}
		result, err = s.ReefData.UpdateImpactType(r.Context(), id, input)
		if err != nil {
			return err
		}
		target := uuid.NewSHA1(uuid.NameSpaceURL, []byte("coast-monitoring:impact_type:"+strconv.Itoa(id)))
		return writeAudit(r, s.AuditLogs, actor, repository.AuditActionUpdate, "impact_type", target, before, result)
	})
	if err != nil {
		writeServiceError(w, err, "更新環境影響指標失敗")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *AdminHandlers) DeleteImpactType(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireReefDataAdmin(w, r, h != nil && h.Mutations != nil && h.ReefData != nil)
	if !ok {
		return
	}
	id, ok := reefDataID(w, r)
	if !ok {
		return
	}
	err := h.Mutations.RunAdminMutation(r.Context(), func(s AdminMutationServices) error {
		if s.ReefData == nil || s.AuditLogs == nil {
			return errAdminMutationUnavailable
		}
		deleted, err := s.ReefData.DeleteImpactType(r.Context(), id)
		if err != nil {
			return err
		}
		target := uuid.NewSHA1(uuid.NameSpaceURL, []byte("coast-monitoring:impact_type:"+strconv.Itoa(id)))
		return writeAudit(r, s.AuditLogs, actor, repository.AuditActionDelete, "impact_type", target, deleted, nil)
	})
	if err != nil {
		writeServiceError(w, err, "刪除環境影響指標失敗")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

