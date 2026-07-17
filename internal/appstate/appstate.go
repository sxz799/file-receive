package appstate

import (
	"fmt"
	"sync"
	"time"

	"file-receive/internal/models"
)

// Progress 进度管理
type Progress struct {
	mu      sync.Mutex
	clients map[string]chan<- models.UploadProgress
	nextID  int
}

// AppState 应用状态
type AppState struct {
	mu           sync.Mutex
	records      []models.UploadRecord
	nextRecordID int
	Progress     *Progress
}

func NewProgress() *Progress {
	return &Progress{
		clients: make(map[string]chan<- models.UploadProgress),
		nextID:  1,
	}
}

func (p *Progress) AddClient() (string, <-chan models.UploadProgress) {
	p.mu.Lock()
	defer p.mu.Unlock()
	id := fmt.Sprintf("client-%d", p.nextID)
	p.nextID++
	ch := make(chan models.UploadProgress, 100)
	p.clients[id] = ch
	return id, ch
}

func (p *Progress) RemoveClient(id string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if ch, ok := p.clients[id]; ok {
		close(ch)
		delete(p.clients, id)
	}
}

func (p *Progress) Broadcast(progress models.UploadProgress) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, ch := range p.clients {
		select {
		case ch <- progress:
		default:
		}
	}
}

func NewAppState() *AppState {
	return &AppState{
		records:      make([]models.UploadRecord, 0),
		nextRecordID: 1,
		Progress:     NewProgress(),
	}
}

func (s *AppState) AddRecord(filename string, size int64, path string) models.UploadRecord {
	s.mu.Lock()
	defer s.mu.Unlock()

	record := models.UploadRecord{
		ID:         fmt.Sprintf("rec-%d", s.nextRecordID),
		Filename:   filename,
		Size:       size,
		Path:       path,
		UploadedAt: time.Now(),
	}

	s.nextRecordID++
	s.records = append(s.records, record)
	return record
}

func (s *AppState) GetRecords() []models.UploadRecord {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 返回副本，避免外部修改
	records := make([]models.UploadRecord, len(s.records))
	copy(records, s.records)
	return records
}
