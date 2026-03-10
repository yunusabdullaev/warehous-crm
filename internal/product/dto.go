package product

type CreateProductRequest struct {
	SKU         string `json:"sku"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Unit        string `json:"unit"`
}

type UpdateProductRequest struct {
	Name              *string `json:"name,omitempty"`
	Description       *string `json:"description,omitempty"`
	Unit              *string `json:"unit,omitempty"`
	LowStockThreshold *int    `json:"low_stock_threshold,omitempty"`
}

type ProductResponse struct {
	ID                string `json:"id"`
	SKU               string `json:"sku"`
	Name              string `json:"name"`
	Description       string `json:"description,omitempty"`
	Unit              string `json:"unit"`
	LowStockThreshold *int   `json:"low_stock_threshold,omitempty"`
	CreatedAt         string `json:"created_at"`
	UpdatedAt         string `json:"updated_at"`
}

func ToResponse(p *Product) *ProductResponse {
	return &ProductResponse{
		ID:                p.ID.Hex(),
		SKU:               p.SKU,
		Name:              p.Name,
		Description:       p.Description,
		Unit:              p.Unit,
		LowStockThreshold: p.LowStockThreshold,
		CreatedAt:         p.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:         p.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}
