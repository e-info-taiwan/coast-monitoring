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

func TestAdminUsersRequiresAdmin(t *testing.T) {
	handlers := testAuthHandlers()
	handlers.Auth = &fakeHTTPAuthService{sessionUser: activeVolunteer()}
	users := &fakeAdminUserService{}
	router := NewRouter(Dependencies{
		AuthHandlers:  handlers,
		AdminHandlers: &AdminHandlers{Users: users},
	})

	req := adminRequest(http.MethodGet, "/api/admin/users", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
	if users.listCalled {
		t.Fatal("admin user service was called for volunteer")
	}
}

func TestAdminUserResponseDoesNotExposeSecrets(t *testing.T) {
	googleSub := "raw-google-sub"
	user := service.User{
		ID:          uuid.New(),
		Email:       "volunteer@example.com",
		Name:        "Volunteer",
		Role:        policy.RoleVolunteer,
		Status:      policy.StatusActive,
		GoogleSub:   &googleSub,
		HasPassword: true,
		CreatedAt:   time.Date(2026, 5, 28, 1, 2, 3, 0, time.UTC),
		UpdatedAt:   time.Date(2026, 5, 28, 4, 5, 6, 0, time.UTC),
	}
	handlers := testAuthHandlers()
	handlers.Auth = &fakeHTTPAuthService{sessionUser: activeAdminHTTP()}
	router := NewRouter(Dependencies{
		AuthHandlers:  handlers,
		AdminHandlers: &AdminHandlers{Users: &fakeAdminUserService{users: []service.User{user}}},
	})

	req := adminRequest(http.MethodGet, "/api/admin/users", nil)
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
	got := body[0]
	for _, forbidden := range []string{"password_hash", "passwordHash", "google_sub", "googleSub"} {
		if _, ok := got[forbidden]; ok {
			t.Fatalf("admin user response leaked %q: %#v", forbidden, got)
		}
	}
	if got["hasGoogle"] != true || got["hasPassword"] != true {
		t.Fatalf("login flags = hasGoogle %v hasPassword %v, want true/true", got["hasGoogle"], got["hasPassword"])
	}
}

func TestVolunteerCannotCallAdminLocations(t *testing.T) {
	handlers := testAuthHandlers()
	handlers.Auth = &fakeHTTPAuthService{sessionUser: activeVolunteer()}
	catalog := &fakeAdminCatalogService{}
	router := NewRouter(Dependencies{
		AuthHandlers:  handlers,
		AdminHandlers: &AdminHandlers{Catalog: catalog},
	})

	req := adminRequest(http.MethodGet, "/api/admin/locations", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
	if catalog.listLocationsCalled {
		t.Fatal("admin catalog service was called for volunteer")
	}
}

func TestAdminCanCreateLocation(t *testing.T) {
	handlers := testAuthHandlers()
	admin := activeAdminHTTP()
	handlers.Auth = &fakeHTTPAuthService{sessionUser: admin}
	catalog := &fakeAdminCatalogService{}
	auditLogs := &fakeAdminAuditLogService{}
	router := NewRouter(Dependencies{
		AuthHandlers: handlers,
		AdminHandlers: &AdminHandlers{
			Catalog:   catalog,
			AuditLogs: auditLogs,
		},
	})

	req := adminRequest(http.MethodPost, "/api/admin/locations", bytes.NewBufferString(`{"chineseName":" 澎湖 ","englishName":" Penghu "}`))
	req.RemoteAddr = "192.0.2.50:12345"
	req.Header.Set("User-Agent", "admin-test")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d, body %s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if catalog.createdLocation.ChineseName != " 澎湖 " || catalog.createdLocation.EnglishName != " Penghu " {
		t.Fatalf("create input = %+v, want raw request values passed to service", catalog.createdLocation)
	}
	var body CatalogResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("Decode body error = %v", err)
	}
	if body.ChineseName != "澎湖" || body.EnglishName != "Penghu" {
		t.Fatalf("response = %+v, want created location DTO", body)
	}
	if len(auditLogs.created) != 1 {
		t.Fatalf("audit log count = %d, want 1", len(auditLogs.created))
	}
	audit := auditLogs.created[0]
	if audit.Action != repository.AuditActionCreate || audit.TargetTable != "locations" || audit.ActorUserID == nil || *audit.ActorUserID != admin.ID {
		t.Fatalf("audit log = %+v, want create locations by admin", audit)
	}
}

func adminRequest(method, target string, body *bytes.Buffer) *http.Request {
	var reader *bytes.Buffer
	if body == nil {
		reader = bytes.NewBuffer(nil)
	} else {
		reader = body
	}
	req := httptest.NewRequest(method, target, reader)
	req.Header.Set("X-CSRF-Token", "csrf-token")
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "coast_session", Value: "session-token"})
	return req
}

