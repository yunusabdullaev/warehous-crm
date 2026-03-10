package auth

import (
	"strconv"
	"strings"
	"time"

	"warehouse-crm/configs"
	"warehouse-crm/internal/session"
	jwtPkg "warehouse-crm/pkg/jwt"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type Handler struct {
	service    *Service
	sessionSvc *session.Service
	tenantCol  *mongo.Collection
	cfg        *configs.Config
}

func NewHandler(service *Service, sessionSvc *session.Service, cfg *configs.Config, tenantCol ...*mongo.Collection) *Handler {
	h := &Handler{service: service, sessionSvc: sessionSvc, cfg: cfg}
	if len(tenantCol) > 0 {
		h.tenantCol = tenantCol[0]
	}
	return h
}

// ── Cookie helpers ──

func (h *Handler) setRefreshCookie(c *fiber.Ctx, token string, maxAge int) {
	sameSite := "Lax"
	switch strings.ToLower(h.cfg.CookieSameSite) {
	case "strict":
		sameSite = "Strict"
	case "none":
		sameSite = "None"
	}
	c.Cookie(&fiber.Cookie{
		Name:     "wms_refresh",
		Value:    token,
		Path:     "/api/v1/auth",
		Domain:   h.cfg.CookieDomain,
		MaxAge:   maxAge,
		Secure:   h.cfg.CookieSecure,
		HTTPOnly: true,
		SameSite: sameSite,
	})
}

func (h *Handler) setAccessCookie(c *fiber.Ctx, token string) {
	sameSite := "Lax"
	switch strings.ToLower(h.cfg.CookieSameSite) {
	case "strict":
		sameSite = "Strict"
	case "none":
		sameSite = "None"
	}
	c.Cookie(&fiber.Cookie{
		Name:     "wms_access",
		Value:    token,
		Path:     "/",
		Domain:   h.cfg.CookieDomain,
		MaxAge:   h.cfg.AccessTokenTTLMin * 60,
		Secure:   h.cfg.CookieSecure,
		HTTPOnly: true,
		SameSite: sameSite,
	})
}

func (h *Handler) setSessionIDCookie(c *fiber.Ctx, sessionID string) {
	c.Cookie(&fiber.Cookie{
		Name:     "wms_session_id",
		Value:    sessionID,
		Path:     "/",
		Domain:   h.cfg.CookieDomain,
		MaxAge:   h.cfg.RefreshTokenTTLDays * 86400,
		Secure:   h.cfg.CookieSecure,
		HTTPOnly: true,
		SameSite: "Lax",
	})
}

func (h *Handler) clearCookies(c *fiber.Ctx) {
	c.Cookie(&fiber.Cookie{Name: "wms_refresh", Value: "", Path: "/api/v1/auth", MaxAge: -1})
	c.Cookie(&fiber.Cookie{Name: "wms_access", Value: "", Path: "/", MaxAge: -1})
	c.Cookie(&fiber.Cookie{Name: "wms_session_id", Value: "", Path: "/", MaxAge: -1})
}

func clientIP(c *fiber.Ctx) string {
	if ip := c.Get("X-Forwarded-For"); ip != "" {
		return strings.Split(ip, ",")[0]
	}
	return c.IP()
}

// ── Auth Endpoints ──

func (h *Handler) Register(c *fiber.Ctx) error {
	var req RegisterRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	if req.Username == "" || req.Password == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "username and password are required"})
	}
	if req.Role == "" {
		req.Role = RoleOperator
	}

	// Enforce maxUsers limit if tenant is specified
	if req.TenantID != "" && req.Role != RoleSuperAdmin && h.tenantCol != nil {
		tenantOID, err := primitive.ObjectIDFromHex(req.TenantID)
		if err == nil {
			var tenant struct {
				Status string `bson:"status"`
				Plan   string `bson:"plan"`
				Limits struct {
					MaxUsers int `bson:"max_users"`
				} `bson:"limits"`
			}
			err = h.tenantCol.FindOne(c.Context(), bson.M{"_id": tenantOID}).Decode(&tenant)
			if err == nil {
				if tenant.Status == "SUSPENDED" {
					return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
						"error":   "TENANT_SUSPENDED",
						"message": "Tenant suspended. Contact your administrator.",
					})
				}
				if tenant.Limits.MaxUsers > 0 {
					currentUsers, _ := h.service.repo.CountByTenant(c.Context(), tenantOID)
					if currentUsers >= int64(tenant.Limits.MaxUsers) {
						return c.Status(fiber.StatusPaymentRequired).JSON(fiber.Map{
							"error":   "PLAN_LIMIT",
							"limit":   "maxUsers",
							"allowed": tenant.Limits.MaxUsers,
							"current": currentUsers,
							"plan":    tenant.Plan,
							"message": "Plan limit reached. Upgrade your plan to increase this limit.",
						})
					}
				}
			}
		}
	}

	resp, err := h.service.Register(c.Context(), &req)
	if err != nil {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(fiber.StatusCreated).JSON(resp)
}

