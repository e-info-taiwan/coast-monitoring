package repository

import (
	"errors"
	"testing"

	"coast-monitoring/internal/service"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestTranslateErrorMapsPostgresCodes(t *testing.T) {
	tests := []struct {
		name string
		code string
		want error
	}{
		{name: "unique violation", code: "23505", want: service.ErrConflict},
		{name: "foreign key violation", code: "23503", want: service.ErrInvalidReference},
		{name: "check violation", code: "23514", want: service.ErrValidation},
		{name: "invalid text representation", code: "22P02", want: service.ErrValidation},
		{name: "data exception class", code: "22001", want: service.ErrValidation},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := translateError(&pgconn.PgError{Code: tt.code})
			if !errors.Is(err, tt.want) {
				t.Fatalf("translateError = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestRequireRowsAffectedReturnsNotFoundForZeroRows(t *testing.T) {
	err := requireRowsAffected(pgconn.NewCommandTag("DELETE 0"), nil)
	if !errors.Is(err, service.ErrNotFound) {
		t.Fatalf("requireRowsAffected error = %v, want %v", err, service.ErrNotFound)
	}
}

func TestRequireRowsAffectedReturnsTranslatedExecError(t *testing.T) {
	err := requireRowsAffected(pgconn.CommandTag{}, &pgconn.PgError{Code: "23503"})
	if !errors.Is(err, service.ErrInvalidReference) {
		t.Fatalf("requireRowsAffected error = %v, want %v", err, service.ErrInvalidReference)
	}
}
