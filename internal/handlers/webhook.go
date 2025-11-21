package handlers

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"payment-service/internal/database"
	"payment-service/internal/models"
	stripeClient "payment-service/internal/stripe"
	"time"

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

	// Récupérer le paiement depuis la DB pour avoir toutes les infos
	var payment models.Payment
	if err := database.GetDB().Where("payment_intent_id = ?", paymentIntent.ID).First(&payment).Error; err != nil {
		log.Printf("❌ Payment not found in DB: %v", err)
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

	// Envoyer notification de confirmation (non bloquant)
	go func() {
		orderData := map[string]interface{}{
			"orderNumber": payment.OrderID,
			"amount":      payment.Amount,
			"currency":    payment.Currency,
			"status":      "paid",
			"tenant_id":   payment.TenantID,
		}
		// TODO: Récupérer l'email réel depuis user-service ou order-service
		email := "customer@example.com"
		if err := sendOrderConfirmation(email, orderData); err != nil {
			log.Printf("⚠️ Failed to send notification: %v", err)
		}
	}()

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

// sendOrderConfirmation envoie une notification de confirmation de commande
func sendOrderConfirmation(email string, orderData map[string]interface{}) error {
	notificationURL := os.Getenv("NOTIFICATION_SERVICE_URL")
	if notificationURL == "" {
		notificationURL = "http://notification-service:6000"
	}

	payload := map[string]interface{}{
		"email": email,
		"order_data": orderData,
		"tenant_settings": map[string]string{
			"name":  "SaaS Platform",
			"email": "contact@saas-platform.com",
			"url":   "http://localhost:3001",
		},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		log.Printf("⚠️ Failed to marshal notification payload: %v", err)
		return err
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Post(
		notificationURL+"/api/v1/notifications/order-confirmation",
		"application/json",
		bytes.NewBuffer(jsonData),
	)

	if err != nil {
		log.Printf("⚠️ Failed to send order confirmation: %v", err)
		return err // Non bloquant selon votre logique métier
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		log.Printf("✅ Order confirmation queued for %s", email)
	} else {
		log.Printf("⚠️ Order confirmation returned status %d", resp.StatusCode)
	}

	return nil
}

