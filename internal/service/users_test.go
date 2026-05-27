package service

import (
	"context"
	"errors"
	"testing"

	"coast-monitoring/internal/auth"
	"coast-monitoring/internal/policy"

	"github.com/google/uuid"
)

func TestVolunteerCannotListUsers(t *testing.T) {
	svc := UserService{Users: &fakeUserRepository{}}
	actor := policy.User{
		ID:     uuid.New(),
		Email:  "volunteer@example.com",
		Name:   "Volunteer",
		Role:   policy.RoleVolunteer,
		Status: policy.StatusActive,
	}

	_, err := svc.ListUsers(context.Background(), actor)

	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("ListUsers error = %v, want %v", err, ErrForbidden)
	}
}

func TestAdminCanCreateVolunteer(t *testing.T) {
	repo := &fakeUserRepository{}
	svc := UserService{Users: repo}
	actor := policy.User{
		ID:     uuid.New(),
		Email:  "admin@example.com",
		Name:   "Admin",
		Role:   policy.RoleAdmin,
		Status: policy.StatusActive,
	}

	user, err := svc.CreateUser(context.Background(), actor, CreateUserInput{
		Email:    "VOLUNTEER@EXAMPLE.COM",
		Name:     " Volunteer ",
		Role:     policy.RoleVolunteer,
		Status:   policy.StatusActive,
		Password: "password123",
	})
	if err != nil {
		t.Fatalf("CreateUser error = %v", err)
	}

	if user.Email != "volunteer@example.com" {
		t.Fatalf("created email = %q, want lowercase email", user.Email)
	}
	if user.Name != "Volunteer" {
		t.Fatalf("created name = %q, want trimmed name", user.Name)
	}
	if user.Role != policy.RoleVolunteer {
		t.Fatalf("created role = %q, want %q", user.Role, policy.RoleVolunteer)
	}
	if !user.HasPassword {
		t.Fatal("created user HasPassword = false, want true")
	}
	if repo.created.PasswordHash == "" {
		t.Fatal("password hash was not stored")
	}
	if repo.created.PasswordHash == "password123" {
		t.Fatal("password was stored without hashing")
	}
	if !auth.VerifyPassword(repo.created.PasswordHash, "password123") {
		t.Fatal("stored password hash does not verify")
	}
}

func TestCreateUserRejectsInvalidEmail(t *testing.T) {
	svc := UserService{Users: &fakeUserRepository{}}

	_, err := svc.CreateUser(context.Background(), activeAdmin(), CreateUserInput{
		Email:    "not-an-email",
		Name:     "Volunteer",
		Role:     policy.RoleVolunteer,
		Status:   policy.StatusActive,
		Password: "password123",
	})

	if !errors.Is(err, ErrValidation) {
		t.Fatalf("CreateUser error = %v, want %v", err, ErrValidation)
	}
}

func TestCreateActiveUserRequiresLoginMechanism(t *testing.T) {
	svc := UserService{Users: &fakeUserRepository{}}

	_, err := svc.CreateUser(context.Background(), activeAdmin(), CreateUserInput{
		Email:  "volunteer@example.com",
		Name:   "Volunteer",
		Role:   policy.RoleVolunteer,
		Status: policy.StatusActive,
	})

	if !errors.Is(err, ErrValidation) {
		t.Fatalf("CreateUser error = %v, want %v", err, ErrValidation)
	}
}

func TestCreateDisabledUserAllowsNoLoginMechanism(t *testing.T) {
	svc := UserService{Users: &fakeUserRepository{}}

	_, err := svc.CreateUser(context.Background(), activeAdmin(), CreateUserInput{
		Email:  "volunteer@example.com",
		Name:   "Volunteer",
		Role:   policy.RoleVolunteer,
		Status: policy.StatusDisabled,
	})

	if err != nil {
		t.Fatalf("CreateUser error = %v", err)
	}
}

