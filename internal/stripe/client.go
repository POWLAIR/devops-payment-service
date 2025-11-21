package stripe

import (
	"os"

	"github.com/stripe/stripe-go/v76"
	"github.com/stripe/stripe-go/v76/paymentintent"
	"github.com/stripe/stripe-go/v76/webhook"
)

// InitStripe initialise le client Stripe
func InitStripe() {
	stripe.Key = os.Getenv("STRIPE_SECRET_KEY")
}

// CreatePaymentIntent crée un Payment Intent Stripe
func CreatePaymentIntent(amount int64, currency, tenantID, orderID string) (*stripe.PaymentIntent, error) {
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