func (h *Handler) Login(c *fiber.Ctx) error {
	var req LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	if req.Username == "" || req.Password == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "username and password are required"})
	}

	ip := clientIP(c)
	ua := c.Get("User-Agent")

	resp, refreshToken, sessionID, err := h.service.Login(c.Context(), &req, ip, ua)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
	}

	// If 2FA required, no cookies — return temp token
	if resp.Requires2FA {
		return c.JSON(resp)
	}

	// Set cookies
	h.setAccessCookie(c, resp.Token)
	h.setRefreshCookie(c, refreshToken, h.cfg.RefreshTokenTTLDays*86400)
	h.setSessionIDCookie(c, sessionID)

	return c.JSON(resp)
}

func (h *Handler) Login2FA(c *fiber.Ctx) error {
	var req Login2FARequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	if req.TempToken == "" || req.Code == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "temp_token and code are required"})
	}

	ip := clientIP(c)
	ua := c.Get("User-Agent")

	resp, refreshToken, sessionID, err := h.service.Login2FA(c.Context(), &req, ip, ua)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
	}

	h.setAccessCookie(c, resp.Token)
	h.setRefreshCookie(c, refreshToken, h.cfg.RefreshTokenTTLDays*86400)
	h.setSessionIDCookie(c, sessionID)

	return c.JSON(resp)
}

func (h *Handler) Refresh(c *fiber.Ctx) error {
	rawRefresh := c.Cookies("wms_refresh")
	if rawRefresh == "" {
		// Try JSON body fallback
		var body struct {
			RefreshToken string `json:"refresh_token"`
		}
		_ = c.BodyParser(&body)
		rawRefresh = body.RefreshToken
	}
	if rawRefresh == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "missing refresh token"})
	}

	ip := clientIP(c)

	// Try to get user info from the (possibly expired) access token for session validation
	accessToken := c.Cookies("wms_access")
	if accessToken == "" {
		auth := c.Get("Authorization")
		if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
			accessToken = auth[7:]
		}
	}

	// The access token may be expired, but we still parse it to get claims
	// RefreshSession in session service handles the actual validation
	var userID, username, role, tenantID string
	if accessToken != "" {
		// Parse without validation (token may be expired)
		claims, _ := parseExpiredToken(accessToken, h.cfg.JWTSecret)
		if claims != nil {
			userID = claims.UserID
			username = claims.Username
			role = claims.Role
			tenantID = claims.TenantID
		}
	}

	newAccess, newRefresh, err := h.sessionSvc.RefreshSession(c.Context(), rawRefresh, userID, username, role, tenantID, ip)
	if err != nil {
		h.clearCookies(c)
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid or expired refresh token"})
	}

	h.setAccessCookie(c, newAccess)
	h.setRefreshCookie(c, newRefresh, h.cfg.RefreshTokenTTLDays*86400)

	return c.JSON(RefreshResponse{Token: newAccess})
}

