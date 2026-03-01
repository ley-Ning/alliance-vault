package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type RefreshTokenRepo struct {
	db *sql.DB
}

func NewRefreshTokenRepo(db *sql.DB) *RefreshTokenRepo {
	return &RefreshTokenRepo{db: db}
}

func (r *RefreshTokenRepo) Create(ctx context.Context, id, userID, tokenJTI string, expiresAt time.Time) error {
	const query = `
INSERT INTO refresh_tokens(id, user_id, token_jti, expires_at)
VALUES ($1, $2, $3, $4);
`

	if _, err := r.db.ExecContext(
		ctx,
		query,
		strings.TrimSpace(id),
		strings.TrimSpace(userID),
		strings.TrimSpace(tokenJTI),
		expiresAt.UTC(),
	); err != nil {
		return fmt.Errorf("create refresh token: %w", err)
	}
	return nil
}

func (r *RefreshTokenRepo) IsActive(ctx context.Context, tokenJTI, userID string, now time.Time) (bool, error) {
	const query = `
SELECT expires_at, revoked_at
FROM refresh_tokens
WHERE token_jti = $1 AND user_id = $2;
`

	var (
		expiresAt time.Time
		revokedAt sql.NullTime
	)

	if err := r.db.QueryRowContext(ctx, query, strings.TrimSpace(tokenJTI), strings.TrimSpace(userID)).Scan(&expiresAt, &revokedAt); err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, fmt.Errorf("get refresh token: %w", err)
	}

	if revokedAt.Valid {
		return false, nil
	}
	if now.UTC().After(expiresAt.UTC()) {
		return false, nil
	}
	return true, nil
}

func (r *RefreshTokenRepo) Revoke(ctx context.Context, tokenJTI string) error {
	const query = `
UPDATE refresh_tokens
SET revoked_at = NOW()
WHERE token_jti = $1 AND revoked_at IS NULL;
`

	if _, err := r.db.ExecContext(ctx, query, strings.TrimSpace(tokenJTI)); err != nil {
		return fmt.Errorf("revoke refresh token: %w", err)
	}
	return nil
}

func (r *RefreshTokenRepo) RevokeByUser(ctx context.Context, userID string) error {
	const query = `
UPDATE refresh_tokens
SET revoked_at = NOW()
WHERE user_id = $1 AND revoked_at IS NULL;
`

	if _, err := r.db.ExecContext(ctx, query, strings.TrimSpace(userID)); err != nil {
		return fmt.Errorf("revoke refresh tokens by user: %w", err)
	}
	return nil
}
