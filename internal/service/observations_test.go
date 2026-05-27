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
	if repo.scopedUpdate {
		t.Fatal("admin update used owner-scoped update path")
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

func TestDisabledActorCannotUpdateObservationAndDoesNotLookup(t *testing.T) {
	repo := &fakeObservationRepository{}
	svc := ObservationService{Observations: repo}
	actor := policy.User{
		ID:     uuid.New(),
		Email:  "admin@example.com",
		Name:   "Admin",
		Role:   policy.RoleAdmin,
		Status: policy.StatusDisabled,
	}

	_, err := svc.Update(context.Background(), actor, uuid.New(), validObservationInput(uuid.New()))

	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("Update error = %v, want %v", err, ErrForbidden)
	}
	if repo.got || repo.updated || repo.scopedUpdate {
		t.Fatal("repository was called for disabled actor update")
	}
}

func TestDisabledActorCannotDeleteObservationAndDoesNotLookup(t *testing.T) {
	repo := &fakeObservationRepository{}
	svc := ObservationService{Observations: repo}
	actor := policy.User{
		ID:     uuid.New(),
		Email:  "admin@example.com",
		Name:   "Admin",
		Role:   policy.RoleAdmin,
		Status: policy.StatusDisabled,
	}

	err := svc.Delete(context.Background(), actor, uuid.New())

	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("Delete error = %v, want %v", err, ErrForbidden)
	}
	if repo.got || repo.deleted || repo.scopedDelete {
		t.Fatal("repository was called for disabled actor delete")
	}
}

func TestVolunteerUpdateUsesOwnerScopedWritePath(t *testing.T) {
	volunteerID := uuid.New()
	observationID := uuid.New()
	repo := &fakeObservationRepository{
		observations: []Observation{{ID: observationID, ObserverID: volunteerID, Count: 1}},
	}
	svc := ObservationService{Observations: repo}
	actor := policy.User{
		ID:     volunteerID,
		Email:  "volunteer@example.com",
		Name:   "Volunteer",
		Role:   policy.RoleVolunteer,
		Status: policy.StatusActive,
	}

	_, err := svc.Update(context.Background(), actor, observationID, validObservationInput(volunteerID))
	if err != nil {
		t.Fatalf("Update error = %v", err)
	}

	if repo.got {
		t.Fatal("volunteer update performed pre-write lookup")
	}
	if repo.updated {
		t.Fatal("volunteer update used id-only update path")
	}
	if !repo.scopedUpdate || repo.expectedObserverID != volunteerID {
		t.Fatalf("volunteer update did not use scoped path with observer %s", volunteerID)
	}
}

func TestVolunteerUpdateFailsIfOwnershipChangesBeforeWrite(t *testing.T) {
	volunteerID := uuid.New()
	otherID := uuid.New()
	observationID := uuid.New()
	repo := &fakeObservationRepository{
		observations: []Observation{{ID: observationID, ObserverID: otherID, Count: 1}},
	}
	svc := ObservationService{Observations: repo}
	actor := policy.User{
		ID:     volunteerID,
		Email:  "volunteer@example.com",
		Name:   "Volunteer",
		Role:   policy.RoleVolunteer,
		Status: policy.StatusActive,
	}

	_, err := svc.Update(context.Background(), actor, observationID, validObservationInput(volunteerID))

	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("Update error = %v, want %v", err, ErrForbidden)
	}
	if repo.updated {
		t.Fatal("volunteer update used id-only update path")
	}
}

func TestObservationRequiresCoreFields(t *testing.T) {
	actor := policy.User{
		ID:     uuid.New(),
		Email:  "admin@example.com",
		Name:   "Admin",
		Role:   policy.RoleAdmin,
		Status: policy.StatusActive,
	}
	valid := ObservationInput{
		ObservedOn: time.Date(2026, 5, 27, 0, 0, 0, 0, time.UTC),
		LocationID: uuid.New(),
		SpeciesID:  uuid.New(),
		ObserverID: uuid.New(),
		Count:      0,
	}
	tests := []struct {
		name  string
		input ObservationInput
	}{
		{name: "observed on", input: ObservationInput{LocationID: valid.LocationID, SpeciesID: valid.SpeciesID, ObserverID: valid.ObserverID}},
		{name: "location id", input: ObservationInput{ObservedOn: valid.ObservedOn, SpeciesID: valid.SpeciesID, ObserverID: valid.ObserverID}},
		{name: "species id", input: ObservationInput{ObservedOn: valid.ObservedOn, LocationID: valid.LocationID, ObserverID: valid.ObserverID}},
		{name: "observer id", input: ObservationInput{ObservedOn: valid.ObservedOn, LocationID: valid.LocationID, SpeciesID: valid.SpeciesID}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := validateObservationInput(actor, tt.input)
			if !errors.Is(err, ErrValidation) {
				t.Fatalf("validateObservationInput error = %v, want %v", err, ErrValidation)
			}
		})
	}
}

