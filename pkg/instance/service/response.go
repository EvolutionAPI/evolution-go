package instance_service

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