package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"alliance-vault/backend/internal/model"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"
)

type UserRepo struct {
	db *sql.DB
}

func NewUserRepo(db *sql.DB) *UserRepo {
	return &UserRepo{db: db}
}

func (r *UserRepo) Create(ctx context.Context, user model.User) (model.User, error) {
	const query = `
INSERT INTO users(
	id,
	username,
	display_name,
	password_hash,
	is_admin,
	is_disabled,
	must_change_password,
	password_changed_at,
	disabled_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING
	id,
	username,
	display_name,
	is_admin,
	is_disabled,
	must_change_password,
	password_hash,
	created_at,
	password_changed_at,
	disabled_at;
`

	row := r.db.QueryRowContext(
		ctx,
		query,
		strings.TrimSpace(user.ID),
		normalizeUsername(user.Username),
		normalizeDisplayName(user.DisplayName, user.Username),
		strings.TrimSpace(user.PasswordHash),
		user.IsAdmin,
		user.IsDisabled,
		user.MustChangePassword,
		user.PasswordChangedAt,
		user.DisabledAt,
	)

	created, err := scanUser(row)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return model.User{}, ErrConflict
		}
		return model.User{}, err
	}
	return created, nil
}

func (r *UserRepo) GetByUsername(ctx context.Context, username string) (model.User, error) {
	const query = `
SELECT
	id,
	username,
	display_name,
	is_admin,
	is_disabled,
	must_change_password,
	password_hash,
	created_at,
	password_changed_at,
	disabled_at
FROM users
WHERE username = $1;
`

	row := r.db.QueryRowContext(ctx, query, normalizeUsername(username))
	user, err := scanUser(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.User{}, ErrNotFound
		}
		return model.User{}, err
	}
	return user, nil
}

func (r *UserRepo) GetByID(ctx context.Context, id string) (model.User, error) {
	const query = `
SELECT
	id,
	username,
	display_name,
	is_admin,
	is_disabled,
	must_change_password,
	password_hash,
	created_at,
	password_changed_at,
	disabled_at
FROM users
WHERE id = $1;
`

	row := r.db.QueryRowContext(ctx, query, strings.TrimSpace(id))
	user, err := scanUser(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.User{}, ErrNotFound
		}
		return model.User{}, err
	}
	return user, nil
}

func (r *UserRepo) Count(ctx context.Context) (int, error) {
	const query = `SELECT COUNT(1) FROM users;`
	var count int
	if err := r.db.QueryRowContext(ctx, query).Scan(&count); err != nil {
		return 0, fmt.Errorf("count users: %w", err)
	}
	return count, nil
}

func (r *UserRepo) CountAdmins(ctx context.Context) (int, error) {
	const query = `SELECT COUNT(1) FROM users WHERE is_admin = TRUE AND is_disabled = FALSE;`
	var count int
	if err := r.db.QueryRowContext(ctx, query).Scan(&count); err != nil {
		return 0, fmt.Errorf("count admins: %w", err)
	}
	return count, nil
}

func (r *UserRepo) List(ctx context.Context, limit int) ([]model.User, error) {
	if limit <= 0 || limit > 300 {
		limit = 100
	}

	const query = `
SELECT
	id,
	username,
	display_name,
	is_admin,
	is_disabled,
	must_change_password,
	password_hash,
	created_at,
	password_changed_at,
	disabled_at
FROM users
ORDER BY created_at ASC
LIMIT $1;
`

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()

	items := make([]model.User, 0, limit)
	for rows.Next() {
		user, scanErr := scanUser(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, user)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate users: %w", err)
	}

	return items, nil
}

func (r *UserRepo) SetAdmin(ctx context.Context, userID string, isAdmin bool) (model.User, error) {
	const query = `
UPDATE users
SET is_admin = $2
WHERE id = $1
RETURNING
	id,
	username,
	display_name,
	is_admin,
	is_disabled,
	must_change_password,
	password_hash,
	created_at,
	password_changed_at,
	disabled_at;
`

	row := r.db.QueryRowContext(ctx, query, strings.TrimSpace(userID), isAdmin)
	updated, err := scanUser(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.User{}, ErrNotFound
		}
		return model.User{}, err
	}
	return updated, nil
}

