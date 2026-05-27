package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"coast-monitoring/internal/auth"
	"coast-monitoring/internal/policy"

	"github.com/google/uuid"
)

type User struct {
	ID          uuid.UUID
	Email       string
	Name        string
	Role        policy.Role
	Status      policy.Status
	GoogleSub   *string
	HasPassword bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type CreateUserInput struct {
	Email     string
	Name      string
	Role      policy.Role
	Status    policy.Status
	GoogleSub *string
	Password  string
}

type UpdateUserInput struct {
	Email     string
	Name      string
	Role      policy.Role
	Status    policy.Status
	GoogleSub *string
	Password  *string
}

type CreateUserRecord struct {
	Email        string
	Name         string
	Role         policy.Role
	Status       policy.Status
	GoogleSub    *string
	PasswordHash string
}

type UpdateUserRecord struct {
	Email        string
	Name         string
	Role         policy.Role
	Status       policy.Status
	GoogleSub    *string
	PasswordHash *string
}

type UserRepository interface {
	ListUsers(ctx context.Context) ([]User, error)
	CreateUser(ctx context.Context, input CreateUserRecord) (User, error)
	UpdateUser(ctx context.Context, id uuid.UUID, input UpdateUserRecord) (User, error)
	DisableUser(ctx context.Context, id uuid.UUID) error
}

type UserService struct {
	Users UserRepository
}

func (s UserService) CreateUser(ctx context.Context, actor policy.User, input CreateUserInput) (User, error) {
	if !policy.CanUseAdminAPI(actor) {
		return User{}, ErrForbidden
	}
	record, err := validateCreateUserInput(input)
	if err != nil {
		return User{}, err
	}
	if strings.TrimSpace(input.Password) != "" {
		hash, err := auth.HashPassword(input.Password)
		if err != nil {
			return User{}, err
		}
		record.PasswordHash = hash
	}
	return s.Users.CreateUser(ctx, record)
}

func (s UserService) UpdateUser(ctx context.Context, actor policy.User, id uuid.UUID, input UpdateUserInput) (User, error) {
	if !policy.CanUseAdminAPI(actor) {
		return User{}, ErrForbidden
	}
	record, err := validateUpdateUserInput(input)
	if err != nil {
		return User{}, err
	}
	if input.Password != nil && strings.TrimSpace(*input.Password) != "" {
		hash, err := auth.HashPassword(*input.Password)
		if err != nil {
			return User{}, err
		}
		record.PasswordHash = &hash
	}
	return s.Users.UpdateUser(ctx, id, record)
}

func (s UserService) DisableUser(ctx context.Context, actor policy.User, id uuid.UUID) error {
	if !policy.CanUseAdminAPI(actor) {
		return ErrForbidden
	}
	return s.Users.DisableUser(ctx, id)
}

func (s UserService) ListUsers(ctx context.Context, actor policy.User) ([]User, error) {
	if !policy.CanUseAdminAPI(actor) {
		return nil, ErrForbidden
	}
	return s.Users.ListUsers(ctx)
}

func validateCreateUserInput(input CreateUserInput) (CreateUserRecord, error) {
	email, name, err := validateUserFields(input.Email, input.Name, input.Role, input.Status)
	if err != nil {
		return CreateUserRecord{}, err
	}
	return CreateUserRecord{
		Email:     email,
		Name:      name,
		Role:      input.Role,
		Status:    input.Status,
		GoogleSub: input.GoogleSub,
	}, nil
}

func validateUpdateUserInput(input UpdateUserInput) (UpdateUserRecord, error) {
	email, name, err := validateUserFields(input.Email, input.Name, input.Role, input.Status)
	if err != nil {
		return UpdateUserRecord{}, err
	}
	return UpdateUserRecord{
		Email:     email,
		Name:      name,
		Role:      input.Role,
		Status:    input.Status,
		GoogleSub: input.GoogleSub,
	}, nil
}

func validateUserFields(email, name string, role policy.Role, status policy.Status) (string, string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return "", "", fmt.Errorf("%w: email is required", ErrValidation)
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return "", "", fmt.Errorf("%w: name is required", ErrValidation)
	}
	if role != policy.RoleAdmin && role != policy.RoleVolunteer {
		return "", "", fmt.Errorf("%w: role must be admin or volunteer", ErrValidation)
	}
	if status != policy.StatusActive && status != policy.StatusDisabled {
		return "", "", fmt.Errorf("%w: status must be active or disabled", ErrValidation)
	}
	return email, name, nil
}
