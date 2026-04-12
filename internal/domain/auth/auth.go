package auth

import "time"

type Role string

const (
	RoleOperator   Role = "operator"
	RoleSuperadmin Role = "superadmin"
)

type Company struct {
	ID             int64     `json:"id"`
	Name           string    `json:"name"`
	ExternalSource string    `json:"external_source,omitempty"`
	ExternalID     string    `json:"external_source_id,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type User struct {
	ID             int64     `json:"id"`
	CompanyID      *int64    `json:"company_id,omitempty"`
	Username       string    `json:"username"`
	DisplayName    string    `json:"display_name"`
	PasswordHash   string    `json:"-"`
	Role           Role      `json:"role"`
	Active         bool      `json:"active"`
	ExternalSource string    `json:"external_source,omitempty"`
	ExternalID     string    `json:"external_source_id,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type Actor struct {
	UserID      int64  `json:"user_id"`
	CompanyID   int64  `json:"company_id"`
	CompanyName string `json:"company_name,omitempty"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name,omitempty"`
	Role        Role   `json:"role"`
}

func (a Actor) IsSuperadmin() bool {
	return a.Role == RoleSuperadmin
}
