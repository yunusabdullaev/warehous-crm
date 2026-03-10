package auth

type RegisterRequest struct {
	Username string `json:"username" validate:"required,min=3,max=50"`
	Password string `json:"password" validate:"required,min=6"`
	Role     string `json:"role" validate:"required,oneof=admin operator viewer"`
	TenantID string `json:"tenant_id,omitempty"`
}

type LoginRequest struct {
	Username string `json:"username" validate:"required"`
	Password string `json:"password" validate:"required"`
}

type Login2FARequest struct {
	TempToken string `json:"temp_token" validate:"required"`
	Code      string `json:"code" validate:"required"`
}

type AuthResponse struct {
	Token       string `json:"token"`
	Requires2FA bool   `json:"requires_2fa,omitempty"`
	TempToken   string `json:"temp_token,omitempty"`
	User        struct {
		ID                  string   `json:"id"`
		Username            string   `json:"username"`
		Role                string   `json:"role"`
		TenantID            string   `json:"tenant_id"`
		AllowedWarehouseIDs []string `json:"allowed_warehouse_ids"`
		DefaultWarehouseID  string   `json:"default_warehouse_id"`
		TwoFactorEnabled    bool     `json:"two_factor_enabled"`
	} `json:"user"`
}

type RefreshResponse struct {
	Token string `json:"token"`
}

type SessionResponse struct {
	ID        string `json:"id"`
	IP        string `json:"ip,omitempty"`
	UserAgent string `json:"user_agent,omitempty"`
	CreatedAt string `json:"created_at"`
	LastUsed  string `json:"last_used_at"`
	Current   bool   `json:"current,omitempty"`
}

type ResetTokenResponse struct {
	Token     string `json:"token"`
	ExpiresIn string `json:"expires_in"`
}

type ResetPasswordRequest struct {
	Token       string `json:"token" validate:"required"`
	NewPassword string `json:"new_password" validate:"required"`
}

type Setup2FAResponse struct {
	Secret string `json:"secret"`
	URI    string `json:"uri"`
	QR     string `json:"qr"` // base64 PNG
}

type Verify2FARequest struct {
	Code string `json:"code" validate:"required"`
}

// ── User Management DTOs ──

type UpdateUserRequest struct {
	Username            string   `json:"username,omitempty"`
	Password            string   `json:"password,omitempty"`
	Role                string   `json:"role,omitempty"`
	TenantID            string   `json:"tenant_id,omitempty"`
	AllowedWarehouseIDs []string `json:"allowed_warehouse_ids,omitempty"`
	DefaultWarehouseID  string   `json:"default_warehouse_id,omitempty"`
}

type UserResponse struct {
	ID                  string   `json:"id"`
	Username            string   `json:"username"`
	Role                string   `json:"role"`
	TenantID            string   `json:"tenant_id"`
	AllowedWarehouseIDs []string `json:"allowed_warehouse_ids"`
	DefaultWarehouseID  string   `json:"default_warehouse_id"`
	TwoFactorEnabled    bool     `json:"two_factor_enabled"`
	CreatedAt           string   `json:"created_at"`
	UpdatedAt           string   `json:"updated_at"`
}

type UserListResponse struct {
	Data  []UserResponse `json:"data"`
	Total int64          `json:"total"`
	Page  int            `json:"page"`
	Limit int            `json:"limit"`
}
