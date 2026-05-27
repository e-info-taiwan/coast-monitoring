package service

import (
	"context"
	"errors"
	"testing"

	"coast-monitoring/internal/policy"

	"github.com/google/uuid"
)

func TestVolunteerCannotCreateLocation(t *testing.T) {
	svc := CatalogService{Catalog: &fakeCatalogRepository{}}
	actor := policy.User{
		ID:     uuid.New(),
		Email:  "volunteer@example.com",
		Name:   "Volunteer",
		Role:   policy.RoleVolunteer,
		Status: policy.StatusActive,
	}

	_, err := svc.CreateLocation(context.Background(), actor, CatalogInput{
		ChineseName: "地點",
		EnglishName: "Location",
	})

	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("CreateLocation error = %v, want %v", err, ErrForbidden)
	}
}

func TestDisabledActorCannotListLocations(t *testing.T) {
	svc := CatalogService{Catalog: &fakeCatalogRepository{}}
	actor := policy.User{
		ID:     uuid.New(),
		Email:  "admin@example.com",
		Name:   "Admin",
		Role:   policy.RoleAdmin,
		Status: policy.StatusDisabled,
	}

	_, err := svc.ListLocations(context.Background(), actor)

	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("ListLocations error = %v, want %v", err, ErrForbidden)
	}
}

func TestCatalogWriteRequiresNames(t *testing.T) {
	svc := CatalogService{Catalog: &fakeCatalogRepository{}}

	_, err := svc.CreateSpecies(context.Background(), activeAdmin(), CatalogInput{EnglishName: "Species"})
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("missing chinese name error = %v, want %v", err, ErrValidation)
	}

	_, err = svc.CreateSpecies(context.Background(), activeAdmin(), CatalogInput{ChineseName: "物種"})
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("missing english name error = %v, want %v", err, ErrValidation)
	}
}

type fakeCatalogRepository struct{}

func (r *fakeCatalogRepository) ListLocations(ctx context.Context) ([]Location, error) {
	return nil, nil
}

func (r *fakeCatalogRepository) CreateLocation(ctx context.Context, input CatalogRecord) (Location, error) {
	return Location{ID: uuid.New(), ChineseName: input.ChineseName, EnglishName: input.EnglishName}, nil
}

func (r *fakeCatalogRepository) UpdateLocation(ctx context.Context, id uuid.UUID, input CatalogRecord) (Location, error) {
	return Location{ID: id, ChineseName: input.ChineseName, EnglishName: input.EnglishName}, nil
}

func (r *fakeCatalogRepository) DeleteLocation(ctx context.Context, id uuid.UUID) error {
	return nil
}

func (r *fakeCatalogRepository) ListSpecies(ctx context.Context) ([]Species, error) {
	return nil, nil
}

func (r *fakeCatalogRepository) CreateSpecies(ctx context.Context, input CatalogRecord) (Species, error) {
	return Species{ID: uuid.New(), ChineseName: input.ChineseName, EnglishName: input.EnglishName}, nil
}

func (r *fakeCatalogRepository) UpdateSpecies(ctx context.Context, id uuid.UUID, input CatalogRecord) (Species, error) {
	return Species{ID: id, ChineseName: input.ChineseName, EnglishName: input.EnglishName}, nil
}

func (r *fakeCatalogRepository) DeleteSpecies(ctx context.Context, id uuid.UUID) error {
	return nil
}
