package configs

import (
	"os"
	"strconv"
)

type Config struct {
	AppPort      string
	MongoURI     string
	DBName       string
	JWTSecret    string
	JWTExpiryHrs int // kept for backward compat

	// Auth hardening
	AccessTokenTTLMin   int
	RefreshTokenTTLDays int
	CookieDomain        string
	CookieSecure        bool
	CookieSameSite      string

	// Stripe billing
	StripeSecretKey       string
	StripeWebhookSecret   string
	StripePricePro        string
	StripePriceEnterprise string
	BillingSuccessURL     string
	BillingCancelURL      string
	StripeWebhookTest     bool

	// Tenant cache
	TenantCacheTTL int // seconds, 0 = no cache
}

func LoadConfig() *Config {
	expiryHrs, _ := strconv.Atoi(getEnv("JWT_EXPIRY_HOURS", "24"))
	accessTTL, _ := strconv.Atoi(getEnv("ACCESS_TOKEN_TTL_MIN", "15"))
	refreshTTL, _ := strconv.Atoi(getEnv("REFRESH_TOKEN_TTL_DAYS", "30"))
	cacheTTL, _ := strconv.Atoi(getEnv("TENANT_CACHE_TTL_SECONDS", "60"))

	// Order: APP_PORT > PORT > default 3000
	port := os.Getenv("APP_PORT")
	if port == "" {
		port = os.Getenv("PORT")
	}
	if port == "" {
		port = "3000"
	}

	return &Config{
		AppPort:      port,
		MongoURI:     getEnv("MONGO_URI", "mongodb://localhost:27017"),
		DBName:       getEnv("DB_NAME", "warehouse_crm"),
		JWTSecret:    getEnv("JWT_SECRET", "default-secret-change-me"),
		JWTExpiryHrs: expiryHrs,

		AccessTokenTTLMin:   accessTTL,
		RefreshTokenTTLDays: refreshTTL,
		CookieDomain:        getEnv("COOKIE_DOMAIN", ""),
		CookieSecure:        getEnv("COOKIE_SECURE", "true") == "true",
		CookieSameSite:      getEnv("COOKIE_SAMESITE", "Lax"),

		StripeSecretKey:       getEnv("STRIPE_SECRET_KEY", ""),
		StripeWebhookSecret:   getEnv("STRIPE_WEBHOOK_SECRET", ""),
		StripePricePro:        getEnv("STRIPE_PRICE_PRO", ""),
		StripePriceEnterprise: getEnv("STRIPE_PRICE_ENTERPRISE", ""),
		BillingSuccessURL:     getEnv("BILLING_SUCCESS_URL", "http://localhost:3000/billing?success=true"),
		BillingCancelURL:      getEnv("BILLING_CANCEL_URL", "http://localhost:3000/billing?canceled=true"),
		StripeWebhookTest:     getEnv("STRIPE_WEBHOOK_TEST", "") == "true",

		TenantCacheTTL: cacheTTL,
	}
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
