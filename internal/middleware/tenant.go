package middleware

import (
	"github.com/gofiber/fiber/v2"
)

// TenantExtractor middleware pour extraire le tenant ID
func TenantExtractor() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Priorité 1: Header X-Tenant-ID
		tenantID := c.Get("X-Tenant-ID")
		
		// Priorité 2: JWT claims
		if tenantID == "" {
			if tid, ok := c.Locals("tenant_id").(string); ok {
				tenantID = tid
			}
		}

		// Priorité 3: Query param
		if tenantID == "" {
			tenantID = c.Query("tenant_id")
		}

		if tenantID == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Tenant ID required",
			})
		}

		// Stocker dans le contexte
		c.Locals("tenant_id", tenantID)
		return c.Next()
	}
}



