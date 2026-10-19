package repository

import (
	"coast-monitoring/internal/service"
	"context"
	"encoding/json"
)

func (r ReefDataRepository) ListContent(ctx context.Context, kind string, public bool) ([]service.PublicContent, error) {
	rows, err := r.db.Query(ctx, `SELECT id,kind,publication_status,data,updated_at::text,site_id FROM public_content WHERE ($1='' OR kind=$1) AND (NOT $2 OR publication_status='published') ORDER BY COALESCE((data->>'sort_order')::int,0),CASE WHEN kind='article' THEN data->>'date' END DESC,id`, kind, public)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []service.PublicContent{}
	for rows.Next() {
		var c service.PublicContent
		if err = rows.Scan(&c.ID, &c.Kind, &c.Status, &c.Data, &c.UpdatedAt, &c.SiteID); err != nil {
			return nil, err
		}
		if public && c.Kind == "development" {
			var d service.ContentData
			if err = json.Unmarshal(c.Data, &d); err != nil {
				return nil, err
			}
			timeline := []service.ContentTimeline{}
			for _, t := range d.Timeline {
				if t.Published {
					timeline = append(timeline, t)
				}
			}
			media := []service.ContentMedia{}
			for _, m := range d.Media {
				if m.Published && m.Valid && m.Licensed {
					media = append(media, m)
				}
			}
			d.Timeline = timeline
			d.Media = media
			c.Data, _ = json.Marshal(d)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
func (r ReefDataRepository) SaveContent(ctx context.Context, id int, c service.PublicContentInput, actor string) (service.PublicContent, error) {
	if err := c.Validate(); err != nil {
		return service.PublicContent{}, err
	}
	raw, err := json.Marshal(c.Data)
	if err != nil {
		return service.PublicContent{}, err
	}
	// Validate all chart member references. A JSON array cannot express a foreign key.
	for _, site := range c.Data.SiteIDs {
		var exists bool
		if err = r.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM site WHERE id=$1)`, site).Scan(&exists); err != nil {
			return service.PublicContent{}, err
		}
		if !exists {
			return service.PublicContent{}, service.ErrInvalidReference
		}
	}
	var result service.PublicContent
	if id == 0 {
		err = r.db.QueryRow(ctx, `WITH saved AS (INSERT INTO public_content(kind,site_id,data) VALUES($1,$2,$3::jsonb) RETURNING *), audit AS (INSERT INTO public_content_audit(content_id,actor_id,after_data) SELECT id,$4::uuid,to_jsonb(saved) FROM saved) SELECT id,kind,publication_status,data,updated_at::text,site_id FROM saved`, c.Kind, c.SiteID, raw, actor).Scan(&result.ID, &result.Kind, &result.Status, &result.Data, &result.UpdatedAt, &result.SiteID)
	} else {
		err = r.db.QueryRow(ctx, `WITH previous AS (SELECT * FROM public_content WHERE id=$1 AND updated_at::text=$2 FOR UPDATE),saved AS (UPDATE public_content a SET site_id=$3,data=$4::jsonb,publication_status=$5,updated_at=clock_timestamp() FROM previous p WHERE a.id=p.id AND a.kind=$6 RETURNING a.*),audit AS (INSERT INTO public_content_audit(content_id,actor_id,before_data,after_data) SELECT p.id,$7::uuid,to_jsonb(p),to_jsonb(saved) FROM previous p JOIN saved ON p.id=saved.id) SELECT id,kind,publication_status,data,updated_at::text,site_id FROM saved`, id, c.Version, c.SiteID, raw, c.Status, c.Kind, actor).Scan(&result.ID, &result.Kind, &result.Status, &result.Data, &result.UpdatedAt, &result.SiteID)
	}
	if err != nil {
		err = translateError(err)
		if err == service.ErrNotFound && id != 0 {
			err = service.ErrConflict
		}
	}
	return result, err
}

func (r ReefDataRepository) PublicAudit(ctx context.Context) ([]json.RawMessage, error) {
	rows, err := r.db.Query(ctx, `SELECT jsonb_build_object('id',id,'kind',kind,'created_at',created_at,'details',details) FROM (
 SELECT id,'內容異動'::text AS kind,created_at,jsonb_build_object('content_id',content_id,'actor_id',actor_id,'before',before_data,'after',after_data) AS details FROM public_content_audit
 UNION ALL SELECT id,'調查發布',created_at,jsonb_build_object('event_id',event_id,'actor_id',actor_id,'before',old_status,'after',new_status) FROM reef_publication_audit
 UNION ALL SELECT id,'匯入作業',created_at,jsonb_build_object('source_sha256',source_sha256,'source_format',source_format,'status',status,'report',report) FROM reef_import_attempt
 ) records ORDER BY created_at DESC LIMIT 200`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []json.RawMessage{}
	for rows.Next() {
		var raw json.RawMessage
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		out = append(out, raw)
	}
	return out, rows.Err()
}
