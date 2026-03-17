package handlers

import (
	"payment-service/internal/database"
	"payment-service/internal/models"
	stripeClient "payment-service/internal/stripe"

	"github.com/gofiber/fiber/v2"
)

// CreatePaymentIntentRequest structure de la requête
type CreatePaymentIntentRequest struct {
	Amount   float64 `json:"amount" validate:"required,gt=0"`
	Currency string  `json:"currency"`
	OrderID  string  `json:"order_id" validate:"required"`
}

// CreatePaymentIntent crée un Payment Intent Stripe
func CreatePaymentIntent(c *fiber.Ctx) error {
	var req CreatePaymentIntentRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Valeurs par défaut
	if req.Currency == "" {
		req.Currency = "eur"
	}

	// Récupérer le tenant ID du contexte
	tenantID, ok := c.Locals("tenant_id").(string)
	if !ok || tenantID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Tenant ID required",
		})
	}

	userID, _ := c.Locals("user_id").(string)
	userEmail, _ := c.Locals("user_email").(string)

	// Calculer la commission (5%)
	commission := req.Amount * 0.05

	// Créer le Payment Intent Stripe
	amountCents := int64(req.Amount * 100) // Convertir en centimes
	pi, err := stripeClient.CreatePaymentIntent(amountCents, req.Currency, tenantID, req.OrderID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create payment intent: " + err.Error(),
		})
	}

	// Sauvegarder dans la DB
	payment := models.Payment{
		TenantID:           tenantID,
		OrderID:            req.OrderID,
		UserID:             userID,
		UserEmail:          userEmail,
		PaymentIntentID:    pi.ID,
		Amount:             req.Amount,
		Currency:           req.Currency,
		Status:             models.StatusPending,
		PlatformCommission: commission,
	}

	if err := database.GetDB().Create(&payment).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to save payment: " + err.Error(),
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"client_secret": pi.ClientSecret,
		"payment_id":    payment.ID,
		"amount":        payment.Amount,
		"commission":    commission,
	})
}

// ListPayments liste les paiements d'un tenant
func ListPayments(c *fiber.Ctx) error {
	tenantID, ok := c.Locals("tenant_id").(string)
	if !ok || tenantID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Tenant ID required",
		})
	}

	var payments []models.Payment
	if err := database.GetDB().Where("tenant_id = ?", tenantID).Order("created_at DESC").Find(&payments).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to retrieve payments",
		})
	}

	return c.JSON(fiber.Map{
		"payments": payments,
		"count":    len(payments),
	})
}

// GetPayment récupère un paiement par ID
func GetPayment(c *fiber.Ctx) error {
	paymentID := c.Params("id")
	tenantID, _ := c.Locals("tenant_id").(string)

	var payment models.Payment
	query := database.GetDB().Where("id = ?", paymentID)
	if tenantID != "" {
		query = query.Where("tenant_id = ?", tenantID)
	}

	if err := query.First(&payment).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Payment not found",
		})
	}

	return c.JSON(payment)
}



