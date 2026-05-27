package httpx

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"coast-monitoring/internal/policy"
	"coast-monitoring/internal/repository"
	"coast-monitoring/internal/service"

	"github.com/google/uuid"
)

func TestAppRoutesRequireSession(t *testing.T) {
	router := NewRouter(Dependencies{
		AuthHandlers: testAuthHandlers(),
		AppHandlers:  &AppHandlers{},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/app/observations", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestAppRoutesDoNotExposeUsersEndpoint(t *testing.T) {
	handlers := testAuthHandlers()
	handlers.Auth = &fakeHTTPAuthService{sessionUser: activeVolunteer()}
	router := NewRouter(Dependencies{
		AuthHandlers: handlers,
		AppHandlers:  &AppHandlers{},
	})

	req := appRequest(http.MethodGet, "/api/app/users", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestVolunteerOnlySeesOwnObservations(t *testing.T) {
	volunteer := activeVolunteer()
	otherID := uuid.New()
	observations := &fakeAppObservationService{observations: []service.Observation{
		testObservation(volunteer.ID, 2),
		testObservation(otherID, 4),
	}}
	router := appRouterForUser(volunteer, observations, nil, nil)

	req := appRequest(http.MethodGet, "/api/app/observations", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var body []map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("Decode body error = %v", err)
	}
	if len(body) != 1 {
		t.Fatalf("body length = %d, want 1", len(body))
	}
	if got := int(body[0]["count"].(float64)); got != 2 {
		t.Fatalf("count = %d, want volunteer observation", got)
	}
	for _, forbidden := range []string{"observerId", "observer", "user", "userId", "email", "role"} {
		if _, ok := body[0][forbidden]; ok {
			t.Fatalf("app observation response leaked %q: %#v", forbidden, body[0])
		}
	}
}

func TestVolunteerCannotUpdateOtherUsersObservation(t *testing.T) {
	volunteer := activeVolunteer()
	otherObservation := testObservation(uuid.New(), 1)
	observations := &fakeAppObservationService{observations: []service.Observation{otherObservation}}
	auditLogs := &fakeAdminAuditLogService{}
	router := appRouterForUser(volunteer, observations, auditLogs, nil)

	req := appRequest(http.MethodPatch, "/api/app/observations/"+otherObservation.ID.String(), bytes.NewBufferString(appObservationJSON(uuid.New(), uuid.New(), uuid.New())))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d, body %s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
	if len(auditLogs.created) != 0 {
		t.Fatalf("audit log count = %d, want 0 for forbidden update", len(auditLogs.created))
	}
}

func TestAdminCanUseAppObservationRoute(t *testing.T) {
	admin := activeAdminHTTP()
	observations := &fakeAppObservationService{}
	auditLogs := &fakeAdminAuditLogService{}
	runner := &fakeAdminMutationRunner{services: AdminMutationServices{
		Observations: observations,
		AuditLogs:    auditLogs,
	}}
	router := appRouterForUser(admin, observations, auditLogs, runner)
	locationID := uuid.New()
	speciesID := uuid.New()
	otherObserverID := uuid.New()

	req := appRequest(http.MethodPost, "/api/app/observations", bytes.NewBufferString(appObservationJSON(locationID, speciesID, otherObserverID)))
	req.RemoteAddr = "192.0.2.55:12345"
	req.Header.Set("User-Agent", "app-test")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d, body %s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if runner.calls != 1 {
		t.Fatalf("mutation runner calls = %d, want 1", runner.calls)
	}
	if observations.created.ObserverID != admin.ID {
		t.Fatalf("created observer = %s, want current admin %s", observations.created.ObserverID, admin.ID)
	}
	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("Decode body error = %v", err)
	}
	if _, ok := body["observerId"]; ok {
		t.Fatalf("app create response leaked observerId: %#v", body)
	}
	if len(auditLogs.created) != 1 {
		t.Fatalf("audit log count = %d, want 1", len(auditLogs.created))
	}
	audit := auditLogs.created[0]
	if audit.Action != repository.AuditActionCreate || audit.TargetTable != "observations" || audit.ActorUserID == nil || *audit.ActorUserID != admin.ID {
		t.Fatalf("audit log = %+v, want create observations by admin", audit)
	}
}

func appRouterForUser(user policy.User, observations *fakeAppObservationService, auditLogs *fakeAdminAuditLogService, runner AdminMutationRunner) http.Handler {
	handlers := testAuthHandlers()
	handlers.Auth = &fakeHTTPAuthService{sessionUser: user}
	if runner == nil {
		runner = &fakeAdminMutationRunner{services: AdminMutationServices{
			Observations: observations,
			AuditLogs:    auditLogs,
		}}
	}
	return NewRouter(Dependencies{
		AuthHandlers: handlers,
		AppHandlers: &AppHandlers{
			Catalog:      &fakeAdminCatalogService{},
			Observations: observations,
			Mutations:    runner,
		},
	})
}

func appRequest(method, target string, body *bytes.Buffer) *http.Request {
	req := adminRequest(method, target, body)
	return req
}

func appObservationJSON(locationID, speciesID, observerID uuid.UUID) string {
	return `{"observedOn":"2026-05-27","locationId":"` + locationID.String() + `","speciesId":"` + speciesID.String() + `","observerId":"` + observerID.String() + `","count":3,"notes":" app notes "}`
}

func testObservation(observerID uuid.UUID, count int) service.Observation {
	return service.Observation{
		ID:         uuid.New(),
		ObservedOn: time.Date(2026, 5, 27, 0, 0, 0, 0, time.UTC),
		LocationID: uuid.New(),
		SpeciesID:  uuid.New(),
		ObserverID: observerID,
		Count:      count,
		Notes:      "notes",
		CreatedAt:  time.Date(2026, 5, 27, 1, 2, 3, 0, time.UTC),
		UpdatedAt:  time.Date(2026, 5, 27, 4, 5, 6, 0, time.UTC),
	}
}

type fakeAppObservationService struct {
	observations []service.Observation
	created      service.ObservationInput
	updated      service.ObservationInput
	deletedID    uuid.UUID
}

func (s *fakeAppObservationService) ListForApp(ctx context.Context, actor policy.User) ([]service.Observation, error) {
	if actor.Role == policy.RoleVolunteer {
		var out []service.Observation
		for _, observation := range s.observations {
			if observation.ObserverID == actor.ID {
				out = append(out, observation)
			}
		}
		return out, nil
	}
	return append([]service.Observation(nil), s.observations...), nil
}

func (s *fakeAppObservationService) ListForAdmin(ctx context.Context, actor policy.User) ([]service.Observation, error) {
	return append([]service.Observation(nil), s.observations...), nil
}

func (s *fakeAppObservationService) Create(ctx context.Context, actor policy.User, input service.ObservationInput) (service.Observation, error) {
	s.created = input
	return service.Observation{
		ID:         uuid.New(),
		ObservedOn: input.ObservedOn,
		LocationID: input.LocationID,
		SpeciesID:  input.SpeciesID,
		ObserverID: input.ObserverID,
		Count:      input.Count,
		Notes:      input.Notes,
		CreatedAt:  time.Date(2026, 5, 27, 1, 2, 3, 0, time.UTC),
		UpdatedAt:  time.Date(2026, 5, 27, 1, 2, 3, 0, time.UTC),
	}, nil
}

func (s *fakeAppObservationService) Update(ctx context.Context, actor policy.User, id uuid.UUID, input service.ObservationInput) (service.Observation, error) {
	for _, observation := range s.observations {
		if observation.ID == id && actor.Role == policy.RoleVolunteer && observation.ObserverID != actor.ID {
			return service.Observation{}, service.ErrForbidden
		}
	}
	s.updated = input
	return service.Observation{
		ID:         id,
		ObservedOn: input.ObservedOn,
		LocationID: input.LocationID,
		SpeciesID:  input.SpeciesID,
		ObserverID: input.ObserverID,
		Count:      input.Count,
		Notes:      input.Notes,
		CreatedAt:  time.Date(2026, 5, 27, 1, 2, 3, 0, time.UTC),
		UpdatedAt:  time.Date(2026, 5, 27, 4, 5, 6, 0, time.UTC),
	}, nil
}

func (s *fakeAppObservationService) Delete(ctx context.Context, actor policy.User, id uuid.UUID) error {
	s.deletedID = id
	return nil
}
