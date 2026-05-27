package policy

import "github.com/google/uuid"

type Role string
type Status string

const (
	RoleAdmin     Role = "admin"
	RoleVolunteer Role = "volunteer"

	StatusActive   Status = "active"
	StatusDisabled Status = "disabled"
)

type User struct {
	ID     uuid.UUID
	Email  string
	Name   string
	Role   Role
	Status Status
}

func CanUseAdminAPI(user User) bool {
	return user.Status == StatusActive && user.Role == RoleAdmin
}

func CanUseAppAPI(user User) bool {
	return user.Status == StatusActive && (user.Role == RoleAdmin || user.Role == RoleVolunteer)
}

func CanManageObservation(user User, observerID uuid.UUID) bool {
	if user.Status != StatusActive {
		return false
	}
	if user.Role == RoleAdmin {
		return true
	}
	return user.Role == RoleVolunteer && user.ID == observerID
}
