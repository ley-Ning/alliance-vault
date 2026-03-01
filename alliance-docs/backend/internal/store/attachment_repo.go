package store

import (
	"context"
	"database/sql"
	"fmt"

	"alliance-vault/backend/internal/model"
)

type AttachmentRepo struct {
	db *sql.DB
}

func NewAttachmentRepo(db *sql.DB) *AttachmentRepo {
	return &AttachmentRepo{db: db}
}

func (r *AttachmentRepo) Create(ctx context.Context, attachment model.Attachment) (model.Attachment, error) {
	const query = `
INSERT INTO attachments(
  id,
  document_id,
  object_key,
  filename,
  content_type,
  size_bytes,
  owner_name,
  storage_provider
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
RETURNING created_at;
`

	if err := r.db.QueryRowContext(
		ctx,
		query,
		attachment.ID,
		attachment.DocumentID,
		attachment.ObjectKey,
		attachment.FileName,
		attachment.ContentType,
		attachment.SizeBytes,
		attachment.Owner,
		attachment.Storage,
	).Scan(&attachment.CreatedAt); err != nil {
		return model.Attachment{}, fmt.Errorf("create attachment: %w", err)
	}

	return attachment, nil
}

func (r *AttachmentRepo) GetByID(ctx context.Context, id string) (model.Attachment, error) {
	const query = `
SELECT id, document_id, object_key, filename, content_type, size_bytes, owner_name, storage_provider, created_at
FROM attachments
WHERE id = $1;
`

	var a model.Attachment
	if err := r.db.QueryRowContext(ctx, query, id).Scan(
		&a.ID,
		&a.DocumentID,
		&a.ObjectKey,
		&a.FileName,
		&a.ContentType,
		&a.SizeBytes,
		&a.Owner,
		&a.Storage,
		&a.CreatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return model.Attachment{}, ErrNotFound
		}
		return model.Attachment{}, fmt.Errorf("get attachment: %w", err)
	}
	return a, nil
}

func (r *AttachmentRepo) ListByDocument(ctx context.Context, documentID string, limit int) ([]model.Attachment, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	const query = `
SELECT id, document_id, object_key, filename, content_type, size_bytes, owner_name, storage_provider, created_at
FROM attachments
WHERE document_id = $1
ORDER BY created_at DESC
LIMIT $2;
`

	rows, err := r.db.QueryContext(ctx, query, documentID, limit)
	if err != nil {
		return nil, fmt.Errorf("list attachments: %w", err)
	}
	defer rows.Close()

	result := make([]model.Attachment, 0, limit)
	for rows.Next() {
		var a model.Attachment
		if err := rows.Scan(
			&a.ID,
			&a.DocumentID,
			&a.ObjectKey,
			&a.FileName,
			&a.ContentType,
			&a.SizeBytes,
			&a.Owner,
			&a.Storage,
			&a.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan attachment: %w", err)
		}
		result = append(result, a)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate attachments: %w", err)
	}

	return result, nil
}

func (r *AttachmentRepo) DeleteByDocument(ctx context.Context, documentID string) error {
	const query = `DELETE FROM attachments WHERE document_id = $1;`
	if _, err := r.db.ExecContext(ctx, query, documentID); err != nil {
		return fmt.Errorf("delete attachments by document: %w", err)
	}
	return nil
}

func (r *AttachmentRepo) DeleteByID(ctx context.Context, id string) (model.Attachment, error) {
	const query = `
DELETE FROM attachments
WHERE id = $1
RETURNING id, document_id, object_key, filename, content_type, size_bytes, owner_name, storage_provider, created_at;
`

	var a model.Attachment
	if err := r.db.QueryRowContext(ctx, query, id).Scan(
		&a.ID,
		&a.DocumentID,
		&a.ObjectKey,
		&a.FileName,
		&a.ContentType,
		&a.SizeBytes,
		&a.Owner,
		&a.Storage,
		&a.CreatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return model.Attachment{}, ErrNotFound
		}
		return model.Attachment{}, fmt.Errorf("delete attachment: %w", err)
	}

	return a, nil
}
