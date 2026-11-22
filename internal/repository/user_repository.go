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

// GetUserByZitadelSub returns the local user already linked to a ZITADEL
// subject, or (nil, nil) when the subject has never signed in before.
func GetUserByZitadelSub(ctx context.Context, sub string) (*model.User, error) {
	user := &model.User{}
	err := database.Pool.QueryRow(ctx,
		`SELECT id, COALESCE(email, ''), COALESCE(display_name, ''), COALESCE(auth_provider, '')
		   FROM users WHERE zitadel_sub = $1`, sub,
	).Scan(&user.ID, &user.Email, &user.DisplayName, &user.AuthProvider)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	user.ZitadelSub = &sub
	return user, nil
}

// ClaimUnlinkedUserByEmail attaches a ZITADEL subject to a local account that
// was created before ZITADEL became the only way in. The caller reaches this
// path only after ZITADEL proved the user owns the email, so the verified claim
// is a safe ownership proof; adopting the row instead of inserting a duplicate
// keeps the user's todos reachable.
//
// The insert path cannot rely on a unique-email violation to detect this case:
// idx_users_email only covers rows with a password hash, and ZITADEL rows have
// none, so two rows with the same email coexist silently. The lookup therefore
// happens up front. Only an unambiguous single unlinked row is adopted, and the
// count is returned so the caller can report ambiguity instead of guessing.
func ClaimUnlinkedUserByEmail(ctx context.Context, sub, email, displayName string) (*model.User, int, error) {
	if email == "" {
		return nil, 0, nil
	}
	tx, err := database.Pool.Begin(ctx)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	rows, err := tx.Query(ctx,
		`SELECT id FROM users
		  WHERE lower(email) = lower($1) AND zitadel_sub IS NULL
		  FOR UPDATE`, email)
	if err != nil {
		return nil, 0, err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, 0, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, len(ids), err
	}
	if len(ids) != 1 {
		return nil, len(ids), nil
	}

	user := &model.User{}
	err = tx.QueryRow(ctx,
		`UPDATE users
		    SET zitadel_sub = $1,
		        auth_provider = 'zitadel',
		        display_name = COALESCE(NULLIF($2, ''), display_name),
		        updated_at = $3
		  WHERE id = $4
		  RETURNING id, zitadel_sub, COALESCE(email, ''), COALESCE(display_name, ''), COALESCE(auth_provider, ''), created_at, updated_at`,
		sub, displayName, time.Now(), ids[0],
	).Scan(&user.ID, &user.ZitadelSub, &user.Email, &user.DisplayName, &user.AuthProvider, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return nil, 1, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, 1, err
	}
	return user, 1, nil
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
