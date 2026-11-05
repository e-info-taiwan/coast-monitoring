package httpx

import (
	"coast-monitoring/internal/service"
	"context"
	"net/http"
	"strings"
)

type siteDisplayStore interface {
	SiteDisplay(context.Context, int, bool) (service.SiteDisplay, error)
	SaveChartSetting(context.Context, service.ChartSetting, string) (service.ChartSetting, error)
	SaveReportLink(context.Context, service.ReportLink, string) error
	SaveSharedGroup(context.Context, service.SharedChartGroup, string) (service.SharedChartGroup, error)
}

func (h *PublicHandlers) displayStore(w http.ResponseWriter) (siteDisplayStore, bool) {
	if h != nil {
		if store, ok := h.Content.(siteDisplayStore); ok {
			return store, true
		}
	}
	writeError(w, 503, "樣點設定服務暫時無法使用")
	return nil, false
}
func (h *PublicHandlers) SiteDisplay(w http.ResponseWriter, r *http.Request) {
	store, ok := h.displayStore(w)
	if !ok {
		return
	}
	admin := strings.HasPrefix(r.URL.Path, "/api/admin/")
	if admin {
		if _, ok := requireReefDataAdmin(w, r, true); !ok {
			return
		}
	}
	id, ok := reefDataID(w, r)
	if !ok {
		return
	}
	data, err := store.SiteDisplay(r.Context(), id, admin)
	if err != nil {
		writeServiceError(w, err, "樣點資料載入失敗")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, 200, data)
}
func (h *PublicHandlers) SaveSiteDisplay(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireReefDataAdmin(w, r, h != nil)
	if !ok {
		return
	}
	store, ok := h.displayStore(w)
	if !ok {
		return
	}
	id, ok := reefDataID(w, r)
	if !ok {
		return
	}
	var result any
	var err error
	switch {
	case strings.HasSuffix(r.URL.Path, "/charts"):
		var in service.ChartSetting
		if !decodeAdminJSON(w, r, &in) {
			return
		}
		in.SiteID = id
		result, err = store.SaveChartSetting(r.Context(), in, actor.ID.String())
	case strings.HasSuffix(r.URL.Path, "/reports"):
		var in service.ReportLink
		if !decodeAdminJSON(w, r, &in) {
			return
		}
		in.SiteID = id
		err = store.SaveReportLink(r.Context(), in, actor.ID.String())
		result = map[string]bool{"saved": err == nil}
	case strings.HasSuffix(r.URL.Path, "/groups"):
		var in service.SharedChartGroup
		if !decodeAdminJSON(w, r, &in) {
			return
		}
		// Editing from a removed member remains possible: use before/after union.
		affected := false
		for _, m := range in.Members {
			affected = affected || m.SiteID == id
		}
		if !affected && in.ID > 0 {
			d, e := store.SiteDisplay(r.Context(), id, true)
			if e != nil {
				writeServiceError(w, e, "")
				return
			}
			for _, g := range d.Groups {
				affected = affected || g.ID == in.ID
			}
		}
		if !affected {
			writeError(w, 400, "目前樣點不在群組影響範圍")
			return
		}
		result, err = store.SaveSharedGroup(r.Context(), in, actor.ID.String())
	default:
		writeError(w, 404, "找不到操作")
		return
	}
	if err != nil {
		writeServiceError(w, err, "樣點設定儲存失敗")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, 200, result)
}
