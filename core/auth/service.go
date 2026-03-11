package auth

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/crypto/bcrypt"

	"warehouse-crm/core/session"
	jwtPkg "warehouse-crm/pkg/jwt"
)

type Service struct {
	repo       Repository
	sessionSvc *session.Service
	tenantCol  *mongo.Collection
	jwtSecret  string
	jwtExpiry  int // hours (backward compat)
	accessTTL  int // minutes
}

func NewService(repo Repository, sessionSvc *session.Service, tenantCol *mongo.Collection, jwtSecret string, jwtExpiry, accessTTLMin int) *Service {
	return &Service{
		repo:       repo,
		sessionSvc: sessionSvc,
		tenantCol:  tenantCol,
		jwtSecret:  jwtSecret,
		jwtExpiry:  jwtExpiry,
		accessTTL:  accessTTLMin,
	}
}

// EnsureSuperAdmin creates the root account if missing.
func (s *Service) EnsureSuperAdmin(ctx context.Context, username, password string) error {
	existing, _ := s.repo.FindByUsername(ctx, username)
	if existing != nil {
		return nil
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	user := &User{
		Username: username,
		Password: string(hashed),
		Role:     RoleSuperAdmin,
	}

	return s.repo.Create(ctx, user)
}

// ── Authentication ──

func (s *Service) Register(ctx context.Context, req *RegisterRequest) (*AuthResponse, error) {
	existing, _ := s.repo.FindByUsername(ctx, req.Username)
	if existing != nil {
		return nil, errors.New("username already exists")
	}

	if err := ValidatePassword(req.Password); err != nil {
		return nil, err
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	user := &User{
		Username: req.Username,
		Password: string(hashedPassword),
		Role:     req.Role,
	}

	if req.TenantID != "" {
		oid, err := primitive.ObjectIDFromHex(req.TenantID)
		if err == nil {
			user.TenantID = oid
		}
	}

	if err := s.repo.Create(ctx, user); err != nil {
		return nil, err
	}

	token, err := s.sessionSvc.IssueAccessToken(
		user.ID.Hex(), user.Username, user.Role, zeroSafeHex(user.TenantID),
	)
	if err != nil {
		return nil, err
	}

	return buildAuthResponse(token, user), nil
}

// Login authenticates user, checks brute-force, handles 2FA.
// Returns (AuthResponse, refreshToken, sessionID hex, error).
func (s *Service) Login(ctx context.Context, req *LoginRequest, ip, userAgent string) (*AuthResponse, string, string, error) {
	// Brute-force check
	locked, _ := s.sessionSvc.CheckBruteForce(ctx, req.Username, ip)
	if locked {
		return nil, "", "", errors.New("account temporarily locked — try again later")
	}

	user, err := s.repo.FindByUsername(ctx, req.Username)
	if err != nil {
		s.sessionSvc.RecordLoginAttempt(ctx, req.Username, ip, false)
		return nil, "", "", errors.New("invalid credentials")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		s.sessionSvc.RecordLoginAttempt(ctx, req.Username, ip, false)
		return nil, "", "", errors.New("invalid credentials")
	}

	s.sessionSvc.RecordLoginAttempt(ctx, req.Username, ip, true)

	// Check 2FA
	if user.TwoFactorEnabled {
		tempToken, err := s.sessionSvc.IssueTempToken(user.ID.Hex(), user.Username)
		if err != nil {
			return nil, "", "", err
		}
		resp := &AuthResponse{}
		resp.Requires2FA = true
		resp.TempToken = tempToken
		resp.User.ID = user.ID.Hex()
		resp.User.Username = user.Username
		return resp, "", "", nil
	}

	// No 2FA — issue tokens
	token, err := s.sessionSvc.IssueAccessToken(
		user.ID.Hex(), user.Username, user.Role, zeroSafeHex(user.TenantID),
	)
	if err != nil {
		return nil, "", "", err
	}

	// Create session with refresh token
	sessionID, refreshToken, err := s.sessionSvc.CreateSession(ctx, user.ID, user.TenantID, ip, userAgent)
	if err != nil {
		return nil, "", "", err
	}

	resp := buildAuthResponse(token, user)
	return resp, refreshToken, sessionID.Hex(), nil
}

// Login2FA completes the second step of 2FA login.
func (s *Service) Login2FA(ctx context.Context, req *Login2FARequest, ip, userAgent string) (*AuthResponse, string, string, error) {
	claims, err := s.sessionSvc.ValidateTempToken(req.TempToken)
	if err != nil {
		return nil, "", "", errors.New("invalid or expired temporary token")
	}

	userID, err := primitive.ObjectIDFromHex(claims.UserID)
	if err != nil {
		return nil, "", "", errors.New("invalid token")
	}

	// Validate TOTP code
	if err := s.validate2FACode(ctx, userID, req.Code); err != nil {
		return nil, "", "", err
	}

	user, err := s.repo.FindByID(ctx, userID)
	if err != nil {
		return nil, "", "", errors.New("user not found")
	}

	token, err := s.sessionSvc.IssueAccessToken(
		user.ID.Hex(), user.Username, user.Role, zeroSafeHex(user.TenantID),
	)
	if err != nil {
		return nil, "", "", err
	}

	sessionID, refreshToken, err := s.sessionSvc.CreateSession(ctx, user.ID, user.TenantID, ip, userAgent)
	if err != nil {
		return nil, "", "", err
	}

	resp := buildAuthResponse(token, user)
	return resp, refreshToken, sessionID.Hex(), nil
}

// Refresh rotates the refresh token and issues a new access token.
func (s *Service) Refresh(ctx context.Context, rawRefreshToken, ip string) (accessToken, newRefreshToken string, err error) {
	// We need user info from the current access token — but it may be expired.
	// So we decode from the refresh token's session.
	return s.sessionSvc.RefreshSession(ctx, rawRefreshToken, "", "", "", "", ip)
}

// RefreshWithClaims rotates with known user claims.
func (s *Service) RefreshWithClaims(ctx context.Context, rawRefreshToken string, claims *jwtPkg.Claims, ip string) (string, string, error) {
	return s.sessionSvc.RefreshSession(ctx, rawRefreshToken, claims.UserID, claims.Username, claims.Role, claims.TenantID, ip)
}

// ResetPassword validates a reset token and changes password.
func (s *Service) ResetPassword(ctx context.Context, req *ResetPasswordRequest) error {
	if err := ValidatePassword(req.NewPassword); err != nil {
		return err
	}

	userID, err := s.sessionSvc.ValidateResetToken(ctx, req.Token)
	if err != nil {
		return err
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	if err := s.repo.Update(ctx, userID, bson.M{"password": string(hashed)}); err != nil {
		return err
	}

	// Consume the token
	_ = s.sessionSvc.ConsumeResetToken(ctx, req.Token)

	// Revoke all sessions for this user
	_, _ = s.sessionSvc.RevokeAllUserSessions(ctx, userID)

	return nil
}

// ── User Management ──

func (s *Service) ListUsers(ctx context.Context, page, limit int, callerTenantID primitive.ObjectID, callerRole string) (*UserListResponse, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	var users []User
	var total int64
	var err error

	if callerRole == RoleSuperAdmin {
		users, err = s.repo.List(ctx, page, limit)
		if err != nil {
			return nil, err
		}
		total, err = s.repo.Count(ctx)
	} else {
		users, err = s.repo.ListByTenant(ctx, callerTenantID, page, limit)
		if err != nil {
			return nil, err
		}
		total, err = s.repo.CountByTenant(ctx, callerTenantID)
	}
	if err != nil {
		return nil, err
	}

	data := make([]UserResponse, 0, len(users))
	for _, u := range users {
		data = append(data, toUserResponse(&u))
	}

	return &UserListResponse{
		Data:  data,
		Total: total,
		Page:  page,
		Limit: limit,
	}, nil
}

func (s *Service) GetUser(ctx context.Context, id primitive.ObjectID) (*UserResponse, error) {
	user, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, errors.New("user not found")
	}
	resp := toUserResponse(user)
	return &resp, nil
}

func (s *Service) UpdateUser(ctx context.Context, id primitive.ObjectID, req *UpdateUserRequest) (*UserResponse, error) {
	user, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, errors.New("user not found")
	}

	update := bson.M{}

	if req.Username != "" && req.Username != user.Username {
		existing, _ := s.repo.FindByUsername(ctx, req.Username)
		if existing != nil {
			return nil, errors.New("username already exists")
		}
		update["username"] = req.Username
	}

	if req.Role != "" {
		update["role"] = req.Role
	}

	if req.Password != "" {
		if err := ValidatePassword(req.Password); err != nil {
			return nil, err
		}
		hashed, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			return nil, err
		}
		update["password"] = string(hashed)
	}

	if req.TenantID != "" {
		oid, err := primitive.ObjectIDFromHex(req.TenantID)
		if err == nil {
			update["tenant_id"] = oid
		}
	}

	if req.AllowedWarehouseIDs != nil {
		ids := make([]primitive.ObjectID, 0, len(req.AllowedWarehouseIDs))
		for _, s := range req.AllowedWarehouseIDs {
			oid, err := primitive.ObjectIDFromHex(s)
			if err == nil {
				ids = append(ids, oid)
			}
		}
		update["allowed_warehouse_ids"] = ids
	}

	if req.DefaultWarehouseID != "" {
		oid, err := primitive.ObjectIDFromHex(req.DefaultWarehouseID)
		if err == nil {
			update["default_warehouse_id"] = oid
		}
	}

	// Resolve final role for validation
	finalRole := user.Role
	if r, ok := update["role"]; ok {
		finalRole = r.(string)
	}

	finalAllowed := user.AllowedWarehouseIDs
	if ids, ok := update["allowed_warehouse_ids"]; ok {
		finalAllowed = ids.([]primitive.ObjectID)
	}
	finalDefault := user.DefaultWarehouseID
	if oid, ok := update["default_warehouse_id"]; ok {
		finalDefault = oid.(primitive.ObjectID)
	}

	if finalRole != RoleAdmin && finalRole != RoleSuperAdmin && len(finalAllowed) == 0 {
		return nil, errors.New("non-admin users must have at least one allowed warehouse")
	}

	if !finalDefault.IsZero() && len(finalAllowed) > 0 {
		found := false
		for _, id := range finalAllowed {
			if id == finalDefault {
				found = true
				break
			}
		}
		if !found {
			return nil, errors.New("default warehouse must be in the allowed warehouses list")
		}
	}

	if len(update) == 0 {
		resp := toUserResponse(user)
		return &resp, nil
	}

	if err := s.repo.Update(ctx, id, update); err != nil {
		return nil, err
	}

	updated, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	resp := toUserResponse(updated)
	return &resp, nil
}