func activeAdminHTTP() policy.User {
	return policy.User{
		ID:     uuid.New(),
		Email:  "admin@example.com",
		Name:   "Admin",
		Role:   policy.RoleAdmin,
		Status: policy.StatusActive,
	}
}

func activeVolunteer() policy.User {
	return policy.User{
		ID:     uuid.New(),
		Email:  "volunteer@example.com",
		Name:   "Volunteer",
		Role:   policy.RoleVolunteer,
		Status: policy.StatusActive,
	}
}

type fakeAdminUserService struct {
	users      []service.User
	listCalled bool
}

func (s *fakeAdminUserService) ListUsers(ctx context.Context, actor policy.User) ([]service.User, error) {
	s.listCalled = true
	return s.users, nil
}

func (s *fakeAdminUserService) CreateUser(ctx context.Context, actor policy.User, input service.CreateUserInput) (service.User, error) {
	return service.User{ID: uuid.New(), Email: input.Email, Name: input.Name, Role: input.Role, Status: input.Status}, nil
}

func (s *fakeAdminUserService) UpdateUser(ctx context.Context, actor policy.User, id uuid.UUID, input service.UpdateUserInput) (service.User, error) {
	return service.User{ID: id}, nil
}

func (s *fakeAdminUserService) DisableUser(ctx context.Context, actor policy.User, id uuid.UUID) error {
	return nil
}

type fakeAdminCatalogService struct {
	createdLocation     service.CatalogInput
	listLocationsCalled bool
}

func (s *fakeAdminCatalogService) ListLocations(ctx context.Context, actor policy.User) ([]service.Location, error) {
	s.listLocationsCalled = true
	return nil, nil
}

func (s *fakeAdminCatalogService) CreateLocation(ctx context.Context, actor policy.User, input service.CatalogInput) (service.Location, error) {
	s.createdLocation = input
	return service.Location{
		ID:          uuid.New(),
		ChineseName: "澎湖",
		EnglishName: "Penghu",
		CreatedAt:   time.Date(2026, 5, 28, 1, 2, 3, 0, time.UTC),
		UpdatedAt:   time.Date(2026, 5, 28, 1, 2, 3, 0, time.UTC),
	}, nil
}

func (s *fakeAdminCatalogService) UpdateLocation(ctx context.Context, actor policy.User, id uuid.UUID, input service.CatalogInput) (service.Location, error) {
	return service.Location{ID: id, ChineseName: input.ChineseName, EnglishName: input.EnglishName}, nil
}

func (s *fakeAdminCatalogService) DeleteLocation(ctx context.Context, actor policy.User, id uuid.UUID) error {
	return nil
}

func (s *fakeAdminCatalogService) ListSpecies(ctx context.Context, actor policy.User) ([]service.Species, error) {
	return nil, nil
}

func (s *fakeAdminCatalogService) CreateSpecies(ctx context.Context, actor policy.User, input service.CatalogInput) (service.Species, error) {
	return service.Species{ID: uuid.New(), ChineseName: input.ChineseName, EnglishName: input.EnglishName}, nil
}

func (s *fakeAdminCatalogService) UpdateSpecies(ctx context.Context, actor policy.User, id uuid.UUID, input service.CatalogInput) (service.Species, error) {
	return service.Species{ID: id, ChineseName: input.ChineseName, EnglishName: input.EnglishName}, nil
}

func (s *fakeAdminCatalogService) DeleteSpecies(ctx context.Context, actor policy.User, id uuid.UUID) error {
	return nil
}

type fakeAdminAuditLogService struct {
	created []repository.CreateAuditLogRecord
}

func (s *fakeAdminAuditLogService) ListAuditLogs(ctx context.Context) ([]repository.AuditLog, error) {
	return nil, nil
}

func (s *fakeAdminAuditLogService) CreateAuditLog(ctx context.Context, input repository.CreateAuditLogRecord) (repository.AuditLog, error) {
	s.created = append(s.created, input)
	return repository.AuditLog{ID: uuid.New(), Action: input.Action, TargetTable: input.TargetTable, TargetID: input.TargetID}, nil
}
