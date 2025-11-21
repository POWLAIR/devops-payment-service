package middleware

import (
	"os"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

// JWTAuth middleware pour valider les tokens JWT
func JWTAuth() fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Missing authorization header",
			})
		}

		// Extract token
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == authHeader {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Invalid authorization format",
			})
		}

		// Parse token
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			// Vérifier l'algorithme
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fiber.NewError(fiber.StatusUnauthorized, "Invalid signing method")
			}
			return []byte(os.Getenv("JWT_SECRET")), nil
		})

		if err != nil || !token.Valid {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Invalid token",
			})
		}

		// Stocker les claims dans le contexte
		if claims, ok := token.Claims.(jwt.MapClaims); ok {
			c.Locals("user_id", claims["sub"])
			c.Locals("tenant_id", claims["tenant_id"])
		}

		return c.Next()
	}
}

