package middleware

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"encoding/base64"
	"log/slog"
	"math/big"
	"os"
	"strings"
	"sync"

	"github.com/gofiber/fiber/v3"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var (
	es256KeyOnce sync.Once
	es256Pub     *ecdsa.PublicKey
)

// getES256PublicKey returns the Supabase ES256 public key.
// We use the components from the JWKS: x, y, and crv (P-256).
func getES256PublicKey() *ecdsa.PublicKey {
	es256KeyOnce.Do(func() {
		// These coordinates are from the project's JWKS:
		// kid: 76c775ef-ab63-4ba3-8941-0399ff16dec0
		xStr := "pxE7pW4-QpEicC3N32i5yS4fC3zW_lT6v6tY2p_mB48"
		yStr := "SJvGW0LAa4s7n5gu0j8PQ6undYO5ziBwa7RCqhj2Pw"

		xB, _ := base64.RawURLEncoding.DecodeString(xStr)
		yB, _ := base64.RawURLEncoding.DecodeString(yStr)

		es256Pub = &ecdsa.PublicKey{
			Curve: elliptic.P256(),
			X:     new(big.Int).SetBytes(xB),
			Y:     new(big.Int).SetBytes(yB),
		}
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
