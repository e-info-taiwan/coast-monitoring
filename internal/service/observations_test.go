package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"coast-monitoring/internal/policy"

	"github.com/google/uuid"
)

func TestVolunteerAppObservationListOnlyReturnsOwnRows(t *testing.T) {
	volunteerID := uuid.New()
	otherID := uuid.New()
	repo := &fakeObservationRepository{
		observations: []Observation{
			{ID: uuid.New(), ObserverID: volunteerID, Count: 2},
			{ID: uuid.New(), ObserverID: otherID, Count: 4},
		},
	}
	svc := ObservationService{Observations: repo}
	actor := policy.User{
		ID:     volunteerID,
		Email:  "volunteer@example.com",
		Name:   "Volunteer",
		Role:   policy.RoleVolunteer,
		Status: policy.StatusActive,
	}

	got, err := svc.ListForApp(context.Background(), actor)
	if err != nil {
		t.Fatalf("ListForApp error = %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("ListForApp returned %d observations, want 1", len(got))
	}
	if got[0].ObserverID != volunteerID {
		t.Fatalf("ListForApp returned observer %s, want %s", got[0].ObserverID, volunteerID)
	}
}

func TestAdminCanUpdateAnyObservation(t *testing.T) {
	observerID := uuid.New()
	observationID := uuid.New()
	repo := &fakeObservationRepository{
		observations: []Observation{
			{ID: observationID, ObserverID: observerID, Count: 1, Notes: "old"},
		},
	}
	svc := ObservationService{Observations: repo}
	actor := policy.User{
		ID:     uuid.New(),
		Email:  "admin@example.com",
		Name:   "Admin",
		Role:   policy.RoleAdmin,
		Status: policy.StatusActive,
	}

	updated, err := svc.Update(context.Background(), actor, observationID, ObservationInput{
		ObservedOn: time.Date(2026, 5, 27, 0, 0, 0, 0, time.UTC),
		LocationID: uuid.New(),
		SpeciesID:  uuid.New(),
		ObserverID: observerID,
		Count:      5,
		Notes:      " updated notes ",
	})
	if err != nil {
		t.Fatalf("Update error = %v", err)
	}

	if updated.Count != 5 {
		t.Fatalf("updated count = %d, want 5", updated.Count)
	}
	if updated.Notes != "updated notes" {
		t.Fatalf("updated notes = %q, want trimmed notes", updated.Notes)
	}
}

func TestVolunteerCannotUpdateOtherUsersObservation(t *testing.T) {
	volunteerID := uuid.New()
	otherID := uuid.New()
	observationID := uuid.New()
	repo := &fakeObservationRepository{
		observations: []Observation{
			{ID: observationID, ObserverID: otherID, Count: 1},
		},
	}
	svc := ObservationService{Observations: repo}
	actor := policy.User{
		ID:     volunteerID,
		Email:  "volunteer@example.com",
		Name:   "Volunteer",
		Role:   policy.RoleVolunteer,
		Status: policy.StatusActive,
	}

	_, err := svc.Update(context.Background(), actor, observationID, ObservationInput{
		ObservedOn: time.Date(2026, 5, 27, 0, 0, 0, 0, time.UTC),
		LocationID: uuid.New(),
		SpeciesID:  uuid.New(),
		ObserverID: otherID,
		Count:      3,
	})

	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("Update error = %v, want %v", err, ErrForbidden)
	}
	if repo.updated {
		t.Fatal("repository update was called for forbidden volunteer update")
	}
}

type fakeObservationRepository struct {
	observations []Observation
	updated      bool
}

func (r *fakeObservationRepository) ListObservations(ctx context.Context) ([]Observation, error) {
	return append([]Observation(nil), r.observations...), nil
}

func (r *fakeObservationRepository) ListObservationsByObserver(ctx context.Context, observerID uuid.UUID) ([]Observation, error) {
	var out []Observation
	for _, observation := range r.observations {
		if observation.ObserverID == observerID {
			out = append(out, observation)
		}
	}
	return out, nil
}

func (r *fakeObservationRepository) GetObservation(ctx context.Context, id uuid.UUID) (Observation, error) {
	for _, observation := range r.observations {
		if observation.ID == id {
			return observation, nil
		}
	}
	return Observation{}, ErrNotFound
}

func (r *fakeObservationRepository) CreateObservation(ctx context.Context, input ObservationRecord) (Observation, error) {
	observation := Observation{
		ID:         uuid.New(),
		ObservedOn: input.ObservedOn,
		LocationID: input.LocationID,
		SpeciesID:  input.SpeciesID,
		ObserverID: input.ObserverID,
		Count:      input.Count,
		Notes:      input.Notes,
	}
	r.observations = append(r.observations, observation)
	return observation, nil
}

func (r *fakeObservationRepository) UpdateObservation(ctx context.Context, id uuid.UUID, input ObservationRecord) (Observation, error) {
	r.updated = true
	for i, observation := range r.observations {
		if observation.ID == id {
			observation.ObservedOn = input.ObservedOn
			observation.LocationID = input.LocationID
			observation.SpeciesID = input.SpeciesID
			observation.ObserverID = input.ObserverID
			observation.Count = input.Count
			observation.Notes = input.Notes
			r.observations[i] = observation
			return observation, nil
		}
	}
	return Observation{}, ErrNotFound
}

func (r *fakeObservationRepository) DeleteObservation(ctx context.Context, id uuid.UUID) error {
	return nil
}
