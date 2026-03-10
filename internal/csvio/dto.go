package csvio

// RowError represents a validation error in a specific CSV row.
type RowError struct {
	Row     int    `json:"row"`
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ImportReport is returned after a CSV import operation.
type ImportReport struct {
	Inserted int        `json:"inserted"`
	Updated  int        `json:"updated"`
	Skipped  int        `json:"skipped"`
	Failed   int        `json:"failed"`
	Errors   []RowError `json:"errors"`
}
