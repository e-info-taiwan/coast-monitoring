package repository

import (
	"context"

	"coast-monitoring/internal/service"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CatalogRepository struct {
	db *pgxpool.Pool
}

func NewCatalogRepository(db *pgxpool.Pool) CatalogRepository {
	return CatalogRepository{db: db}
}

func (r CatalogRepository) ListLocations(ctx context.Context) ([]service.Location, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, chinese_name, english_name, created_at, updated_at
		FROM locations
		ORDER BY chinese_name, english_name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var locations []service.Location
	for rows.Next() {
		var location service.Location
		if err := rows.Scan(&location.ID, &location.ChineseName, &location.EnglishName, &location.CreatedAt, &location.UpdatedAt); err != nil {
			return nil, err
		}
		locations = append(locations, location)
	}
	return locations, rows.Err()
}

func (r CatalogRepository) CreateLocation(ctx context.Context, input service.CatalogRecord) (service.Location, error) {
	var location service.Location
	err := r.db.QueryRow(ctx, `
		INSERT INTO locations (chinese_name, english_name, created_by, updated_by)
		VALUES ($1, $2, $3, $3)
		RETURNING id, chinese_name, english_name, created_at, updated_at
	`, input.ChineseName, input.EnglishName, input.ActorID).Scan(&location.ID, &location.ChineseName, &location.EnglishName, &location.CreatedAt, &location.UpdatedAt)
	return location, translateError(err)
}

func (r CatalogRepository) UpdateLocation(ctx context.Context, id uuid.UUID, input service.CatalogRecord) (service.Location, error) {
	var location service.Location
	err := r.db.QueryRow(ctx, `
		UPDATE locations
		SET chinese_name = $2, english_name = $3, updated_by = $4, updated_at = now()
		WHERE id = $1
		RETURNING id, chinese_name, english_name, created_at, updated_at
	`, id, input.ChineseName, input.EnglishName, input.ActorID).Scan(&location.ID, &location.ChineseName, &location.EnglishName, &location.CreatedAt, &location.UpdatedAt)
	return location, translateError(err)
}

func (r CatalogRepository) DeleteLocation(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx, `DELETE FROM locations WHERE id = $1`, id)
	return err
}

func (r CatalogRepository) ListSpecies(ctx context.Context) ([]service.Species, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, chinese_name, english_name, created_at, updated_at
		FROM species
		ORDER BY chinese_name, english_name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var speciesList []service.Species
	for rows.Next() {
		var species service.Species
		if err := rows.Scan(&species.ID, &species.ChineseName, &species.EnglishName, &species.CreatedAt, &species.UpdatedAt); err != nil {
			return nil, err
		}
		speciesList = append(speciesList, species)
	}
	return speciesList, rows.Err()
}

func (r CatalogRepository) CreateSpecies(ctx context.Context, input service.CatalogRecord) (service.Species, error) {
	var species service.Species
	err := r.db.QueryRow(ctx, `
		INSERT INTO species (chinese_name, english_name, created_by, updated_by)
		VALUES ($1, $2, $3, $3)
		RETURNING id, chinese_name, english_name, created_at, updated_at
	`, input.ChineseName, input.EnglishName, input.ActorID).Scan(&species.ID, &species.ChineseName, &species.EnglishName, &species.CreatedAt, &species.UpdatedAt)
	return species, translateError(err)
}

func (r CatalogRepository) UpdateSpecies(ctx context.Context, id uuid.UUID, input service.CatalogRecord) (service.Species, error) {
	var species service.Species
	err := r.db.QueryRow(ctx, `
		UPDATE species
		SET chinese_name = $2, english_name = $3, updated_by = $4, updated_at = now()
		WHERE id = $1
		RETURNING id, chinese_name, english_name, created_at, updated_at
	`, id, input.ChineseName, input.EnglishName, input.ActorID).Scan(&species.ID, &species.ChineseName, &species.EnglishName, &species.CreatedAt, &species.UpdatedAt)
	return species, translateError(err)
}

func (r CatalogRepository) DeleteSpecies(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx, `DELETE FROM species WHERE id = $1`, id)
	return err
}
