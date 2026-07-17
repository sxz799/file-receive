package models

import (
	"time"
)

// UploadRecord 上传记录
type UploadRecord struct {
	ID         string    `json:"id"`
	Filename   string    `json:"filename"`
	Size       int64     `json:"size"`
	Path       string    `json:"path"`
	UploadedAt time.Time `json:"uploaded_at"`
}

// UploadProgress 上传进度
type UploadProgress struct {
	Filename string  `json:"filename"`
	Total    int64   `json:"total"`
	Current  int64   `json:"current"`
	Percent  float64 `json:"percent"`
	Done     bool    `json:"done"`
	Error    string  `json:"error,omitempty"`
}
