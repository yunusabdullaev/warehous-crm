package notify

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"

	lotPkg "warehouse-crm/core/lot"
	productPkg "warehouse-crm/core/product"
	stockPkg "warehouse-crm/core/stock"
)

// Service dispatches notifications for operational events.
type Service struct {
	repo       *Repository
	bot        *TelegramBot
	productSvc *productPkg.Service
	stockSvc   *stockPkg.Service
	lotSvc     *lotPkg.Service
}

func NewService(repo *Repository, bot *TelegramBot, productSvc *productPkg.Service, stockSvc *stockPkg.Service, lotSvc *lotPkg.Service) *Service {
	return &Service{
		repo:       repo,
		bot:        bot,
		productSvc: productSvc,
		stockSvc:   stockSvc,
		lotSvc:     lotSvc,
	}
}

// ── Event dispatchers ──

// NotifyOrderConfirmed — 📋 Order confirmed.
func (s *Service) NotifyOrderConfirmed(orderNo, clientName string, itemCount int) {
	text := fmt.Sprintf(
		"📋 <b>Order Confirmed</b>\n\nOrder: <b>%s</b>\nClient: %s\nItems: %d",
		EscapeHTML(orderNo), EscapeHTML(clientName), itemCount,
	)
	s.send(text)
}

// NotifyOrderPicking — 🎯 Picking started.
func (s *Service) NotifyOrderPicking(orderNo string) {
	text := fmt.Sprintf(
		"🎯 <b>Picking Started</b>\n\nOrder: <b>%s</b>",
		EscapeHTML(orderNo),
	)
	s.send(text)
}

// NotifyPickTaskDone — ✅ Pick task completed.
func (s *Service) NotifyPickTaskDone(orderID, taskID string, pickedQty, plannedQty int) {
	text := fmt.Sprintf(
		"✅ <b>Pick Task Done</b>\n\nOrder: %s\nTask: %s\nPicked: %d / %d",
		EscapeHTML(orderID), EscapeHTML(taskID), pickedQty, plannedQty,
	)
	s.send(text)
}

// NotifyOrderShipped — 🚚 Order shipped.
func (s *Service) NotifyOrderShipped(orderNo, clientName string, itemCount int) {
	text := fmt.Sprintf(
		"🚚 <b>Order Shipped</b>\n\nOrder: <b>%s</b>\nClient: %s\nItems: %d",
		EscapeHTML(orderNo), EscapeHTML(clientName), itemCount,
	)
	s.send(text)
}

// NotifyLowStock — ⚠️ Low stock with 6-hour dedup.
func (s *Service) NotifyLowStock(productID primitive.ObjectID, productName, sku string, availableQty int, threshold int) {
	dedupKey := fmt.Sprintf("LOW_STOCK:%s", productID.Hex())

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	dedup, err := s.repo.GetAlertDedup(ctx, dedupKey)
	if err != nil {
		slog.Error("notify: dedup lookup failed", "error", err)
		return
	}
	if dedup != nil && time.Since(dedup.LastSentAt) < 6*time.Hour {
		slog.Debug("notify: low stock alert suppressed (dedup)", "product_id", productID.Hex())
		return
	}

	text := fmt.Sprintf(
		"⚠️ <b>Low Stock Alert</b>\n\nProduct: <b>%s</b> (%s)\nAvailable: %d\nThreshold: %d",
		EscapeHTML(productName), EscapeHTML(sku), availableQty, threshold,
	)
	s.send(text)

	_ = s.repo.UpsertAlertDedup(ctx, dedupKey)
}

// CheckLowStock checks stock level for a product and triggers alert if below threshold.
func (s *Service) CheckLowStock(productID primitive.ObjectID) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	prod, err := s.productSvc.GetByID(ctx, productID)
	if err != nil || prod.LowStockThreshold == nil {
		return
	}

	stocks, err := s.stockSvc.ListByProduct(ctx, productID)
	if err != nil {
		slog.Error("notify: stock lookup failed", "error", err)
		return
	}

	totalQty := 0
	for _, st := range stocks {
		totalQty += st.Quantity
	}

	if totalQty <= *prod.LowStockThreshold {
		s.NotifyLowStock(productID, prod.Name, prod.SKU, totalQty, *prod.LowStockThreshold)
	}
}

