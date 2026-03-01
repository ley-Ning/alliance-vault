package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"alliance-vault/backend/internal/model"

	"github.com/google/uuid"
)

type DocumentVersionRepo struct {
	db *sql.DB
}

func NewDocumentVersionRepo(db *sql.DB) *DocumentVersionRepo {
	return &DocumentVersionRepo{db: db}
}

func (r *DocumentVersionRepo) CreateSnapshot(
	ctx context.Context,
	doc model.Document,
	event model.DocumentVersionEvent,
	createdBy string,
) (model.DocumentVersion, error) {
	tagsRaw, err := json.Marshal(normalizeTags(doc.Tags))
	if err != nil {
		return model.DocumentVersion{}, fmt.Errorf("marshal version tags: %w", err)
	}

	const query = `
WITH next_version AS (
  SELECT COALESCE(MAX(version_no), 0) + 1 AS version_no
  FROM document_versions
  WHERE document_id = $1
)
INSERT INTO document_versions (
  id,
  document_id,
  version_no,
  title,
  content,
  tags,
  status,
  owner_name,
  event_type,
  created_by
)
SELECT
  $2,
  $1,
  next_version.version_no,
  $3,
  $4,
  $5::jsonb,
  $6,
  $7,
  $8,
  $9
FROM next_version
RETURNING
  id,
  document_id,
  version_no,
  title,
  content,
  tags,
  status,
  owner_name,
  event_type,
  created_by,
  created_at;
`

	row := r.db.QueryRowContext(
		ctx,
		query,
		strings.TrimSpace(doc.ID),
		uuid.NewString(),
		normalizeTitle(doc.Title),
		normalizeContent(doc.Content),
		string(tagsRaw),
		normalizeStatus(doc.Status),
		normalizeOwner(doc.Owner),
		normalizeVersionEvent(event),
		normalizeVersionOperator(createdBy),
	)

	version, err := scanDocumentVersion(row)
	if err != nil {
		return model.DocumentVersion{}, err
	}
	return version, nil
}

func (r *DocumentVersionRepo) ListByDocument(
	ctx context.Context,
	documentID string,
	limit int,
) ([]model.DocumentVersion, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	const query = `
SELECT
  id,
  document_id,
  version_no,
  title,
  content,
  tags,
  status,
  owner_name,
  event_type,
  created_by,
  created_at
FROM document_versions
WHERE document_id = $1
ORDER BY version_no DESC
LIMIT $2;
`

	rows, err := r.db.QueryContext(ctx, query, strings.TrimSpace(documentID), limit)
	if err != nil {
		return nil, fmt.Errorf("list document versions: %w", err)
	}
	defer rows.Close()

	items := make([]model.DocumentVersion, 0, limit)
	for rows.Next() {
		item, scanErr := scanDocumentVersion(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate document versions: %w", err)
	}

	return items, nil
}

func (r *DocumentVersionRepo) GetByID(
	ctx context.Context,
	documentID string,
	versionID string,
) (model.DocumentVersion, error) {
	const query = `
SELECT
  id,
  document_id,
  version_no,
  title,
  content,
  tags,
  status,
  owner_name,
  event_type,
  created_by,
  created_at
FROM document_versions
WHERE document_id = $1
  AND id = $2;
`

	row := r.db.QueryRowContext(ctx, query, strings.TrimSpace(documentID), strings.TrimSpace(versionID))
	version, err := scanDocumentVersion(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.DocumentVersion{}, ErrNotFound
		}
		if errors.Is(err, ErrNotFound) {
			return model.DocumentVersion{}, ErrNotFound
		}
		return model.DocumentVersion{}, err
	}
	return version, nil
}

type scanVersionRow interface {
	Scan(dest ...any) error
}

func scanDocumentVersion(row scanVersionRow) (model.DocumentVersion, error) {
	var (
		item      model.DocumentVersion
		tagsRaw   []byte
		statusRaw string
		eventRaw  string
	)

	if err := row.Scan(
		&item.ID,
		&item.DocumentID,
		&item.Version,
		&item.Title,
		&item.Content,
		&tagsRaw,
		&statusRaw,
		&item.Owner,
		&eventRaw,
		&item.CreatedBy,
		&item.CreatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.DocumentVersion{}, ErrNotFound
		}
		return model.DocumentVersion{}, fmt.Errorf("scan document version: %w", err)
	}

	status := normalizeStatus(model.DocumentStatus(statusRaw))
	item.Status = status
	item.Event = normalizeVersionEvent(model.DocumentVersionEvent(eventRaw))
	item.CreatedBy = normalizeVersionOperator(item.CreatedBy)

	if len(tagsRaw) == 0 {
		item.Tags = []string{"未分类"}
		return item, nil
	}

	if err := json.Unmarshal(tagsRaw, &item.Tags); err != nil {
		return model.DocumentVersion{}, fmt.Errorf("unmarshal document version tags: %w", err)
	}
	item.Tags = normalizeTags(item.Tags)
	item.CreatedAt = item.CreatedAt.UTC()

	return item, nil
}

func normalizeVersionEvent(raw model.DocumentVersionEvent) model.DocumentVersionEvent {
	switch raw {
	case model.DocumentVersionEventCreate,
		model.DocumentVersionEventUpdate,
		model.DocumentVersionEventDelete,
		model.DocumentVersionEventRestore,
		model.DocumentVersionEventRollbackBackup:
		return raw
	default:
		return model.DocumentVersionEventUpdate
	}
}

func normalizeVersionOperator(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "system"
	}
	return trimmed
}
