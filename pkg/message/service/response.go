package message_service

import (
	message_model "github.com/EvolutionAPI/evolution-go/pkg/message/model"
)

type TimestampResponse struct {
	Timestamp string `json:"timestamp"`
}

type DownloadMediaResponse struct {
	Base64    string `json:"base64"`
	Timestamp string `json:"timestamp"`
}

type MessageStatusResponse struct {
	Result    *message_model.Message `json:"result"`
	Timestamp string                 `json:"timestamp"`
}

type MessageIDResponse struct {
	MessageId string `json:"messageId"`
	Timestamp string `json:"timestamp"`
}