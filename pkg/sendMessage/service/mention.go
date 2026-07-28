package send_service

import (
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
)

// participantMentionJID prefers the phone-number JID when available.
// Mentions require @s.whatsapp.net; participant.JID may be @lid.
func participantMentionJID(p types.GroupParticipant) string {
	if !p.PhoneNumber.IsEmpty() {
		return p.PhoneNumber.String()
	}
	return p.JID.String()
}

// setMessageMentionedJIDs writes ContextInfo.MentionedJID for the given message type.
// DocumentMessage may live under DocumentWithCaptionMessage when a caption is set.
func setMessageMentionedJIDs(msg *waE2E.Message, messageType string, mentionedJIDs []string) {
	if msg == nil || len(mentionedJIDs) == 0 {
		return
	}

	switch messageType {
	case "ExtendedTextMessage":
		if msg.ExtendedTextMessage == nil {
			return
		}
		if msg.ExtendedTextMessage.ContextInfo == nil {
			msg.ExtendedTextMessage.ContextInfo = &waE2E.ContextInfo{}
		}
		msg.ExtendedTextMessage.ContextInfo.MentionedJID = mentionedJIDs
	case "ImageMessage":
		if msg.ImageMessage == nil {
			return
		}
		if msg.ImageMessage.ContextInfo == nil {
			msg.ImageMessage.ContextInfo = &waE2E.ContextInfo{}
		}
		msg.ImageMessage.ContextInfo.MentionedJID = mentionedJIDs
	case "VideoMessage":
		if msg.VideoMessage == nil {
			return
		}
		if msg.VideoMessage.ContextInfo == nil {
			msg.VideoMessage.ContextInfo = &waE2E.ContextInfo{}
		}
		msg.VideoMessage.ContextInfo.MentionedJID = mentionedJIDs
	case "PtvMessage":
		if msg.PtvMessage == nil {
			return
		}
		if msg.PtvMessage.ContextInfo == nil {
			msg.PtvMessage.ContextInfo = &waE2E.ContextInfo{}
		}
		msg.PtvMessage.ContextInfo.MentionedJID = mentionedJIDs
	case "AudioMessage":
		if msg.AudioMessage == nil {
			return
		}
		if msg.AudioMessage.ContextInfo == nil {
			msg.AudioMessage.ContextInfo = &waE2E.ContextInfo{}
		}
		msg.AudioMessage.ContextInfo.MentionedJID = mentionedJIDs
	case "DocumentMessage":
		if msg.DocumentMessage != nil {
			if msg.DocumentMessage.ContextInfo == nil {
				msg.DocumentMessage.ContextInfo = &waE2E.ContextInfo{}
			}
			msg.DocumentMessage.ContextInfo.MentionedJID = mentionedJIDs
		} else if msg.DocumentWithCaptionMessage != nil &&
			msg.DocumentWithCaptionMessage.Message != nil &&
			msg.DocumentWithCaptionMessage.Message.DocumentMessage != nil {
			doc := msg.DocumentWithCaptionMessage.Message.DocumentMessage
			if doc.ContextInfo == nil {
				doc.ContextInfo = &waE2E.ContextInfo{}
			}
			doc.ContextInfo.MentionedJID = mentionedJIDs
		}
	case "PollCreationMessage":
		if msg.PollCreationMessage == nil {
			return
		}
		if msg.PollCreationMessage.ContextInfo == nil {
			msg.PollCreationMessage.ContextInfo = &waE2E.ContextInfo{}
		}
		msg.PollCreationMessage.ContextInfo.MentionedJID = mentionedJIDs
	case "StickerMessage":
		if msg.StickerMessage == nil {
			return
		}
		if msg.StickerMessage.ContextInfo == nil {
			msg.StickerMessage.ContextInfo = &waE2E.ContextInfo{}
		}
		msg.StickerMessage.ContextInfo.MentionedJID = mentionedJIDs
	case "LocationMessage":
		if msg.LocationMessage == nil {
			return
		}
		if msg.LocationMessage.ContextInfo == nil {
			msg.LocationMessage.ContextInfo = &waE2E.ContextInfo{}
		}
		msg.LocationMessage.ContextInfo.MentionedJID = mentionedJIDs
	case "ContactMessage":
		if msg.ContactMessage == nil {
			return
		}
		if msg.ContactMessage.ContextInfo == nil {
			msg.ContactMessage.ContextInfo = &waE2E.ContextInfo{}
		}
		msg.ContactMessage.ContextInfo.MentionedJID = mentionedJIDs
	}
}
