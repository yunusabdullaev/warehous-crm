package billing

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/stripe/stripe-go/v81"
	billingportal "github.com/stripe/stripe-go/v81/billingportal/session"
	checkoutsession "github.com/stripe/stripe-go/v81/checkout/session"
	stripecustomer "github.com/stripe/stripe-go/v81/customer"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	"warehouse-crm/internal/middleware"
	"warehouse-crm/internal/tenant"
)

// StripeConfig holds Stripe-related configuration.
type StripeConfig struct {
	SecretKey       string
	WebhookSecret   string
	PricePro        string
	PriceEnterprise string
	SuccessURL      string
	CancelURL       string
	WebhookTestMode bool
}

type Service struct {
	repo      *Repository
	tenantCol *mongo.Collection
	tenantSvc *tenant.Service
	cfg       StripeConfig
}

func NewService(repo *Repository, tenantCol *mongo.Collection, tenantSvc *tenant.Service, cfg StripeConfig) *Service {
	// Set the global Stripe API key
	if cfg.SecretKey != "" {
		stripe.Key = cfg.SecretKey
	}
	return &Service{
		repo:      repo,
		tenantCol: tenantCol,
		tenantSvc: tenantSvc,
		cfg:       cfg,
	}
}

// ── Checkout Session ──

func (s *Service) CreateCheckoutSession(ctx context.Context, tenantID primitive.ObjectID, plan string) (string, error) {
	t, err := s.tenantSvc.GetByID(ctx, tenantID)
	if err != nil {
		return "", errors.New("tenant not found")
	}

	priceID := s.priceIDForPlan(plan)
	if priceID == "" {
		return "", errors.New("invalid plan: must be PRO or ENTERPRISE")
	}

	// Create or reuse Stripe customer
	customerID := t.StripeCustomerID
	if customerID == "" {
		params := &stripe.CustomerParams{
			Name:  stripe.String(t.Name),
			Email: nil, // optional: add tenant email if available
		}
		params.AddMetadata("tenant_id", tenantID.Hex())
		params.AddMetadata("tenant_code", t.Code)
		cust, err := stripecustomer.New(params)
		if err != nil {
			return "", err
		}
		customerID = cust.ID

		// Store customer ID on tenant
		s.tenantCol.UpdateOne(ctx, bson.M{"_id": tenantID}, bson.M{
			"$set": bson.M{
				"stripe_customer_id": customerID,
				"updated_at":         time.Now().UTC(),
			},
		})
	}

	// Create checkout session
	sessionParams := &stripe.CheckoutSessionParams{
		Customer: stripe.String(customerID),
		Mode:     stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{Price: stripe.String(priceID), Quantity: stripe.Int64(1)},
		},
		SuccessURL: stripe.String(s.cfg.SuccessURL),
		CancelURL:  stripe.String(s.cfg.CancelURL),
	}
	sessionParams.AddMetadata("tenant_id", tenantID.Hex())

	sess, err := checkoutsession.New(sessionParams)
	if err != nil {
		return "", err
	}

	return sess.URL, nil
}

// ── Portal Session ──

func (s *Service) CreatePortalSession(ctx context.Context, tenantID primitive.ObjectID) (string, error) {
	t, err := s.tenantSvc.GetByID(ctx, tenantID)
	if err != nil {
		return "", errors.New("tenant not found")
	}
	if t.StripeCustomerID == "" {
		return "", errors.New("no billing account found. Subscribe to a plan first")
	}

	params := &stripe.BillingPortalSessionParams{
		Customer:  stripe.String(t.StripeCustomerID),
		ReturnURL: stripe.String(s.cfg.SuccessURL),
	}
	sess, err := billingportal.New(params)
	if err != nil {
		return "", err
	}

	return sess.URL, nil
}

// ── Billing Status ──

