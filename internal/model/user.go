package model

import "time"

// User represents a user in the system.
type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"-"`
	PasswordHash string    `json:"-"`
	ZitadelSub   *string   `json:"-"`
	AuthProvider string    `json:"auth_provider"`
	DisplayName  string    `json:"display_name"`
	AvatarURL    string    `json:"avatar_url"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
