package service

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"coast-monitoring/internal/policy"

	"github.com/google/uuid"
)

const maxObservationNotesRunes = 1000

type Observation struct {
	ID         uuid.UUID
	ObservedOn time.Time
	LocationID uuid.UUID
	SpeciesID  uuid.UUID
	ObserverID uuid.UUID
	Count      int
	Notes      string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type ObservationInput struct {
	ObservedOn time.Time
	LocationID uuid.UUID
	SpeciesID  uuid.UUID
	ObserverID uuid.UUID
	Count      int
	Notes      string
}

type ObservationRecord struct {
	ObservedOn time.Time
	LocationID uuid.UUID
	SpeciesID  uuid.UUID
	ObserverID uuid.UUID
	Count      int
	Notes      string
	ActorID    uuid.UUID
}

type ObservationRepository interface {
	ListObservations(ctx context.Context) ([]Observation, error)
	ListObservationsByObserver(ctx context.Context, observerID uuid.UUID) ([]Observation, error)
	GetObservation(ctx context.Context, id uuid.UUID) (Observation, error)
	CreateObservation(ctx context.Context, input ObservationRecord) (Observation, error)
	UpdateObservation(ctx context.Context, id uuid.UUID, input ObservationRecord) (Observation, error)
	DeleteObservation(ctx context.Context, id uuid.UUID) error
}

type ObservationService struct {
	Observations ObservationRepository
}

func (s ObservationService) ListForAdmin(ctx context.Context, actor policy.User) ([]Observation, error) {
	if !policy.CanUseAdminAPI(actor) {
		return nil, ErrForbidden
	}
	return s.Observations.ListObservations(ctx)
}

func (s ObservationService) ListForApp(ctx context.Context, actor policy.User) ([]Observation, error) {
	if !policy.CanUseAppAPI(actor) {
		return nil, ErrForbidden
	}
	if actor.Role == policy.RoleVolunteer {
		return s.Observations.ListObservationsByObserver(ctx, actor.ID)
	}
	return s.Observations.ListObservations(ctx)
}

func (s ObservationService) Create(ctx context.Context, actor policy.User, input ObservationInput) (Observation, error) {
	if !policy.CanUseAppAPI(actor) {
		return Observation{}, ErrForbidden
	}
	if actor.Role == policy.RoleVolunteer && input.ObserverID != actor.ID {
		return Observation{}, ErrForbidden
	}
	record, err := validateObservationInput(actor, input)
	if err != nil {
		return Observation{}, err
	}
	return s.Observations.CreateObservation(ctx, record)
}

func (s ObservationService) Update(ctx context.Context, actor policy.User, id uuid.UUID, input ObservationInput) (Observation, error) {
	existing, err := s.Observations.GetObservation(ctx, id)
	if err != nil {
		return Observation{}, err
	}
	if !policy.CanManageObservation(actor, existing.ObserverID) {
		return Observation{}, ErrForbidden
	}
	if actor.Role == policy.RoleVolunteer && input.ObserverID != actor.ID {
		return Observation{}, ErrForbidden
	}
	record, err := validateObservationInput(actor, input)
	if err != nil {
		return Observation{}, err
	}
	return s.Observations.UpdateObservation(ctx, id, record)
}

func (s ObservationService) Delete(ctx context.Context, actor policy.User, id uuid.UUID) error {
	existing, err := s.Observations.GetObservation(ctx, id)
	if err != nil {
		return err
	}
	if !policy.CanManageObservation(actor, existing.ObserverID) {
		return ErrForbidden
	}
	return s.Observations.DeleteObservation(ctx, id)
}

func validateObservationInput(actor policy.User, input ObservationInput) (ObservationRecord, error) {
	if input.Count < 0 {
		return ObservationRecord{}, fmt.Errorf("%w: count must be zero or greater", ErrValidation)
	}
	notes := strings.TrimSpace(input.Notes)
	if utf8.RuneCountInString(notes) > maxObservationNotesRunes {
		return ObservationRecord{}, fmt.Errorf("%w: notes must be 1000 characters or fewer", ErrValidation)
	}
	return ObservationRecord{
		ObservedOn: input.ObservedOn,
		LocationID: input.LocationID,
		SpeciesID:  input.SpeciesID,
		ObserverID: input.ObserverID,
		Count:      input.Count,
		Notes:      notes,
		ActorID:    actor.ID,
	}, nil
}
