package repository

import (
	"errors"
	"fmt"
	"strings"

	"coast-monitoring/internal/service"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func translateError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return service.ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			return fmt.Errorf("%w: %s", service.ErrConflict, pgErr.Message)
		case "23503":
			return fmt.Errorf("%w: %s", service.ErrInvalidReference, pgErr.Message)
		case "23514":
			return fmt.Errorf("%w: %s", service.ErrValidation, pgErr.Message)
		}
		if strings.HasPrefix(pgErr.Code, "22") {
			return fmt.Errorf("%w: %s", service.ErrValidation, pgErr.Message)
		}
	}
	return err
}

func requireRowsAffected(tag pgconn.CommandTag, err error) error {
	if err != nil {
		return translateError(err)
	}
	if tag.RowsAffected() == 0 {
		return service.ErrNotFound
	}
	return nil
}
