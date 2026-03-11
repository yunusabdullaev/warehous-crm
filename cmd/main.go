package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/joho/godotenv"

	"warehouse-crm/configs"
	"warehouse-crm/core/adjustment"
	"warehouse-crm/core/auth"
	"warehouse-crm/core/billing"
	"warehouse-crm/core/csvio"
	"warehouse-crm/core/dashboard"
	"warehouse-crm/core/history"
	"warehouse-crm/core/inbound"
	"warehouse-crm/core/location"
	"warehouse-crm/core/lot"
	"warehouse-crm/core/middleware"
	"warehouse-crm/core/notify"
	"warehouse-crm/core/order"
	"warehouse-crm/core/orderdoc"
	"warehouse-crm/core/outbound"
	"warehouse-crm/core/picking"
	"warehouse-crm/core/product"
	"warehouse-crm/core/qrlabel"
	"warehouse-crm/core/reports"
	"warehouse-crm/core/reservation"
	"warehouse-crm/core/returndoc"
	"warehouse-crm/core/returns"
	"warehouse-crm/core/session"
	"warehouse-crm/core/stock"
	"warehouse-crm/core/tenant"
	"warehouse-crm/core/warehouse"
	"warehouse-crm/pkg/database"
	"warehouse-crm/pkg/logger"
	"warehouse-crm/pkg/metrics"
)