// SendTest sends a test message.
func (s *Service) SendTest() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	settings, err := s.repo.GetSettings(ctx)
	if err != nil {
		return fmt.Errorf("read settings: %w", err)
	}
	if !settings.TelegramEnabled {
		return fmt.Errorf("telegram is disabled")
	}
	if settings.TelegramToken == "" || settings.TelegramChatIDs == "" {
		return fmt.Errorf("telegram token or chat IDs not configured")
	}

	text := "🧪 <b>Test Message</b>\n\nWarehouse CRM notifications are working!"
	s.bot.SendAll(settings.TelegramToken, settings.TelegramChatIDs, text)
	return nil
}

// NotifyReturnCreated — 📦 Return created.
func (s *Service) NotifyReturnCreated(rmaNo, orderNo, clientName string) {
	text := fmt.Sprintf(
		"📦 <b>Return Created</b>\n\nRMA: <b>%s</b>\nOrder: %s\nClient: %s",
		EscapeHTML(rmaNo), EscapeHTML(orderNo), EscapeHTML(clientName),
	)
	s.send(text)
}

// NotifyReturnReceived — ✅ Return received.
func (s *Service) NotifyReturnReceived(rmaNo, orderNo string, itemCount int) {
	text := fmt.Sprintf(
		"✅ <b>Return Received</b>\n\nRMA: <b>%s</b>\nOrder: %s\nItems: %d",
		EscapeHTML(rmaNo), EscapeHTML(orderNo), itemCount,
	)
	s.send(text)
}

// ── Expiry Digest ──────────────────────────────────────────

