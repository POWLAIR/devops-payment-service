package handlers

import (
	"encoding/json"
	"log"
	"payment-service/internal/database"
	"payment-service/internal/models"
	stripeClient "payment-service/internal/stripe"

	"github.com/gofiber/fiber/v2"
	"github.com/stripe/stripe-go/v76"
)

// StripeWebhook gère les webhooks Stripe
func StripeWebhook(c *fiber.Ctx) error {
	payload := c.Body()
	signature := c.Get("Stripe-Signature")

	// Vérifier la signature
	event, err := stripeClient.VerifyWebhookSignature(payload, signature)
	if err != nil {
		log.Printf("❌ Webhook signature verification failed: %v", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid signature",
		})
	}

	// Traiter l'événement selon son type
	switch event.Type {
	case "payment_intent.succeeded":
		return handlePaymentSucceeded(event)
	case "payment_intent.payment_failed":
		return handlePaymentFailed(event)
	case "payment_intent.canceled":
		return handlePaymentCanceled(event)
	default:
		log.Printf("ℹ️ Unhandled event type: %s", event.Type)
	}

	return c.JSON(fiber.Map{"received": true})
}

// handlePaymentSucceeded traite un paiement réussi
func handlePaymentSucceeded(event stripe.Event) error {
	var paymentIntent stripe.PaymentIntent
	if err := json.Unmarshal(event.Data.Raw, &paymentIntent); err != nil {
		log.Printf("❌ Error parsing webhook JSON: %v", err)
		return err
	}

	// Mettre à jour le paiement dans la DB
	result := database.GetDB().Model(&models.Payment{}).
		Where("payment_intent_id = ?", paymentIntent.ID).
		Updates(map[string]interface{}{
			"status": models.StatusSucceeded,
		})

	if result.Error != nil {
		log.Printf("❌ Failed to update payment: %v", result.Error)
		return result.Error
	}

	log.Printf("✅ Payment succeeded: %s", paymentIntent.ID)
	return nil
}

// handlePaymentFailed traite un paiement échoué
func handlePaymentFailed(event stripe.Event) error {
	var paymentIntent stripe.PaymentIntent
	if err := json.Unmarshal(event.Data.Raw, &paymentIntent); err != nil {
		return err
	}

	result := database.GetDB().Model(&models.Payment{}).
		Where("payment_intent_id = ?", paymentIntent.ID).
		Updates(map[string]interface{}{
			"status": models.StatusFailed,
		})

	if result.Error != nil {
		log.Printf("❌ Failed to update payment: %v", result.Error)
		return result.Error
	}

	log.Printf("⚠️ Payment failed: %s", paymentIntent.ID)
	return nil
}

// handlePaymentCanceled traite un paiement annulé
func handlePaymentCanceled(event stripe.Event) error {
	var paymentIntent stripe.PaymentIntent
	if err := json.Unmarshal(event.Data.Raw, &paymentIntent); err != nil {
		return err
	}

	result := database.GetDB().Model(&models.Payment{}).
		Where("payment_intent_id = ?", paymentIntent.ID).
		Updates(map[string]interface{}{
			"status": models.StatusCancelled,
		})

	if result.Error != nil {
		log.Printf("❌ Failed to update payment: %v", result.Error)
		return result.Error
	}

	log.Printf("ℹ️ Payment canceled: %s", paymentIntent.ID)
	return nil
}

