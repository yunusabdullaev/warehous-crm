package location

type CreateLocationRequest struct {
	Code  string `json:"code"`
	Name  string `json:"name"`
	Zone  string `json:"zone,omitempty"`
	Aisle string `json:"aisle,omitempty"`
	Rack  string `json:"rack,omitempty"`
	Level string `json:"level,omitempty"`
}

type UpdateLocationRequest struct {
	Name  *string `json:"name,omitempty"`
	Zone  *string `json:"zone,omitempty"`
	Aisle *string `json:"aisle,omitempty"`
	Rack  *string `json:"rack,omitempty"`
	Level *string `json:"level,omitempty"`
}

type LocationResponse struct {
	ID        string `json:"id"`
	Code      string `json:"code"`
	Name      string `json:"name"`
	Zone      string `json:"zone,omitempty"`
	Aisle     string `json:"aisle,omitempty"`
	Rack      string `json:"rack,omitempty"`
	Level     string `json:"level,omitempty"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

func ToResponse(l *Location) *LocationResponse {
	return &LocationResponse{
		ID:        l.ID.Hex(),
		Code:      l.Code,
		Name:      l.Name,
		Zone:      l.Zone,
		Aisle:     l.Aisle,
		Rack:      l.Rack,
		Level:     l.Level,
		CreatedAt: l.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt: l.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}
