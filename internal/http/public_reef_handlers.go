package httpx

import (
	"coast-monitoring/internal/service"
	"context"
	"encoding/csv"
	"encoding/json"
	"github.com/go-chi/chi/v5"
	"net/http"
	"strconv"
	"strings"
)

type PublicReefService interface {
	PublicSites(context.Context, service.PublicReefFilter) ([]json.RawMessage, error)
	PublicSeries(context.Context, int, service.PublicReefFilter) ([]service.PublicReefSeries, error)
	SetPublication(context.Context, int, string, string) error
}
type PublicContentService interface {
	PublicAudit(context.Context) ([]json.RawMessage, error)
	ListContent(context.Context, string, bool) ([]service.PublicContent, error)
	SaveContent(context.Context, int, service.PublicContentInput, string) (service.PublicContent, error)
}

type PublicHandlers struct {
	Reef    PublicReefService
	Content PublicContentService
}

func (h *PublicHandlers) ready(w http.ResponseWriter) bool {
	if h == nil || h.Reef == nil {
		writeError(w, 503, "公開資料服務暫時無法使用")
		return false
	}
	return true
}
func (h *PublicHandlers) Sites(w http.ResponseWriter, r *http.Request) {
	if !h.ready(w) {
		return
	}
	f, err := service.ParsePublicReefFilter(r.URL.Query())
	if err != nil {
		writeServiceError(w, err, "")
		return
	}
	data, err := h.Reef.PublicSites(r.Context(), f)
	if err != nil {
		writeServiceError(w, err, "樣點載入失敗")
		return
	}
	writeJSON(w, 200, data)
}
func (h *PublicHandlers) Series(w http.ResponseWriter, r *http.Request) {
	if !h.ready(w) {
		return
	}
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id <= 0 {
		writeError(w, 400, "無效樣點")
		return
	}
	f, err := service.ParsePublicReefFilter(r.URL.Query())
	if err != nil {
		writeServiceError(w, err, "")
		return
	}
	data, err := h.Reef.PublicSeries(r.Context(), id, f)
	if err != nil {
		writeServiceError(w, err, "圖表資料載入失敗")
		return
	}
	if strings.HasSuffix(r.URL.Path, ".csv") {
		writeReefCSV(w, data)
		return
	}
	writeJSON(w, 200, map[string]any{"site_id": id, "filters": f, "series": data, "availability": len(data) > 0})
}
func (h *PublicHandlers) Compare(w http.ResponseWriter, r *http.Request) {
	if !h.ready(w) {
		return
	}
	ids := strings.Split(r.URL.Query().Get("site_ids"), ",")
	if len(ids) != 2 || ids[0] == ids[1] {
		writeError(w, 400, "比較需剛好兩個不同樣點")
		return
	}
	f, err := service.ParsePublicReefFilter(r.URL.Query())
	if err != nil {
		writeServiceError(w, err, "")
		return
	}
	results := []any{}
	all := []service.PublicReefSeries{}
	for _, v := range ids {
		id, e := strconv.Atoi(v)
		if e != nil || id <= 0 {
			writeError(w, 400, "無效樣點")
			return
		}
		data, e := h.Reef.PublicSeries(r.Context(), id, f)
		if e != nil {
			writeServiceError(w, e, "比較載入失敗")
			return
		}
		results = append(results, map[string]any{"site_id": id, "series": data, "availability": len(data) > 0})
		all = append(all, data...)
	}
	if strings.HasSuffix(r.URL.Path, ".csv") {
		writeReefCSV(w, all)
		return
	}
	writeJSON(w, 200, map[string]any{"filters": f, "sites": results})
}
func writeReefCSV(w http.ResponseWriter, data []service.PublicReefSeries) {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="reef-check.csv"`)
	_, _ = w.Write([]byte{0xef, 0xbb, 0xbf})
	c := csv.NewWriter(w)
	defer c.Flush()
	_ = c.Write([]string{"site_id", "site_name", "event_id", "survey_date", "event_time", "depth_m", "chart", "key", "name", "size_class", "fish_size_mode", "value", "unit", "total", "mean", "sd", "se", "n", "calculation_status", "missing_reason", "data_updated_at"})
	num := func(v *float64) string {
		if v == nil {
			return ""
		}
		return strconv.FormatFloat(*v, 'f', -1, 64)
	}
	for _, a := range data {
		mode := ""
		if a.Mode != nil {
			mode = *a.Mode
		}
		cells := []string{strconv.Itoa(a.SiteID), a.SiteName, a.EventID, a.Date, a.Time, strconv.FormatFloat(a.Depth, 'f', -1, 64), a.Chart, a.Key, a.Name, a.Size, mode, num(a.Value), a.Unit, num(a.Total), num(a.Mean), num(a.SD), num(a.SE), strconv.Itoa(a.N), a.Status, a.MissingReason, a.UpdatedAt}
		for i, v := range cells {
			if len(v) > 0 && strings.ContainsAny(v[:1], "=+-@\t\r") {
				cells[i] = "'" + v
			}
		}
		_ = c.Write(cells)
	}
}
func (h *PublicHandlers) Publish(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireReefDataAdmin(w, r, h != nil && h.Reef != nil)
	if !ok {
		return
	}
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id <= 0 {
		writeError(w, 400, "無效場次")
		return
	}
	var req struct {
		Status  string `json:"publication_status"`
		Confirm bool   `json:"confirm"`
	}
	if !decodeAdminJSON(w, r, &req) {
		return
	}
	if req.Status == "archived" && !req.Confirm {
		writeError(w, 400, "封存需二次確認")
		return
	}
	if err = h.Reef.SetPublication(r.Context(), id, req.Status, actor.ID.String()); err != nil {
		writeServiceError(w, err, "發布失敗")
		return
	}
	writeJSON(w, 200, map[string]any{"id": id, "publication_status": req.Status})
}

func (h *PublicHandlers) Contents(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Content == nil {
		writeError(w, 503, "內容服務暫時無法使用")
		return
	}
	public := !strings.HasPrefix(r.URL.Path, "/api/admin/")
	kind := chi.URLParam(r, "kind")
	if !public {
		if _, ok := requireReefDataAdmin(w, r, true); !ok {
			return
		}
	}
	data, err := h.Content.ListContent(r.Context(), kind, public)
	if err != nil {
		writeServiceError(w, err, "內容載入失敗")
		return
	}
	writeJSON(w, 200, data)
}
func (h *PublicHandlers) SaveContent(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireReefDataAdmin(w, r, h != nil && h.Content != nil)
	if !ok {
		return
	}
	var input service.PublicContentInput
	if !decodeAdminJSON(w, r, &input) {
		return
	}
	input.Kind = chi.URLParam(r, "kind")
	id := 0
	if v := chi.URLParam(r, "id"); v != "" {
		var err error
		id, err = strconv.Atoi(v)
		if err != nil || id <= 0 {
			writeError(w, 400, "無效內容編號")
			return
		}
	}
	data, err := h.Content.SaveContent(r.Context(), id, input, actor.ID.String())
	if err != nil {
		writeServiceError(w, err, "內容儲存失敗")
		return
	}
	status := 200
	if id == 0 {
		status = 201
	}
	writeJSON(w, status, data)
}

func (h *PublicHandlers) Audit(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireReefDataAdmin(w, r, h != nil && h.Content != nil); !ok {
		return
	}
	data, err := h.Content.PublicAudit(r.Context())
	if err != nil {
		writeServiceError(w, err, "稽核紀錄載入失敗")
		return
	}
	writeJSON(w, 200, data)
}
