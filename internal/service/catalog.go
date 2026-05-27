package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"coast-monitoring/internal/policy"

	"github.com/google/uuid"
)

type Location struct {
	ID          uuid.UUID
	ChineseName string
	EnglishName string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Species struct {
	ID          uuid.UUID
	ChineseName string
	EnglishName string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type CatalogInput struct {
	ChineseName string
	EnglishName string
}

type CatalogRecord struct {
	ChineseName string
	EnglishName string
	ActorID     uuid.UUID
}

type CatalogRepository interface {
	ListLocations(ctx context.Context) ([]Location, error)
	CreateLocation(ctx context.Context, input CatalogRecord) (Location, error)
	UpdateLocation(ctx context.Context, id uuid.UUID, input CatalogRecord) (Location, error)
	DeleteLocation(ctx context.Context, id uuid.UUID) error
	ListSpecies(ctx context.Context) ([]Species, error)
	CreateSpecies(ctx context.Context, input CatalogRecord) (Species, error)
	UpdateSpecies(ctx context.Context, id uuid.UUID, input CatalogRecord) (Species, error)
	DeleteSpecies(ctx context.Context, id uuid.UUID) error
}

type CatalogService struct {
	Catalog CatalogRepository
}

func (s CatalogService) ListLocations(ctx context.Context, actor policy.User) ([]Location, error) {
	if !policy.CanUseAppAPI(actor) {
		return nil, ErrForbidden
	}
	return s.Catalog.ListLocations(ctx)
}

func (s CatalogService) CreateLocation(ctx context.Context, actor policy.User, input CatalogInput) (Location, error) {
	record, err := validateCatalogWrite(actor, input)
	if err != nil {
		return Location{}, err
	}
	return s.Catalog.CreateLocation(ctx, record)
}

func (s CatalogService) UpdateLocation(ctx context.Context, actor policy.User, id uuid.UUID, input CatalogInput) (Location, error) {
	record, err := validateCatalogWrite(actor, input)
	if err != nil {
		return Location{}, err
	}
	return s.Catalog.UpdateLocation(ctx, id, record)
}

func (s CatalogService) DeleteLocation(ctx context.Context, actor policy.User, id uuid.UUID) error {
	if !policy.CanUseAdminAPI(actor) {
		return ErrForbidden
	}
	return s.Catalog.DeleteLocation(ctx, id)
}

func (s CatalogService) ListSpecies(ctx context.Context, actor policy.User) ([]Species, error) {
	if !policy.CanUseAppAPI(actor) {
		return nil, ErrForbidden
	}
	return s.Catalog.ListSpecies(ctx)
}

func (s CatalogService) CreateSpecies(ctx context.Context, actor policy.User, input CatalogInput) (Species, error) {
	record, err := validateCatalogWrite(actor, input)
	if err != nil {
		return Species{}, err
	}
	return s.Catalog.CreateSpecies(ctx, record)
}

func (s CatalogService) UpdateSpecies(ctx context.Context, actor policy.User, id uuid.UUID, input CatalogInput) (Species, error) {
	record, err := validateCatalogWrite(actor, input)
	if err != nil {
		return Species{}, err
	}
	return s.Catalog.UpdateSpecies(ctx, id, record)
}

func (s CatalogService) DeleteSpecies(ctx context.Context, actor policy.User, id uuid.UUID) error {
	if !policy.CanUseAdminAPI(actor) {
		return ErrForbidden
	}
	return s.Catalog.DeleteSpecies(ctx, id)
}

func validateCatalogWrite(actor policy.User, input CatalogInput) (CatalogRecord, error) {
	if !policy.CanUseAdminAPI(actor) {
		return CatalogRecord{}, ErrForbidden
	}
	chineseName := strings.TrimSpace(input.ChineseName)
	if chineseName == "" {
		return CatalogRecord{}, fmt.Errorf("%w: chinese name is required", ErrValidation)
	}
	englishName := strings.TrimSpace(input.EnglishName)
	if englishName == "" {
		return CatalogRecord{}, fmt.Errorf("%w: english name is required", ErrValidation)
	}
	return CatalogRecord{
		ChineseName: chineseName,
		EnglishName: englishName,
		ActorID:     actor.ID,
	}, nil
}
