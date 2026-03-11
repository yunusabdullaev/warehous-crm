package handler

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/adaptor/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	fiberRecover "github.com/gofiber/fiber/v2/middleware/recover"

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
)

var (
	app      *fiber.App
	initOnce sync.Once
	initErr  error
)

func Handler(w http.ResponseWriter, r *http.Request) {
	initOnce.Do(func() {
		cfg := configs.LoadConfig()
		logger.Init()

		slog.Info("initializing vercel function")

		// Mask MONGO_URI for logging
		maskedURI := cfg.MongoURI
		if strings.Contains(maskedURI, "@") && strings.Contains(maskedURI, "://") {
			parts := strings.Split(maskedURI, "@")
			prefix := strings.Split(parts[0], "://")
			maskedURI = prefix[0] + "://***:***@" + parts[1]
		}
		slog.Info("database config", "uri", maskedURI, "db", cfg.DBName)

		if strings.Contains(cfg.MongoURI, "localhost") {
			slog.Warn("MONGO_URI is set to localhost - this will fail on Vercel unless configured")
		}

		client, err := database.Connect(cfg.MongoURI)
		if err != nil {
			slog.Error("failed to connect to mongo", "error", err)
			initErr = err
			return
		}

		db := client.Database(cfg.DBName)

		// Run indexes in background to avoid blocking Vercel cold start response
		go func() {
			defer func() {
				if r := recover(); r != nil {
					slog.Error("panic in background index creation", "error", r)
				}
			}()
			slog.Info("starting async index creation")
			database.EnsureIndexes(db)
			slog.Info("async index creation completed")
		}()

		authRepo := auth.NewRepository(database.GetCollection(client, cfg.DBName, "users"))
		productRepo := product.NewRepository(database.GetCollection(client, cfg.DBName, "products"))
		locationRepo := location.NewRepository(database.GetCollection(client, cfg.DBName, "locations"))
		stockRepo := stock.NewRepository(database.GetCollection(client, cfg.DBName, "stocks"))
		historyRepo := history.NewRepository(database.GetCollection(client, cfg.DBName, "history"))
		inboundRepo := inbound.NewRepository(database.GetCollection(client, cfg.DBName, "inbounds"))
		outboundRepo := outbound.NewRepository(database.GetCollection(client, cfg.DBName, "outbounds"))
		adjustmentRepo := adjustment.NewRepository(database.GetCollection(client, cfg.DBName, "adjustments"))
		orderRepo := order.NewRepository(
			database.GetCollection(client, cfg.DBName, "orders"),
			database.GetCollection(client, cfg.DBName, "counters"),
		)
		reservationRepo := reservation.NewRepository(database.GetCollection(client, cfg.DBName, "reservations"))
		pickingRepo := picking.NewRepository(
			database.GetCollection(client, cfg.DBName, "pick_tasks"),
			database.GetCollection(client, cfg.DBName, "pick_events"),
		)
		lotRepo := lot.NewRepository(database.GetCollection(client, cfg.DBName, "lots"))
		warehouseRepo := warehouse.NewRepository(database.GetCollection(client, cfg.DBName, "warehouses"))
		tenantRepo := tenant.NewRepository(database.GetCollection(client, cfg.DBName, "tenants"))

		sessionRepo := session.NewRepository(
			database.GetCollection(client, cfg.DBName, "sessions"),
			database.GetCollection(client, cfg.DBName, "login_attempts"),
			database.GetCollection(client, cfg.DBName, "reset_tokens"),
		)
		sessionSvc := session.NewService(sessionRepo, cfg.JWTSecret, cfg.AccessTokenTTLMin, cfg.RefreshTokenTTLDays)

		authSvc := auth.NewService(authRepo, sessionSvc,
			database.GetCollection(client, cfg.DBName, "tenants"),
			cfg.JWTSecret, cfg.JWTExpiryHrs, cfg.AccessTokenTTLMin)

		productSvc := product.NewService(productRepo)
		locationSvc := location.NewService(locationRepo)
		stockSvc := stock.NewService(stockRepo)
		historySvc := history.NewService(historyRepo)
		lotSvc := lot.NewService(lotRepo)
		warehouseSvc := warehouse.NewService(warehouseRepo, db)
		tenantSvc := tenant.NewService(tenantRepo, db)

		inboundSvc := inbound.NewService(inboundRepo, stockSvc, historySvc, lotSvc)
		outboundSvc := outbound.NewService(outboundRepo, stockSvc, historySvc)
		adjustmentSvc := adjustment.NewService(adjustmentRepo, stockSvc, historySvc)
		reservationSvc := reservation.NewService(reservationRepo, stockSvc, historySvc)
		pickingSvc := picking.NewService(pickingRepo, stockSvc, historySvc, lotSvc)

		notifyRepo := notify.NewRepository(
			database.GetCollection(client, cfg.DBName, "settings"),
			database.GetCollection(client, cfg.DBName, "alerts"),
		)
		telegramBot := notify.NewTelegramBot()
		notifySvc := notify.NewService(notifyRepo, telegramBot, productSvc, stockSvc, lotSvc)
		pickingSvc.SetNotifySvc(notifySvc)

		orderSvc := order.NewService(orderRepo, reservationSvc, stockSvc, outboundSvc, historySvc, pickingSvc, notifySvc)
		dashboardSvc := dashboard.NewService(db)
		reportsSvc := reports.NewService(db)

		returnsRepo := returns.NewRepository(
			database.GetCollection(client, cfg.DBName, "returns"),
			database.GetCollection(client, cfg.DBName, "return_items"),
			database.GetCollection(client, cfg.DBName, "qc_holds"),
			database.GetCollection(client, cfg.DBName, "counters"),
		)
		returnsSvc := returns.NewService(returnsRepo, orderSvc, stockSvc, historySvc, notifySvc, lotSvc)

		authHandler := auth.NewHandler(authSvc, sessionSvc, cfg, database.GetCollection(client, cfg.DBName, "tenants"))
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
			database.GetCollection(client, cfg.DBName, "products"),
			database.GetCollection(client, cfg.DBName, "locations"),
		)
		csvioHandler := csvio.NewHandler(csvioSvc)
		qrlabelHandler := qrlabel.NewHandler(locationSvc)
		orderDocHandler := orderdoc.NewHandler(orderSvc, pickingSvc, productSvc, locationSvc, outboundSvc)
		notifyHandler := notify.NewHandler(notifySvc, notifyRepo)
		returnsHandler := returns.NewHandler(returnsSvc)
		returndocHandler := returndoc.NewHandler(returnsSvc, productSvc)
		warehouseHandler := warehouse.NewHandler(warehouseSvc)
		tenantHandler := tenant.NewHandler(tenantSvc)

		billingRepo := billing.NewRepository(database.GetCollection(client, cfg.DBName, "billing_events"))
		billingSvc := billing.NewService(billingRepo,
			database.GetCollection(client, cfg.DBName, "tenants"),
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

		// Create Fiber app
		app = fiber.New()
		app.Use(fiberRecover.New())

		corsOrigins := os.Getenv("CORS_ORIGINS")
		if corsOrigins == "" {
			corsOrigins = "*"
		}
		app.Use(cors.New(cors.Config{
			AllowOrigins:     corsOrigins,
			AllowHeaders:     "Origin,Content-Type,Authorization,X-Warehouse-Id",
			AllowCredentials: true,
		}))

		api := app.Group("/api/v1")

		// Ensure default data exists
		ctxStartup, cancelStartup := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancelStartup()
		_, _ = tenantSvc.EnsureDefault(ctxStartup)
		_, _ = warehouseSvc.EnsureDefault(ctxStartup)

		// Public
		auth.RegisterPublicRoutes(api, authHandler)
		api.Post("/webhooks/stripe", billingHandler.Webhook)

		tenantCol := database.GetCollection(client, cfg.DBName, "tenants")
		protected := api.Group("", middleware.AuthMiddleware(cfg.JWTSecret),
			middleware.RequireTenantActive(tenantCol),
			middleware.WarehouseContext(
				database.GetCollection(client, cfg.DBName, "users"),
				database.GetCollection(client, cfg.DBName, "warehouses"),
			))

		allRoles := middleware.RequireRoles("admin", "operator", "loader")
		adminOperator := middleware.RequireRoles("admin", "operator")
		adminOnly := middleware.RequireRoles("admin")
		superOnly := middleware.RequireRoles("superadmin")

		// FULL ROUTES REGISTRATION

		// Tenants
		tenantsR := protected.Group("/tenants", superOnly)
		tenantsR.Get("/", tenantHandler.List)
		tenantsR.Get("/:id", tenantHandler.GetByID)
		tenantsR.Post("/", tenantHandler.Create)
		tenantsR.Put("/:id", tenantHandler.Update)
		tenantsR.Delete("/:id", tenantHandler.Delete)
		tenantsR.Get("/:id/usage", tenantHandler.GetUsage)
		tenantsR.Put("/:id/plan", tenantHandler.UpdatePlan)

		// Billing
		billingR := protected.Group("/billing", adminOnly)
		billingR.Post("/checkout-session", billingHandler.CreateCheckoutSession)
		billingR.Post("/portal-session", billingHandler.CreatePortalSession)
		billingR.Get("/status", billingHandler.GetBillingStatus)

		// Warehouses
		whR := protected.Group("/warehouses", adminOnly)
		whR.Get("/", warehouseHandler.List)
		whR.Get("/:id", warehouseHandler.GetByID)
		whR.Post("/", warehouseHandler.Create)
		whR.Put("/:id", warehouseHandler.Update)
		whR.Delete("/:id", warehouseHandler.Delete)

		// Products
		prR := protected.Group("/products")
		prR.Get("/", allRoles, productHandler.List)
		prR.Get("/:id", allRoles, productHandler.GetByID)
		prR.Post("/", adminOnly, productHandler.Create)
		prR.Put("/:id", adminOnly, productHandler.Update)
		prR.Delete("/:id", adminOnly, productHandler.Delete)

		// Locations
		locR := protected.Group("/locations")
		locR.Get("/", allRoles, locationHandler.List)
		locR.Get("/:id", allRoles, locationHandler.GetByID)
		locR.Post("/", adminOnly, locationHandler.Create)
		locR.Put("/:id", adminOnly, locationHandler.Update)
		locR.Delete("/:id", adminOnly, locationHandler.Delete)
		locR.Get("/:id/qr", adminOnly, qrlabelHandler.QRCode)
		locR.Get("/:id/label", adminOnly, qrlabelHandler.Label)

		// Inbound
		inR := protected.Group("/inbound", allRoles)
		inR.Post("/", inboundHandler.Create)
		inR.Get("/", inboundHandler.List)
		inR.Get("/:id", inboundHandler.GetByID)
		inR.Post("/:id/reverse", adminOperator, inboundHandler.Reverse)

		// Outbound
		outR := protected.Group("/outbound", adminOperator)
		outR.Post("/", outboundHandler.Create)
		outR.Get("/", outboundHandler.List)
		outR.Get("/:id", outboundHandler.GetByID)
		outR.Post("/:id/reverse", outboundHandler.Reverse)

		// Adjustments
		adjR := protected.Group("/adjustments", adminOperator)
		adjR.Post("/", adjustmentHandler.Create)
		adjR.Get("/", adjustmentHandler.List)

		// Orders
		orR := protected.Group("/orders")
		orR.Get("/", allRoles, orderHandler.List)
		orR.Get("/:id", allRoles, orderHandler.GetByID)
		orR.Post("/", adminOperator, orderHandler.Create)
		orR.Put("/:id", adminOperator, orderHandler.Update)
		orR.Post("/:id/confirm", adminOperator, orderHandler.Confirm)
		orR.Post("/:id/cancel", adminOperator, orderHandler.Cancel)
		orR.Post("/:id/start-pick", adminOperator, orderHandler.StartPick)
		orR.Post("/:id/ship", adminOperator, orderHandler.Ship)
		orR.Get("/:id/pick-tasks", allRoles, pickingHandler.GetTasksByOrder)
		orR.Get("/:id/picklist.pdf", allRoles, orderDocHandler.PickListPDF)
		orR.Get("/:id/deliverynote.pdf", adminOperator, orderDocHandler.DeliveryNotePDF)
		orR.Get("/:id/label", allRoles, orderDocHandler.LabelPDF)

		// Pick Tasks
		pckR := protected.Group("/pick-tasks")
		pckR.Get("/my", allRoles, pickingHandler.MyTasks)
		pckR.Get("/:id", allRoles, pickingHandler.GetTask)
		pckR.Post("/:id/assign", adminOperator, pickingHandler.Assign)
		pckR.Post("/:id/scan", allRoles, pickingHandler.Scan)

		// Reservations
		resR := protected.Group("/reservations", adminOperator)
		resR.Get("/", reservationHandler.List)
		resR.Post("/release", reservationHandler.Release)

		// Stock
		stkR := protected.Group("/stock", allRoles)
		stkR.Get("/", stockHandler.ListAll)
		stkR.Get("/product/:id", stockHandler.GetByProduct)
		stkR.Get("/location/:id", stockHandler.GetByLocation)

		// History
		hisR := protected.Group("/history", adminOperator)
		hisR.Get("/", historyHandler.List)

		// Users
		usrR := protected.Group("/users", adminOnly)
		usrR.Get("/", authHandler.List)
		usrR.Get("/:id", authHandler.GetByID)
		usrR.Put("/:id", authHandler.Update)
		usrR.Delete("/:id", authHandler.Delete)
		usrR.Post("/:id/reset-token", authHandler.GenerateResetToken)
		usrR.Post("/:id/revoke-sessions", authHandler.RevokeUserSessions)

		// Dashboard
		dshR := protected.Group("/dashboard", adminOperator)
		dshR.Get("/summary", dashboardHandler.Summary)

		// Reports
		repR := protected.Group("/reports", adminOnly)
		repR.Get("/movements", reportsHandler.Movements)
		repR.Get("/stock", reportsHandler.StockReport)
		repR.Get("/orders", reportsHandler.OrderReport)
		repR.Get("/picking", reportsHandler.PickingReport)
		repR.Get("/returns", reportsHandler.ReturnsReport)
		repR.Get("/expiry", reportsHandler.ExpiryReport)

		// Import/Export
		impR := protected.Group("/import", adminOnly)
		impR.Post("/products", csvioHandler.ImportProducts)
		impR.Post("/locations", csvioHandler.ImportLocations)
		expR := protected.Group("/export", adminOnly)
		expR.Get("/products", csvioHandler.ExportProducts)
		expR.Get("/locations", csvioHandler.ExportLocations)

		// Settings
		setR := protected.Group("/settings", adminOnly)
		setR.Get("/notifications", notifyHandler.Get)
		setR.Put("/notifications", notifyHandler.Update)
		setR.Post("/notifications/test", notifyHandler.Test)

		// Returns
		retR := protected.Group("/returns")
		retR.Get("/", allRoles, returnsHandler.List)
		retR.Get("/:id", allRoles, returnsHandler.GetByID)
		retR.Post("/", adminOperator, returnsHandler.Create)
		retR.Post("/:id/receive", adminOperator, returnsHandler.Receive)
		retR.Post("/:id/cancel", adminOperator, returnsHandler.Cancel)
		retR.Get("/:id/note.pdf", allRoles, returndocHandler.NotePDF)

		// Lots
		lotR := protected.Group("/lots")
		lotR.Get("/", allRoles, lotHandler.List)
		lotR.Get("/:id", allRoles, lotHandler.GetByID)
		lotR.Post("/", adminOperator, lotHandler.Create)

		// Protected Auth
		auth.RegisterProtectedRoutes(protected, authHandler)

		// Health Check
		api.Get("/health", func(c *fiber.Ctx) error {
			return c.JSON(fiber.Map{"status": "ok", "vercel": true, "time": time.Now().Format(time.RFC3339)})
		})
	})

	// Final bridge
	if app == nil {
		errMsg := "Service Initialization Failed"
		if initErr != nil {
			errMsg = initErr.Error()
		}
		slog.Error("app initialization failed", "error", errMsg)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"Internal Server Error","message":"` + errMsg + ` Check environment variables (MONGO_URI) and database connectivity."}`))
		return
	}

	adaptor.FiberApp(app)(w, r)
}
