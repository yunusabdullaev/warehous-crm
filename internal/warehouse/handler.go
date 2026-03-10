package warehouse

import (
	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Create(c *fiber.Ctx) error {
	var req struct {
		Code    string `json:"code"`
		Name    string `json:"name"`
		Address string `json:"address"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}
	if req.Code == "" || req.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "code and name are required"})
	}

	// Set tenant from caller context
	tenantID := getTenantFromLocals(c)

	w := &Warehouse{Code: req.Code, Name: req.Name, Address: req.Address, TenantID: tenantID}
	if err := h.svc.Create(c.Context(), w); err != nil {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(w)
}

func (h *Handler) List(c *fiber.Ctx) error {
	role, _ := c.Locals("role").(string)
	tenantID := getTenantFromLocals(c)

	var warehouses []*Warehouse
	var err error

	if role == "superadmin" {
		warehouses, err = h.svc.List(c.Context())
	} else if !tenantID.IsZero() {
		warehouses, err = h.svc.ListByTenant(c.Context(), tenantID)
	} else {
		warehouses, err = h.svc.List(c.Context())
	}

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if warehouses == nil {
		warehouses = []*Warehouse{}
	}
	return c.JSON(fiber.Map{"data": warehouses})
}

func (h *Handler) GetByID(c *fiber.Ctx) error {
	id, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}
	w, err := h.svc.GetByID(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "warehouse not found"})
	}

	// Tenant isolation: non-superadmin can only view warehouses in their tenant
	role, _ := c.Locals("role").(string)
	if role != "superadmin" {
		callerTenant := getTenantFromLocals(c)
		if !callerTenant.IsZero() && !w.TenantID.IsZero() && callerTenant != w.TenantID {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "cross-tenant access denied"})
		}
	}

	return c.JSON(w)
}

func (h *Handler) Update(c *fiber.Ctx) error {
	id, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}

	// Tenant isolation check
	role, _ := c.Locals("role").(string)
	if role != "superadmin" {
		w, err := h.svc.GetByID(c.Context(), id)
		if err != nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "warehouse not found"})
		}
		callerTenant := getTenantFromLocals(c)
		if !callerTenant.IsZero() && !w.TenantID.IsZero() && callerTenant != w.TenantID {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "cross-tenant access denied"})
		}
	}

	var req struct {
		Code    string `json:"code"`
		Name    string `json:"name"`
		Address string `json:"address"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}
	if err := h.svc.Update(c.Context(), id, req.Code, req.Name, req.Address); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"message": "warehouse updated"})
}

func (h *Handler) Delete(c *fiber.Ctx) error {
	id, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}

	// Tenant isolation check
	role, _ := c.Locals("role").(string)
	if role != "superadmin" {
		w, err := h.svc.GetByID(c.Context(), id)
		if err != nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "warehouse not found"})
		}
		callerTenant := getTenantFromLocals(c)
		if !callerTenant.IsZero() && !w.TenantID.IsZero() && callerTenant != w.TenantID {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "cross-tenant access denied"})
		}
	}

	if err := h.svc.Delete(c.Context(), id); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"message": "warehouse deleted"})
}

// getTenantFromLocals extracts tenantID from context locals.
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
