// Package call_history persists the public lifecycle of WhatsApp calls.
package call_history

import (
	"log/slog"
	"time"

	call_runtime "github.com/evolution-foundation/evolution-go/pkg/call/runtime"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Record is a durable call-history entry. Private call keys, signaling nodes
// and media data are intentionally never stored here.
type Record struct {
	ID              uint       `json:"-" gorm:"primaryKey"`
	InstanceID      string     `json:"instanceId" gorm:"size:191;not null;uniqueIndex:idx_call_history_instance_call;index"`
	CallID          string     `json:"callId" gorm:"size:191;not null;uniqueIndex:idx_call_history_instance_call"`
	Peer            string     `json:"peer" gorm:"size:255"`
	Direction       string     `json:"direction" gorm:"size:16"`
	State           string     `json:"state" gorm:"size:16"`
	Video           bool       `json:"video"`
	StartedAt       time.Time  `json:"startedAt" gorm:"not null;index"`
	AnsweredAt      *time.Time `json:"answeredAt,omitempty"`
	EndedAt         *time.Time `json:"endedAt,omitempty"`
	DurationSeconds int64      `json:"durationSeconds"`
	EndReason       string     `json:"endReason,omitempty" gorm:"size:191"`
	Error           string     `json:"error,omitempty" gorm:"type:text"`
	AnsweredBy      string     `json:"answeredBy,omitempty" gorm:"size:191"`
	CreatedAt       time.Time  `json:"createdAt" gorm:"autoCreateTime"`
	UpdatedAt       time.Time  `json:"updatedAt" gorm:"autoUpdateTime"`
}

func (Record) TableName() string {
	return "call_history"
}

// Store is the subset consumed by the call HTTP service.
type Store interface {
	List(instanceID string, limit int) ([]Record, error)
}

type Service struct {
	db     *gorm.DB
	writes chan writeRequest
}

type writeRequest struct {
	entry Record
	flush chan struct{}
}

func NewService(db *gorm.DB) *Service {
	service := &Service{db: db}
	if db != nil {
		service.writes = make(chan writeRequest, 256)
		go service.persist()
	}
	return service
}

// Record queues lifecycle writes in order. The dedicated writer keeps a slower
// database out of the WhatsApp handler while ensuring an earlier ringing write
// cannot overwrite a later terminal state.
func (s *Service) Record(instanceID string, call call_runtime.Call) {
	if s == nil || s.db == nil || s.writes == nil || instanceID == "" || call.ID == "" {
		return
	}
	s.writes <- writeRequest{entry: entryFromCall(instanceID, call)}
}

func (s *Service) persist() {
	if s == nil || s.writes == nil {
		return
	}
	for request := range s.writes {
		if request.flush != nil {
			close(request.flush)
			continue
		}
		if err := s.Upsert(request.entry); err != nil {
			slog.Warn("persist WhatsApp call history", "instance", request.entry.InstanceID, "call_id", request.entry.CallID, "err", err)
		}
	}
}

func (s *Service) Upsert(entry Record) error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "instance_id"}, {Name: "call_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"peer", "direction", "state", "video", "answered_at", "ended_at",
			"duration_seconds", "end_reason", "error", "answered_by", "updated_at",
		}),
	}).Create(&entry).Error
}

func (s *Service) List(instanceID string, limit int) ([]Record, error) {
	if s == nil || s.db == nil || instanceID == "" {
		return []Record{}, nil
	}
	// A history request made immediately after accepting or ending a call must
	// include the lifecycle writes already queued by that request. The barrier
	// preserves the non-blocking event path while making this read consistent.
	if s.writes != nil {
		flushed := make(chan struct{})
		s.writes <- writeRequest{flush: flushed}
		<-flushed
	}
	if limit < 1 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	entries := make([]Record, 0)
	err := s.db.Where("instance_id = ?", instanceID).Order("started_at DESC").Limit(limit).Find(&entries).Error
	return entries, err
}

func entryFromCall(instanceID string, call call_runtime.Call) Record {
	startedAt := call.CreatedAt.UTC()
	if startedAt.IsZero() {
		startedAt = time.Now().UTC()
	}
	entry := Record{
		InstanceID: instanceID,
		CallID:     call.ID,
		Peer:       call.Peer,
		Direction:  string(call.Direction),
		State:      string(call.State),
		Video:      call.Video,
		StartedAt:  startedAt,
		EndReason:  call.EndReason,
		Error:      call.Error,
		AnsweredBy: call.AnsweredBy,
	}
	if call.AnsweredAt != nil {
		answeredAt := call.AnsweredAt.UTC()
		entry.AnsweredAt = &answeredAt
	}
	if call.State == call_runtime.StateEnded || call.State == call_runtime.StateFailed {
		endedAt := call.UpdatedAt.UTC()
		if endedAt.IsZero() {
			endedAt = time.Now().UTC()
		}
		entry.EndedAt = &endedAt
		if endedAt.After(startedAt) {
			entry.DurationSeconds = int64(endedAt.Sub(startedAt).Seconds())
		}
	}
	return entry
}