func TestCreateUserTrimsGoogleSub(t *testing.T) {
	repo := &fakeUserRepository{}
	svc := UserService{Users: repo}
	googleSub := " google-sub "

	_, err := svc.CreateUser(context.Background(), activeAdmin(), CreateUserInput{
		Email:     "volunteer@example.com",
		Name:      "Volunteer",
		Role:      policy.RoleVolunteer,
		Status:    policy.StatusActive,
		GoogleSub: &googleSub,
	})
	if err != nil {
		t.Fatalf("CreateUser error = %v", err)
	}

	if repo.created.GoogleSub == nil || *repo.created.GoogleSub != "google-sub" {
		t.Fatalf("created google sub = %v, want trimmed value", repo.created.GoogleSub)
	}
}

func TestUpdateUserRejectsEmptyPassword(t *testing.T) {
	id := uuid.New()
	svc := UserService{Users: &fakeUserRepository{users: []User{{
		ID:          id,
		Email:       "volunteer@example.com",
		Name:        "Volunteer",
		Role:        policy.RoleVolunteer,
		Status:      policy.StatusActive,
		HasPassword: true,
	}}}}
	password := " "

	_, err := svc.UpdateUser(context.Background(), activeAdmin(), id, UpdateUserInput{
		Password: &password,
	})

	if !errors.Is(err, ErrValidation) {
		t.Fatalf("UpdateUser error = %v, want %v", err, ErrValidation)
	}
}

func TestUpdateUserPatchNameAndRoleSucceeds(t *testing.T) {
	id := uuid.New()
	repo := &fakeUserRepository{
		users: []User{{
			ID:          id,
			Email:       "volunteer@example.com",
			Name:        "Volunteer",
			Role:        policy.RoleVolunteer,
			Status:      policy.StatusActive,
			HasPassword: true,
		}},
	}
	svc := UserService{Users: repo}
	name := " Updated Name "
	role := policy.RoleAdmin

	user, err := svc.UpdateUser(context.Background(), activeAdmin(), id, UpdateUserInput{
		Name: &name,
		Role: &role,
	})
	if err != nil {
		t.Fatalf("UpdateUser error = %v", err)
	}

	if user.Name != "Updated Name" {
		t.Fatalf("updated name = %q, want trimmed patched name", user.Name)
	}
	if user.Role != policy.RoleAdmin {
		t.Fatalf("updated role = %q, want %q", user.Role, policy.RoleAdmin)
	}
	if user.Email != "volunteer@example.com" {
		t.Fatalf("updated email = %q, want existing email preserved", user.Email)
	}
	if user.Status != policy.StatusActive {
		t.Fatalf("updated status = %q, want existing status preserved", user.Status)
	}
}

func TestUpdateUserNilFieldsPreserveExistingValues(t *testing.T) {
	id := uuid.New()
	googleSub := "google-sub"
	repo := &fakeUserRepository{
		users: []User{{
			ID:          id,
			Email:       "volunteer@example.com",
			Name:        "Volunteer",
			Role:        policy.RoleVolunteer,
			Status:      policy.StatusActive,
			GoogleSub:   &googleSub,
			HasPassword: false,
		}},
	}
	svc := UserService{Users: repo}

	user, err := svc.UpdateUser(context.Background(), activeAdmin(), id, UpdateUserInput{})
	if err != nil {
		t.Fatalf("UpdateUser error = %v", err)
	}

	if user.Email != "volunteer@example.com" || user.Name != "Volunteer" || user.Role != policy.RoleVolunteer || user.Status != policy.StatusActive {
		t.Fatalf("nil patch did not preserve existing user: %+v", user)
	}
	if user.GoogleSub == nil || *user.GoogleSub != googleSub {
		t.Fatalf("nil GoogleSub patch = %v, want existing GoogleSub preserved", user.GoogleSub)
	}
}

