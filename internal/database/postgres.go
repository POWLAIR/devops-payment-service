package database

import (
	"fmt"
	"log"
	"os"

	"payment-service/internal/models"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

// Connect établit la connexion PostgreSQL
func Connect() error {
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable",
		os.Getenv("DB_HOST"),
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_NAME"),
		os.Getenv("DB_PORT"),
	)

	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	log.Println("✅ Database connection established")
	return nil
}

// AutoMigrate exécute les migrations
func AutoMigrate() error {
	// Vérifier si l'enum existe déjà
	checkEnumSQL := `SELECT EXISTS (
		SELECT 1 FROM pg_type WHERE typname = 'payment_status_enum'
	)`
	var enumExists bool
	if err := DB.Raw(checkEnumSQL).Scan(&enumExists).Error; err != nil {
		return fmt.Errorf("failed to check enum type: %w", err)
	}

	// Créer le type enum s'il n'existe pas
	if !enumExists {
		createEnumSQL := `CREATE TYPE payment_status_enum AS ENUM ('pending', 'processing', 'succeeded', 'failed', 'refunded', 'cancelled')`
		if err := DB.Exec(createEnumSQL).Error; err != nil {
			return fmt.Errorf("failed to create enum type: %w", err)
		}
		log.Println("✅ Created payment_status_enum type")
	}

	if err := DB.AutoMigrate(&models.Payment{}); err != nil {
		return fmt.Errorf("failed to migrate: %w", err)
	}
	log.Println("✅ Database migrations completed")
	return nil
}

// GetDB retourne l'instance DB
func GetDB() *gorm.DB {
	return DB
}