func (h *Handler) Logout(c *fiber.Ctx) error {
	rawRefresh := c.Cookies("wms_refresh")
	if rawRefresh == "" {
		var body struct {
			RefreshToken string `json:"refresh_token"`
		}
		_ = c.BodyParser(&body)
		rawRefresh = body.RefreshToken
	}
	if rawRefresh != "" {
		_ = h.sessionSvc.RevokeByRefreshToken(c.Context(), rawRefresh)
	}
	h.clearCookies(c)
	return c.JSON(fiber.Map{"message": "logged out"})
}

func (h *Handler) LogoutAll(c *fiber.Ctx) error {
	userIDStr, _ := c.Locals("userID").(string)
	userID, err := primitive.ObjectIDFromHex(userIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid user"})
	}

	count, err := h.sessionSvc.RevokeAllUserSessions(c.Context(), userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	h.clearCookies(c)
	return c.JSON(fiber.Map{"message": "all sessions revoked", "revoked": count})
}

func (h *Handler) ListSessions(c *fiber.Ctx) error {
	userIDStr, _ := c.Locals("userID").(string)

	// Admin can list sessions for another user
	targetUserID := c.Query("user_id", userIDStr)
	callerRole, _ := c.Locals("role").(string)
	if targetUserID != userIDStr && callerRole != RoleSuperAdmin && callerRole != RoleAdmin {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "access denied"})
	}

	uid, err := primitive.ObjectIDFromHex(targetUserID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid user id"})
	}

	sessions, err := h.sessionSvc.ListActiveSessions(c.Context(), uid)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	currentSessionID := c.Cookies("wms_session_id")

	out := make([]SessionResponse, 0, len(sessions))
	for _, sess := range sessions {
		sr := SessionResponse{
			ID:        sess.ID.Hex(),
			IP:        sess.IP,
			UserAgent: sess.UserAgent,
			CreatedAt: sess.CreatedAt.Format(time.RFC3339),
			LastUsed:  sess.LastUsedAt.Format(time.RFC3339),
			Current:   sess.ID.Hex() == currentSessionID,
		}
		out = append(out, sr)
	}

	return c.JSON(fiber.Map{"sessions": out})
}

func (h *Handler) RevokeSession(c *fiber.Ctx) error {
	sessionID, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid session id"})
	}

	if err := h.sessionSvc.RevokeSession(c.Context(), sessionID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"message": "session revoked"})
}

// ── Password Reset ──

func (h *Handler) GenerateResetToken(c *fiber.Ctx) error {
	targetUserID, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid user id"})
	}

	callerIDStr, _ := c.Locals("userID").(string)
	callerID, _ := primitive.ObjectIDFromHex(callerIDStr)

	token, err := h.sessionSvc.GenerateResetToken(c.Context(), targetUserID, callerID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(ResetTokenResponse{
		Token:     token,
		ExpiresIn: "30 minutes",
	})
}

func (h *Handler) ResetPassword(c *fiber.Ctx) error {
	var req ResetPasswordRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	if req.Token == "" || req.NewPassword == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "token and new_password are required"})
	}

	if err := h.service.ResetPassword(c.Context(), &req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"message": "password reset successfully"})
}

// ── 2FA ──

func (h *Handler) Setup2FA(c *fiber.Ctx) error {
	userIDStr, _ := c.Locals("userID").(string)
	userID, err := primitive.ObjectIDFromHex(userIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid user"})
	}

	resp, err := h.service.Setup2FA(c.Context(), userID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(resp)
}

func (h *Handler) Verify2FA(c *fiber.Ctx) error {
	var req Verify2FARequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	userIDStr, _ := c.Locals("userID").(string)
	userID, err := primitive.ObjectIDFromHex(userIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid user"})
	}

	if err := h.service.Verify2FA(c.Context(), userID, req.Code); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"message": "2FA enabled successfully"})
}

