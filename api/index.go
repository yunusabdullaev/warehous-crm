package handler

import (
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gofiber/adaptor/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/recover"

	"warehouse-crm/configs"
	"warehouse-crm/internal/adjustment"
	"warehouse-crm/internal/auth"
	"warehouse-crm/internal/billing"
	"warehouse-crm/internal/csvio"
	"warehouse-crm/internal/dashboard"
	"warehouse-crm/internal/history"
	"warehouse-crm/internal/inbound"
	"warehouse-crm/internal/location"
	"warehouse-crm/internal/lot"
	"warehouse-crm/internal/middleware"
	"warehouse-crm/internal/notify"
	"warehouse-crm/internal/order"
	"warehouse-crm/internal/orderdoc"
	"warehouse-crm/internal/outbound"
	"warehouse-crm/internal/picking"
	"warehouse-crm/internal/product"
	"warehouse-crm/internal/qrlabel"
	"warehouse-crm/internal/reports"
	"warehouse-crm/internal/reservation"
	"warehouse-crm/internal/returndoc"
	"warehouse-crm/internal/returns"
	"warehouse-crm/internal/session"
	"warehouse-crm/internal/stock"
	"warehouse-crm/internal/tenant"
	"warehouse-crm/internal/warehouse"
	"warehouse-crm/pkg/database"
	"warehouse-crm/pkg/logger"
)

var (
	app      *fiber.App
	initOnce sync.Once
)

