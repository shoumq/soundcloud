package service

import (
	"context"
	"sync"
	"time"

	"soundcloud/internal/domain"
	"soundcloud/internal/repository"
)

type ImportJobStatus string

const (
	ImportJobPending   ImportJobStatus = "pending"
	ImportJobRunning   ImportJobStatus = "running"
	ImportJobCompleted ImportJobStatus = "completed"
	ImportJobFailed    ImportJobStatus = "failed"
)

type AlbumImportJob struct {
	ID        string          `json:"id"`
	Type      string          `json:"type"`
	Status    ImportJobStatus `json:"status"`
	SourceURL string          `json:"source_url"`
	OwnerID   string          `json:"owner_id"`
	Album     *domain.Album   `json:"album,omitempty"`
	Error     string          `json:"error,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

type ImportJobService struct {
	albums *AlbumService

	mu   sync.RWMutex
	jobs map[string]AlbumImportJob
}

func NewImportJobService(albums *AlbumService) *ImportJobService {
	return &ImportJobService{
		albums: albums,
		jobs:   make(map[string]AlbumImportJob),
	}
}

func (s *ImportJobService) StartSoundCloudAlbumImport(ownerID, sourceURL string) AlbumImportJob {
	now := time.Now().UTC()
	job := AlbumImportJob{
		ID:        newID(),
		Type:      "soundcloud_album_import",
		Status:    ImportJobPending,
		SourceURL: sourceURL,
		OwnerID:   ownerID,
		CreatedAt: now,
		UpdatedAt: now,
	}

	s.mu.Lock()
	s.jobs[job.ID] = job
	s.mu.Unlock()

	go s.runSoundCloudAlbumImport(job.ID, ownerID, sourceURL)

	return job
}

func (s *ImportJobService) Get(jobID, ownerID string) (AlbumImportJob, error) {
	s.mu.RLock()
	job, ok := s.jobs[jobID]
	s.mu.RUnlock()
	if !ok {
		return AlbumImportJob{}, repository.ErrNotFound
	}
	if ownerID != "" && job.OwnerID != ownerID {
		return AlbumImportJob{}, repository.ErrNotFound
	}
	return job, nil
}

func (s *ImportJobService) runSoundCloudAlbumImport(jobID, ownerID, sourceURL string) {
	s.update(jobID, func(job *AlbumImportJob) {
		job.Status = ImportJobRunning
	})

	album, err := s.albums.ImportSoundCloud(context.Background(), ownerID, sourceURL)
	if err != nil {
		s.update(jobID, func(job *AlbumImportJob) {
			job.Status = ImportJobFailed
			job.Error = err.Error()
		})
		return
	}

	s.update(jobID, func(job *AlbumImportJob) {
		job.Status = ImportJobCompleted
		job.Album = &album
		job.Error = ""
	})
}

func (s *ImportJobService) update(jobID string, fn func(*AlbumImportJob)) {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, ok := s.jobs[jobID]
	if !ok {
		return
	}

	fn(&job)
	job.UpdatedAt = time.Now().UTC()
	s.jobs[jobID] = job
}
