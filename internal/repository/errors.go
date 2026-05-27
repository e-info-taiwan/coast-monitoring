package repository

import (
	"errors"

	"coast-monitoring/internal/service"

	"github.com/jackc/pgx/v5"
)

func translateError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return service.ErrNotFound
	}
	return err
}