type BillingStatusResponse struct {
	Plan              string `json:"plan"`
	Status            string `json:"status"`
	BillingStatus     string `json:"billing_status"`
	StripeCustomerID  string `json:"stripe_customer_id,omitempty"`
	CurrentPeriodEnd  string `json:"current_period_end,omitempty"`
	CancelAtPeriodEnd bool   `json:"cancel_at_period_end"`
}

func (s *Service) GetBillingStatus(ctx context.Context, tenantID primitive.ObjectID) (*BillingStatusResponse, error) {
	t, err := s.tenantSvc.GetByID(ctx, tenantID)
	if err != nil {
		return nil, errors.New("tenant not found")
	}

	resp := &BillingStatusResponse{
		Plan:              t.Plan,
		Status:            t.Status,
		BillingStatus:     t.BillingStatus,
		StripeCustomerID:  t.StripeCustomerID,
		CancelAtPeriodEnd: t.CancelAtPeriodEnd,
	}
	if !t.CurrentPeriodEnd.IsZero() {
		resp.CurrentPeriodEnd = t.CurrentPeriodEnd.Format(time.RFC3339)
	}

	return resp, nil
}

// ── Webhook Event Processing ──

// HandleWebhookEvent processes a Stripe event. Returns true if already processed (idempotent).
func (s *Service) HandleWebhookEvent(ctx context.Context, event stripe.Event) (bool, error) {
	// Idempotency check
	exists, err := s.repo.EventExists(ctx, event.ID)
	if err != nil {
		slog.Error("billing: idempotency check failed", "error", err)
		// continue processing — better to double-process than to drop
	}
	if exists {
		slog.Info("billing: event already processed", "event_id", event.ID)
		return true, nil
	}

	var tenantID primitive.ObjectID

	switch event.Type {
	case "checkout.session.completed":
		tenantID, err = s.handleCheckoutCompleted(ctx, event)
	case "customer.subscription.created", "customer.subscription.updated":
		tenantID, err = s.handleSubscriptionUpsert(ctx, event)
	case "customer.subscription.deleted":
		tenantID, err = s.handleSubscriptionDeleted(ctx, event)
	case "invoice.payment_failed":
		tenantID, err = s.handlePaymentFailed(ctx, event)
	case "invoice.payment_succeeded":
		tenantID, err = s.handlePaymentSucceeded(ctx, event)
	default:
		slog.Debug("billing: unhandled event type", "type", event.Type)
		return false, nil
	}

	if err != nil {
		return false, err
	}

	// Record event for idempotency
	s.repo.InsertEvent(ctx, &BillingEvent{
		TenantID:      tenantID,
		StripeEventID: event.ID,
		Type:          string(event.Type),
	})

	return false, nil
}

// ── Event handlers ──

func (s *Service) handleCheckoutCompleted(ctx context.Context, event stripe.Event) (primitive.ObjectID, error) {
	var sess stripe.CheckoutSession
	if err := decodeEventData(event, &sess); err != nil {
		return primitive.NilObjectID, err
	}

	tenantID, err := s.tenantIDFromMetadataOrCustomer(ctx, sess.Metadata, sess.Customer)
	if err != nil {
		return primitive.NilObjectID, err
	}

	slog.Info("billing: checkout completed", "tenant_id", tenantID.Hex(), "subscription", sess.Subscription)

	// Store subscription ID
	if sess.Subscription != nil {
		s.tenantCol.UpdateOne(ctx, bson.M{"_id": tenantID}, bson.M{
			"$set": bson.M{
				"stripe_subscription_id": sess.Subscription.ID,
				"updated_at":             time.Now().UTC(),
			},
		})
	}

	return tenantID, nil
}

