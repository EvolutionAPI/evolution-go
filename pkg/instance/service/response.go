package instance_service

import (
	instance_model "github.com/EvolutionAPI/evolution-go/pkg/instance/model"
)

type ConnectResponse struct {
	Jid         string `json:"jid"`
	WebhookUrl  string `json:"webhookUrl"`
	EventString string `json:"eventString"`
}

type SetProxyResponse struct {
	Host    string `json:"host"`
	Port    string `json:"port"`
	HasAuth bool   `json:"hasAuth"`
}

type UpdateAdvancedSettingsResponse struct {
	Message  string                            `json:"message"`
	Settings *instance_model.AdvancedSettings  `json:"settings"`
}