func TestUpdateUserRejectsActivationWithoutLoginMechanism(t *testing.T) {
	id := uuid.New()
	repo := &fakeUserRepository{
		users: []User{
			{
				ID:          id,
				Email:       "volunteer@example.com",
				Name:        "Volunteer",
				Role:        policy.RoleVolunteer,
				Status:      policy.StatusDisabled,
				HasPassword: false,
			},
		},
	}
	svc := UserService{Users: repo}
	status := policy.StatusActive

	_, err := svc.UpdateUser(context.Background(), activeAdmin(), id, UpdateUserInput{
		Status: &status,
	})

	if !errors.Is(err, ErrValidation) {
		t.Fatalf("UpdateUser error = %v, want %v", err, ErrValidation)
	}
}

func TestUpdateUserAllowsActivationWithNewPassword(t *testing.T) {
	id := uuid.New()
	repo := &fakeUserRepository{
		users: []User{
			{
				ID:          id,
				Email:       "volunteer@example.com",
				Name:        "Volunteer",
				Role:        policy.RoleVolunteer,
				Status:      policy.StatusDisabled,
				HasPassword: false,
			},
		},
	}
	svc := UserService{Users: repo}
	password := "password123"
	status := policy.StatusActive

	user, err := svc.UpdateUser(context.Background(), activeAdmin(), id, UpdateUserInput{
		Status:   &status,
		Password: &password,
	})
	if err != nil {
		t.Fatalf("UpdateUser error = %v", err)
	}

	if user.Status != policy.StatusActive {
		t.Fatalf("updated status = %q, want %q", user.Status, policy.StatusActive)
	}
	if !user.HasPassword {
		t.Fatal("updated user HasPassword = false, want true")
	}
}

func TestDisabledActorCannotListUsers(t *testing.T) {
	svc := UserService{Users: &fakeUserRepository{}}
	actor := activeAdmin()
	actor.Status = policy.StatusDisabled

	_, err := svc.ListUsers(context.Background(), actor)

	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("ListUsers error = %v, want %v", err, ErrForbidden)
	}
}

func activeAdmin() policy.User {
	return policy.User{
		ID:     uuid.New(),
		Email:  "admin@example.com",
		Name:   "Admin",
		Role:   policy.RoleAdmin,
		Status: policy.StatusActive,
	}
}

func stringPtr(value string) *string {
	return &value
}

type fakeUserRepository struct {
	users   []User
	created CreateUserRecord
}

func (r *fakeUserRepository) ListUsers(ctx context.Context) ([]User, error) {
	return append([]User(nil), r.users...), nil
}

func (r *fakeUserRepository) GetUser(ctx context.Context, id uuid.UUID) (User, error) {
	for _, user := range r.users {
		if user.ID == id {
			return user, nil
		}
	}
	return User{}, ErrNotFound
}

func (r *fakeUserRepository) CreateUser(ctx context.Context, input CreateUserRecord) (User, error) {
	r.created = input
	user := User{
		ID:          uuid.New(),
		Email:       input.Email,
		Name:        input.Name,
		Role:        input.Role,
		Status:      input.Status,
		GoogleSub:   input.GoogleSub,
		HasPassword: input.PasswordHash != "",
	}
	r.users = append(r.users, user)
	return user, nil
}

func (r *fakeUserRepository) UpdateUser(ctx context.Context, id uuid.UUID, input UpdateUserRecord) (User, error) {
	user := User{
		ID:          id,
		Email:       input.Email,
		Name:        input.Name,
		Role:        input.Role,
		Status:      input.Status,
		GoogleSub:   input.GoogleSub,
		HasPassword: input.PasswordHash != nil && *input.PasswordHash != "",
	}
	for i, existing := range r.users {
		if existing.ID == id {
			if input.GoogleSub == nil {
				user.GoogleSub = existing.GoogleSub
			}
			if input.PasswordHash == nil {
				user.HasPassword = existing.HasPassword
			}
			r.users[i] = user
			break
		}
	}
	return user, nil
}

func (r *fakeUserRepository) DisableUser(ctx context.Context, id uuid.UUID) error {
	return nil
}
