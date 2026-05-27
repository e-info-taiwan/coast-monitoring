package httpx

type CurrentUserResponse struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
	Role  string `json:"role"`
}

type SessionResponse struct {
	Authenticated bool                 `json:"authenticated"`
	User          *CurrentUserResponse `json:"user,omitempty"`
	CSRFToken     string               `json:"csrfToken,omitempty"`
}

type AdminUserResponse struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	Name        string `json:"name"`
	Role        string `json:"role"`
	Status      string `json:"status"`
	HasGoogle   bool   `json:"hasGoogle"`
	HasPassword bool   `json:"hasPassword"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}

type SaveUserRequest struct {
	Email    string `json:"email"`
	Name     string `json:"name"`
	Role     string `json:"role"`
	Status   string `json:"status"`
	Password string `json:"password"`
}

type CatalogResponse struct {
	ID          string `json:"id"`
	ChineseName string `json:"chineseName"`
	EnglishName string `json:"englishName"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}

type SaveCatalogRequest struct {
	ChineseName string `json:"chineseName"`
	EnglishName string `json:"englishName"`
}

type ObservationResponse struct {
	ID         string `json:"id"`
	ObservedOn string `json:"observedOn"`
	LocationID string `json:"locationId"`
	SpeciesID  string `json:"speciesId"`
	ObserverID string `json:"observerId"`
	Count      int    `json:"count"`
	Notes      string `json:"notes"`
	CreatedAt  string `json:"createdAt"`
	UpdatedAt  string `json:"updatedAt"`
}

type SaveObservationRequest struct {
	ObservedOn string `json:"observedOn"`
	LocationID string `json:"locationId"`
	SpeciesID  string `json:"speciesId"`
	ObserverID string `json:"observerId"`
	Count      int    `json:"count"`
	Notes      string `json:"notes"`
}

type AuditLogResponse struct {
	ID          string `json:"id"`
	Action      string `json:"action"`
	TargetTable string `json:"targetTable"`
	TargetID    string `json:"targetId"`
	ActorUserID string `json:"actorUserId,omitempty"`
	ActorEmail  string `json:"actorEmail"`
	BeforeData  any    `json:"beforeData,omitempty"`
	AfterData   any    `json:"afterData,omitempty"`
	Method      string `json:"method"`
	Path        string `json:"path"`
	IP          string `json:"ip"`
	UserAgent   string `json:"userAgent"`
	LoggedAt    string `json:"loggedAt"`
}
