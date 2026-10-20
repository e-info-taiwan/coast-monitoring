package httpx

import (
	"coast-monitoring/internal/report"
	"coast-monitoring/internal/service"
	"net/http"
	"strconv"
	"strings"
)

// ExportReefReport uses the same admin boundary and saved observations as the detail page.
func (h *AdminHandlers) ExportReefReport(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireReefDataAdmin(w, r, h != nil && h.ReefData != nil); !ok {
		return
	}
	id, ok := reefDataID(w, r)
	if !ok {
		return
	}
	detail, err := h.ReefData.Event(r.Context(), id)
	if err != nil {
		writeServiceError(w, err, "無法載入報告資料")
		return
	}
	h.writeReefWorkbook(w, r, []service.ReefDataDetail{detail})
}

func (h *AdminHandlers) ExportReefReports(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireReefDataAdmin(w, r, h != nil && h.ReefData != nil); !ok {
		return
	}
	q := r.URL.Query()
	year, siteID, method, search := q.Get("year"), q.Get("site"), q.Get("method"), strings.ToLower(strings.TrimSpace(q.Get("search")))
	if year != "" {
		n, e := strconv.Atoi(year)
		if e != nil || len(year) != 4 || n < 1900 || n > 9999 {
			writeError(w, 400, "年份格式無效")
			return
		}
	}
	if siteID != "" {
		n, e := strconv.Atoi(siteID)
		if e != nil || n <= 0 {
			writeError(w, 400, "樣點編號無效")
			return
		}
	}
	if method != "" && method != "line" && method != "belt_fish" && method != "belt_invert" {
		writeError(w, 400, "調查方法無效")
		return
	}
	events, err := h.ReefData.ListEvents(r.Context())
	if err != nil {
		writeServiceError(w, err, "無法載入報告場次")
		return
	}
	var selected []service.ReefDataEvent
	for _, e := range events {
		if year != "" && !strings.HasPrefix(e.SurveyDate, year) {
			continue
		}
		if siteID != "" && strconv.Itoa(e.SiteID) != siteID {
			continue
		}
		hasMethod := method == ""
		for _, m := range e.Methods {
			hasMethod = hasMethod || m == method
		}
		if !hasMethod {
			continue
		}
		if search != "" && !strings.Contains(strings.ToLower(strings.Join([]string{e.EventID, e.SiteName, e.SiteEnglish, e.County, e.Region, e.Label}, " ")), search) {
			continue
		}
		selected = append(selected, e)
	}
	if len(selected) == 0 {
		writeError(w, 404, "沒有符合條件的場次")
		return
	}
	// Bound workbook memory and query work; never silently truncate an export.
	if len(selected) > 100 {
		writeError(w, 400, "一次最多匯出 100 個場次，請使用年份或樣點縮小範圍")
		return
	}
	details := make([]service.ReefDataDetail, 0, len(selected))
	for _, e := range selected {
		d, err := h.ReefData.Event(r.Context(), e.ID)
		if err != nil {
			writeServiceError(w, err, "無法載入報告資料")
			return
		}
		details = append(details, d)
	}
	h.writeReefWorkbook(w, r, details)
}
func (h *AdminHandlers) writeReefWorkbook(w http.ResponseWriter, r *http.Request, details []service.ReefDataDetail) {
	taxa, err := h.ReefData.ListTaxa(r.Context())
	if err != nil {
		writeServiceError(w, err, "無法載入物種對照表")
		return
	}
	data, err := report.ReefWorkbook(details, taxa)
	if err != nil {
		writeServiceError(w, err, "無法產生 Excel 報告")
		return
	}
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", `attachment; filename="reef-check-report.xlsx"`)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}
