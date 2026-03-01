package model

import "time"

type Attachment struct {
	ID          string    `json:"id"`
	DocumentID  string    `json:"documentId"`
	ObjectKey   string    `json:"objectKey"`
	FileName    string    `json:"fileName"`
	ContentType string    `json:"contentType"`
	SizeBytes   int64     `json:"sizeBytes"`
	Owner       string    `json:"owner"`
	Storage     string    `json:"storage"`
	CreatedAt   time.Time `json:"createdAt"`
}
