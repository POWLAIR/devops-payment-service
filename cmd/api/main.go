package main

import (
	"fmt"
	"log"
	"os"

	"payment-service/internal/database"
	"payment-service/internal/handlers"
	"payment-service/internal/middleware"
	stripeClient "payment-service/internal/stripe"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/joho/godotenv"
)

func main() {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}

	// Initialize Stripe
	stripeClient.InitStripe()
	log.Println("✅ Stripe initialized")

	// Initialize Database
	if err := database.Connect(); err != nil {
		log.Fatalf("❌ Failed to connect to database: %v", err)
	}

	// Run migrations
	if err := database.AutoMigrate(); err != nil {
		log.Fatalf("❌ Failed to run migrations: %v", err)
	}

	// Initialize Fiber app
	app := fiber.New(fiber.Config{
		AppName: "Payment Service",
	})

	// Global middleware
	app.Use(recover.New())
	app.Use(logger.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins: os.Getenv("CORS_ORIGINS"),
		AllowHeaders: "Origin, Content-Type, Accept, Authorization, X-Tenant-ID",
	}))

	// Health check (public)
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":  "ok",
			"service": "payment-service",
		})
	})

	// API routes
	api := app.Group("/api/v1")

	// Webhook routes (pas de JWT, validation par signature Stripe)
	webhooks := api.Group("/webhooks")
	webhooks.Post("/stripe", handlers.StripeWebhook)

	// Payment routes (protégées par JWT + Tenant)
	payments := api.Group("/payments", middleware.JWTAuth(), middleware.TenantExtractor())
	payments.Post("/create-intent", handlers.CreatePaymentIntent)
	payments.Get("/", handlers.ListPayments)
	payments.Get("/:id", handlers.GetPayment)
	payments.Post("/:id/simulate-success", handlers.SimulatePaymentSuccess)

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "5000"
	}

	log.Printf("🚀 Payment Service (Go) starting on port %s", port)
	if err := app.Listen(fmt.Sprintf(":%s", port)); err != nil {
		log.Fatal(err)
	}
}




