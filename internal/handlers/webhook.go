package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
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

	// Notifier l'order-service du changement de statut
	go func() {
		if err := notifyOrderService(payment.OrderID, payment.ID, "paid"); err != nil {
			log.Printf("⚠️ Failed to notify order-service: %v", err)
		}
	}()

	// Récupérer les détails de la commande depuis order-service
	orderDetails, err := getOrderDetails(payment.OrderID)
	if err != nil {
		log.Printf("⚠️ Failed to get order details: %v", err)
		return nil // Ne pas bloquer le traitement du webhook
	}

	// Récupérer l'email de l'utilisateur depuis auth-service
	userEmail, err := getUserEmail(orderDetails.UserID, payment.TenantID)
	if err != nil {
		log.Printf("⚠️ Failed to get user email: %v", err)
		userEmail = "customer@example.com" // Fallback
	}

	// Envoyer notification de confirmation (non bloquant)
	go func() {
		orderData := map[string]interface{}{
			"orderNumber": payment.OrderID,
			"amount":      payment.Amount,
			"currency":    payment.Currency,
			"status":      "paid",
			"tenant_id":   payment.TenantID,
		}
		if err := sendOrderConfirmation(userEmail, orderData); err != nil {
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

	// Récupérer le paiement pour avoir l'order_id
	var payment models.Payment
	if err := database.GetDB().Where("payment_intent_id = ?", paymentIntent.ID).First(&payment).Error; err != nil {
		log.Printf("❌ Payment not found in DB: %v", err)
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

	// Notifier l'order-service
	go func() {
		if err := notifyOrderService(payment.OrderID, payment.ID, "failed"); err != nil {
			log.Printf("⚠️ Failed to notify order-service: %v", err)
		}
	}()

	return nil
}

// handlePaymentCanceled traite un paiement annulé
func handlePaymentCanceled(event stripe.Event) error {
	var paymentIntent stripe.PaymentIntent
	if err := json.Unmarshal(event.Data.Raw, &paymentIntent); err != nil {
		return err
	}

	// Récupérer le paiement pour avoir l'order_id
	var payment models.Payment
	if err := database.GetDB().Where("payment_intent_id = ?", paymentIntent.ID).First(&payment).Error; err != nil {
		log.Printf("❌ Payment not found in DB: %v", err)
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

	// Notifier l'order-service
	go func() {
		if err := notifyOrderService(payment.OrderID, payment.ID, "cancelled"); err != nil {
			log.Printf("⚠️ Failed to notify order-service: %v", err)
		}
	}()

	return nil
}

// OrderDetails représente les détails d'une commande depuis order-service
type OrderDetails struct {
	ID     string `json:"id"`
	UserID string `json:"userId"`
}

// UserInfo représente les informations d'un utilisateur depuis auth-service
type UserInfo struct {
	Email string `json:"email"`
}

// notifyOrderService notifie l'order-service du changement de statut de paiement
func notifyOrderService(orderID, paymentID, paymentStatus string) error {
	orderServiceURL := os.Getenv("ORDER_SERVICE_URL")
	if orderServiceURL == "" {
		orderServiceURL = "http://order-service:3000"
	}

	payload := map[string]interface{}{
		"orderId":       orderID,
		"paymentId":     paymentID,
		"paymentStatus": paymentStatus,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Post(
		orderServiceURL+"/orders/webhook/payment-update",
		"application/json",
		bytes.NewBuffer(jsonData),
	)

	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		log.Printf("✅ Order-service notified for order %s", orderID)
	} else {
		log.Printf("⚠️ Order-service notification returned status %d", resp.StatusCode)
	}

	return nil
}

// getOrderDetails récupère les détails d'une commande depuis order-service
func getOrderDetails(orderID string) (*OrderDetails, error) {
	orderServiceURL := os.Getenv("ORDER_SERVICE_URL")
	if orderServiceURL == "" {
		orderServiceURL = "http://order-service:3000"
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(orderServiceURL + "/orders/" + orderID)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("order-service returned status %d", resp.StatusCode)
	}

	var orderDetails OrderDetails
	if err := json.NewDecoder(resp.Body).Decode(&orderDetails); err != nil {
		return nil, err
	}

	return &orderDetails, nil
}

// getUserEmail récupère l'email d'un utilisateur depuis auth-service
func getUserEmail(userID, tenantID string) (string, error) {
	authServiceURL := os.Getenv("AUTH_SERVICE_URL")
	if authServiceURL == "" {
		authServiceURL = "http://auth-service:8000"
	}

	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequest("GET", authServiceURL+"/users/"+userID, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("X-Tenant-ID", tenantID)

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("auth-service returned status %d", resp.StatusCode)
	}

	var userInfo UserInfo
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return "", err
	}

	return userInfo.Email, nil
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

