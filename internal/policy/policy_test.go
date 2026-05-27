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
