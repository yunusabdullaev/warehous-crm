package notify

import (
	"strings"

	"github.com/gofiber/fiber/v2"
)

type Handler struct {
	service *Service
	repo    *Repository
}

func NewHandler(service *Service, repo *Repository) *Handler {
	return &Handler{service: service, repo: repo}
}

// Get returns current notification settings (token masked).
func (h *Handler) Get(c *fiber.Ctx) error {
	settings, err := h.repo.GetSettings(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Mask token for display
	maskedToken := ""
	if settings.TelegramToken != "" {
		if len(settings.TelegramToken) > 8 {
			maskedToken = settings.TelegramToken[:4] + strings.Repeat("*", len(settings.TelegramToken)-8) + settings.TelegramToken[len(settings.TelegramToken)-4:]
		} else {
			maskedToken = "****"
		}
	}

	return c.JSON(fiber.Map{
		"telegram_enabled":       settings.TelegramEnabled,
		"telegram_bot_token":     maskedToken,
		"telegram_chat_ids":      settings.TelegramChatIDs,
		"expiry_digest_enabled":  settings.ExpiryDigestEnabled,
		"expiry_digest_days":     settings.ExpiryDigestDays,
		"expiry_digest_time":     settings.ExpiryDigestTime,
		"expiry_digest_chat_ids": settings.ExpiryDigestChatIDs,
		"updated_at":             settings.UpdatedAt,
	})
}

// Update saves notification settings.
func (h *Handler) Update(c *fiber.Ctx) error {
	var req struct {
		TelegramEnabled     bool   `json:"telegram_enabled"`
		TelegramToken       string `json:"telegram_bot_token"`
		TelegramChatIDs     string `json:"telegram_chat_ids"`
		ExpiryDigestEnabled *bool  `json:"expiry_digest_enabled,omitempty"`
		ExpiryDigestDays    *int   `json:"expiry_digest_days,omitempty"`
		ExpiryDigestTime    string `json:"expiry_digest_time,omitempty"`
		ExpiryDigestChatIDs string `json:"expiry_digest_chat_ids,omitempty"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	// If token looks masked (contains ***), keep existing token
	existing, err := h.repo.GetSettings(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	token := req.TelegramToken
	if strings.Contains(token, "***") || token == "" {
		token = existing.TelegramToken
	}

	// Backwards-compatible: keep existing digest settings if not sent
	digestEnabled := existing.ExpiryDigestEnabled
	if req.ExpiryDigestEnabled != nil {
		digestEnabled = *req.ExpiryDigestEnabled
	}
	digestDays := existing.ExpiryDigestDays
	if req.ExpiryDigestDays != nil {
		digestDays = *req.ExpiryDigestDays
	}
	digestTime := existing.ExpiryDigestTime
	if req.ExpiryDigestTime != "" {
		digestTime = req.ExpiryDigestTime
	}
	digestChatIDs := existing.ExpiryDigestChatIDs
	if req.ExpiryDigestChatIDs != "" {
		digestChatIDs = req.ExpiryDigestChatIDs
	}

	settings := &NotifySettings{
		TelegramEnabled:     req.TelegramEnabled,
		TelegramToken:       token,
		TelegramChatIDs:     req.TelegramChatIDs,
		ExpiryDigestEnabled: digestEnabled,
		ExpiryDigestDays:    digestDays,
		ExpiryDigestTime:    digestTime,
		ExpiryDigestChatIDs: digestChatIDs,
	}

	if err := h.repo.UpsertSettings(c.Context(), settings); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"message": "settings updated"})
}

// Test sends a test Telegram message.
func (h *Handler) Test(c *fiber.Ctx) error {
	if err := h.service.SendTest(); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"message": "test message queued"})
}

// RunDigest triggers the expiry digest now.
func (h *Handler) RunDigest(c *fiber.Ctx) error {
	force := c.Query("force", "false") == "true"

	result, err := h.service.RunExpiryDigest(c.Context(), force)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(result)
}