func TestObservationRejectsInvalidCountAndLongNotes(t *testing.T) {
	actor := policy.User{ID: uuid.New()}
	valid := ObservationInput{
		ObservedOn: time.Date(2026, 5, 27, 0, 0, 0, 0, time.UTC),
		LocationID: uuid.New(),
		SpeciesID:  uuid.New(),
		ObserverID: uuid.New(),
		Count:      0,
	}

	negative := valid
	negative.Count = -1
	if _, err := validateObservationInput(actor, negative); !errors.Is(err, ErrValidation) {
		t.Fatalf("negative count error = %v, want %v", err, ErrValidation)
	}

	longNotes := valid
	longNotes.Notes = string(make([]rune, 1001))
	if _, err := validateObservationInput(actor, longNotes); !errors.Is(err, ErrValidation) {
		t.Fatalf("long notes error = %v, want %v", err, ErrValidation)
	}
}

func TestVolunteerCannotDeleteOtherUsersObservation(t *testing.T) {
	volunteerID := uuid.New()
	otherID := uuid.New()
	observationID := uuid.New()
	repo := &fakeObservationRepository{
		observations: []Observation{{ID: observationID, ObserverID: otherID}},
	}
	svc := ObservationService{Observations: repo}
	actor := policy.User{
		ID:     volunteerID,
		Email:  "volunteer@example.com",
		Name:   "Volunteer",
		Role:   policy.RoleVolunteer,
		Status: policy.StatusActive,
	}

	err := svc.Delete(context.Background(), actor, observationID)

	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("Delete error = %v, want %v", err, ErrForbidden)
	}
	if repo.deleted {
		t.Fatal("repository delete was called for forbidden volunteer delete")
	}
}

func TestAdminCanDeleteAnyObservation(t *testing.T) {
	observationID := uuid.New()
	repo := &fakeObservationRepository{
		observations: []Observation{{ID: observationID, ObserverID: uuid.New()}},
	}
	svc := ObservationService{Observations: repo}

	err := svc.Delete(context.Background(), activePolicyAdmin(), observationID)
	if err != nil {
		t.Fatalf("Delete error = %v", err)
	}

	if !repo.deleted {
		t.Fatal("admin delete did not use id-only delete path")
	}
	if repo.scopedDelete {
		t.Fatal("admin delete used owner-scoped delete path")
	}
}

func TestVolunteerDeleteUsesOwnerScopedWritePath(t *testing.T) {
	volunteerID := uuid.New()
	observationID := uuid.New()
	repo := &fakeObservationRepository{
		observations: []Observation{{ID: observationID, ObserverID: volunteerID}},
	}
	svc := ObservationService{Observations: repo}
	actor := policy.User{
		ID:     volunteerID,
		Email:  "volunteer@example.com",
		Name:   "Volunteer",
		Role:   policy.RoleVolunteer,
		Status: policy.StatusActive,
	}

	err := svc.Delete(context.Background(), actor, observationID)
	if err != nil {
		t.Fatalf("Delete error = %v", err)
	}

	if repo.got {
		t.Fatal("volunteer delete performed pre-write lookup")
	}
	if repo.deleted {
		t.Fatal("volunteer delete used id-only delete path")
	}
	if !repo.scopedDelete || repo.expectedObserverID != volunteerID {
		t.Fatalf("volunteer delete did not use scoped path with observer %s", volunteerID)
	}
}

func TestVolunteerDeleteFailsIfOwnershipChangesBeforeWrite(t *testing.T) {
	volunteerID := uuid.New()
	otherID := uuid.New()
	observationID := uuid.New()
	repo := &fakeObservationRepository{
		observations: []Observation{{ID: observationID, ObserverID: otherID}},
	}
	svc := ObservationService{Observations: repo}
	actor := policy.User{
		ID:     volunteerID,
		Email:  "volunteer@example.com",
		Name:   "Volunteer",
		Role:   policy.RoleVolunteer,
		Status: policy.StatusActive,
	}

	err := svc.Delete(context.Background(), actor, observationID)

	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("Delete error = %v, want %v", err, ErrForbidden)
	}
	if repo.deleted {
		t.Fatal("volunteer delete used id-only delete path")
	}
}

func validObservationInput(observerID uuid.UUID) ObservationInput {
	return ObservationInput{
		ObservedOn: time.Date(2026, 5, 27, 0, 0, 0, 0, time.UTC),
		LocationID: uuid.New(),
		SpeciesID:  uuid.New(),
		ObserverID: observerID,
		Count:      1,
	}
}

func activePolicyAdmin() policy.User {
	return policy.User{
		ID:     uuid.New(),
		Email:  "admin@example.com",
		Name:   "Admin",
		Role:   policy.RoleAdmin,
		Status: policy.StatusActive,
	}
}

type fakeObservationRepository struct {
	observations       []Observation
	got                bool
	updated            bool
	deleted            bool
	scopedUpdate       bool
	scopedDelete       bool
	expectedObserverID uuid.UUID
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
	r.got = true
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

func (r *fakeObservationRepository) UpdateObservationForObserver(ctx context.Context, id uuid.UUID, expectedObserverID uuid.UUID, input ObservationRecord) (Observation, error) {
	r.scopedUpdate = true
	r.expectedObserverID = expectedObserverID
	for i, observation := range r.observations {
		if observation.ID == id && observation.ObserverID == expectedObserverID {
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
	r.deleted = true
	return nil
}

func (r *fakeObservationRepository) DeleteObservationForObserver(ctx context.Context, id uuid.UUID, expectedObserverID uuid.UUID) error {
	r.scopedDelete = true
	r.expectedObserverID = expectedObserverID
	for _, observation := range r.observations {
		if observation.ID == id && observation.ObserverID == expectedObserverID {
			return nil
		}
	}
	return ErrNotFound
}