func (s *Service) handleSubscriptionUpsert(ctx context.Context, event stripe.Event) (primitive.ObjectID, error) {
	var sub stripe.Subscription
	if err := decodeEventData(event, &sub); err != nil {
		return primitive.NilObjectID, err
	}

	tenantID, err := s.tenantIDFromCustomerID(ctx, sub.Customer.ID)
	if err != nil {
		return primitive.NilObjectID, err
	}

	// Determine plan from price
	plan := s.planFromPriceID(sub)
	billingStatus := mapStripeStatus(sub.Status)

	// Determine tenant status
	tenantStatus := tenant.StatusActive
	if billingStatus == tenant.BillingPastDue || billingStatus == tenant.BillingIncomplete {
		tenantStatus = tenant.StatusSuspended
	}

	// Get plan defaults
	limits, features := tenant.PlanDefaults(plan)

	update := bson.M{
		"plan":                   plan,
		"status":                 tenantStatus,
		"limits":                 limits,
		"features":               features,
		"stripe_subscription_id": sub.ID,
		"stripe_price_id":        currentPriceID(sub),
		"billing_status":         billingStatus,
		"current_period_end":     time.Unix(sub.CurrentPeriodEnd, 0).UTC(),
		"cancel_at_period_end":   sub.CancelAtPeriodEnd,
		"updated_at":             time.Now().UTC(),
	}

	_, err = s.tenantCol.UpdateOne(ctx, bson.M{"_id": tenantID}, bson.M{"$set": update})
	if err != nil {
		return primitive.NilObjectID, err
	}

	// Invalidate middleware cache
	middleware.InvalidateTenantCache(tenantID.Hex())

	slog.Info("billing: subscription synced",
		"tenant_id", tenantID.Hex(), "plan", plan,
		"billing_status", billingStatus, "tenant_status", tenantStatus)

	return tenantID, nil
}

func (s *Service) handleSubscriptionDeleted(ctx context.Context, event stripe.Event) (primitive.ObjectID, error) {
	var sub stripe.Subscription
	if err := decodeEventData(event, &sub); err != nil {
		return primitive.NilObjectID, err
	}

	tenantID, err := s.tenantIDFromCustomerID(ctx, sub.Customer.ID)
	if err != nil {
		return primitive.NilObjectID, err
	}

	// Downgrade to FREE, keep ACTIVE
	limits, features := tenant.PlanDefaults(tenant.PlanFree)

	update := bson.M{
		"plan":                   tenant.PlanFree,
		"status":                 tenant.StatusActive,
		"limits":                 limits,
		"features":               features,
		"billing_status":         tenant.BillingCanceled,
		"stripe_subscription_id": "",
		"stripe_price_id":        "",
		"cancel_at_period_end":   false,
		"updated_at":             time.Now().UTC(),
	}

	_, err = s.tenantCol.UpdateOne(ctx, bson.M{"_id": tenantID}, bson.M{"$set": update})
	if err != nil {
		return primitive.NilObjectID, err
	}

	middleware.InvalidateTenantCache(tenantID.Hex())
	slog.Info("billing: subscription deleted, downgraded to FREE", "tenant_id", tenantID.Hex())

	return tenantID, nil
}

func (s *Service) handlePaymentFailed(ctx context.Context, event stripe.Event) (primitive.ObjectID, error) {
	var inv stripe.Invoice
	if err := decodeEventData(event, &inv); err != nil {
		return primitive.NilObjectID, err
	}

	if inv.Customer == nil {
		return primitive.NilObjectID, errors.New("no customer on invoice")
	}

	tenantID, err := s.tenantIDFromCustomerID(ctx, inv.Customer.ID)
	if err != nil {
		return primitive.NilObjectID, err
	}

	// Suspend immediately (MVP — no grace period)
	update := bson.M{
		"status":         tenant.StatusSuspended,
		"billing_status": tenant.BillingPastDue,
		"updated_at":     time.Now().UTC(),
	}

	_, err = s.tenantCol.UpdateOne(ctx, bson.M{"_id": tenantID}, bson.M{"$set": update})
	if err != nil {
		return primitive.NilObjectID, err
	}

	middleware.InvalidateTenantCache(tenantID.Hex())
	slog.Warn("billing: payment failed, tenant suspended", "tenant_id", tenantID.Hex())

	return tenantID, nil
}

