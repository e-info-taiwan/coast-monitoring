package repository

import (
	"context"

	"coast-monitoring/internal/service"

	"github.com/google/uuid"
)

type ObservationRepository struct {
	db DBTX
}

func NewObservationRepository(db DBTX) ObservationRepository {
	return ObservationRepository{db: db}
}

func (r ObservationRepository) ListObservations(ctx context.Context) ([]service.Observation, error) {
	return r.listObservations(ctx, "")
}

func (r ObservationRepository) ListObservationsByObserver(ctx context.Context, observerID uuid.UUID) ([]service.Observation, error) {
	return r.listObservations(ctx, "WHERE observer_id = $1", observerID)
}

func (r ObservationRepository) GetObservation(ctx context.Context, id uuid.UUID) (service.Observation, error) {
	return scanObservation(r.db.QueryRow(ctx, `
		SELECT id, observed_on, location_id, species_id, observer_id, count, notes, created_at, updated_at
		FROM observations
		WHERE id = $1
	`, id))
}

func (r ObservationRepository) CreateObservation(ctx context.Context, input service.ObservationRecord) (service.Observation, error) {
	return scanObservation(r.db.QueryRow(ctx, `
		INSERT INTO observations (observed_on, location_id, species_id, observer_id, count, notes, created_by, updated_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $7)
		RETURNING id, observed_on, location_id, species_id, observer_id, count, notes, created_at, updated_at
	`, input.ObservedOn, input.LocationID, input.SpeciesID, input.ObserverID, input.Count, input.Notes, input.ActorID))
}

func (r ObservationRepository) UpdateObservation(ctx context.Context, id uuid.UUID, input service.ObservationRecord) (service.Observation, error) {
	return scanObservation(r.db.QueryRow(ctx, `
		UPDATE observations
		SET observed_on = $2, location_id = $3, species_id = $4, observer_id = $5, count = $6, notes = $7, updated_by = $8, updated_at = now()
		WHERE id = $1
		RETURNING id, observed_on, location_id, species_id, observer_id, count, notes, created_at, updated_at
	`, id, input.ObservedOn, input.LocationID, input.SpeciesID, input.ObserverID, input.Count, input.Notes, input.ActorID))
}

func (r ObservationRepository) UpdateObservationForObserver(ctx context.Context, id uuid.UUID, expectedObserverID uuid.UUID, input service.ObservationRecord) (service.Observation, error) {
	return scanObservation(r.db.QueryRow(ctx, `
		UPDATE observations
		SET observed_on = $3, location_id = $4, species_id = $5, observer_id = $6, count = $7, notes = $8, updated_by = $9, updated_at = now()
		WHERE id = $1 AND observer_id = $2
		RETURNING id, observed_on, location_id, species_id, observer_id, count, notes, created_at, updated_at
	`, id, expectedObserverID, input.ObservedOn, input.LocationID, input.SpeciesID, input.ObserverID, input.Count, input.Notes, input.ActorID))
}

func (r ObservationRepository) DeleteObservation(ctx context.Context, id uuid.UUID) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM observations WHERE id = $1`, id)
	return requireRowsAffected(tag, err)
}

func (r ObservationRepository) DeleteObservationForObserver(ctx context.Context, id uuid.UUID, expectedObserverID uuid.UUID) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM observations WHERE id = $1 AND observer_id = $2`, id, expectedObserverID)
	return requireRowsAffected(tag, err)
}

func (r ObservationRepository) listObservations(ctx context.Context, where string, args ...any) ([]service.Observation, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, observed_on, location_id, species_id, observer_id, count, notes, created_at, updated_at
		FROM observations
		`+where+`
		ORDER BY observed_on DESC, created_at DESC
	`, args...)
	if err != nil {
		return nil, translateError(err)
	}
	defer rows.Close()

	var observations []service.Observation
	for rows.Next() {
		observation, err := scanObservation(rows)
		if err != nil {
			return nil, err
		}
		observations = append(observations, observation)
	}
	return observations, translateError(rows.Err())
}

type observationScanner interface {
	Scan(dest ...any) error
}

func scanObservation(row observationScanner) (service.Observation, error) {
	var observation service.Observation
	err := row.Scan(
		&observation.ID,
		&observation.ObservedOn,
		&observation.LocationID,
		&observation.SpeciesID,
		&observation.ObserverID,
		&observation.Count,
		&observation.Notes,
		&observation.CreatedAt,
		&observation.UpdatedAt,
	)
	return observation, translateError(err)
}
