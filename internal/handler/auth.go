package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"regexp"
	"strings"

	"keuangan_backend/internal/model"

	"github.com/gofiber/fiber/v3"
)

// emailRegex is a simple but effective email validation pattern.
var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

// supabaseAuthError maps known Supabase auth error messages to internal error codes.
// This decouples the frontend from Supabase-specific error strings (Poka-Yoke / Standardization).
func mapSupabaseError(statusCode int, body []byte) model.AuthErrorResponse {
	// Try to parse the Supabase error message
	var supabaseErr struct {
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
		Msg              string `json:"msg"`
	}
	_ = json.Unmarshal(body, &supabaseErr)

	raw := strings.ToLower(supabaseErr.ErrorDescription + supabaseErr.Msg + supabaseErr.Error)

	switch {
	case strings.Contains(raw, "invalid login credentials") || strings.Contains(raw, "invalid password"):
		return model.AuthErrorResponse{Code: "invalid_credentials", Message: "The email or password you entered is incorrect."}
	case strings.Contains(raw, "user already registered") || strings.Contains(raw, "already exists"):
		return model.AuthErrorResponse{Code: "user_already_exists", Message: "An account with this email already exists."}
	case strings.Contains(raw, "email not confirmed"):
		return model.AuthErrorResponse{Code: "email_not_confirmed", Message: "Please confirm your email address before signing in."}
	case statusCode == http.StatusTooManyRequests:
		return model.AuthErrorResponse{Code: "rate_limited", Message: "Too many attempts. Please try again later."}
	default:
		return model.AuthErrorResponse{Code: "auth_error", Message: "Authentication failed. Please try again."}
	}
}

// AuthHandler encapsulates auth-related HTTP handlers.
type AuthHandler struct {
	supabaseURL string
	anonKey     string
	httpClient  *http.Client
}

// NewAuthHandler creates a new AuthHandler, reading config from environment at startup.
// Fails fast if required config is missing (Poka-Yoke: mandatory config).
func NewAuthHandler() *AuthHandler {
	supabaseURL := os.Getenv("SUPABASE_URL")
	anonKey := os.Getenv("SUPABASE_ANON_KEY")

	if supabaseURL == "" || anonKey == "" {
		// Panic at startup, not at request time.
		panic("SUPABASE_URL and SUPABASE_ANON_KEY environment variables are required")
	}

	return &AuthHandler{
		supabaseURL: supabaseURL,
		anonKey:     anonKey,
		httpClient:  &http.Client{},
	}
}

// Login handles POST /v1/auth/login.
func (h *AuthHandler) Login(c fiber.Ctx) error {
	var req model.LoginRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(model.AuthErrorResponse{
			Code:    "invalid_request",
			Message: "Request body is invalid.",
		})
	}

	// --- Pre-flight Validation (Poka-Yoke) ---
	if strings.TrimSpace(req.Email) == "" || strings.TrimSpace(req.Password) == "" {
		return c.Status(fiber.StatusBadRequest).JSON(model.AuthErrorResponse{
			Code:    "missing_fields",
			Message: "Email and password are required.",
		})
	}
	if !emailRegex.MatchString(req.Email) {
		return c.Status(fiber.StatusBadRequest).JSON(model.AuthErrorResponse{
			Code:    "invalid_email",
			Message: "Please provide a valid email address.",
		})
	}
	if len(req.Password) < 6 {
		return c.Status(fiber.StatusBadRequest).JSON(model.AuthErrorResponse{
			Code:    "weak_password",
			Message: "Password must be at least 6 characters.",
		})
	}

	// --- Supabase Proxy ---
	payload, _ := json.Marshal(map[string]string{
		"email":    req.Email,
		"password": req.Password,
	})

	url := fmt.Sprintf("%s/auth/v1/token?grant_type=password", h.supabaseURL)
	supaResp, err := h.doSupabaseRequest(http.MethodPost, url, payload)
	if err != nil {
		slog.Error("Failed to reach Supabase auth", "error", err)
		return c.Status(fiber.StatusServiceUnavailable).JSON(model.AuthErrorResponse{
			Code:    "service_unavailable",
			Message: "Authentication service is temporarily unavailable.",
		})
	}
	defer supaResp.Body.Close()

	respBody, _ := io.ReadAll(supaResp.Body)

	if supaResp.StatusCode != http.StatusOK {
		mapped := mapSupabaseError(supaResp.StatusCode, respBody)
		return c.Status(fiber.StatusUnauthorized).JSON(mapped)
	}

	// --- Map to standardized AuthResponse ---
	var raw struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int    `json:"expires_in"`
		User         struct {
			ID    string `json:"id"`
			Email string `json:"email"`
		} `json:"user"`
	}
	if err := json.Unmarshal(respBody, &raw); err != nil {
		slog.Error("Failed to parse Supabase login response", "error", err)
		return c.Status(fiber.StatusInternalServerError).JSON(model.AuthErrorResponse{
			Code:    "internal_error",
			Message: "An internal error occurred.",
		})
	}

	return c.Status(fiber.StatusOK).JSON(model.AuthResponse{
		AccessToken:  raw.AccessToken,
		RefreshToken: raw.RefreshToken,
		TokenType:    raw.TokenType,
		ExpiresIn:    raw.ExpiresIn,
		User: model.AuthUser{
			ID:    raw.User.ID,
			Email: raw.User.Email,
		},
	})
}

