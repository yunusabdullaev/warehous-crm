package billing

import (
	"io"
	"log/slog"

	"github.com/gofiber/fiber/v2"
	"github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/webhook"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// CreateCheckoutSession creates a Stripe checkout session for subscription.
// POST /billing/checkout-session
// Body: {"plan": "PRO"|"ENTERPRISE"}
func (h *Handler) CreateCheckoutSession(c *fiber.Ctx) error {
	tenantID, err := h.callerTenantID(c)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	var req struct {
		Plan string `json:"plan"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}
	if req.Plan != "PRO" && req.Plan != "ENTERPRISE" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "plan must be PRO or ENTERPRISE"})
	}

	url, err := h.svc.CreateCheckoutSession(c.Context(), tenantID, req.Plan)
	if err != nil {
		slog.Error("billing: checkout session failed", "error", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"url": url})
}

// CreatePortalSession creates a Stripe billing portal session.
// POST /billing/portal-session
func (h *Handler) CreatePortalSession(c *fiber.Ctx) error {
	tenantID, err := h.callerTenantID(c)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	url, err := h.svc.CreatePortalSession(c.Context(), tenantID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"url": url})
}

// GetBillingStatus returns the billing status for the caller's tenant.
// GET /billing/status
func (h *Handler) GetBillingStatus(c *fiber.Ctx) error {
	tenantID, err := h.callerTenantID(c)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	status, err := h.svc.GetBillingStatus(c.Context(), tenantID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(status)
}

// Webhook handles Stripe webhook events.
// POST /webhooks/stripe (no auth)
func (h *Handler) Webhook(c *fiber.Ctx) error {
	body := c.Body()
	sigHeader := c.Get("Stripe-Signature")

	var event stripe.Event

	if h.svc.cfg.WebhookTestMode {
		// Dev-only: parse without signature verification
		slog.Warn("billing: webhook test mode — signature verification SKIPPED")
		if err := decodeJSON(body, &event); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid payload"})
		}
	} else {
		var err error
		event, err = webhook.ConstructEvent(body, sigHeader, h.svc.cfg.WebhookSecret)
		if err != nil {
			slog.Error("billing: webhook signature verification failed", "error", err)
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid signature"})
		}
	}

	alreadyProcessed, err := h.svc.HandleWebhookEvent(c.Context(), event)
	if err != nil {
		slog.Error("billing: webhook processing failed", "error", err, "event_id", event.ID, "type", event.Type)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "processing failed"})
	}
	if alreadyProcessed {
		return c.JSON(fiber.Map{"status": "already_processed"})
	}

	return c.JSON(fiber.Map{"status": "ok"})
}

// callerTenantID extracts the tenant ID from JWT context.
// For superadmins, it can be overridden via query param ?tenant_id=xxx.
func (h *Handler) callerTenantID(c *fiber.Ctx) (primitive.ObjectID, error) {
	role, _ := c.Locals("role").(string)

	// Superadmin can operate on any tenant
	if role == "superadmin" {
		if qid := c.Query("tenant_id"); qid != "" {
			return primitive.ObjectIDFromHex(qid)
		}
	}

	tid, _ := c.Locals("tenantID").(string)
	if tid == "" {
		return primitive.NilObjectID, io.ErrUnexpectedEOF
	}
	return primitive.ObjectIDFromHex(tid)
}
