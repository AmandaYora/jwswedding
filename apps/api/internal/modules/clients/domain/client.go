package domain

import "time"

// Client adalah master pasangan (D1, D2) — satu baris per pasangan, lepas
// dari project. Punya 1..n ClientContact berperan (Bride / Groom / Family
// Representative), masing-masing bisa punya akun portal.
type Client struct {
	ID        int64
	TenantID  int64
	BrideName string
	GroomName string
	Phone     string
	Email     string
	Notes     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// DisplayName menggabungkan nama pasangan seperti dokumen menuliskannya.
func (c Client) DisplayName() string {
	switch {
	case c.BrideName != "" && c.GroomName != "":
		return c.BrideName + " & " + c.GroomName
	case c.BrideName != "":
		return c.BrideName
	default:
		return c.GroomName
	}
}
