package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"alliance-vault/backend/internal/model"
)

type DocumentRepo struct {
	db *sql.DB
}

type DocumentPatch struct {
	Title   *string
	Content *string
	Tags    *[]string
	Status  *model.DocumentStatus
	Owner   *string
}

func NewDocumentRepo(db *sql.DB) *DocumentRepo {
	return &DocumentRepo{db: db}
}

func (r *DocumentRepo) List(ctx context.Context, limit int) ([]model.Document, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}

	const query = `
SELECT id, title, content, tags, status, owner_name, created_at, updated_at
FROM documents
WHERE deleted_at IS NULL
ORDER BY updated_at DESC
LIMIT $1;
`

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("list documents: %w", err)
	}
	defer rows.Close()

	items := make([]model.Document, 0, limit)
	for rows.Next() {
		doc, err := scanDocument(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, doc)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate documents: %w", err)
	}

	return items, nil
}

func (r *DocumentRepo) ListDeleted(ctx context.Context, limit int) ([]model.Document, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}

	const query = `
SELECT id, title, content, tags, status, owner_name, created_at, updated_at
FROM documents
WHERE deleted_at IS NOT NULL
ORDER BY deleted_at DESC
LIMIT $1;
`

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("list deleted documents: %w", err)
	}
	defer rows.Close()

	items := make([]model.Document, 0, limit)
	for rows.Next() {
		doc, scanErr := scanDocument(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, doc)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate deleted documents: %w", err)
	}

	return items, nil
}

func (r *DocumentRepo) GetByID(ctx context.Context, id string) (model.Document, error) {
	const query = `
SELECT id, title, content, tags, status, owner_name, created_at, updated_at
FROM documents
WHERE id = $1
  AND deleted_at IS NULL;
`

	row := r.db.QueryRowContext(ctx, query, strings.TrimSpace(id))
	doc, err := scanDocument(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return model.Document{}, ErrNotFound
		}
		return model.Document{}, err
	}
	return doc, nil
}

func (r *DocumentRepo) GetByIDAny(ctx context.Context, id string) (model.Document, error) {
	const query = `
SELECT id, title, content, tags, status, owner_name, created_at, updated_at
FROM documents
WHERE id = $1;
`

	row := r.db.QueryRowContext(ctx, query, strings.TrimSpace(id))
	doc, err := scanDocument(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return model.Document{}, ErrNotFound
		}
		if errors.Is(err, ErrNotFound) {
			return model.Document{}, ErrNotFound
		}
		return model.Document{}, err
	}
	return doc, nil
}

func (r *DocumentRepo) Create(ctx context.Context, doc model.Document) (model.Document, error) {
	tagsRaw, err := json.Marshal(normalizeTags(doc.Tags))
	if err != nil {
		return model.Document{}, fmt.Errorf("marshal tags: %w", err)
	}

	const query = `
INSERT INTO documents(id, title, content, tags, status, owner_name)
VALUES ($1, $2, $3, $4::jsonb, $5, $6)
RETURNING id, title, content, tags, status, owner_name, created_at, updated_at;
`

	row := r.db.QueryRowContext(
		ctx,
		query,
		doc.ID,
		normalizeTitle(doc.Title),
		normalizeContent(doc.Content),
		string(tagsRaw),
		normalizeStatus(doc.Status),
		normalizeOwner(doc.Owner),
	)

	created, err := scanDocument(row)
	if err != nil {
		return model.Document{}, err
	}
	return created, nil
}

func (r *DocumentRepo) Update(ctx context.Context, id string, patch DocumentPatch) (model.Document, error) {
	var tagsRaw *string
	if patch.Tags != nil {
		encoded, err := json.Marshal(normalizeTags(*patch.Tags))
		if err != nil {
			return model.Document{}, fmt.Errorf("marshal tags: %w", err)
		}
		t := string(encoded)
		tagsRaw = &t
	}

	const query = `
UPDATE documents
SET
  title = CASE WHEN $2 THEN $3 ELSE title END,
  content = CASE WHEN $4 THEN $5 ELSE content END,
  tags = CASE WHEN $6 THEN $7::jsonb ELSE tags END,
  status = CASE WHEN $8 THEN $9 ELSE status END,
  owner_name = CASE WHEN $10 THEN $11 ELSE owner_name END,
  updated_at = NOW()
WHERE id = $1
  AND deleted_at IS NULL
RETURNING id, title, content, tags, status, owner_name, created_at, updated_at;
`

	row := r.db.QueryRowContext(
		ctx,
		query,
		strings.TrimSpace(id),
		patch.Title != nil,
		normalizeNullableTitle(patch.Title),
		patch.Content != nil,
		normalizeNullableContent(patch.Content),
		patch.Tags != nil,
		tagsRaw,
		patch.Status != nil,
		normalizeNullableStatus(patch.Status),
		patch.Owner != nil,
		normalizeNullableOwner(patch.Owner),
	)

	updated, err := scanDocument(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return model.Document{}, ErrNotFound
		}
		return model.Document{}, err
	}
	return updated, nil
}

