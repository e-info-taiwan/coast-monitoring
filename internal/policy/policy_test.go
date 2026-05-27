package policy

import (
	"testing"

	"github.com/google/uuid"
)

func TestAdminCanUseAdminAPI(t *testing.T) {
	user := User{ID: uuid.New(), Role: RoleAdmin, Status: StatusActive}
	if !CanUseAdminAPI(user) {
		t.Fatal("admin should use admin API")
	}
}

func TestVolunteerCannotUseAdminAPI(t *testing.T) {
	user := User{ID: uuid.New(), Role: RoleVolunteer, Status: StatusActive}
	if CanUseAdminAPI(user) {
		t.Fatal("volunteer should not use admin API")
	}
}

func TestDisabledUserCannotUseAPIs(t *testing.T) {
	user := User{ID: uuid.New(), Role: RoleAdmin, Status: StatusDisabled}
	if CanUseAdminAPI(user) || CanUseAppAPI(user) {
		t.Fatal("disabled user should not use APIs")
	}
}

func TestCanUseAppAPI(t *testing.T) {
	tests := []struct {
		name string
		user User
		want bool
	}{
		{
			name: "active admin",
			user: User{ID: uuid.New(), Role: RoleAdmin, Status: StatusActive},
			want: true,
		},
		{
			name: "active volunteer",
			user: User{ID: uuid.New(), Role: RoleVolunteer, Status: StatusActive},
			want: true,
		},
		{
			name: "unknown role",
			user: User{ID: uuid.New(), Role: Role("unknown"), Status: StatusActive},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CanUseAppAPI(tt.user); got != tt.want {
				t.Fatalf("CanUseAppAPI() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUnknownRoleCannotUseAdminAPI(t *testing.T) {
	user := User{ID: uuid.New(), Role: Role("unknown"), Status: StatusActive}
	if CanUseAdminAPI(user) {
		t.Fatal("unknown role should not use admin API")
	}
}

func TestObservationOwnership(t *testing.T) {
	ownerID := uuid.New()
	otherID := uuid.New()
	owner := User{ID: ownerID, Role: RoleVolunteer, Status: StatusActive}
	if !CanManageObservation(owner, ownerID) {
		t.Fatal("volunteer should manage own observation")
	}
	if CanManageObservation(owner, otherID) {
		t.Fatal("volunteer should not manage another user's observation")
	}
	admin := User{ID: otherID, Role: RoleAdmin, Status: StatusActive}
	if !CanManageObservation(admin, ownerID) {
		t.Fatal("admin should manage any observation")
	}
}

func TestDisabledUsersCannotManageObservations(t *testing.T) {
	ownerID := uuid.New()
	otherID := uuid.New()

	tests := []struct {
		name       string
		user       User
		observerID uuid.UUID
	}{
		{
			name:       "disabled volunteer own observation",
			user:       User{ID: ownerID, Role: RoleVolunteer, Status: StatusDisabled},
			observerID: ownerID,
		},
		{
			name:       "disabled admin any observation",
			user:       User{ID: otherID, Role: RoleAdmin, Status: StatusDisabled},
			observerID: ownerID,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if CanManageObservation(tt.user, tt.observerID) {
				t.Fatal("disabled user should not manage observation")
			}
		})
	}
}