func main() {
	// Load .env (ignore error if running in Docker with env vars)
	_ = godotenv.Load()

	// Initialize structured JSON logger
	logger.Init()

	// Load config
	cfg := configs.LoadConfig()

	slog.Info("starting warehouse CRM",
		"port", cfg.AppPort,
		"db", cfg.DBName,
	)

	// Connect to MongoDB
	mongoClient, err := database.Connect(cfg.MongoURI)
	if err != nil {
		slog.Error("failed to connect to MongoDB", "error", err)
		os.Exit(1)
	}
	defer database.Disconnect(mongoClient)

	// Get database reference
	db := mongoClient.Database(cfg.DBName)

	// Ensure indexes (idempotent)
	database.EnsureIndexes(db)

	// Initialize Prometheus metrics
	metrics.Init()

	// Configure tenant cache TTL (0 = disabled, useful for testing)
	middleware.SetCacheTTL(cfg.TenantCacheTTL)

	// ──────────────────────────────────────────────
	// Repositories
	// ──────────────────────────────────────────────
	authRepo := auth.NewRepository(database.GetCollection(mongoClient, cfg.DBName, "users"))
	productRepo := product.NewRepository(database.GetCollection(mongoClient, cfg.DBName, "products"))
	locationRepo := location.NewRepository(database.GetCollection(mongoClient, cfg.DBName, "locations"))
	stockRepo := stock.NewRepository(database.GetCollection(mongoClient, cfg.DBName, "stocks"))
	historyRepo := history.NewRepository(database.GetCollection(mongoClient, cfg.DBName, "history"))
	inboundRepo := inbound.NewRepository(database.GetCollection(mongoClient, cfg.DBName, "inbounds"))
	outboundRepo := outbound.NewRepository(database.GetCollection(mongoClient, cfg.DBName, "outbounds"))
	adjustmentRepo := adjustment.NewRepository(database.GetCollection(mongoClient, cfg.DBName, "adjustments"))
	orderRepo := order.NewRepository(
		database.GetCollection(mongoClient, cfg.DBName, "orders"),
		database.GetCollection(mongoClient, cfg.DBName, "counters"),
	)
	reservationRepo := reservation.NewRepository(database.GetCollection(mongoClient, cfg.DBName, "reservations"))
	pickingRepo := picking.NewRepository(
		database.GetCollection(mongoClient, cfg.DBName, "pick_tasks"),
		database.GetCollection(mongoClient, cfg.DBName, "pick_events"),
	)
	lotRepo := lot.NewRepository(database.GetCollection(mongoClient, cfg.DBName, "lots"))
	warehouseRepo := warehouse.NewRepository(database.GetCollection(mongoClient, cfg.DBName, "warehouses"))
	tenantRepo := tenant.NewRepository(database.GetCollection(mongoClient, cfg.DBName, "tenants"))

	// ──────────────────────────────────────────────
	// Session module
	// ──────────────────────────────────────────────
	sessionRepo := session.NewRepository(
		database.GetCollection(mongoClient, cfg.DBName, "sessions"),
		database.GetCollection(mongoClient, cfg.DBName, "login_attempts"),
		database.GetCollection(mongoClient, cfg.DBName, "reset_tokens"),
	)
	sessionSvc := session.NewService(sessionRepo, cfg.JWTSecret, cfg.AccessTokenTTLMin, cfg.RefreshTokenTTLDays)

	// ──────────────────────────────────────────────
	// Services
	// ──────────────────────────────────────────────
	authSvc := auth.NewService(authRepo, sessionSvc,
		database.GetCollection(mongoClient, cfg.DBName, "tenants"),
		cfg.JWTSecret, cfg.JWTExpiryHrs, cfg.AccessTokenTTLMin)
	productSvc := product.NewService(productRepo)
	locationSvc := location.NewService(locationRepo)
	stockSvc := stock.NewService(stockRepo)
	historySvc := history.NewService(historyRepo)
	lotSvc := lot.NewService(lotRepo)
	warehouseSvc := warehouse.NewService(warehouseRepo, db)
	tenantSvc := tenant.NewService(tenantRepo, db)

	// Ensure default tenant and warehouse exist
	ctxStartup, cancelStartup := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelStartup()
	if _, err := tenantSvc.EnsureDefault(ctxStartup); err != nil {
		slog.Error("failed to ensure default tenant", "error", err)
		os.Exit(1)
	}
	if _, err := warehouseSvc.EnsureDefault(ctxStartup); err != nil {
		slog.Error("failed to ensure default warehouse", "error", err)
		os.Exit(1)
	}

	inboundSvc := inbound.NewService(inboundRepo, stockSvc, historySvc, lotSvc)
	outboundSvc := outbound.NewService(outboundRepo, stockSvc, historySvc)
	adjustmentSvc := adjustment.NewService(adjustmentRepo, stockSvc, historySvc)
	reservationSvc := reservation.NewService(reservationRepo, stockSvc, historySvc)
	pickingSvc := picking.NewService(pickingRepo, stockSvc, historySvc, lotSvc)

	// Notify
	notifyRepo := notify.NewRepository(
		database.GetCollection(mongoClient, cfg.DBName, "settings"),
		database.GetCollection(mongoClient, cfg.DBName, "alerts"),
	)
	telegramBot := notify.NewTelegramBot()
	notifySvc := notify.NewService(notifyRepo, telegramBot, productSvc, stockSvc, lotSvc)
	pickingSvc.SetNotifySvc(notifySvc)

	orderSvc := order.NewService(orderRepo, reservationSvc, stockSvc, outboundSvc, historySvc, pickingSvc, notifySvc)
	dashboardSvc := dashboard.NewService(db)
	reportsSvc := reports.NewService(db)

	// Returns
	returnsRepo := returns.NewRepository(
		database.GetCollection(mongoClient, cfg.DBName, "returns"),
		database.GetCollection(mongoClient, cfg.DBName, "return_items"),
		database.GetCollection(mongoClient, cfg.DBName, "qc_holds"),
		database.GetCollection(mongoClient, cfg.DBName, "counters"),
	)
	returnsSvc := returns.NewService(returnsRepo, orderSvc, stockSvc, historySvc, notifySvc, lotSvc)

	// ──────────────────────────────────────────────
	// Handlers
	// ──────────────────────────────────────────────
	authHandler := auth.NewHandler(authSvc, sessionSvc, cfg, database.GetCollection(mongoClient, cfg.DBName, "tenants"))
	productHandler := product.NewHandler(productSvc)
	locationHandler := location.NewHandler(locationSvc)
	stockHandler := stock.NewHandler(stockSvc, reservationSvc)
	historyHandler := history.NewHandler(historySvc)
	inboundHandler := inbound.NewHandler(inboundSvc)
	outboundHandler := outbound.NewHandler(outboundSvc)
	adjustmentHandler := adjustment.NewHandler(adjustmentSvc)
	orderHandler := order.NewHandler(orderSvc)
	reservationHandler := reservation.NewHandler(reservationSvc)
	pickingHandler := picking.NewHandler(pickingSvc)
	dashboardHandler := dashboard.NewHandler(dashboardSvc)
	reportsHandler := reports.NewHandler(reportsSvc)
	lotHandler := lot.NewHandler(lotSvc)
	csvioSvc := csvio.NewService(
		database.GetCollection(mongoClient, cfg.DBName, "products"),
		database.GetCollection(mongoClient, cfg.DBName, "locations"),
	)
	csvioHandler := csvio.NewHandler(csvioSvc)
	qrlabelHandler := qrlabel.NewHandler(locationSvc)
	orderDocHandler := orderdoc.NewHandler(orderSvc, pickingSvc, productSvc, locationSvc, outboundSvc)
	notifyHandler := notify.NewHandler(notifySvc, notifyRepo)
	returnsHandler := returns.NewHandler(returnsSvc)
	returndocHandler := returndoc.NewHandler(returnsSvc, productSvc)
	warehouseHandler := warehouse.NewHandler(warehouseSvc)
	tenantHandler := tenant.NewHandler(tenantSvc)

	// Billing
	billingRepo := billing.NewRepository(database.GetCollection(mongoClient, cfg.DBName, "billing_events"))
	billingSvc := billing.NewService(billingRepo,
		database.GetCollection(mongoClient, cfg.DBName, "tenants"),
		tenantSvc,
		billing.StripeConfig{
			SecretKey:       cfg.StripeSecretKey,
			WebhookSecret:   cfg.StripeWebhookSecret,
			PricePro:        cfg.StripePricePro,
			PriceEnterprise: cfg.StripePriceEnterprise,
			SuccessURL:      cfg.BillingSuccessURL,
			CancelURL:       cfg.BillingCancelURL,
			WebhookTestMode: cfg.StripeWebhookTest,
		},
	)
	billingHandler := billing.NewHandler(billingSvc)

	// ──────────────────────────────────────────────
	// Fiber App
	// ──────────────────────────────────────────────
	app := fiber.New(fiber.Config{
		AppName:   "Warehouse CRM v1.0",
		BodyLimit: 50 * 1024 * 1024, // 50 MB for Excel uploads
	})

	// Global middleware
	app.Use(recover.New())
	corsOrigins := os.Getenv("CORS_ORIGINS")
	if corsOrigins == "" {
		corsOrigins = "http://localhost:3000,http://localhost:3001"
	}
	app.Use(cors.New(cors.Config{
		AllowOrigins:     corsOrigins,
		AllowHeaders:     "Origin,Content-Type,Authorization,X-Warehouse-Id",
		AllowCredentials: true,
	}))
	app.Use(middleware.RequestLogger())
	app.Use(metrics.PrometheusMiddleware())

	// Health check
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok", "service": "warehouse-crm"})
	})

	// Prometheus metrics endpoint
	app.Get("/metrics", metrics.MetricsHandler())

	// ──────────────────────────────────────────────
	// API Routes
	// ──────────────────────────────────────────────
	api := app.Group("/api/v1")

	// Public routes (no auth required)
	auth.RegisterPublicRoutes(api, authHandler)

	// Stripe webhook (public — signature verified internally)
	api.Post("/webhooks/stripe", billingHandler.Webhook)

	tenantCol := database.GetCollection(mongoClient, cfg.DBName, "tenants")

	// Protected base group (JWT required for all below)
	protected := api.Group("", middleware.AuthMiddleware(cfg.JWTSecret),
		middleware.RequireTenantActive(tenantCol),
		middleware.WarehouseContext(
			database.GetCollection(mongoClient, cfg.DBName, "users"),
			database.GetCollection(mongoClient, cfg.DBName, "warehouses"),
		))

	// ── Role shorthands ──
	allRoles := middleware.RequireRoles("admin", "operator", "loader")
	adminOperator := middleware.RequireRoles("admin", "operator")
	adminOnly := middleware.RequireRoles("admin")
	superOnly := middleware.RequireRoles("superadmin")

	// ── Tenants (superadmin only) ──
	tenantsR := protected.Group("/tenants", superOnly)
	tenantsR.Get("/", tenantHandler.List)
	tenantsR.Get("/:id", tenantHandler.GetByID)
	tenantsR.Post("/", tenantHandler.Create)
	tenantsR.Put("/:id", tenantHandler.Update)
	tenantsR.Delete("/:id", tenantHandler.Delete)
	tenantsR.Get("/:id/usage", tenantHandler.GetUsage)
	tenantsR.Put("/:id/plan", tenantHandler.UpdatePlan)

	// ── Billing (admin + superadmin) ──
	billingR := protected.Group("/billing", middleware.RequireRoles("admin"))
	billingR.Post("/checkout-session", billingHandler.CreateCheckoutSession)
	billingR.Post("/portal-session", billingHandler.CreatePortalSession)
	billingR.Get("/status", billingHandler.GetBillingStatus)

	// ── Warehouses (admin only) ──
	warehousesR := protected.Group("/warehouses", adminOnly)
	warehousesR.Get("/", warehouseHandler.List)
	warehousesR.Get("/:id", warehouseHandler.GetByID)
	warehousesR.Post("/", middleware.EnforceLimit(tenantCol, db, "maxWarehouses"), warehouseHandler.Create)
	warehousesR.Put("/:id", warehouseHandler.Update)
	warehousesR.Delete("/:id", warehouseHandler.Delete)

	// ── Products ──
	products := protected.Group("/products")
	products.Get("/", allRoles, productHandler.List)
	products.Get("/:id", allRoles, productHandler.GetByID)
	products.Post("/", adminOnly, middleware.EnforceLimit(tenantCol, db, "maxProducts"), productHandler.Create)
	products.Put("/:id", adminOnly, productHandler.Update)
	products.Delete("/:id", adminOnly, productHandler.Delete)

	// ── Locations ──
	locations := protected.Group("/locations")
	locations.Get("/", allRoles, locationHandler.List)
	locations.Get("/:id", allRoles, locationHandler.GetByID)
	locations.Post("/", adminOnly, locationHandler.Create)
	locations.Put("/:id", adminOnly, locationHandler.Update)
	locations.Delete("/:id", adminOnly, locationHandler.Delete)
	locations.Get("/:id/qr", adminOnly, middleware.RequireFeature(tenantCol, "enableQrLabels"), qrlabelHandler.QRCode)
	locations.Get("/:id/label", adminOnly, middleware.RequireFeature(tenantCol, "enableQrLabels"), qrlabelHandler.Label)

	// ── Inbound (all roles can create + view; reversal RBAC enforced in service) ──
	inboundR := protected.Group("/inbound", allRoles)
	inboundR.Post("/", inboundHandler.Create)
	inboundR.Get("/", inboundHandler.List)
	inboundR.Get("/:id", inboundHandler.GetByID)
	inboundR.Post("/:id/reverse", adminOperator, inboundHandler.Reverse)

	// ── Outbound (admin + operator only; reversal RBAC enforced in service) ──
	outboundR := protected.Group("/outbound", adminOperator)
	outboundR.Post("/", outboundHandler.Create)
	outboundR.Get("/", outboundHandler.List)
	outboundR.Get("/:id", outboundHandler.GetByID)
	outboundR.Post("/:id/reverse", outboundHandler.Reverse)

	// ── Adjustments (admin + operator only) ──
	adjustmentR := protected.Group("/adjustments", adminOperator)
	adjustmentR.Post("/", adjustmentHandler.Create)
	adjustmentR.Get("/", adjustmentHandler.List)

	// ── Orders (admin + operator create/modify; all roles can view) ──
	ordersR := protected.Group("/orders")
	ordersR.Get("/", allRoles, orderHandler.List)
	ordersR.Get("/:id", allRoles, orderHandler.GetByID)
	ordersR.Post("/", adminOperator, middleware.EnforceLimit(tenantCol, db, "maxDailyOrders"), orderHandler.Create)
	ordersR.Put("/:id", adminOperator, orderHandler.Update)
	ordersR.Post("/:id/confirm", adminOperator, orderHandler.Confirm)
	ordersR.Post("/:id/cancel", adminOperator, orderHandler.Cancel)
	ordersR.Post("/:id/start-pick", adminOperator, orderHandler.StartPick)
	ordersR.Post("/:id/ship", adminOperator, orderHandler.Ship)
	ordersR.Get("/:id/pick-tasks", allRoles, pickingHandler.GetTasksByOrder)
	ordersR.Get("/:id/picklist.pdf", allRoles, orderDocHandler.PickListPDF)
	ordersR.Get("/:id/deliverynote.pdf", adminOperator, orderDocHandler.DeliveryNotePDF)
	ordersR.Get("/:id/label", allRoles, orderDocHandler.LabelPDF)

	// ── Pick Tasks (loader can view & scan, admin/operator assign) ──
	pickR := protected.Group("/pick-tasks")
	pickR.Get("/my", allRoles, pickingHandler.MyTasks)
	pickR.Get("/:id", allRoles, pickingHandler.GetTask)
	pickR.Post("/:id/assign", adminOperator, pickingHandler.Assign)
	pickR.Post("/:id/scan", allRoles, pickingHandler.Scan)

	// ── Reservations (admin + operator) ──
	reservationsR := protected.Group("/reservations", adminOperator)
	reservationsR.Get("/", reservationHandler.List)
	reservationsR.Post("/release", reservationHandler.Release)

	// ── Stock (all roles can view) ──
	stockR := protected.Group("/stock", allRoles)
	stockR.Get("/", stockHandler.ListAll)
	stockR.Get("/product/:id", stockHandler.GetByProduct)
	stockR.Get("/location/:id", stockHandler.GetByLocation)

	// ── History (admin + operator) ──
	historyR := protected.Group("/history", adminOperator)
	historyR.Get("/", historyHandler.List)

	// ── Auth Protected Routes ──
	auth.RegisterProtectedRoutes(protected, authHandler)

	// ── Users (admin only) ──
	usersR := protected.Group("/users", adminOnly)
	usersR.Get("/", authHandler.List)
	usersR.Get("/:id", authHandler.GetByID)
	usersR.Put("/:id", authHandler.Update)
	usersR.Delete("/:id", authHandler.Delete)
	usersR.Post("/:id/reset-token", authHandler.GenerateResetToken)
	usersR.Post("/:id/revoke-sessions", authHandler.RevokeUserSessions)

	// ── Dashboard (admin + operator) ──
	dashboardR := protected.Group("/dashboard", adminOperator)
	dashboardR.Get("/summary", dashboardHandler.Summary)

	// ── Reports (admin only, feature-gated) ──
	reportsR := protected.Group("/reports", adminOnly, middleware.RequireFeature(tenantCol, "enableReports"))
	reportsR.Get("/movements", reportsHandler.Movements)
	reportsR.Get("/stock", reportsHandler.StockReport)
	reportsR.Get("/orders", reportsHandler.OrderReport)
	reportsR.Get("/picking", reportsHandler.PickingReport)
	reportsR.Get("/returns", reportsHandler.ReturnsReport)
	reportsR.Get("/expiry", reportsHandler.ExpiryReport)

	// ── Excel Import / Export (admin only, export feature-gated) ──
	importR := protected.Group("/import", adminOnly)
	importR.Post("/products", middleware.EnforceLimit(tenantCol, db, "maxProducts"), csvioHandler.ImportProducts)
	importR.Post("/locations", csvioHandler.ImportLocations)

	exportR := protected.Group("/export", adminOnly, middleware.RequireFeature(tenantCol, "enableApiExport"))
	exportR.Get("/products", csvioHandler.ExportProducts)
	exportR.Get("/locations", csvioHandler.ExportLocations)

	// ── Settings (admin only) ──
	settingsR := protected.Group("/settings", adminOnly)
	settingsR.Get("/notifications", notifyHandler.Get)
	settingsR.Put("/notifications", notifyHandler.Update)
	settingsR.Post("/notifications/test", notifyHandler.Test)

	// ── Returns (feature-gated; admin/operator create/manage; all roles can view) ──
	returnsR := protected.Group("/returns", middleware.RequireFeature(tenantCol, "enableReturns"))
	returnsR.Get("/", allRoles, returnsHandler.List)
	returnsR.Get("/:id", allRoles, returnsHandler.GetByID)
	returnsR.Post("/", adminOperator, returnsHandler.Create)
	returnsR.Post("/:id/items", allRoles, returnsHandler.AddItem)
	returnsR.Post("/:id/receive", adminOperator, returnsHandler.Receive)
	returnsR.Post("/:id/cancel", adminOperator, returnsHandler.Cancel)
	returnsR.Get("/:id/note.pdf", allRoles, returndocHandler.NotePDF)

	// ── Lots (feature-gated; admin + operator manage; all can view) ──
	lotsR := protected.Group("/lots", middleware.RequireFeature(tenantCol, "enableLots"))
	lotsR.Get("/", allRoles, lotHandler.List)
	lotsR.Get("/:id", allRoles, lotHandler.GetByID)
	lotsR.Post("/", adminOperator, lotHandler.Create)

	// ── Alerts (admin only, feature-gated) ──
	alertsR := protected.Group("/alerts", adminOnly, middleware.RequireFeature(tenantCol, "enableExpiryDigest"))
	alertsR.Post("/expiry-digest/run", notifyHandler.RunDigest)

	// ── CLI Subcommand: expiry-digest ──
	if len(os.Args) > 1 && os.Args[1] == "expiry-digest" {
		days := 14
		force := false
		for _, arg := range os.Args[2:] {
			if len(arg) > 7 && arg[:7] == "--days=" {
				if d, err := strconv.Atoi(arg[7:]); err == nil && d > 0 {
					days = d
				}
			}
			if arg == "--force" {
				force = true
			}
		}
		_ = days // days override is in settings
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		result, err := notifySvc.RunExpiryDigest(ctx, force)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
			os.Exit(1)
		}
		if result.Skipped {
			fmt.Printf("SKIPPED: %s\n", result.Reason)
		} else {
			fmt.Printf("SENT: total=%d urgent=%d warning=%d notice=%d\n",
				result.Total, result.Urgent, result.Warning, result.Notice)
		}
		os.Exit(0)
	}

	// ──────────────────────────────────────────────
	// Graceful Shutdown
	// ──────────────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := app.Listen(":" + cfg.AppPort); err != nil {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	slog.Info("server started", "port", cfg.AppPort)

	<-quit
	slog.Info("shutting down server...")
	if err := app.Shutdown(); err != nil {
		slog.Error("server shutdown error", "error", err)
	}
	slog.Info("server stopped")
}
