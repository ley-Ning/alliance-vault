package model

import "time"

type User struct {
	ID                 string     `json:"id"`
	Username           string     `json:"username"`
	DisplayName        string     `json:"displayName"`
	IsAdmin            bool       `json:"isAdmin"`
	IsDisabled         bool       `json:"isDisabled"`
	MustChangePassword bool       `json:"mustChangePassword"`
	PasswordHash       string     `json:"-"`
	CreatedAt          time.Time  `json:"createdAt"`
	PasswordChangedAt  *time.Time `json:"passwordChangedAt,omitempty"`
	DisabledAt         *time.Time `json:"disabledAt,omitempty"`
}
