package stripe

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/stripe/stripe-go/v76"
	"github.com/stripe/stripe-go/v76/paymentintent"
	"github.com/stripe/stripe-go/v76/webhook"
)

// IsSimulationMode retourne true si la clé Stripe est un placeholder
func IsSimulationMode() bool {
	key := os.Getenv("STRIPE_SECRET_KEY")
	return key == "" || strings.HasPrefix(key, "sk_test_example") || key == "sk_test_example"
}

// InitStripe initialise le client Stripe
func InitStripe() {
	stripe.Key = os.Getenv("STRIPE_SECRET_KEY")
	if IsSimulationMode() {
		log.Println("⚠️  Stripe simulation mode: STRIPE_SECRET_KEY is a placeholder — payment intents will be simulated")
	}
}

// CreatePaymentIntent crée un Payment Intent Stripe (ou simulé en dev)
func CreatePaymentIntent(amount int64, currency, tenantID, orderID string) (*stripe.PaymentIntent, error) {
	if IsSimulationMode() {
		fakeID := fmt.Sprintf("pi_sim_%s", uuid.New().String()[:16])
		fakeSecret := fmt.Sprintf("%s_secret_%s", fakeID, uuid.New().String()[:8])
		log.Printf("⚠️  Stripe simulation: created fake PaymentIntent %s for order %s", fakeID, orderID)
		return &stripe.PaymentIntent{
			ID:           fakeID,
			ClientSecret: fakeSecret,
			Amount:       amount,
			Currency:     stripe.Currency(currency),
			Status:       stripe.PaymentIntentStatusRequiresPaymentMethod,
			Created:      time.Now().Unix(),
		}, nil
	}

	params := &stripe.PaymentIntentParams{
		Amount:   stripe.Int64(amount),
		Currency: stripe.String(currency),
		Metadata: map[string]string{
			"tenant_id": tenantID,
			"order_id":  orderID,
		},
	}

	return paymentintent.New(params)
}

// VerifyWebhookSignature vérifie la signature du webhook Stripe
func VerifyWebhookSignature(payload []byte, signature string) (stripe.Event, error) {
	secret := os.Getenv("STRIPE_WEBHOOK_SECRET")
	return webhook.ConstructEvent(payload, signature, secret)
}

// GetPaymentIntent récupère un Payment Intent
func GetPaymentIntent(id string) (*stripe.PaymentIntent, error) {
	return paymentintent.Get(id, nil)
}



