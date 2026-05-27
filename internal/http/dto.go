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