// Register handles POST /v1/auth/register.
func (h *AuthHandler) Register(c fiber.Ctx) error {
	var req model.RegisterRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(model.AuthErrorResponse{
			Code:    "invalid_request",
			Message: "Request body is invalid.",
		})
	}

	// --- Pre-flight Validation (Poka-Yoke) ---
	if strings.TrimSpace(req.Email) == "" || strings.TrimSpace(req.Password) == "" {
		return c.Status(fiber.StatusBadRequest).JSON(model.AuthErrorResponse{
			Code:    "missing_fields",
			Message: "Email and password are required.",
		})
	}
	if !emailRegex.MatchString(req.Email) {
		return c.Status(fiber.StatusBadRequest).JSON(model.AuthErrorResponse{
			Code:    "invalid_email",
			Message: "Please provide a valid email address.",
		})
	}
	if len(req.Password) < 6 {
		return c.Status(fiber.StatusBadRequest).JSON(model.AuthErrorResponse{
			Code:    "weak_password",
			Message: "Password must be at least 6 characters.",
		})
	}

	// --- Supabase Proxy ---
	payload, _ := json.Marshal(map[string]interface{}{
		"email":    req.Email,
		"password": req.Password,
		"data": map[string]string{
			"full_name": req.FullName,
		},
	})

	url := fmt.Sprintf("%s/auth/v1/signup", h.supabaseURL)
	supaResp, err := h.doSupabaseRequest(http.MethodPost, url, payload)
	if err != nil {
		slog.Error("Failed to reach Supabase signup", "error", err)
		return c.Status(fiber.StatusServiceUnavailable).JSON(model.AuthErrorResponse{
			Code:    "service_unavailable",
			Message: "Authentication service is temporarily unavailable.",
		})
	}
	defer supaResp.Body.Close()

	respBody, _ := io.ReadAll(supaResp.Body)

	if supaResp.StatusCode != http.StatusOK && supaResp.StatusCode != http.StatusCreated {
		mapped := mapSupabaseError(supaResp.StatusCode, respBody)
		return c.Status(fiber.StatusBadRequest).JSON(mapped)
	}

	// Supabase signup response structure
	var raw struct {
		ID    string `json:"id"`
		Email string `json:"email"`
	}
	_ = json.Unmarshal(respBody, &raw)

	slog.Info("New user registered", "email", raw.Email)
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"message": "Registration successful. Please check your email to confirm your account.",
		"user": model.AuthUser{
			ID:    raw.ID,
			Email: raw.Email,
		},
	})
}

// doSupabaseRequest is a helper to call Supabase's Auth API with the required headers.
func (h *AuthHandler) doSupabaseRequest(method, url string, payload []byte) (*http.Response, error) {
	req, err := http.NewRequest(method, url, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("apikey", h.anonKey)
	return h.httpClient.Do(req)
}
