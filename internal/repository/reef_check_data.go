package repository

import (
	"coast-monitoring/internal/service"
	"context"
	"encoding/json"
)

type ReefDataRepository struct{ db DBTX }

func NewReefDataRepository(db DBTX) ReefDataRepository { return ReefDataRepository{db: db} }

const reefEventSelect = `SELECT jsonb_build_object(
 'id',e.id,'event_id',e.event_id,'survey_id',s.id,'survey_date',e.survey_date::text,
 'event_time',e.event_time,'depth_m',e.depth_m,'site_id',p.id,'site_name',p.name_zh,
 'site_english',COALESCE(p.name_en,''),'region',COALESCE(p.region,''),'county',COALESCE(p.county,''),
 'location',COALESCE(p.location,''),'latitude',p.latitude,'longitude',p.longitude,
 'start_date',s.start_date::text,'end_date',s.end_date::text,'label',COALESCE(s.label,''),
 'methods',COALESCE((SELECT jsonb_agg(t.method ORDER BY t.method) FROM transect t WHERE t.event_id=e.event_id),'[]'::jsonb))
 FROM event e JOIN survey s ON s.id=e.survey_id JOIN site p ON p.id=s.site_id `

func (r ReefDataRepository) ListEvents(ctx context.Context) ([]service.ReefDataEvent, error) {
	rows, err := r.db.Query(ctx, reefEventSelect+` ORDER BY e.survey_date DESC,p.name_zh,e.depth_m,e.event_time,e.id`)
	if err != nil {
		return nil, translateError(err)
	}
	defer rows.Close()
	out := []service.ReefDataEvent{}
	for rows.Next() {
		var raw []byte
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		var e service.ReefDataEvent
		if err = json.Unmarshal(raw, &e); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, translateError(rows.Err())
}

func (r ReefDataRepository) Codes(ctx context.Context) ([]service.ReefDataCode, error) {
	rows, err := r.db.Query(ctx, `SELECT code,numeric_code,name_zh,is_active FROM substrate_type ORDER BY sort_order,code`)
	if err != nil {
		return nil, translateError(err)
	}
	defer rows.Close()
	out := []service.ReefDataCode{}
	for rows.Next() {
		var c service.ReefDataCode
		if err = rows.Scan(&c.Code, &c.NumericCode, &c.NameZH, &c.Active); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, translateError(rows.Err())
}

func (r ReefDataRepository) Event(ctx context.Context, id int) (service.ReefDataDetail, error) {
	detail := service.ReefDataDetail{Transects: []service.ReefDataTransect{}}
	var raw []byte
	err := r.db.QueryRow(ctx, reefEventSelect+` WHERE e.id=$1`, id).Scan(&raw)
	if err != nil {
		return detail, translateError(err)
	}
	if err = json.Unmarshal(raw, &detail.Event); err != nil {
		return detail, err
	}
	rows, err := r.db.Query(ctx, `SELECT id FROM transect WHERE event_id=$1 ORDER BY method`, detail.Event.EventID)
	if err != nil {
		return detail, translateError(err)
	}
	ids := []int{}
	for rows.Next() {
		var tid int
		if err = rows.Scan(&tid); err != nil {
			rows.Close()
			return detail, err
		}
		ids = append(ids, tid)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return detail, translateError(err)
	}
	for _, tid := range ids {
		t, e := r.Transect(ctx, tid, false)
		if e != nil {
			return detail, e
		}
		detail.Transects = append(detail.Transects, t)
	}
	return detail, nil
}

func (r ReefDataRepository) Transect(ctx context.Context, id int, lock bool) (service.ReefDataTransect, error) {
	var t service.ReefDataTransect
	// The parent lock serializes UI edits; the fingerprint also catches import changes.
	if lock {
		var n int
		if err := r.db.QueryRow(ctx, `SELECT id FROM transect WHERE id=$1 FOR UPDATE`, id).Scan(&n); err != nil {
			return t, translateError(err)
		}
	}
	var raw []byte
	err := r.db.QueryRow(ctx, `SELECT jsonb_build_object(
 'id',t.id,'event_id',t.event_id,'method',t.method,'start_time',t.start_time,
 'water_temp_c',t.water_temp_c,'visibility_min_m',t.visibility_min_m,'visibility_max_m',t.visibility_max_m,
 'comments',t.comments,'rkc_bleaching_note',t.rkc_bleaching_note,
 'points',COALESCE((SELECT jsonb_agg(to_jsonb(p) ORDER BY p.substrate_layer,p.position_m,p.id) FROM substrate_point p WHERE p.transect_id=t.id),'[]'::jsonb),
 'bleaching',COALESCE((SELECT jsonb_agg(to_jsonb(b) ORDER BY b.segment,b.id) FROM substrate_bleaching b WHERE b.transect_id=t.id),'[]'::jsonb),
 'belt',COALESCE((SELECT jsonb_agg(to_jsonb(b)||jsonb_build_object('taxon_group',x.taxon_group,'name_zh',x.name_zh,'name_en',COALESCE(x.name_en,''),'size_class',COALESCE(x.size_class,''),'is_aggregate',x.is_aggregate) ORDER BY x.sort_order,x.id,b.segment,b.id) FROM belt_observation b JOIN taxon x ON x.id=b.taxon_id WHERE b.transect_id=t.id),'[]'::jsonb),
 'impacts',COALESCE((SELECT jsonb_agg(to_jsonb(i)||jsonb_build_object('impact_group',x.impact_group,'name_zh',x.name_zh,'name_en',COALESCE(x.name_en,''),'value_type',x.value_type,'has_raw_count',x.has_raw_count) ORDER BY x.sort_order,x.id,i.segment,i.id) FROM impact_observation i JOIN impact_type x ON x.id=i.impact_type_id WHERE i.transect_id=t.id),'[]'::jsonb),
 'participants',COALESCE((SELECT jsonb_agg(to_jsonb(p)||jsonb_build_object('name_zh',COALESCE(d.name_zh,''),'name_en',COALESCE(d.name_en,''),'reef_check_code',COALESCE(d.reef_check_code,'')) ORDER BY p.role,p.id) FROM transect_participant p JOIN diver d ON d.id=p.diver_id WHERE p.transect_id=t.id),'[]'::jsonb))
 FROM transect t WHERE t.id=$1 AND t.event_id IS NOT NULL`, id).Scan(&raw)
	if err != nil {
		return t, translateError(err)
	}
	if err = json.Unmarshal(raw, &t); err != nil {
		return t, err
	}
	t.Version = t.Revision()
	return t, nil
}

// Update must run in the same transaction as Transect(lock=true), validation and audit logging.
// Only existing child rows are updated: missing methods, layers, taxa and roster links stay missing.
func (r ReefDataRepository) Update(ctx context.Context, id int, u service.ReefDataUpdate) (service.ReefDataTransect, error) {
	if m := u.Metadata; m != nil {
		_, err := r.db.Exec(ctx, `UPDATE transect SET start_time=$2,water_temp_c=$3,visibility_min_m=$4,visibility_max_m=$5,comments=$6,rkc_bleaching_note=$7 WHERE id=$1`, id, m.StartTime, m.WaterTemp, m.VisibilityMin, m.VisibilityMax, m.Comments, m.RKCNote)
		if err != nil {
			return service.ReefDataTransect{}, translateError(err)
		}
	}
	for _, c := range u.Changes {
		var query string
		var args []any
		switch c.Kind {
		case "point":
			query = `UPDATE substrate_point SET substrate_code=$3 WHERE transect_id=$1 AND id=$2`
			args = []any{id, c.ID, c.Code}
		case "bleaching":
			query = `UPDATE substrate_bleaching SET hc_bleached_count=$3,sc_bleached_count=$4 WHERE transect_id=$1 AND id=$2`
			args = []any{id, c.ID, c.HC, c.SC}
		case "belt":
			query = `UPDATE belt_observation SET count=$3 WHERE transect_id=$1 AND id=$2`
			args = []any{id, c.ID, c.Value}
		case "impact":
			query = `UPDATE impact_observation SET raw_value=$3 WHERE transect_id=$1 AND id=$2`
			args = []any{id, c.ID, c.Value}
		default:
			return service.ReefDataTransect{}, service.ErrValidation
		}
		tag, err := r.db.Exec(ctx, query, args...)
		if err != nil {
			return service.ReefDataTransect{}, translateError(err)
		}
		if tag.RowsAffected() != 1 {
			return service.ReefDataTransect{}, service.ErrConflict
		}
	}
	return r.Transect(ctx, id, false)
}
