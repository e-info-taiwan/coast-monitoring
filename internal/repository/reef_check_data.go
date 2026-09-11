package repository

import (
	"coast-monitoring/internal/service"
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
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
  'participants',COALESCE((SELECT jsonb_agg(to_jsonb(p)||jsonb_build_object('name_zh',COALESCE(d.name_zh,''),'name_en',COALESCE(d.name_en,''),'reef_check_code',COALESCE(d.reef_check_code,''),'user_id',COALESCE(p.user_id,d.user_id),'user_email',COALESCE(u.email,''),'user_name',COALESCE(u.name,'')) ORDER BY p.role,p.id) FROM transect_participant p JOIN diver d ON d.id=p.diver_id LEFT JOIN users u ON u.id=COALESCE(p.user_id,d.user_id) WHERE p.transect_id=t.id),'[]'::jsonb))
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

var nonAlnumRe = regexp.MustCompile(`[^a-zA-Z0-9]+`)

func cleanSlug(s string) string {
	slug := strings.ToLower(nonAlnumRe.ReplaceAllString(s, "-"))
	slug = strings.Trim(slug, "-")
	if slug == "" {
		return "site"
	}
	return slug
}

func (r ReefDataRepository) CreateEvent(ctx context.Context, input service.ReefDataCreateInput) (service.ReefDataDetail, error) {
	var siteNameEN, siteNameZH string
	err := r.db.QueryRow(ctx, `SELECT COALESCE(name_en, ''), name_zh FROM site WHERE id=$1`, input.SiteID).Scan(&siteNameEN, &siteNameZH)
	if err != nil {
		return service.ReefDataDetail{}, translateError(err)
	}

	var surveyID int
	err = r.db.QueryRow(ctx, `
		INSERT INTO survey (site_id, start_date, end_date, label)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (site_id, start_date) DO UPDATE SET end_date=GREATEST(survey.end_date, EXCLUDED.end_date)
		RETURNING id
	`, input.SiteID, input.StartDate, input.EndDate, input.Label).Scan(&surveyID)
	if err != nil {
		return service.ReefDataDetail{}, translateError(err)
	}

	slug := cleanSlug(siteNameEN)
	if slug == "site" && siteNameZH != "" {
		slug = fmt.Sprintf("site-%d", input.SiteID)
	}
	timeSlug := strings.ReplaceAll(input.EventTime, ":", "-")
	baseEventID := fmt.Sprintf("%s_%s_%s_%.1fm", slug, input.SurveyDate, timeSlug, input.DepthM)
	finalEventID := baseEventID

	var exists bool
	for suffix := 1; ; suffix++ {
		err = r.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM event WHERE event_id=$1)`, finalEventID).Scan(&exists)
		if err != nil {
			return service.ReefDataDetail{}, translateError(err)
		}
		if !exists {
			break
		}
		finalEventID = fmt.Sprintf("%s_%d", baseEventID, suffix)
	}

	var eventID int
	err = r.db.QueryRow(ctx, `
		INSERT INTO event (survey_id, event_id, survey_date, event_time, depth_m)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`, surveyID, finalEventID, input.SurveyDate, input.EventTime, input.DepthM).Scan(&eventID)
	if err != nil {
		return service.ReefDataDetail{}, translateError(err)
	}

	for _, method := range input.Methods {
		var transectID int
		err = r.db.QueryRow(ctx, `
			INSERT INTO transect (event_id, method, depth_m, survey_date, start_time)
			VALUES ($1, $2, $3, $4, $5)
			RETURNING id
		`, finalEventID, method, input.DepthM, input.SurveyDate, input.EventTime).Scan(&transectID)
		if err != nil {
			return service.ReefDataDetail{}, translateError(err)
		}

		if method == "line" {
			for seg := 1; seg <= 4; seg++ {
				baseM := float64((seg - 1) * 25)
				for i := 0; i < 40; i++ {
					posM := baseM + float64(i)*0.5
					_, err = r.db.Exec(ctx, `
						INSERT INTO substrate_point (transect_id, segment, position_m, substrate_code, substrate_layer)
						VALUES ($1, $2, $3, 'NA', 'surface')
					`, transectID, seg, posM)
					if err != nil {
						return service.ReefDataDetail{}, translateError(err)
					}
				}
				_, err = r.db.Exec(ctx, `
					INSERT INTO substrate_bleaching (transect_id, segment, hc_bleached_count, sc_bleached_count)
					VALUES ($1, $2, 0, 0)
				`, transectID, seg)
				if err != nil {
					return service.ReefDataDetail{}, translateError(err)
				}
			}
		} else if method == "belt_fish" {
			taxaRows, err := r.db.Query(ctx, `SELECT id FROM taxon WHERE taxon_group='fish' AND is_active ORDER BY sort_order, id`)
			if err != nil {
				return service.ReefDataDetail{}, translateError(err)
			}
			var taxaIDs []int
			for taxaRows.Next() {
				var tid int
				if err := taxaRows.Scan(&tid); err == nil {
					taxaIDs = append(taxaIDs, tid)
				}
			}
			taxaRows.Close()
			for _, tid := range taxaIDs {
				for seg := 1; seg <= 4; seg++ {
					_, _ = r.db.Exec(ctx, `
						INSERT INTO belt_observation (transect_id, taxon_id, segment, count)
						VALUES ($1, $2, $3, 0)
					`, transectID, tid, seg)
				}
			}
		} else if method == "belt_invert" {
			taxaRows, err := r.db.Query(ctx, `SELECT id FROM taxon WHERE taxon_group IN ('invert', 'rare') AND is_active ORDER BY sort_order, id`)
			if err != nil {
				return service.ReefDataDetail{}, translateError(err)
			}
			var taxaIDs []int
			for taxaRows.Next() {
				var tid int
				if err := taxaRows.Scan(&tid); err == nil {
					taxaIDs = append(taxaIDs, tid)
				}
			}
			taxaRows.Close()
			for _, tid := range taxaIDs {
				for seg := 1; seg <= 4; seg++ {
					_, _ = r.db.Exec(ctx, `
						INSERT INTO belt_observation (transect_id, taxon_id, segment, count)
						VALUES ($1, $2, $3, 0)
					`, transectID, tid, seg)
				}
			}
			impactRows, err := r.db.Query(ctx, `SELECT id FROM impact_type WHERE is_active ORDER BY sort_order, id`)
			if err != nil {
				return service.ReefDataDetail{}, translateError(err)
			}
			var impactIDs []int
			for impactRows.Next() {
				var iid int
				if err := impactRows.Scan(&iid); err == nil {
					impactIDs = append(impactIDs, iid)
				}
			}
			impactRows.Close()
			for _, iid := range impactIDs {
				for seg := 1; seg <= 4; seg++ {
					_, _ = r.db.Exec(ctx, `
						INSERT INTO impact_observation (transect_id, impact_type_id, segment, raw_value)
						VALUES ($1, $2, $3, 0)
					`, transectID, iid, seg)
				}
			}
		}
	}

	return r.Event(ctx, eventID)
}

func (r ReefDataRepository) DeleteEvent(ctx context.Context, id int) (service.ReefDataDetail, error) {
	detail, err := r.Event(ctx, id)
	if err != nil {
		return detail, err
	}
	var surveyID int
	err = r.db.QueryRow(ctx, `SELECT survey_id FROM event WHERE id=$1`, id).Scan(&surveyID)
	if err != nil {
		return detail, translateError(err)
	}
	tag, err := r.db.Exec(ctx, `DELETE FROM event WHERE id=$1`, id)
	if err != nil {
		return detail, translateError(err)
	}
	if tag.RowsAffected() == 0 {
		return detail, service.ErrNotFound
	}
	var remaining int
	_ = r.db.QueryRow(ctx, `SELECT count(*) FROM event WHERE survey_id=$1`, surveyID).Scan(&remaining)
	if remaining == 0 {
		_, _ = r.db.Exec(ctx, `DELETE FROM survey WHERE id=$1`, surveyID)
	}
	return detail, nil
}

func (r ReefDataRepository) Sites(ctx context.Context) ([]service.ReefDataSite, error) {
	rows, err := r.db.Query(ctx, `SELECT id, name_zh, COALESCE(name_en, ''), COALESCE(region, ''), COALESCE(county, ''), COALESCE(location, ''), latitude, longitude FROM site WHERE is_active ORDER BY name_zh`)
	if err != nil {
		return nil, translateError(err)
	}
	defer rows.Close()
	var out []service.ReefDataSite
	for rows.Next() {
		var s service.ReefDataSite
		if err := rows.Scan(&s.ID, &s.NameZH, &s.NameEN, &s.Region, &s.County, &s.Location, &s.Latitude, &s.Longitude); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, translateError(rows.Err())
}

func (r ReefDataRepository) Users(ctx context.Context) ([]service.ReefDataUser, error) {
	rows, err := r.db.Query(ctx, `SELECT id, email, name, role FROM users WHERE status = 'active' ORDER BY email`)
	if err != nil {
		return nil, translateError(err)
	}
	defer rows.Close()
	var out []service.ReefDataUser
	for rows.Next() {
		var u service.ReefDataUser
		if err := rows.Scan(&u.ID, &u.Email, &u.Name, &u.Role); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, translateError(rows.Err())
}

func (r ReefDataRepository) Divers(ctx context.Context) ([]service.ReefDataDiver, error) {
	rows, err := r.db.Query(ctx, `SELECT d.id, COALESCE(d.name_zh, ''), COALESCE(d.name_en, ''), COALESCE(d.reef_check_code, ''), d.user_id, COALESCE(u.email, '') FROM diver d LEFT JOIN users u ON u.id = d.user_id WHERE d.is_active ORDER BY d.name_zh, d.name_en`)
	if err != nil {
		return nil, translateError(err)
	}
	defer rows.Close()
	var out []service.ReefDataDiver
	for rows.Next() {
		var d service.ReefDataDiver
		if err := rows.Scan(&d.ID, &d.NameZH, &d.NameEN, &d.ReefCheckCode, &d.UserID, &d.UserEmail); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, translateError(rows.Err())
}

func (r ReefDataRepository) AddParticipant(ctx context.Context, transectID int, p service.ReefDataParticipantInput) error {
	diverID := 0
	if p.DiverID != nil && *p.DiverID > 0 {
		diverID = *p.DiverID
		if p.UserID != nil {
			_, _ = r.db.Exec(ctx, `UPDATE diver SET user_id=$2 WHERE id=$1 AND user_id IS NULL`, diverID, p.UserID)
		}
	} else if p.UserID != nil {
		err := r.db.QueryRow(ctx, `SELECT id FROM diver WHERE user_id=$1`, p.UserID).Scan(&diverID)
		if err != nil {
			var name, email string
			if err := r.db.QueryRow(ctx, `SELECT name, email FROM users WHERE id=$1`, p.UserID).Scan(&name, &email); err != nil {
				return translateError(err)
			}
			displayName := p.NameZH
			if displayName == "" {
				displayName = name
			}
			if displayName == "" {
				displayName = email
			}
			err = r.db.QueryRow(ctx, `INSERT INTO diver (name_zh, user_id) VALUES ($1, $2) RETURNING id`, displayName, p.UserID).Scan(&diverID)
			if err != nil {
				return translateError(err)
			}
		}
	} else if p.NameZH != "" || p.NameEN != "" {
		err := r.db.QueryRow(ctx, `INSERT INTO diver (name_zh, name_en) VALUES ($1, $2) RETURNING id`, p.NameZH, p.NameEN).Scan(&diverID)
		if err != nil {
			return translateError(err)
		}
	}
	if diverID == 0 {
		return fmt.Errorf("%w: 請指定潛水員或使用者", service.ErrValidation)
	}
	role := p.Role
	if role == "" {
		role = "member"
	}
	_, err := r.db.Exec(ctx, `
		INSERT INTO transect_participant (transect_id, diver_id, user_id, role)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (transect_id, diver_id, role) DO UPDATE SET user_id=EXCLUDED.user_id
	`, transectID, diverID, p.UserID, role)
	return translateError(err)
}

func (r ReefDataRepository) RemoveParticipant(ctx context.Context, participantID int) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM transect_participant WHERE id=$1`, participantID)
	if err != nil {
		return translateError(err)
	}
	if tag.RowsAffected() == 0 {
		return service.ErrNotFound
	}
	return nil
}