// RunExpiryDigest builds and sends the daily expiry digest.
// If force is true, dedup is ignored.
func (s *Service) RunExpiryDigest(ctx context.Context, force bool) (*DigestResult, error) {
	settings, err := s.repo.GetSettings(ctx)
	if err != nil {
		return nil, fmt.Errorf("read settings: %w", err)
	}

	if !settings.ExpiryDigestEnabled {
		return &DigestResult{Skipped: true, Reason: "expiry digest disabled"}, nil
	}

	if settings.TelegramToken == "" {
		return &DigestResult{Skipped: true, Reason: "telegram token not configured"}, nil
	}

	days := settings.ExpiryDigestDays
	if days <= 0 {
		days = 14
	}

	// Dedup check
	today := time.Now().UTC().Format("2006-01-02")
	dedupKey := fmt.Sprintf("EXPIRY_DIGEST:%s", today)

	if !force {
		dedup, err := s.repo.GetAlertDedup(ctx, dedupKey)
		if err != nil {
			return nil, fmt.Errorf("dedup lookup: %w", err)
		}
		if dedup != nil {
			return &DigestResult{Skipped: true, Reason: "already sent today"}, nil
		}
	}

	// Fetch expiring lots
	lots, err := s.lotSvc.FindExpiring(ctx, days)
	if err != nil {
		return nil, fmt.Errorf("find expiring lots: %w", err)
	}

	// Determine chat IDs
	chatIDs := settings.ExpiryDigestChatIDs
	if chatIDs == "" {
		chatIDs = settings.TelegramChatIDs
	}
	if chatIDs == "" {
		return &DigestResult{Skipped: true, Reason: "no chat IDs configured"}, nil
	}

	// If no lots expiring, send all-clear
	if len(lots) == 0 {
		text := fmt.Sprintf("✅ <b>Expiry Digest</b>\n\nNo expiring lots in the next %d days.", days)
		s.bot.SendAll(settings.TelegramToken, chatIDs, text)
		_ = s.repo.UpsertAlertDedup(ctx, dedupKey)
		return &DigestResult{Sent: true, Total: 0}, nil
	}

	// Group lots by urgency
	type enrichedLot struct {
		lot         *lotPkg.Lot
		productName string
		productSKU  string
		totalQty    int
		daysLeft    int
	}

	var urgent, warning, notice []enrichedLot
	now := time.Now().UTC()

	for _, l := range lots {
		if l.ExpDate == nil {
			continue
		}
		daysLeft := int(math.Ceil(l.ExpDate.Sub(now).Hours() / 24))

		// Enrich with product info
		pName := l.ProductID.Hex()[:6]
		pSKU := "—"
		prod, perr := s.productSvc.GetByID(ctx, l.ProductID)
		if perr == nil && prod != nil {
			pName = prod.Name
			pSKU = prod.SKU
		}

		// Sum stock qty for this lot
		totalQty := 0
		stocks, serr := s.stockSvc.ListByLot(ctx, l.ID)
		if serr == nil {
			for _, st := range stocks {
				totalQty += st.Quantity
			}
		}

		el := enrichedLot{
			lot:         l,
			productName: pName,
			productSKU:  pSKU,
			totalQty:    totalQty,
			daysLeft:    daysLeft,
		}

		switch {
		case daysLeft <= 3:
			urgent = append(urgent, el)
		case daysLeft <= 7:
			warning = append(warning, el)
		default:
			notice = append(notice, el)
		}
	}

	// Build message
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📋 <b>Expiry Digest</b> — %s\n", today))
	sb.WriteString(fmt.Sprintf("%d lots expiring within %d days\n", len(lots), days))

	if len(urgent) > 0 {
		sb.WriteString(fmt.Sprintf("\n🔴 <b>URGENT (0-3 days) — %d lots</b>\n", len(urgent)))
		for _, el := range urgent {
			sb.WriteString(fmt.Sprintf("  • <b>%s</b> (%s)\n    Lot: %s | Exp: %s | Qty: %d",
				EscapeHTML(el.productName), EscapeHTML(el.productSKU),
				EscapeHTML(el.lot.LotNo), el.lot.ExpDate.Format("2006-01-02"), el.totalQty))
			if el.daysLeft <= 0 {
				sb.WriteString(" ⚠️ EXPIRED")
			} else {
				sb.WriteString(fmt.Sprintf(" (%dd left)", el.daysLeft))
			}
			sb.WriteString("\n")
		}
	}

	if len(warning) > 0 {
		sb.WriteString(fmt.Sprintf("\n🟡 <b>WARNING (4-7 days) — %d lots</b>\n", len(warning)))
		for _, el := range warning {
			sb.WriteString(fmt.Sprintf("  • <b>%s</b> (%s)\n    Lot: %s | Exp: %s | Qty: %d (%dd left)\n",
				EscapeHTML(el.productName), EscapeHTML(el.productSKU),
				EscapeHTML(el.lot.LotNo), el.lot.ExpDate.Format("2006-01-02"), el.totalQty, el.daysLeft))
		}
	}

	if len(notice) > 0 {
		sb.WriteString(fmt.Sprintf("\n🟢 <b>NOTICE (8-%d days) — %d lots</b>\n", days, len(notice)))
		for _, el := range notice {
			sb.WriteString(fmt.Sprintf("  • <b>%s</b> (%s)\n    Lot: %s | Exp: %s | Qty: %d (%dd left)\n",
				EscapeHTML(el.productName), EscapeHTML(el.productSKU),
				EscapeHTML(el.lot.LotNo), el.lot.ExpDate.Format("2006-01-02"), el.totalQty, el.daysLeft))
		}
	}

	s.bot.SendAll(settings.TelegramToken, chatIDs, sb.String())
	_ = s.repo.UpsertAlertDedup(ctx, dedupKey)

	return &DigestResult{
		Sent:    true,
		Total:   len(lots),
		Urgent:  len(urgent),
		Warning: len(warning),
		Notice:  len(notice),
	}, nil
}

// ── Internal ──

func (s *Service) send(text string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	settings, err := s.repo.GetSettings(ctx)
	if err != nil {
		slog.Error("notify: settings lookup failed", "error", err)
		return
	}
	if !settings.TelegramEnabled {
		slog.Debug("notify: telegram disabled, skipping")
		return
	}
	if settings.TelegramToken == "" || settings.TelegramChatIDs == "" {
		slog.Warn("notify: telegram token or chat IDs missing")
		return
	}

	s.bot.SendAll(settings.TelegramToken, settings.TelegramChatIDs, text)
}
