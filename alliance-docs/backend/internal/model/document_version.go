package model

import "time"

type DocumentVersionEvent string

const (
	DocumentVersionEventCreate         DocumentVersionEvent = "create"
	DocumentVersionEventUpdate         DocumentVersionEvent = "update"
	DocumentVersionEventDelete         DocumentVersionEvent = "delete"
	DocumentVersionEventRestore        DocumentVersionEvent = "restore"
	DocumentVersionEventRollbackBackup DocumentVersionEvent = "rollback_backup"
)

type DocumentVersion struct {
	ID         string               `json:"id"`
	DocumentID string               `json:"documentId"`
	Version    int                  `json:"version"`
	Title      string               `json:"title"`
	Content    string               `json:"content"`
	Tags       []string             `json:"tags"`
	Status     DocumentStatus       `json:"status"`
	Owner      string               `json:"owner"`
	Event      DocumentVersionEvent `json:"event"`
	CreatedBy  string               `json:"createdBy"`
	CreatedAt  time.Time            `json:"createdAt"`
}
