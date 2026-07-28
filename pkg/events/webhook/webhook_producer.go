package webhook_producer

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	producer_interfaces "github.com/evolution-foundation/evolution-go/pkg/events/interfaces"
	logger_wrapper "github.com/evolution-foundation/evolution-go/pkg/logger"
)

type webhookProducer struct {
	url           string
	loggerWrapper *logger_wrapper.LoggerManager
}

func NewWebhookProducer(
	url string,
	loggerWrapper *logger_wrapper.LoggerManager,
) producer_interfaces.Producer {
	return &webhookProducer{
		url:           url,
		loggerWrapper: loggerWrapper,
	}
}

func (p *webhookProducer) Produce(
	queueName string,
	payload []byte,
	webhookUrl string,
	userID string,
) error {
	splitQueue := strings.Split(queueName, ".")

	if len(splitQueue) < 2 {
		return nil
	}

	if p.url != "" {
		go p.sendWebhookWithRetry(p.url, payload, 5, 30*time.Second, userID)
	}

	// Support multiple webhook URLs per instance. The instance Webhook field
	// may contain several URLs (separated by newline, comma or semicolon, or a
	// JSON array). The SAME request is delivered to each address. Fully
	// backward compatible with a single URL.
	for _, url := range splitWebhookURLs(webhookUrl) {
		u := url
		go p.sendWebhookWithRetry(u, payload, 5, 30*time.Second, userID)
	}

	return nil
}

// splitWebhookURLs splits the instance webhook string into one or more URLs.
// It accepts a JSON array (["https://a","https://b"]) OR a list separated by
// newline / comma / semicolon. It drops duplicates, empty entries and the
// "disabled" marker.
func splitWebhookURLs(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "disabled" {
		return nil
	}

	var parts []string
	if strings.HasPrefix(raw, "[") {
		var arr []string
		if err := json.Unmarshal([]byte(raw), &arr); err == nil {
			parts = arr
		}
	}
	if parts == nil {
		parts = strings.FieldsFunc(raw, func(r rune) bool {
			return r == '\n' || r == '\r' || r == ',' || r == ';'
		})
	}

	out := make([]string, 0, len(parts))
	seen := make(map[string]bool, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" || p == "disabled" || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	return out
}

func (p *webhookProducer) sendWebhookWithRetry(url string, body []byte, maxRetries int, retryInterval time.Duration, userID string) {
	for i := 0; i < maxRetries; i++ {
		err, responseBody, statusCode := p.sendWebhook(url, body, userID)
		if err == nil {
			p.loggerWrapper.GetLogger(userID).LogInfo("[%s] webhook sent successfully - url: %s, status: %d, response: %s", userID, url, statusCode, string(responseBody))
			return
		}
		p.loggerWrapper.GetLogger(userID).LogWarn("[%s] webhook failed - url: %s, attempt: %d, error: %v", userID, url, i+1, err)

		time.Sleep(retryInterval)
	}
	p.loggerWrapper.GetLogger(userID).LogError("[%s] webhook failed after maximum retries - url: %s", userID, url)
}

func (p *webhookProducer) sendWebhook(url string, body []byte, userID string) (error, []byte, int) {
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return err, nil, 0
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err, nil, 0
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("erro ao ler resposta: %v", err), nil, 0
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return errors.New("received non-2xx response: " + resp.Status), responseBody, resp.StatusCode
	}

	return nil, responseBody, resp.StatusCode
}

// CreateGlobalQueues não faz nada para webhook producer
func (p *webhookProducer) CreateGlobalQueues() error {
	return nil
}
