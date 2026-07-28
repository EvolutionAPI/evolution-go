package send_service

import (
	"testing"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
)

func TestParticipantMentionJID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		p    types.GroupParticipant
		want string
	}{
		{
			name: "prefers_phone_number_over_lid",
			p: types.GroupParticipant{
				JID:         types.NewJID("123456789012345", types.HiddenUserServer),
				PhoneNumber: types.NewJID("5511999999999", types.DefaultUserServer),
			},
			want: "5511999999999@s.whatsapp.net",
		},
		{
			name: "falls_back_to_jid_when_phone_empty",
			p: types.GroupParticipant{
				JID: types.NewJID("5511888888888", types.DefaultUserServer),
			},
			want: "5511888888888@s.whatsapp.net",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := participantMentionJID(tt.p)
			if got != tt.want {
				t.Fatalf("participantMentionJID() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSetMessageMentionedJIDs(t *testing.T) {
	t.Parallel()

	mentioned := []string{"5511999999999@s.whatsapp.net", "5511888888888@s.whatsapp.net"}

	tests := []struct {
		name        string
		messageType string
		msg         *waE2E.Message
		wantFrom    func(msg *waE2E.Message) []string
	}{
		{
			name:        "doc_without_caption",
			messageType: "DocumentMessage",
			msg: &waE2E.Message{
				DocumentMessage: &waE2E.DocumentMessage{
					Caption: proto.String(""),
				},
			},
			wantFrom: func(msg *waE2E.Message) []string {
				return msg.DocumentMessage.ContextInfo.MentionedJID
			},
		},
		{
			name:        "doc_with_caption",
			messageType: "DocumentMessage",
			msg: &waE2E.Message{
				DocumentWithCaptionMessage: &waE2E.FutureProofMessage{
					Message: &waE2E.Message{
						DocumentMessage: &waE2E.DocumentMessage{
							Caption: proto.String("hello @all"),
						},
					},
				},
			},
			wantFrom: func(msg *waE2E.Message) []string {
				return msg.DocumentWithCaptionMessage.Message.DocumentMessage.ContextInfo.MentionedJID
			},
		},
		{
			name:        "image",
			messageType: "ImageMessage",
			msg: &waE2E.Message{
				ImageMessage: &waE2E.ImageMessage{
					Caption: proto.String("caption"),
				},
			},
			wantFrom: func(msg *waE2E.Message) []string {
				return msg.ImageMessage.ContextInfo.MentionedJID
			},
		},
		{
			name:        "video",
			messageType: "VideoMessage",
			msg: &waE2E.Message{
				VideoMessage: &waE2E.VideoMessage{
					Caption: proto.String("caption"),
				},
			},
			wantFrom: func(msg *waE2E.Message) []string {
				return msg.VideoMessage.ContextInfo.MentionedJID
			},
		},
		{
			name:        "extended_text",
			messageType: "ExtendedTextMessage",
			msg: &waE2E.Message{
				ExtendedTextMessage: &waE2E.ExtendedTextMessage{
					Text: proto.String("@todos"),
				},
			},
			wantFrom: func(msg *waE2E.Message) []string {
				return msg.ExtendedTextMessage.ContextInfo.MentionedJID
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			setMessageMentionedJIDs(tt.msg, tt.messageType, mentioned)
			got := tt.wantFrom(tt.msg)
			if len(got) != len(mentioned) {
				t.Fatalf("MentionedJID len = %d, want %d (got %#v)", len(got), len(mentioned), got)
			}
			for i := range mentioned {
				if got[i] != mentioned[i] {
					t.Fatalf("MentionedJID[%d] = %q, want %q", i, got[i], mentioned[i])
				}
			}
		})
	}
}

func TestSetMessageMentionedJIDs_DocWithCaptionDoesNotPanic(t *testing.T) {
	t.Parallel()

	// Reproduces the #114 failure mode: DocumentMessage is nil after wrap.
	msg := &waE2E.Message{
		DocumentWithCaptionMessage: &waE2E.FutureProofMessage{
			Message: &waE2E.Message{
				DocumentMessage: &waE2E.DocumentMessage{
					Caption: proto.String("doc caption"),
				},
			},
		},
	}

	setMessageMentionedJIDs(msg, "DocumentMessage", []string{"5511999999999@s.whatsapp.net"})

	got := msg.DocumentWithCaptionMessage.Message.DocumentMessage.ContextInfo.MentionedJID
	if len(got) != 1 || got[0] != "5511999999999@s.whatsapp.net" {
		t.Fatalf("unexpected MentionedJID: %#v", got)
	}
	if msg.DocumentMessage != nil {
		t.Fatal("DocumentMessage should remain nil for captioned documents")
	}
}
