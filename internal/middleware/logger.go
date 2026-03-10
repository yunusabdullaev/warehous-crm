package middleware

import (
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

func RequestLogger() fiber.Handler {
	return func(c *fiber.Ctx) error {
		requestID := uuid.New().String()
		c.Locals("requestID", requestID)
		c.Set("X-Request-ID", requestID)

		start := time.Now()
		err := c.Next()
		latency := time.Since(start)

		slog.Info("request",
			"request_id", requestID,
			"method", c.Method(),
			"path", c.Path(),
			"status", c.Response().StatusCode(),
			"latency_ms", latency.Milliseconds(),
			"ip", c.IP(),
			"user_agent", c.Get("User-Agent"),
		)

		return err
	}
}
