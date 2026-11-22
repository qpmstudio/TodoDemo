package repository

import (
	"context"
	"errors"
	"time"

	"github.com/DevenWen/TodoDemo/internal/database"
	"github.com/DevenWen/TodoDemo/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// ErrEmailConflict is returned when a ZITADEL account would reuse an email
// that already belongs to another local account. TODO does not merge accounts
// by email, so the login is rejected instead.
var ErrEmailConflict = errors.New("repository: email already registered")

// UpsertUser creates a new user or updates an existing one by GitHub ID.
func UpsertUser(ctx context.Context, user *model.User) error {
	now := time.Now()
	return database.Pool.QueryRow(ctx,
		`INSERT INTO users (github_id, github_login, github_avatar_url, display_name, auth_provider, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, 'github', $5, $6)
		 ON CONFLICT (github_id)
		 DO UPDATE SET github_login = $2, github_avatar_url = $3, display_name = $4, updated_at = $6
		 RETURNING id, COALESCE(auth_provider, ''), created_at, updated_at`,
		user.GitHubID,
		user.GitHubLogin,
		user.GitHubAvatarURL,
		user.DisplayName,
		now,
		now,
	).Scan(&user.ID, &user.AuthProvider, &user.CreatedAt, &user.UpdatedAt)
}

// UpsertUserByZitadelSub creates or updates the local user that belongs to a
// ZITADEL subject. The subject is the identity key: email is only profile data,
// so a verified email that already belongs to another account is a conflict
// rather than a reason to merge accounts.
func UpsertUserByZitadelSub(ctx context.Context, sub, email, displayName string) (*model.User, error) {
	now := time.Now()
	user := &model.User{
		ZitadelSub:   &sub,
		Email:        email,
		DisplayName:  displayName,
		AuthProvider: "zitadel",
	}

	var emailArg *string
	if email != "" {
		emailArg = &email
	}

	err := database.Pool.QueryRow(ctx,
		`INSERT INTO users (zitadel_sub, email, display_name, auth_provider, created_at, updated_at)
		 VALUES ($1, $2, $3, 'zitadel', $4, $5)
		 ON CONFLICT (zitadel_sub) WHERE zitadel_sub IS NOT NULL
		 DO UPDATE SET
		   email = COALESCE(EXCLUDED.email, users.email),
		   display_name = COALESCE(NULLIF(EXCLUDED.display_name, ''), users.display_name),
		   updated_at = EXCLUDED.updated_at
		 RETURNING id, zitadel_sub, COALESCE(email, ''), COALESCE(display_name, ''), COALESCE(auth_provider, ''), created_at, updated_at`,
		sub,
		emailArg,
		displayName,
		now,
		now,
	).Scan(&user.ID, &user.ZitadelSub, &user.Email, &user.DisplayName, &user.AuthProvider, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "idx_users_email" {
			return nil, ErrEmailConflict
		}
		return nil, err
	}
	return user, nil
}

// ClaimUnlinkedUserByEmail attaches a ZITADEL subject to a local account that
// was created before ZITADEL became the only way in. The caller only reaches
// this path after ZITADEL proved the user owns the email, so the verified claim
// is a safe ownership proof; the local row is adopted instead of duplicated,
// which keeps the user's todos. It returns (nil, nil) when no unlinked row has
// that email, so the caller can report a real conflict.
func ClaimUnlinkedUserByEmail(ctx context.Context, sub, email string) (*model.User, error) {
	if email == "" {
		return nil, nil
	}
	now := time.Now()
	user := &model.User{}
	err := database.Pool.QueryRow(ctx,
		`UPDATE users
		    SET zitadel_sub = $1, auth_provider = 'zitadel', updated_at = $2
		  WHERE id = (
		        SELECT id FROM users
		         WHERE lower(email) = lower($3) AND zitadel_sub IS NULL
		         ORDER BY (password_hash IS NULL), created_at
		         LIMIT 1)
		  RETURNING id, zitadel_sub, COALESCE(email, ''), COALESCE(display_name, ''), COALESCE(auth_provider, ''), created_at, updated_at`,
		sub,
		now,
		email,
	).Scan(&user.ID, &user.ZitadelSub, &user.Email, &user.DisplayName, &user.AuthProvider, &user.CreatedAt, &user.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return user, nil
}

// GetUserByID retrieves a user by their UUID.
func GetUserByID(ctx context.Context, id string) (*model.User, error) {
	user := &model.User{}
	err := database.Pool.QueryRow(ctx,
		`SELECT id, github_id, COALESCE(github_login, ''), COALESCE(github_avatar_url, ''), COALESCE(email, ''), zitadel_sub, COALESCE(auth_provider, ''), display_name, created_at, updated_at
		 FROM users WHERE id = $1`, id,
	).Scan(&user.ID, &user.GitHubID, &user.GitHubLogin, &user.GitHubAvatarURL, &user.Email, &user.ZitadelSub, &user.AuthProvider, &user.DisplayName, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return user, nil
}

// CreateUserByEmail creates a new user with email and password auth.
func CreateUserByEmail(ctx context.Context, email, passwordHash, displayName string) (*model.User, error) {
	now := time.Now()
	user := &model.User{
		Email:        email,
		PasswordHash: passwordHash,
		DisplayName:  displayName,
		AuthProvider: "password",
	}
	err := database.Pool.QueryRow(ctx,
		`INSERT INTO users (email, password_hash, display_name, auth_provider, created_at, updated_at)
		 VALUES ($1, $2, $3, 'password', $4, $5)
		 RETURNING id, created_at, updated_at`,
		user.Email,
		user.PasswordHash,
		user.DisplayName,
		now,
		now,
	).Scan(&user.ID, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return user, nil
}

// GetUserByEmail retrieves a user by email (only for email-auth users).
func GetUserByEmail(ctx context.Context, email string) (*model.User, error) {
	user := &model.User{}
	err := database.Pool.QueryRow(ctx,
		`SELECT id, github_id, COALESCE(github_login, ''), COALESCE(github_avatar_url, ''), email, password_hash, zitadel_sub, COALESCE(auth_provider, ''), display_name, created_at, updated_at
		 FROM users WHERE email = $1 AND password_hash IS NOT NULL`, email,
	).Scan(&user.ID, &user.GitHubID, &user.GitHubLogin, &user.GitHubAvatarURL, &user.Email, &user.PasswordHash, &user.ZitadelSub, &user.AuthProvider, &user.DisplayName, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return user, nil
}
