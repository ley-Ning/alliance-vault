package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"alliance-vault/backend/internal/model"

	"github.com/google/uuid"
)

type DocumentPermissionRepo struct {
	db *sql.DB
}

type DocumentPermissionDetail struct {
	DocumentID  string
	UserID      string
	AccessLevel model.DocumentPermissionAccess
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Username    string
	DisplayName string
	IsAdmin     bool
	IsDisabled  bool
}

func NewDocumentPermissionRepo(db *sql.DB) *DocumentPermissionRepo {
	return &DocumentPermissionRepo{db: db}
}

func (r *DocumentPermissionRepo) ListByDocument(ctx context.Context, documentID string) ([]DocumentPermissionDetail, error) {
	const query = `
SELECT
  p.document_id,
  p.user_id,
  p.access_level,
  p.created_at,
  p.updated_at,
  u.username,
  u.display_name,
  u.is_admin,
  u.is_disabled
FROM document_permissions p
JOIN users u ON u.id = p.user_id
WHERE p.document_id = $1
ORDER BY p.updated_at DESC, u.created_at ASC;
`

	rows, err := r.db.QueryContext(ctx, query, strings.TrimSpace(documentID))
	if err != nil {
		return nil, fmt.Errorf("list document permissions: %w", err)
	}
	defer rows.Close()

	items := make([]DocumentPermissionDetail, 0, 16)
	for rows.Next() {
		item, scanErr := scanDocumentPermissionDetail(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate document permissions: %w", err)
	}

	return items, nil
}

func (r *DocumentPermissionRepo) Upsert(
	ctx context.Context,
	documentID string,
	userID string,
	accessLevel model.DocumentPermissionAccess,
) (DocumentPermissionDetail, error) {
	level := normalizeAccessLevel(accessLevel)
	if level == "" {
		return DocumentPermissionDetail{}, fmt.Errorf("invalid access level")
	}

	const query = `
WITH upserted AS (
  INSERT INTO document_permissions (
    id,
    document_id,
    user_id,
    access_level
  )
  VALUES ($1, $2, $3, $4)
  ON CONFLICT (document_id, user_id)
  DO UPDATE SET
    access_level = EXCLUDED.access_level,
    updated_at = NOW()
  RETURNING
    document_id,
    user_id,
    access_level,
    created_at,
    updated_at
)
SELECT
  upserted.document_id,
  upserted.user_id,
  upserted.access_level,
  upserted.created_at,
  upserted.updated_at,
  u.username,
  u.display_name,
  u.is_admin,
  u.is_disabled
FROM upserted
JOIN users u ON u.id = upserted.user_id;
`

	row := r.db.QueryRowContext(
		ctx,
		query,
		uuid.NewString(),
		strings.TrimSpace(documentID),
		strings.TrimSpace(userID),
		level,
	)

	item, err := scanDocumentPermissionDetail(row)
	if err != nil {
		return DocumentPermissionDetail{}, err
	}
	return item, nil
}

func (r *DocumentPermissionRepo) Delete(ctx context.Context, documentID string, userID string) error {
	const query = `
DELETE FROM document_permissions
WHERE document_id = $1 AND user_id = $2;
`

	result, err := r.db.ExecContext(ctx, query, strings.TrimSpace(documentID), strings.TrimSpace(userID))
	if err != nil {
		return fmt.Errorf("delete document permission: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("document permission rows affected: %w", err)
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *DocumentPermissionRepo) GetAccess(
	ctx context.Context,
	documentID string,
	userID string,
	isAdmin bool,
) (canRead bool, canEdit bool, err error) {
	if isAdmin {
		return true, true, nil
	}

	const query = `
SELECT
  EXISTS(SELECT 1 FROM document_permissions WHERE document_id = $1) AS has_rules,
  EXISTS(
    SELECT 1 FROM document_permissions
    WHERE document_id = $1
      AND user_id = $2
      AND access_level IN ('read', 'edit')
  ) AS can_read,
  EXISTS(
    SELECT 1 FROM document_permissions
    WHERE document_id = $1
      AND user_id = $2
      AND access_level = 'edit'
  ) AS can_edit;
`

	var hasRules bool
	if scanErr := r.db.QueryRowContext(
		ctx,
		query,
		strings.TrimSpace(documentID),
		strings.TrimSpace(userID),
	).Scan(&hasRules, &canRead, &canEdit); scanErr != nil {
		return false, false, fmt.Errorf("query document access: %w", scanErr)
	}

	if !hasRules {
		return true, true, nil
	}
	return canRead, canEdit, nil
}

func normalizeAccessLevel(level model.DocumentPermissionAccess) model.DocumentPermissionAccess {
	switch level {
	case model.DocumentPermissionRead, model.DocumentPermissionEdit:
		return level
	default:
		return ""
	}
}

type scanPermissionRow interface {
	Scan(dest ...any) error
}

func scanDocumentPermissionDetail(row scanPermissionRow) (DocumentPermissionDetail, error) {
	var (
		item      DocumentPermissionDetail
		levelRaw  string
		createdAt time.Time
		updatedAt time.Time
	)

	if err := row.Scan(
		&item.DocumentID,
		&item.UserID,
		&levelRaw,
		&createdAt,
		&updatedAt,
		&item.Username,
		&item.DisplayName,
		&item.IsAdmin,
		&item.IsDisabled,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return DocumentPermissionDetail{}, ErrNotFound
		}
		return DocumentPermissionDetail{}, fmt.Errorf("scan document permission: %w", err)
	}

	level := normalizeAccessLevel(model.DocumentPermissionAccess(levelRaw))
	if level == "" {
		return DocumentPermissionDetail{}, fmt.Errorf("invalid stored access level")
	}

	item.AccessLevel = level
	item.CreatedAt = createdAt.UTC()
	item.UpdatedAt = updatedAt.UTC()

	return item, nil
}
