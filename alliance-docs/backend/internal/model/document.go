package model

import "time"

type DocumentStatus string

const (
	DocumentStatusDraft     DocumentStatus = "草稿"
	DocumentStatusReviewing DocumentStatus = "评审中"
	DocumentStatusPublished DocumentStatus = "已发布"
)

type Document struct {
	ID        string         `json:"id"`
	Title     string         `json:"title"`
	Content   string         `json:"content"`
	Tags      []string       `json:"tags"`
	Status    DocumentStatus `json:"status"`
	Owner     string         `json:"owner"`
	CanEdit   bool           `json:"canEdit"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
}

type DocumentPermissionAccess string

const (
	DocumentPermissionRead DocumentPermissionAccess = "read"
	DocumentPermissionEdit DocumentPermissionAccess = "edit"
)

type DocumentPermission struct {
	DocumentID  string                   `json:"documentId"`
	UserID      string                   `json:"userId"`
	AccessLevel DocumentPermissionAccess `json:"accessLevel"`
	CreatedAt   time.Time                `json:"createdAt"`
	UpdatedAt   time.Time                `json:"updatedAt"`
}
