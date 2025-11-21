package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Payment struct {
	ID                  string    `gorm:"type:uuid;primaryKey" json:"id"`
	TenantID            string    `gorm:"type:uuid;not null;index" json:"tenant_id"`
	OrderID             string    `gorm:"type:uuid" json:"order_id"`
	PaymentIntentID     string    `gorm:"unique;not null" json:"payment_intent_id"`
	Amount              float64   `gorm:"not null" json:"amount"`
	Currency            string    `gorm:"default:eur" json:"currency"`
	Status              string    `gorm:"type:payment_status_enum;default:pending" json:"status"`
	PlatformCommission  float64   `json:"platform_commission"`
	StripeAccountID     string    `json:"stripe_account_id,omitempty"`
	Metadata            string    `gorm:"type:jsonb" json:"metadata,omitempty"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

// BeforeCreate hook pour générer UUID
func (p *Payment) BeforeCreate(tx *gorm.DB) error {
	if p.ID == "" {
		p.ID = uuid.New().String()
	}
	return nil
}

// PaymentStatus constants
const (
	StatusPending    = "pending"
	StatusProcessing = "processing"
	StatusSucceeded  = "succeeded"
	StatusFailed     = "failed"
	StatusRefunded   = "refunded"
	StatusCancelled  = "cancelled"
)

