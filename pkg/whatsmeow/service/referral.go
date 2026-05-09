package whatsmeow_service

import (
	"encoding/json"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"google.golang.org/protobuf/encoding/protojson"
)

func extractReferralFromMessage(message *waE2E.Message) json.RawMessage {
	contextInfo := getContextInfoFromMessage(message)
	if contextInfo == nil || contextInfo.GetExternalAdReply() == nil {
		return nil
	}

	referral, err := protojson.Marshal(contextInfo.GetExternalAdReply())
	if err != nil || len(referral) == 0 {
		return nil
	}

	return json.RawMessage(referral)
}

func getContextInfoFromMessage(message *waE2E.Message) *waE2E.ContextInfo {
	if message == nil {
		return nil
	}

	if extendedText := message.GetExtendedTextMessage(); extendedText != nil {
		return extendedText.GetContextInfo()
	}

	if image := message.GetImageMessage(); image != nil {
		return image.GetContextInfo()
	}

	if audio := message.GetAudioMessage(); audio != nil {
		return audio.GetContextInfo()
	}

	if document := message.GetDocumentMessage(); document != nil {
		return document.GetContextInfo()
	}

	if video := message.GetVideoMessage(); video != nil {
		return video.GetContextInfo()
	}

	return nil
}
