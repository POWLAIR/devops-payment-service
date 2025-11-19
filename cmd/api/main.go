package main

import (
	"fmt"
	"log"
	"os"

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

	// Initialize Fiber app
	app := fiber.New(fiber.Config{
		AppName: "Payment Service",
	})

	// Middleware
	app.Use(recover.New())
	app.Use(logger.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins: os.Getenv("CORS_ORIGINS"),
		AllowHeaders: "Origin, Content-Type, Accept, Authorization, X-Tenant-ID",
	}))

	// Health check
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":  "ok",
			"service": "payment-service",
		})
	})

	// Routes
	api := app.Group("/api/v1")
	
	// Payment routes
	payments := api.Group("/payments")
	payments.Post("/create-intent", func(c *fiber.Ctx) error {
		// TODO: Implement
		return c.JSON(fiber.Map{"message": "Create payment intent"})
	})
	
	payments.Get("/", func(c *fiber.Ctx) error {
		// TODO: Implement
		return c.JSON(fiber.Map{"message": "List payments"})
	})

	// Webhook routes
	webhooks := api.Group("/webhooks")
	webhooks.Post("/stripe", func(c *fiber.Ctx) error {
		// TODO: Implement
		return c.JSON(fiber.Map{"received": true})
	})

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "5000"
	}

	log.Printf("🚀 Payment Service starting on port %s", port)
	if err := app.Listen(fmt.Sprintf(":%s", port)); err != nil {
		log.Fatal(err)
	}
}

