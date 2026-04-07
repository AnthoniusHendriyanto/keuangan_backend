package middleware

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"math/big"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var (
	es256KeyOnce sync.Once
	es256Pub     *ecdsa.PublicKey
)

// getES256PublicKey returns the Supabase ES256 public key by fetching it directly.
func getES256PublicKey() *ecdsa.PublicKey {
	es256KeyOnce.Do(func() {
		client := &http.Client{Timeout: 10 * time.Second}
		url := os.Getenv("SUPABASE_URL") + "/auth/v1/.well-known/jwks.json"
		
		resp, err := client.Get(url)
		if err != nil {
			slog.Error("Failed to fetch JWKS", "error", err)
			return
		}
		defer resp.Body.Close()

		var data struct {
			Keys []struct {
				X string `json:"x"`
				Y string `json:"y"`
			} `json:"keys"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&data); err != nil || len(data.Keys) == 0 {
			slog.Error("Failed to decode JWKS", "error", err)
			return
		}

		xStr := data.Keys[0].X
		yStr := data.Keys[0].Y

		xB, _ := base64.RawURLEncoding.DecodeString(xStr)
		yB, _ := base64.RawURLEncoding.DecodeString(yStr)

		es256Pub = &ecdsa.PublicKey{
			Curve: elliptic.P256(),
			X:     new(big.Int).SetBytes(xB),
			Y:     new(big.Int).SetBytes(yB),
		}
		slog.Info("Successfully fetched and cached ES256 Public Key from Supabase JWKS")
	})
	return es256Pub
}

// AuthMiddleware validates the Supabase JWT token.
func AuthMiddleware(c fiber.Ctx) error {
	authHeader := c.Get("Authorization")
	if authHeader == "" {
		slog.Warn("Missing authorization header")
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Missing authorization header"})
	}

	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		slog.Warn("Invalid authorization format")
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid authorization format"})
	}

	tokenString := parts[1]

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		alg, _ := token.Header["alg"].(string)

		// 1. Handle ES256 (Native Supabase tokens)
		if alg == "ES256" {
			return getES256PublicKey(), nil
		}

		// 2. Handle HS256 (Legacy secrets or service roles)
		if alg == "HS256" {
			secret := os.Getenv("SUPABASE_JWT_SECRET")
			
			// Handle potentially base64 encoded secrets from Supabase
			if decoded, err := base64.StdEncoding.DecodeString(secret); err == nil {
				return decoded, nil
			}
			
			return []byte(secret), nil
		}

		slog.Warn("Unsupported signing algorithm", "alg", alg)
		return nil, jwt.ErrSignatureInvalid
	})

	if err != nil {
		slog.Warn("JWT verification failed", "error", err)
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid or expired token", "details": err.Error()})
	}

	if !token.Valid {
		slog.Warn("JWT token is invalid")
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid or expired token"})
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		slog.Warn("Invalid token claims format")
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid token claims format"})
	}

	sub, ok := claims["sub"].(string)
	if !ok {
		slog.Warn("Sub claim missing in token")
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Sub claim missing"})
	}

	userID, err := uuid.Parse(sub)
	if err != nil {
		slog.Warn("Invalid user ID format in sub claim", "sub", sub)
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid user ID format in token"})
	}

	c.Locals("userID", userID)
	return c.Next()
}