func (s *Service) DeleteUser(ctx context.Context, id primitive.ObjectID, callerID string) error {
	if id.Hex() == callerID {
		return errors.New("cannot delete yourself")
	}

	_, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return errors.New("user not found")
	}

	return s.repo.Delete(ctx, id)
}

// ── helpers ──

func buildAuthResponse(token string, user *User) *AuthResponse {
	resp := &AuthResponse{}
	resp.Token = token
	resp.User.ID = user.ID.Hex()
	resp.User.Username = user.Username
	resp.User.Role = user.Role
	resp.User.TenantID = zeroSafeHex(user.TenantID)
	resp.User.AllowedWarehouseIDs = objectIDsToStrings(user.AllowedWarehouseIDs)
	resp.User.DefaultWarehouseID = zeroSafeHex(user.DefaultWarehouseID)
	resp.User.TwoFactorEnabled = user.TwoFactorEnabled
	return resp
}

func toUserResponse(u *User) UserResponse {
	return UserResponse{
		ID:                  u.ID.Hex(),
		Username:            u.Username,
		Role:                u.Role,
		TenantID:            zeroSafeHex(u.TenantID),
		AllowedWarehouseIDs: objectIDsToStrings(u.AllowedWarehouseIDs),
		DefaultWarehouseID:  zeroSafeHex(u.DefaultWarehouseID),
		TwoFactorEnabled:    u.TwoFactorEnabled,
		CreatedAt:           u.CreatedAt.Format(time.RFC3339),
		UpdatedAt:           u.UpdatedAt.Format(time.RFC3339),
	}
}

func objectIDsToStrings(ids []primitive.ObjectID) []string {
	if ids == nil {
		return []string{}
	}
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = id.Hex()
	}
	return out
}

func zeroSafeHex(id primitive.ObjectID) string {
	if id.IsZero() {
		return ""
	}
	return id.Hex()
}
