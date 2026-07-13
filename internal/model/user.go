package model

import "time"

// User represents a user in the system.
type User struct {
	ID              string    `json:"id"`
	GitHubID        int64     `json:"-"`
	GitHubLogin     string    `json:"github_login"`
	GitHubAvatarURL string    `json:"github_avatar_url"`
	DisplayName     string    `json:"display_name"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}