func (s *Service) handlePaymentSucceeded(ctx context.Context, event stripe.Event) (primitive.ObjectID, error) {
	var inv stripe.Invoice
	if err := decodeEventData(event, &inv); err != nil {
		return primitive.NilObjectID, err
	}

	if inv.Customer == nil {
		return primitive.NilObjectID, errors.New("no customer on invoice")
	}

	tenantID, err := s.tenantIDFromCustomerID(ctx, inv.Customer.ID)
	if err != nil {
		return primitive.NilObjectID, err
	}

	// Reactivate if was suspended due to payment
	update := bson.M{
		"status":         tenant.StatusActive,
		"billing_status": tenant.BillingActive,
		"updated_at":     time.Now().UTC(),
	}

	_, err = s.tenantCol.UpdateOne(ctx, bson.M{"_id": tenantID}, bson.M{"$set": update})
	if err != nil {
		return primitive.NilObjectID, err
	}

	middleware.InvalidateTenantCache(tenantID.Hex())
	slog.Info("billing: payment succeeded, tenant reactivated", "tenant_id", tenantID.Hex())

	return tenantID, nil
}

// ── Helpers ──

func (s *Service) priceIDForPlan(plan string) string {
	switch plan {
	case tenant.PlanPro:
		return s.cfg.PricePro
	case tenant.PlanEnterprise:
		return s.cfg.PriceEnterprise
	default:
		return ""
	}
}

func (s *Service) planFromPriceID(sub stripe.Subscription) string {
	pid := currentPriceID(sub)
	switch pid {
	case s.cfg.PricePro:
		return tenant.PlanPro
	case s.cfg.PriceEnterprise:
		return tenant.PlanEnterprise
	default:
		return tenant.PlanFree
	}
}

func currentPriceID(sub stripe.Subscription) string {
	if sub.Items != nil && len(sub.Items.Data) > 0 {
		return sub.Items.Data[0].Price.ID
	}
	return ""
}

func mapStripeStatus(status stripe.SubscriptionStatus) string {
	switch status {
	case stripe.SubscriptionStatusTrialing:
		return tenant.BillingTrialing
	case stripe.SubscriptionStatusActive:
		return tenant.BillingActive
	case stripe.SubscriptionStatusPastDue:
		return tenant.BillingPastDue
	case stripe.SubscriptionStatusCanceled:
		return tenant.BillingCanceled
	case stripe.SubscriptionStatusIncomplete, stripe.SubscriptionStatusIncompleteExpired:
		return tenant.BillingIncomplete
	default:
		return string(status)
	}
}

func (s *Service) tenantIDFromMetadataOrCustomer(ctx context.Context, metadata map[string]string, cust *stripe.Customer) (primitive.ObjectID, error) {
	// Try metadata first
	if tid, ok := metadata["tenant_id"]; ok {
		oid, err := primitive.ObjectIDFromHex(tid)
		if err == nil {
			return oid, nil
		}
	}
	// Fall back to customer
	if cust != nil {
		return s.tenantIDFromCustomerID(ctx, cust.ID)
	}
	return primitive.NilObjectID, errors.New("cannot determine tenant from event")
}

func (s *Service) tenantIDFromCustomerID(ctx context.Context, customerID string) (primitive.ObjectID, error) {
	var t struct {
		ID primitive.ObjectID `bson:"_id"`
	}
	err := s.tenantCol.FindOne(ctx, bson.M{"stripe_customer_id": customerID}).Decode(&t)
	if err != nil {
		return primitive.NilObjectID, errors.New("tenant not found for stripe customer: " + customerID)
	}
	return t.ID, nil
}

// decodeEventData unmarshals the event Data.Raw into the target type.
func decodeEventData(event stripe.Event, target interface{}) error {
	return decodeJSON(event.Data.Raw, target)
}
