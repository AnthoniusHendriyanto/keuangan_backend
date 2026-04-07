package model

// LoginRequest is the request body for POST /v1/auth/login.
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// RegisterRequest is the request body for POST /v1/auth/register.
type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	FullName string `json:"full_name"`
}

// AuthUser is the simplified user object returned in the auth response.
type AuthUser struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

// AuthResponse is the standardized response returned from the backend for all auth operations.
// It wraps the Supabase JWT so the frontend never needs to know the underlying provider.
type AuthResponse struct {
	AccessToken  string   `json:"access_token"`
	RefreshToken string   `json:"refresh_token"`
	TokenType    string   `json:"token_type"`
	ExpiresIn    int      `json:"expires_in"`
	User         AuthUser `json:"user"`
}

// AuthErrorResponse is the standardized error returned for all auth failures.
type AuthErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
