package repository

import (
	"context"
	"time"

	"github.com/DevenWen/TodoDemo/internal/database"
	"github.com/DevenWen/TodoDemo/internal/model"
)

// UpsertUser creates a new user or updates an existing one by GitHub ID.
func UpsertUser(ctx context.Context, user *model.User) error {
	now := time.Now()
	return database.Pool.QueryRow(ctx,
		`INSERT INTO users (github_id, github_login, github_avatar_url, display_name, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 ON CONFLICT (github_id)
		 DO UPDATE SET github_login = $2, github_avatar_url = $3, display_name = $4, updated_at = $6
		 RETURNING id, created_at, updated_at`,
		user.GitHubID,
		user.GitHubLogin,
		user.GitHubAvatarURL,
		user.DisplayName,
		now,
		now,
	).Scan(&user.ID, &user.CreatedAt, &user.UpdatedAt)
}

// GetUserByID retrieves a user by their UUID.
func GetUserByID(ctx context.Context, id string) (*model.User, error) {
	user := &model.User{}
	err := database.Pool.QueryRow(ctx,
		`SELECT id, github_id, github_login, github_avatar_url, display_name, created_at, updated_at
		 FROM users WHERE id = $1`, id,
	).Scan(&user.ID, &user.GitHubID, &user.GitHubLogin, &user.GitHubAvatarURL, &user.DisplayName, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return user, nil
}