func (h *Handler) Disable2FA(c *fiber.Ctx) error {
	userIDStr, _ := c.Locals("userID").(string)
	userID, err := primitive.ObjectIDFromHex(userIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid user"})
	}

	if err := h.service.Disable2FA(c.Context(), userID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"message": "2FA disabled"})
}

// ── User Management Handlers ──

func (h *Handler) List(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))

	callerRole, _ := c.Locals("role").(string)
	callerTenantID := getTenantFromLocals(c)

	resp, err := h.service.ListUsers(c.Context(), page, limit, callerTenantID, callerRole)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(resp)
}

func (h *Handler) GetByID(c *fiber.Ctx) error {
	id, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}

	resp, err := h.service.GetUser(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err.Error()})
	}

	callerRole, _ := c.Locals("role").(string)
	if callerRole != RoleSuperAdmin {
		callerTenantID := getTenantFromLocals(c)
		targetTenantID, _ := primitive.ObjectIDFromHex(resp.TenantID)
		if !callerTenantID.IsZero() && !targetTenantID.IsZero() && callerTenantID != targetTenantID {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "cross-tenant access denied"})
		}
	}

	return c.JSON(resp)
}

func (h *Handler) Update(c *fiber.Ctx) error {
	id, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}

	var req UpdateUserRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	callerRole, _ := c.Locals("role").(string)
	if callerRole != RoleSuperAdmin {
		target, err := h.service.GetUser(c.Context(), id)
		if err != nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err.Error()})
		}
		callerTenantID := getTenantFromLocals(c)
		targetTenantID, _ := primitive.ObjectIDFromHex(target.TenantID)
		if !callerTenantID.IsZero() && !targetTenantID.IsZero() && callerTenantID != targetTenantID {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "cross-tenant access denied"})
		}
		req.TenantID = ""
	}

	resp, err := h.service.UpdateUser(c.Context(), id, &req)
	if err != nil {
		if err.Error() == "user not found" {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(resp)
}

func (h *Handler) Delete(c *fiber.Ctx) error {
	id, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}

	callerRole, _ := c.Locals("role").(string)
	if callerRole != RoleSuperAdmin {
		target, err := h.service.GetUser(c.Context(), id)
		if err != nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err.Error()})
		}
		callerTenantID := getTenantFromLocals(c)
		targetTenantID, _ := primitive.ObjectIDFromHex(target.TenantID)
		if !callerTenantID.IsZero() && !targetTenantID.IsZero() && callerTenantID != targetTenantID {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "cross-tenant access denied"})
		}
	}

	callerID, _ := c.Locals("userID").(string)

	if err := h.service.DeleteUser(c.Context(), id, callerID); err != nil {
		if err.Error() == "user not found" {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err.Error()})
		}
		if err.Error() == "cannot delete yourself" {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"message": "user deleted"})
}

func (h *Handler) RevokeUserSessions(c *fiber.Ctx) error {
	targetUserID, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid user id"})
	}

	count, err := h.sessionSvc.RevokeAllUserSessions(c.Context(), targetUserID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"message": "sessions revoked", "revoked": count})
}

// ── Helpers ──

func getTenantFromLocals(c *fiber.Ctx) primitive.ObjectID {
	tenantIDStr, _ := c.Locals("tenantID").(string)
	if tenantIDStr == "" {
		return primitive.NilObjectID
	}
	oid, err := primitive.ObjectIDFromHex(tenantIDStr)
	if err != nil {
		return primitive.NilObjectID
	}
	return oid
}

// parseExpiredToken parses a JWT without validating expiry.
// Used during refresh to extract user info from an expired access token.
func parseExpiredToken(tokenStr, secret string) (*struct {
	UserID   string
	Username string
	Role     string
	TenantID string
}, error) {
	claims, err := jwtPkg.ParseTokenIgnoringExpiry(tokenStr, secret)
	if err != nil {
		return nil, err
	}
	return &struct {
		UserID   string
		Username string
		Role     string
		TenantID string
	}{claims.UserID, claims.Username, claims.Role, claims.TenantID}, nil
}