func normalizeUsername(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

func normalizeDisplayName(raw, fallback string) string {
	value := strings.TrimSpace(raw)
	if value != "" {
		return value
	}
	if v := strings.TrimSpace(fallback); v != "" {
		return v
	}
	return "用户"
}

func (r *UserRepo) UpdatePassword(ctx context.Context, userID, passwordHash string, mustChangePassword bool) (model.User, error) {
	const query = `
UPDATE users
SET
	password_hash = $2,
	must_change_password = $3,
	password_changed_at = CASE WHEN $3 THEN NULL ELSE NOW() END
WHERE id = $1
RETURNING
	id,
	username,
	display_name,
	is_admin,
	is_disabled,
	must_change_password,
	password_hash,
	created_at,
	password_changed_at,
	disabled_at;
`

	row := r.db.QueryRowContext(
		ctx,
		query,
		strings.TrimSpace(userID),
		strings.TrimSpace(passwordHash),
		mustChangePassword,
	)

	updated, err := scanUser(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.User{}, ErrNotFound
		}
		return model.User{}, err
	}
	return updated, nil
}

func (r *UserRepo) SetDisabled(ctx context.Context, userID string, disabled bool) (model.User, error) {
	const query = `
UPDATE users
SET
	is_disabled = $2,
	disabled_at = CASE WHEN $2 THEN NOW() ELSE NULL END
WHERE id = $1
RETURNING
	id,
	username,
	display_name,
	is_admin,
	is_disabled,
	must_change_password,
	password_hash,
	created_at,
	password_changed_at,
	disabled_at;
`

	row := r.db.QueryRowContext(ctx, query, strings.TrimSpace(userID), disabled)
	updated, err := scanUser(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.User{}, ErrNotFound
		}
		return model.User{}, err
	}
	return updated, nil
}

func (r *UserRepo) DeleteByID(ctx context.Context, userID string) error {
	const query = `DELETE FROM users WHERE id = $1;`

	result, err := r.db.ExecContext(ctx, query, strings.TrimSpace(userID))
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read delete user rows affected: %w", err)
	}
	if affected == 0 {
		return ErrNotFound
	}

	return nil
}

func (r *UserRepo) EnsureDefaultAdmin(ctx context.Context, username, password, displayName string) (model.User, bool, error) {
	normalizedUsername := normalizeUsername(username)
	if normalizedUsername == "" {
		return model.User{}, false, fmt.Errorf("default admin username cannot be empty")
	}
	if strings.TrimSpace(password) == "" {
		return model.User{}, false, fmt.Errorf("default admin password cannot be empty")
	}

	existing, err := r.GetByUsername(ctx, normalizedUsername)
	if err == nil {
		return existing, false, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return model.User{}, false, err
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(strings.TrimSpace(password)), bcrypt.DefaultCost)
	if err != nil {
		return model.User{}, false, fmt.Errorf("hash default admin password: %w", err)
	}

	created, err := r.Create(ctx, model.User{
		ID:                 uuid.NewString(),
		Username:           normalizedUsername,
		DisplayName:        normalizeDisplayName(displayName, normalizedUsername),
		IsAdmin:            true,
		IsDisabled:         false,
		MustChangePassword: true,
		PasswordHash:       string(hashedPassword),
		PasswordChangedAt:  nil,
	})
	if err != nil {
		if errors.Is(err, ErrConflict) {
			existing, getErr := r.GetByUsername(ctx, normalizedUsername)
			if getErr != nil {
				return model.User{}, false, getErr
			}
			return existing, false, nil
		}
		return model.User{}, false, err
	}

	return created, true, nil
}

func scanUser(row interface{ Scan(dest ...any) error }) (model.User, error) {
	var user model.User
	var passwordChangedAt sql.NullTime
	var disabledAt sql.NullTime
	if err := row.Scan(
		&user.ID,
		&user.Username,
		&user.DisplayName,
		&user.IsAdmin,
		&user.IsDisabled,
		&user.MustChangePassword,
		&user.PasswordHash,
		&user.CreatedAt,
		&passwordChangedAt,
		&disabledAt,
	); err != nil {
		return model.User{}, fmt.Errorf("scan user: %w", err)
	}
	if passwordChangedAt.Valid {
		value := passwordChangedAt.Time.UTC()
		user.PasswordChangedAt = &value
	}
	if disabledAt.Valid {
		value := disabledAt.Time.UTC()
		user.DisabledAt = &value
	}
	return user, nil
}
