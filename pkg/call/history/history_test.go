package call_history

import (
	"testing"
	"time"

	call_runtime "github.com/evolution-foundation/evolution-go/pkg/call/runtime"
)

func TestEntryFromCallKeepsOutcomeAndDuration(t *testing.T) {
	startedAt := time.Date(2026, time.August, 4, 12, 0, 0, 0, time.UTC)
	endedAt := startedAt.Add(95 * time.Second)
	answeredAt := startedAt.Add(5 * time.Second)
	entry := entryFromCall("instance-1", call_runtime.Call{
		ID:         "call-1",
		Peer:       "5511999999999@s.whatsapp.net",
		Direction:  call_runtime.DirectionIncoming,
		State:      call_runtime.StateEnded,
		CreatedAt:  startedAt,
		UpdatedAt:  endedAt,
		AnsweredAt: &answeredAt,
		AnsweredBy: "Jefferson",
		EndReason:  "user_ended",
	})

	if entry.DurationSeconds != 95 {
		t.Fatalf("unexpected duration: %d", entry.DurationSeconds)
	}
	if entry.AnsweredBy != "Jefferson" || entry.AnsweredAt == nil || !entry.AnsweredAt.Equal(answeredAt) {
		t.Fatalf("answer metadata was not persisted: %+v", entry)
	}
	if entry.EndedAt == nil || !entry.EndedAt.Equal(endedAt) {
		t.Fatalf("end metadata was not persisted: %+v", entry)
	}
}