func Handler(w http.ResponseWriter, r *http.Request) {
	initOnce.Do(func() {
		logger.Init()
		cfg := configs.LoadConfig()

		mongoClient, err := database.Connect(cfg.MongoURI)
		if err != nil {
			panic(err)
		}

		db := mongoClient.Database(cfg.DBName)
		database.EnsureIndexes(db)

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

		sessionRepo := session.NewRepository(
			database.GetCollection(mongoClient, cfg.DBName, "sessions"),
			database.GetCollection(mongoClient, cfg.DBName, "login_attempts"),
			database.GetCollection(mongoClient, cfg.DBName, "reset_tokens"),
		)
		sessionSvc := session.NewService(sessionRepo, cfg.JWTSecret, cfg.AccessTokenTTLMin, cfg.RefreshTokenTTLDays)

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

		inboundSvc := inbound.NewService(inboundRepo, stockSvc, historySvc, lotSvc)
		outboundSvc := outbound.NewService(outboundRepo, stockSvc, historySvc)
		adjustmentSvc := adjustment.NewService(adjustmentRepo, stockSvc, historySvc)
		reservationSvc := reservation.NewService(reservationRepo, stockSvc, historySvc)
		pickingSvc := picking.NewService(pickingRepo, stockSvc, historySvc, lotSvc)

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

		returnsRepo := returns.NewRepository(
			database.GetCollection(mongoClient, cfg.DBName, "returns"),
			database.GetCollection(mongoClient, cfg.DBName, "return_items"),
			database.GetCollection(mongoClient, cfg.DBName, "qc_holds"),
			database.GetCollection(mongoClient, cfg.DBName, "counters"),
		)
		returnsSvc := returns.NewService(returnsRepo, orderSvc, stockSvc, historySvc, notifySvc, lotSvc)

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

		app = fiber.New()
		app.Use(recover.New())

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
		auth.RegisterPublicRoutes(api, authHandler)
		api.Post("/webhooks/stripe", billingHandler.Webhook)

		tenantCol := database.GetCollection(mongoClient, cfg.DBName, "tenants")
		protected := api.Group("", middleware.AuthMiddleware(cfg.JWTSecret),
			middleware.RequireTenantActive(tenantCol),
			middleware.WarehouseContext(
				database.GetCollection(mongoClient, cfg.DBName, "users"),
				database.GetCollection(mongoClient, cfg.DBName, "warehouses"),
			))

		allRoles := middleware.RequireRoles("admin", "operator", "loader")
		adminOperator := middleware.RequireRoles("admin", "operator")
		adminOnly := middleware.RequireRoles("admin")
		superOnly := middleware.RequireRoles("superadmin")

		// Register ALL routes as in main.go
		tenantsR := protected.Group("/tenants", superOnly)
		tenantsR.Get("/", tenantHandler.List)
		tenantsR.Get("/:id", tenantHandler.GetByID)
		tenantsR.Post("/", tenantHandler.Create)
		tenantsR.Put("/:id", tenantHandler.Update)
		tenantsR.Delete("/:id", tenantHandler.Delete)
		tenantsR.Get("/:id/usage", tenantHandler.GetUsage)
		tenantsR.Put("/:id/plan", tenantHandler.UpdatePlan)

		billingR := protected.Group("/billing", adminOnly)
		billingR.Post("/checkout-session", billingHandler.CreateCheckoutSession)
		billingR.Post("/portal-session", billingHandler.CreatePortalSession)
		billingR.Get("/status", billingHandler.GetBillingStatus)

		whR := protected.Group("/warehouses", adminOnly)
		whR.Get("/", warehouseHandler.List)
		whR.Get("/:id", warehouseHandler.GetByID)
		whR.Post("/", warehouseHandler.Create)
		whR.Put("/:id", warehouseHandler.Update)
		whR.Delete("/:id", warehouseHandler.Delete)

		prR := protected.Group("/products")
		prR.Get("/", allRoles, productHandler.List)
		prR.Get("/:id", allRoles, productHandler.GetByID)
		prR.Post("/", adminOnly, productHandler.Create)
		prR.Put("/:id", adminOnly, productHandler.Update)
		prR.Delete("/:id", adminOnly, productHandler.Delete)

		locR := protected.Group("/locations")
		locR.Get("/", allRoles, locationHandler.List)
		locR.Get("/:id", allRoles, locationHandler.GetByID)
		locR.Post("/", adminOnly, locationHandler.Create)
		locR.Put("/:id", adminOnly, locationHandler.Update)
		locR.Delete("/:id", adminOnly, locationHandler.Delete)
		locR.Get("/:id/qr", adminOnly, qrlabelHandler.QRCode)
		locR.Get("/:id/label", adminOnly, qrlabelHandler.Label)

		inR := protected.Group("/inbound", allRoles)
		inR.Post("/", inboundHandler.Create)
		inR.Get("/", inboundHandler.List)
		inR.Get("/:id", inboundHandler.GetByID)
		inR.Post("/:id/reverse", adminOperator, inboundHandler.Reverse)

		outR := protected.Group("/outbound", adminOperator)
		outR.Post("/", outboundHandler.Create)
		outR.Get("/", outboundHandler.List)
		outR.Get("/:id", outboundHandler.GetByID)
		outR.Post("/:id/reverse", outboundHandler.Reverse)

		adjR := protected.Group("/adjustments", adminOperator)
		adjR.Post("/", adjustmentHandler.Create)
		adjR.Get("/", adjustmentHandler.List)

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

		pickR := protected.Group("/pick-tasks")
		pickR.Get("/my", allRoles, pickingHandler.MyTasks)
		pickR.Get("/:id", allRoles, pickingHandler.GetTask)
		pickR.Post("/:id/assign", adminOperator, pickingHandler.Assign)
		pickR.Post("/:id/scan", allRoles, pickingHandler.Scan)

		resR := protected.Group("/reservations", adminOperator)
		resR.Get("/", reservationHandler.List)
		resR.Post("/release", reservationHandler.Release)

		stR := protected.Group("/stock", allRoles)
		stR.Get("/", stockHandler.ListAll)
		stR.Get("/product/:id", stockHandler.GetByProduct)
		stR.Get("/location/:id", stockHandler.GetByLocation)

		histR := protected.Group("/history", adminOperator)
		histR.Get("/", historyHandler.List)

		auth.RegisterProtectedRoutes(protected, authHandler)

		usR := protected.Group("/users", adminOnly)
		usR.Get("/", authHandler.List)
		usR.Get("/:id", authHandler.GetByID)
		usR.Put("/:id", authHandler.Update)
		usR.Delete("/:id", authHandler.Delete)
		usR.Post("/:id/reset-token", authHandler.GenerateResetToken)
		usR.Post("/:id/revoke-sessions", authHandler.RevokeUserSessions)

		dashR := protected.Group("/dashboard", adminOperator)
		dashR.Get("/summary", dashboardHandler.Summary)

		repR := protected.Group("/reports", adminOnly)
		repR.Get("/movements", reportsHandler.Movements)
		repR.Get("/stock", reportsHandler.StockReport)
		repR.Get("/orders", reportsHandler.OrderReport)
		repR.Get("/picking", reportsHandler.PickingReport)
		repR.Get("/returns", reportsHandler.ReturnsReport)
		repR.Get("/expiry", reportsHandler.ExpiryReport)

		impR := protected.Group("/import", adminOnly)
		impR.Post("/products", csvioHandler.ImportProducts)
		impR.Post("/locations", csvioHandler.ImportLocations)

		expR := protected.Group("/export", adminOnly)
		expR.Get("/products", csvioHandler.ExportProducts)
		expR.Get("/locations", csvioHandler.ExportLocations)

		setR := protected.Group("/settings", adminOnly)
		setR.Get("/notifications", notifyHandler.Get)
		setR.Put("/notifications", notifyHandler.Update)
		setR.Post("/notifications/test", notifyHandler.Test)

		retR := protected.Group("/returns")
		retR.Get("/", allRoles, returnsHandler.List)
		retR.Get("/:id", allRoles, returnsHandler.GetByID)
		retR.Post("/", adminOperator, returnsHandler.Create)
		retR.Post("/:id/receive", adminOperator, returnsHandler.Receive)
		retR.Post("/:id/cancel", adminOperator, returnsHandler.Cancel)
		retR.Get("/:id/note.pdf", allRoles, returndocHandler.NotePDF)

		lotR := protected.Group("/lots")
		lotR.Get("/", allRoles, lotHandler.List)
		lotR.Get("/:id", allRoles, lotHandler.GetByID)
		lotR.Post("/", adminOperator, lotHandler.Create)

		api.Get("/health", func(c *fiber.Ctx) error {
			return c.JSON(fiber.Map{"status": "ok", "vercel": true, "time": time.Now().Format(time.RFC3339)})
		})
	})

	adaptor.FiberApp(app)(w, r)
}