func (r *DocumentRepo) Delete(ctx context.Context, id string) error {
	const query = `
UPDATE documents
SET deleted_at = NOW(), updated_at = NOW()
WHERE id = $1
  AND deleted_at IS NULL;
`
	result, err := r.db.ExecContext(ctx, query, strings.TrimSpace(id))
	if err != nil {
		return fmt.Errorf("soft delete document: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *DocumentRepo) Restore(ctx context.Context, id string) (model.Document, error) {
	const query = `
UPDATE documents
SET deleted_at = NULL, updated_at = NOW()
WHERE id = $1
  AND deleted_at IS NOT NULL
RETURNING id, title, content, tags, status, owner_name, created_at, updated_at;
`

	row := r.db.QueryRowContext(ctx, query, strings.TrimSpace(id))
	doc, err := scanDocument(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, ErrNotFound) {
			return model.Document{}, ErrNotFound
		}
		return model.Document{}, err
	}
	return doc, nil
}

type scanRow interface {
	Scan(dest ...any) error
}

func scanDocument(row scanRow) (model.Document, error) {
	var (
		doc     model.Document
		tagsRaw []byte
	)
	if err := row.Scan(
		&doc.ID,
		&doc.Title,
		&doc.Content,
		&tagsRaw,
		&doc.Status,
		&doc.Owner,
		&doc.CreatedAt,
		&doc.UpdatedAt,
	); err != nil {
		return model.Document{}, err
	}

	if len(tagsRaw) == 0 {
		doc.Tags = []string{}
		return doc, nil
	}

	if err := json.Unmarshal(tagsRaw, &doc.Tags); err != nil {
		return model.Document{}, fmt.Errorf("unmarshal tags: %w", err)
	}

	doc.Tags = normalizeTags(doc.Tags)
	return doc, nil
}

func normalizeTags(tags []string) []string {
	if len(tags) == 0 {
		return []string{"未分类"}
	}

	seen := make(map[string]struct{}, len(tags))
	result := make([]string, 0, len(tags))
	for _, tag := range tags {
		t := strings.TrimSpace(tag)
		if t == "" {
			continue
		}
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		result = append(result, t)
	}

	if len(result) == 0 {
		return []string{"未分类"}
	}
	return result
}

func normalizeTitle(raw string) string {
	v := strings.TrimSpace(raw)
	if v == "" {
		return "未命名文档"
	}
	return v
}

func normalizeContent(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return "<p></p>"
	}
	return raw
}

func normalizeStatus(raw model.DocumentStatus) model.DocumentStatus {
	switch raw {
	case model.DocumentStatusReviewing, model.DocumentStatusPublished:
		return raw
	default:
		return model.DocumentStatusDraft
	}
}

func normalizeOwner(raw string) string {
	v := strings.TrimSpace(raw)
	if v == "" {
		return "未分配"
	}
	return v
}

func normalizeNullableTitle(raw *string) *string {
	if raw == nil {
		return nil
	}
	v := normalizeTitle(*raw)
	return &v
}

func normalizeNullableContent(raw *string) *string {
	if raw == nil {
		return nil
	}
	v := normalizeContent(*raw)
	return &v
}

func normalizeNullableStatus(raw *model.DocumentStatus) *model.DocumentStatus {
	if raw == nil {
		return nil
	}
	v := normalizeStatus(*raw)
	return &v
}

func normalizeNullableOwner(raw *string) *string {
	if raw == nil {
		return nil
	}
	v := normalizeOwner(*raw)
	return &v
}
