package send_service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	config "github.com/EvolutionAPI/evolution-go/pkg/config"
	instance_model "github.com/EvolutionAPI/evolution-go/pkg/instance/model"
	logger_wrapper "github.com/EvolutionAPI/evolution-go/pkg/logger"
	"github.com/EvolutionAPI/evolution-go/pkg/utils"
	whatsmeow_service "github.com/EvolutionAPI/evolution-go/pkg/whatsmeow/service"
	"github.com/chai2010/webp"
	"github.com/gabriel-vasile/mimetype"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"golang.org/x/net/html"
	"google.golang.org/protobuf/proto"
)

type SendService interface {
	SendText(data *TextStruct, instance *instance_model.Instance) (*MessageSendStruct, error)
	SendLink(data *LinkStruct, instance *instance_model.Instance) (*MessageSendStruct, error)
	SendMediaUrl(data *MediaStruct, instance *instance_model.Instance) (*MessageSendStruct, error)
	SendMediaFile(data *MediaStruct, fileData []byte, instance *instance_model.Instance) (*MessageSendStruct, error)
	SendPoll(data *PollStruct, instance *instance_model.Instance) (*MessageSendStruct, error)
	SendSticker(data *StickerStruct, instance *instance_model.Instance) (*MessageSendStruct, error)
	SendLocation(data *LocationStruct, instance *instance_model.Instance) (*MessageSendStruct, error)
	SendContact(data *ContactStruct, instance *instance_model.Instance) (*MessageSendStruct, error)
	SendButton(data *ButtonStruct, instance *instance_model.Instance) (*MessageSendStruct, error)
	SendList(data *ListStruct, instance *instance_model.Instance) (*MessageSendStruct, error)
}

type sendService struct {
	clientPointer    map[string]*whatsmeow.Client
	whatsmeowService whatsmeow_service.WhatsmeowService
	config           *config.Config
	loggerWrapper    *logger_wrapper.LoggerManager
}

type SendDataStruct struct {
	Id           string
	Number       string
	Delay        int32
	MentionAll   bool
	MentionedJID []string
	FormatJid    *bool
	Quoted       QuotedStruct
	MediaHandle  string
}

type QuotedStruct struct {
	MessageID   string `json:"messageId"`
	Participant string `json:"participant"`
}

type TextStruct struct {
	Number       string       `json:"number"`
	Text         string       `json:"text"`
	Id           string       `json:"id"`
	Delay        int32        `json:"delay"`
	MentionedJID []string     `json:"mentionedJid"`
	MentionAll   bool         `json:"mentionAll"`
	FormatJid    *bool        `json:"formatJid,omitempty"`
	Quoted       QuotedStruct `json:"quoted"`
}

type LinkStruct struct {
	Number       string       `json:"number"`
	Text         string       `json:"text"`
	Title        string       `json:"title"`
	Url          string       `json:"url"`
	Description  string       `json:"description"`
	ImgUrl       string       `json:"imgUrl"`
	Id           string       `json:"id"`
	Delay        int32        `json:"delay"`
	MentionedJID []string     `json:"mentionedJid"`
	MentionAll   bool         `json:"mentionAll"`
	FormatJid    *bool        `json:"formatJid,omitempty"`
	Quoted       QuotedStruct `json:"quoted"`
}

type MediaStruct struct {
	Number       string       `json:"number"`
	Url          string       `json:"url"`
	Type         string       `json:"type"`
	Caption      string       `json:"caption"`
	Filename     string       `json:"filename"`
	Id           string       `json:"id"`
	Delay        int32        `json:"delay"`
	MentionedJID []string     `json:"mentionedJid"`
	MentionAll   bool         `json:"mentionAll"`
	FormatJid    *bool        `json:"formatJid,omitempty"`
	Quoted       QuotedStruct `json:"quoted"`
}

type PollStruct struct {
	Id           string       `json:"id"`
	Number       string       `json:"number"`
	Question     string       `json:"question"`
	MaxAnswer    int          `json:"maxAnswer"`
	Options      []string     `json:"options"`
	Delay        int32        `json:"delay"`
	MentionedJID []string     `json:"mentionedJid"`
	MentionAll   bool         `json:"mentionAll"`
	FormatJid    *bool        `json:"formatJid,omitempty"`
	Quoted       QuotedStruct `json:"quoted"`
}

type StickerStruct struct {
	Number           string       `json:"number"`
	Sticker          string       `json:"sticker"`
	Id               string       `json:"id"`
	Delay            int32        `json:"delay"`
	MentionedJID     []string     `json:"mentionedJid"`
	MentionAll       bool         `json:"mentionAll"`
	FormatJid        *bool        `json:"formatJid,omitempty"`
	TransparentColor string       `json:"transparentColor,omitempty"`
	Quoted           QuotedStruct `json:"quoted"`
}

type LocationStruct struct {
	Number       string       `json:"number"`
	Id           string       `json:"id"`
	Name         string       `json:"name"`
	Latitude     float64      `json:"latitude"`
	Longitude    float64      `json:"longitude"`
	Address      string       `json:"address"`
	Delay        int32        `json:"delay"`
	MentionedJID []string     `json:"mentionedJid"`
	MentionAll   bool         `json:"mentionAll"`
	FormatJid    *bool        `json:"formatJid,omitempty"`
	Quoted       QuotedStruct `json:"quoted"`
}

type ContactStruct struct {
	Number       string            `json:"number"`
	Id           string            `json:"id"`
	Vcard        utils.VCardStruct `json:"vcard"`
	Delay        int32             `json:"delay"`
	MentionedJID []string          `json:"mentionedJid"`
	MentionAll   bool              `json:"mentionAll"`
	FormatJid    *bool             `json:"formatJid,omitempty"`
	Quoted       QuotedStruct      `json:"quoted"`
}

type Button struct {
	Type        string `json:"type"`
	DisplayText string `json:"displayText"`
	Id          string `json:"id"`
	CopyCode    string `json:"copyCode"`
	URL         string `json:"url"`
	PhoneNumber string `json:"phoneNumber"`
	Currency    string `json:"currency"`
	Name        string `json:"name"`
	KeyType     string `json:"keyType"`
	Key         string `json:"key"`
}

type ButtonStruct struct {
	Number       string       `json:"number"`
	Title        string       `json:"title"`
	Description  string       `json:"description"`
	Footer       string       `json:"footer"`
	Buttons      []Button     `json:"buttons"`
	Delay        int32        `json:"delay"`
	MentionedJID []string     `json:"mentionedJid"`
	MentionAll   bool         `json:"mentionAll"`
	FormatJid    *bool        `json:"formatJid,omitempty"`
	Quoted       QuotedStruct `json:"quoted"`
}

type Row struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	RowId       string `json:"rowId"`
}

type Section struct {
	Title string `json:"title"`
	Rows  []Row  `json:"rows"`
}

type ListStruct struct {
	Number       string       `json:"number"`
	Title        string       `json:"title"`
	Description  string       `json:"description"`
	ButtonText   string       `json:"buttonText"`
	FooterText   string       `json:"footerText"`
	Sections     []Section    `json:"sections"`
	Delay        int32        `json:"delay"`
	MentionedJID []string     `json:"mentionedJid"`
	MentionAll   bool         `json:"mentionAll"`
	FormatJid    *bool        `json:"formatJid,omitempty"`
	Quoted       QuotedStruct `json:"quoted"`
}

type CarouselButtonStruct struct {
	Type        string `json:"type"`
	DisplayText string `json:"displayText"`
	Id          string `json:"id"`
	CopyCode    string `json:"copyCode,omitempty"`
}

type CarouselCardHeaderStruct struct {
	Title    string `json:"title"`
	Subtitle string `json:"subtitle"`
	ImageUrl string `json:"imageUrl,omitempty"`
	VideoUrl string `json:"videoUrl,omitempty"`
}

type CarouselCardBodyStruct struct {
	Text string `json:"text"`
}

type CarouselCardStruct struct {
	Header  CarouselCardHeaderStruct `json:"header"`
	Body    CarouselCardBodyStruct   `json:"body"`
	Footer  string                   `json:"footer,omitempty"`
	Buttons []CarouselButtonStruct   `json:"buttons,omitempty"`
}

type CarouselStruct struct {
	Number    string             `json:"number"`
	Body      string             `json:"body,omitempty"`
	Footer    string             `json:"footer,omitempty"`
	Delay     int32              `json:"delay"`
	FormatJid *bool              `json:"formatJid,omitempty"`
	Quoted    QuotedStruct       `json:"quoted"`
	Cards     []CarouselCardStruct `json:"cards"`
}

type MessageSendStruct struct {
	Info               types.MessageInfo
	Message            *waE2E.Message
	MessageContextInfo *waE2E.ContextInfo
}

func (s *sendService) ensureClientConnected(instanceId string) (*whatsmeow.Client, error) {
	client := s.clientPointer[instanceId]
	s.loggerWrapper.GetLogger(instanceId).LogInfo("[%s] Checking client connection status - Client exists: %v", instanceId, client != nil)

	if client == nil {
		s.loggerWrapper.GetLogger(instanceId).LogInfo("[%s] No client found, attempting to start new instance", instanceId)
		err := s.whatsmeowService.StartInstance(instanceId)
		if err != nil {
			s.loggerWrapper.GetLogger(instanceId).LogError("[%s] Failed to start instance: %v", instanceId, err)
			return nil, errors.New("no active session found")
		}

		s.loggerWrapper.GetLogger(instanceId).LogInfo("[%s] Instance started, waiting 2 seconds...", instanceId)
		time.Sleep(2 * time.Second)

		client = s.clientPointer[instanceId]
		s.loggerWrapper.GetLogger(instanceId).LogInfo("[%s] Checking new client - Exists: %v, Connected: %v",
			instanceId,
			client != nil,
			client != nil && client.IsConnected())

		if client == nil || !client.IsConnected() {
			s.loggerWrapper.GetLogger(instanceId).LogError("[%s] New client validation failed - Exists: %v, Connected: %v",
				instanceId,
				client != nil,
				client != nil && client.IsConnected())
			return nil, errors.New("no active session found")
		}
	} else if !client.IsConnected() {
		s.loggerWrapper.GetLogger(instanceId).LogError("[%s] Existing client is disconnected - Connected status: %v",
			instanceId,
			client.IsConnected())
		return nil, errors.New("client disconnected")
	}

	s.loggerWrapper.GetLogger(instanceId).LogInfo("[%s] Client successfully validated - Connected: %v", instanceId, client.IsConnected())
	return client, nil
}

// ensureClientConnectedWithRetry attempts to ensure client connection with automatic reconnection and retry
func (s *sendService) ensureClientConnectedWithRetry(instanceId string, maxRetries int) (*whatsmeow.Client, error) {
	for attempt := 1; attempt <= maxRetries; attempt++ {
		s.loggerWrapper.GetLogger(instanceId).LogInfo("[%s] Connection attempt %d/%d", instanceId, attempt, maxRetries)

		client, err := s.ensureClientConnected(instanceId)
		if err == nil {
			return client, nil
		}

		// Check if it's a disconnection error that we can retry
		if err.Error() == "client disconnected" || err.Error() == "no active session found" {
			s.loggerWrapper.GetLogger(instanceId).LogWarn("[%s] Client disconnected on attempt %d/%d, attempting reconnection...", instanceId, attempt, maxRetries)

			// Attempt to reconnect the client
			reconnectErr := s.whatsmeowService.ReconnectClient(instanceId)
			if reconnectErr != nil {
				s.loggerWrapper.GetLogger(instanceId).LogError("[%s] Failed to reconnect client on attempt %d: %v", instanceId, attempt, reconnectErr)
			} else {
				s.loggerWrapper.GetLogger(instanceId).LogInfo("[%s] Reconnection initiated on attempt %d, waiting 3 seconds...", instanceId, attempt)
				time.Sleep(3 * time.Second)
			}

			// If this is not the last attempt, continue to retry
			if attempt < maxRetries {
				waitTime := time.Duration(attempt*2) * time.Second // Progressive backoff
				s.loggerWrapper.GetLogger(instanceId).LogInfo("[%s] Waiting %v before retry attempt %d", instanceId, waitTime, attempt+1)
				time.Sleep(waitTime)
				continue
			}
		}

		// If it's the last attempt or a non-retryable error, return the error
		s.loggerWrapper.GetLogger(instanceId).LogError("[%s] Failed to ensure client connection after %d attempts: %v", instanceId, attempt, err)
		return nil, err
	}

	return nil, fmt.Errorf("failed to connect client after %d attempts", maxRetries)
}

func validateMessageFields(phone string, formatJid *bool, messageID *string, participant *string) (types.JID, error) {
	// Apply formatting if formatJid is true (default)
	shouldFormat := true // Default value
	if formatJid != nil {
		shouldFormat = *formatJid
	}

	var finalPhone string
	if shouldFormat {
		// Extract raw number if it's already a JID, then apply CreateJID formatting
		rawNumber := phone
		if strings.Contains(phone, "@s.whatsapp.net") {
			rawNumber = strings.Split(phone, "@")[0]
		}

		normalizedJID, err := utils.CreateJID(rawNumber)
		if err != nil {
			// If CreateJID fails, try with ParseJID as fallback
			recipient, ok := utils.ParseJID(phone)
			if !ok {
				return types.NewJID("", types.DefaultUserServer), fmt.Errorf("could not parse phone: %s", phone)
			}
			finalPhone = recipient.String()
		} else {
			finalPhone = normalizedJID
		}
	} else {
		// Use phone as received without formatting
		finalPhone = phone
	}

	recipient, ok := utils.ParseJID(finalPhone)
	if !ok {
		return types.NewJID("", types.DefaultUserServer), errors.New("could not parse formatted phone")
	}

	if messageID != nil {
		if participant == nil {
			return types.NewJID("", types.DefaultUserServer), errors.New("missing Participant in ContextInfo")
		}
	}

	if participant != nil {
		if messageID == nil {
			return types.NewJID("", types.DefaultUserServer), errors.New("missing StanzaId in ContextInfo")
		}
	}

	return recipient, nil
}

// validateAndCheckUserExists validates message fields and checks if the user exists on WhatsApp
// Now uses the new approach: CheckUser with formatJid=false by default, and uses remoteJID for messaging
func (s *sendService) validateAndCheckUserExists(phone string, formatJid *bool, messageID *string, participant *string, instance *instance_model.Instance) (types.JID, error) {
	// Skip WhatsApp check if disabled in config
	if !s.config.CheckUserExists {
		s.loggerWrapper.GetLogger(instance.Id).LogDebug("[%s] User existence check disabled by configuration", instance.Id)
		// Use original validation logic when check is disabled
		return validateMessageFields(phone, formatJid, messageID, participant)
	}

	// Skip WhatsApp check for group messages, broadcast, newsletter, and LID
	if strings.Contains(phone, "@g.us") || strings.Contains(phone, "@broadcast") || strings.Contains(phone, "@newsletter") || strings.Contains(phone, "@lid") {
		return validateMessageFields(phone, formatJid, messageID, participant)
	}

	// Get the client to check if user exists on WhatsApp
	client, err := s.ensureClientConnected(instance.Id)
	if err != nil {
		return types.NewJID("", types.DefaultUserServer), fmt.Errorf("failed to connect client: %v", err)
	}

	// Use CheckUser approach: formatJid=false by default
	formatJidForCheck := false

	// First attempt with formatJid=false
	remoteJID, found, err := s.checkSingleUserExists(client, phone, formatJidForCheck, instance.Id)
	if err != nil {
		s.loggerWrapper.GetLogger(instance.Id).LogWarn("[%s] Failed to check user existence: %v", instance.Id, err)
		// Continue with sending even if check fails (network issues, etc.)
		return validateMessageFields(phone, formatJid, messageID, participant)
	}

	// If not found with formatJid=false, try with formatJid=true as fallback
	if !found {
		s.loggerWrapper.GetLogger(instance.Id).LogInfo("[%s] User not found with formatJid=false, trying with formatJid=true", instance.Id)
		remoteJIDRetry, foundRetry, errRetry := s.checkSingleUserExists(client, phone, true, instance.Id)
		if errRetry == nil && foundRetry {
			remoteJID = remoteJIDRetry
			found = foundRetry
		}
	}

	if !found {
		return types.NewJID("", types.DefaultUserServer), fmt.Errorf("number %s is not registered on WhatsApp", phone)
	}

	s.loggerWrapper.GetLogger(instance.Id).LogInfo("[%s] Number %s verified as valid WhatsApp user, using remoteJID: %s", instance.Id, phone, remoteJID)

	// Validate the remoteJID with formatJid=false for message sending
	formatJidFalse := false
	return validateMessageFields(remoteJID, &formatJidFalse, messageID, participant)
}

// checkSingleUserExists checks if a single user exists on WhatsApp with the specified formatJid setting
// Returns: remoteJID, found, error
func (s *sendService) checkSingleUserExists(client *whatsmeow.Client, phone string, formatJid bool, instanceId string) (string, bool, error) {
	phoneNumbers, err := utils.PrepareNumbersForWhatsAppCheck([]string{phone}, &formatJid)
	if err != nil {
		return "", false, fmt.Errorf("failed to prepare number for WhatsApp check: %v", err)
	}

	// Check if the number exists on WhatsApp
	resp, err := client.IsOnWhatsApp(context.Background(), phoneNumbers)
	if err != nil {
		return "", false, fmt.Errorf("failed to check if number %s exists on WhatsApp: %v", phoneNumbers[0], err)
	}

	// Verify if the number was found
	if len(resp) == 0 {
		return "", false, fmt.Errorf("number %s not found in WhatsApp response", phoneNumbers[0])
	}

	// Check if the first result indicates the number is on WhatsApp
	if !resp[0].IsIn {
		return "", false, nil // Not an error, just not found
	}

	// Return the remoteJID from WhatsApp's response
	remoteJID := fmt.Sprintf("%v", resp[0].JID)
	return remoteJID, true, nil
}

func findURL(text string) string {
	urlRegex := `http[s]?://(?:[a-zA-Z]|[0-9]|[$-_@.&+]|[!*\\(\\),]|(?:%[0-9a-fA-F][0-9a-fA-F]))+`
	re := regexp.MustCompile(urlRegex)
	urls := re.FindAllString(text, -1)
	if len(urls) > 0 {
		return urls[0]
	}
	return ""
}

func (s *sendService) SendText(data *TextStruct, instance *instance_model.Instance) (*MessageSendStruct, error) {
	return s.sendTextWithRetry(data, instance, 3) // 3 tentativas máximas
}

func (s *sendService) sendTextWithRetry(data *TextStruct, instance *instance_model.Instance, maxRetries int) (*MessageSendStruct, error) {
	for attempt := 1; attempt <= maxRetries; attempt++ {
		s.loggerWrapper.GetLogger(instance.Id).LogInfo("[%s] SendText attempt %d/%d", instance.Id, attempt, maxRetries)

		_, err := s.ensureClientConnectedWithRetry(instance.Id, 2)
		if err != nil {
			if attempt == maxRetries {
				return nil, err
			}
			continue
		}

		msg := &waE2E.Message{
			ExtendedTextMessage: &waE2E.ExtendedTextMessage{
				Text: &data.Text,
			},
		}

		message, err := s.SendMessage(instance, msg, "ExtendedTextMessage", &SendDataStruct{
			Id:           data.Id,
			Number:       data.Number,
			Quoted:       data.Quoted,
			Delay:        data.Delay,
			MentionAll:   data.MentionAll,
			MentionedJID: data.MentionedJID,
			FormatJid:    data.FormatJid,
		})

		if err != nil {
			// Check if it's a client disconnection error
			if strings.Contains(err.Error(), "client disconnected") || strings.Contains(err.Error(), "no active session") {
				s.loggerWrapper.GetLogger(instance.Id).LogWarn("[%s] SendText failed due to disconnection on attempt %d/%d: %v", instance.Id, attempt, maxRetries, err)
				if attempt < maxRetries {
					waitTime := time.Duration(attempt) * time.Second
					s.loggerWrapper.GetLogger(instance.Id).LogInfo("[%s] Waiting %v before retry", instance.Id, waitTime)
					time.Sleep(waitTime)
					continue
				}
			}
			return nil, err
		}

		s.loggerWrapper.GetLogger(instance.Id).LogInfo("[%s] SendText successful on attempt %d", instance.Id, attempt)
		return message, nil
	}

	return nil, fmt.Errorf("failed to send text after %d attempts", maxRetries)
}

func fetchLinkMetadata(url string) (string, string, string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", "", "", err
	}
	defer resp.Body.Close()

	doc, err := html.Parse(resp.Body)
	if err != nil {
		return "", "", "", err
	}

	var title, description, imgURL string

	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode {
			if n.Data == "title" && n.FirstChild != nil {
				title = n.FirstChild.Data
			}
			if n.Data == "meta" {
				var property, content string
				for _, attr := range n.Attr {
					if attr.Key == "property" || attr.Key == "name" {
						property = attr.Val
					}
					if attr.Key == "content" {
						content = attr.Val
					}
				}

				if (property == "description" || property == "og:description") && content != "" {
					description = content
				}

				if property == "og:image" && content != "" {
					imgURL = content
				}
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}

	f(doc)

	return title, description, imgURL, nil
}

func (s *sendService) SendLink(data *LinkStruct, instance *instance_model.Instance) (*MessageSendStruct, error) {
	return s.sendLinkWithRetry(data, instance, 3)
}

func (s *sendService) sendLinkWithRetry(data *LinkStruct, instance *instance_model.Instance, maxRetries int) (*MessageSendStruct, error) {
	for attempt := 1; attempt <= maxRetries; attempt++ {
		s.loggerWrapper.GetLogger(instance.Id).LogInfo("[%s] SendLink attempt %d/%d", instance.Id, attempt, maxRetries)

		_, err := s.ensureClientConnectedWithRetry(instance.Id, 2)
		if err != nil {
			if attempt == maxRetries {
				return nil, err
			}
			continue
		}

		matchedText := findURL(data.Text)

		if matchedText != "" {
			title, description, imgUrl, err := fetchLinkMetadata(matchedText)
			if err != nil {
				if attempt == maxRetries {
					return nil, err
				}
				continue
			}

			data.Title = title
			data.Description = description
			data.ImgUrl = imgUrl
		}

		var fileData []byte
		if data.ImgUrl != "" {
			resp, err := http.Get(data.ImgUrl)
			if err != nil {
				if attempt == maxRetries {
					return nil, err
				}
				continue
			}
			defer resp.Body.Close()
			fileData, _ = io.ReadAll(resp.Body)
		}

		previewType := waE2E.ExtendedTextMessage_VIDEO
		msg := &waE2E.Message{
			ExtendedTextMessage: &waE2E.ExtendedTextMessage{
				Text:          &data.Text,
				Title:         &data.Title,
				MatchedText:   &matchedText,
				JPEGThumbnail: fileData,
				Description:   &data.Description,
				PreviewType:   &previewType,
			},
		}

		message, err := s.SendMessage(instance, msg, "ExtendedTextMessage", &SendDataStruct{
			Id:           data.Id,
			Number:       data.Number,
			Quoted:       data.Quoted,
			Delay:        data.Delay,
			MentionAll:   data.MentionAll,
			MentionedJID: data.MentionedJID,
			FormatJid:    data.FormatJid,
		})

		if err != nil {
			// Check if it's a client disconnection error
			if strings.Contains(err.Error(), "client disconnected") || strings.Contains(err.Error(), "no active session") {
				s.loggerWrapper.GetLogger(instance.Id).LogWarn("[%s] SendLink failed due to disconnection on attempt %d/%d: %v", instance.Id, attempt, maxRetries, err)
				if attempt < maxRetries {
					waitTime := time.Duration(attempt) * time.Second
					s.loggerWrapper.GetLogger(instance.Id).LogInfo("[%s] Waiting %v before retry", instance.Id, waitTime)
					time.Sleep(waitTime)
					continue
				}
			}
			return nil, err
		}

		s.loggerWrapper.GetLogger(instance.Id).LogInfo("[%s] SendLink successful on attempt %d", instance.Id, attempt)
		return message, nil
	}

	return nil, fmt.Errorf("failed to send link after %d attempts", maxRetries)
}

type ConvertAudio struct {
	Url    string `json:"url,omitempty"`
	Base64 string `json:"base64,omitempty"`
}

type ApiResponse struct {
	Duration int    `json:"duration"`
	Audio    string `json:"audio"`
}

func convertAudioWithApi(apiUrl string, apiKey string, convertData ConvertAudio) ([]byte, int, error) {
	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)

	// Adiciona o campo "url" ao form-data se a URL for fornecida
	if convertData.Url != "" {
		err := writer.WriteField("url", convertData.Url)
		if err != nil {
			return nil, 0, fmt.Errorf("erro ao adicionar a URL no form-data: %v", err)
		}
	}

	// Adiciona o campo "base64" ao form-data se a string base64 for fornecida
	if convertData.Base64 != "" {
		err := writer.WriteField("base64", convertData.Base64)
		if err != nil {
			return nil, 0, fmt.Errorf("erro ao adicionar o base64 no form-data: %v", err)
		}
	}

	// Fecha o writer multipart
	err := writer.Close()
	if err != nil {
		return nil, 0, fmt.Errorf("erro ao finalizar o form-data: %v", err)
	}

	req, err := http.NewRequest("POST", apiUrl, &requestBody)
	if err != nil {
		return nil, 0, fmt.Errorf("erro ao criar a requisição: %v", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("apikey", apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("erro ao enviar a requisição: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, fmt.Errorf("erro ao ler a resposta: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, 0, fmt.Errorf("requisição falhou com status: %d, resposta: %s", resp.StatusCode, string(body))
	}

	var apiResponse ApiResponse
	err = json.Unmarshal(body, &apiResponse)
	if err != nil {
		return nil, 0, fmt.Errorf("erro ao deserializar a resposta: %v", err)
	}

	base64ToBytes, err := base64.StdEncoding.DecodeString(apiResponse.Audio)
	if err != nil {
		return nil, 0, fmt.Errorf("erro ao decodificar o áudio: %v", err)
	}

	return base64ToBytes, apiResponse.Duration, nil
}

func convertAudioToOpusWithDuration(inputData []byte) ([]byte, int, error) {
	cmd := exec.Command("ffmpeg", "-i", "pipe:0",
		"-f",
		"ogg",
		"-vn",
		"-c:a",
		"libopus",
		"-avoid_negative_ts",
		"make_zero",
		"-b:a",
		"128k",
		"-ar",
		"48000",
		"-ac",
		"1",
		"-write_xing",
		"0",
		"-compression_level",
		"10",
		"-application",
		"voip",
		"-fflags",
		"+bitexact",
		"-flags",
		"+bitexact",
		"-id3v2_version",
		"0",
		"-map_metadata",
		"-1",
		"-map_chapters",
		"-1",
		"-write_bext",
		"0",
		"pipe:1",
	)

	var outBuffer bytes.Buffer
	var errBuffer bytes.Buffer

	cmd.Stdin = bytes.NewReader(inputData)
	cmd.Stdout = &outBuffer
	cmd.Stderr = &errBuffer

	err := cmd.Run()
	if err != nil {
		return nil, 0, fmt.Errorf("error during conversion: %v, details: %s", err, errBuffer.String())
	}

	convertedData := outBuffer.Bytes()

	outputText := errBuffer.String()

	splitTime := strings.Split(outputText, "time=")

	if len(splitTime) < 2 {
		return nil, 0, errors.New("duração não encontrada")
	}

	// Use the last occurrence of time= in case there are multiple
	timeString := splitTime[len(splitTime)-1]

	re := regexp.MustCompile(`(\d+):(\d+):(\d+\.\d+)`)
	matches := re.FindStringSubmatch(timeString)
	if len(matches) != 4 {
		return nil, 0, errors.New("formato de duração não encontrado")
	}

	hours, _ := strconv.ParseFloat(matches[1], 64)
	minutes, _ := strconv.ParseFloat(matches[2], 64)
	seconds, _ := strconv.ParseFloat(matches[3], 64)
	duration := int(hours*3600 + minutes*60 + seconds)

	return convertedData, duration, nil
}

func (s *sendService) SendMediaFile(data *MediaStruct, fileData []byte, instance *instance_model.Instance) (*MessageSendStruct, error) {
	return s.sendMediaFileWithRetry(data, fileData, instance, 3)
}

func (s *sendService) sendMediaFileWithRetry(data *MediaStruct, fileData []byte, instance *instance_model.Instance, maxRetries int) (*MessageSendStruct, error) {
	for attempt := 1; attempt <= maxRetries; attempt++ {
		s.loggerWrapper.GetLogger(instance.Id).LogInfo("[%s] SendMediaFile attempt %d/%d", instance.Id, attempt, maxRetries)

		client, err := s.ensureClientConnectedWithRetry(instance.Id, 2)
		if err != nil {
			if attempt == maxRetries {
				return nil, err
			}
			continue
		}

		mime, _ := mimetype.DetectReader(bytes.NewReader(fileData))
		mimeType := mime.String()

		var uploadType whatsmeow.MediaType
		var duration int

		switch data.Type {
		case "image":
			if mimeType != "image/jpeg" && mimeType != "image/png" && mimeType != "image/webp" {
				errMsg := fmt.Sprintf("Invalid file format: '%s'. Only 'image/jpeg', 'image/png' and 'image/webp' are accepted", mimeType)
				return nil, errors.New(errMsg)
			}
			if mimeType == "image/webp" {
				mimeType = "image/jpeg"
			}
			uploadType = whatsmeow.MediaImage
		case "video":
			if mimeType != "video/mp4" {
				errMsg := fmt.Sprintf("Invalid file format: '%s'. Only 'video/mp4' is accepted", mimeType)
				return nil, errors.New(errMsg)
			}
			uploadType = whatsmeow.MediaVideo
		case "audio":
			converterApiUrl := s.config.ApiAudioConverter
			converterApiKey := s.config.ApiAudioConverterKey
			var convertedData []byte
			var err error
			if converterApiUrl == "" {

				convertedData, duration, err = convertAudioToOpusWithDuration(fileData)
				if err != nil {
					return nil, err
				}
			} else {
				convertedData, duration, err = convertAudioWithApi(converterApiUrl, converterApiKey, ConvertAudio{Base64: base64.StdEncoding.EncodeToString(fileData)})
				if err != nil {
					return nil, err
				}
			}

			fileData = convertedData
			mimeType = "audio/ogg; codecs=opus"
			uploadType = whatsmeow.MediaAudio
		case "document":
			uploadType = whatsmeow.MediaDocument
		default:
			return nil, errors.New("invalid media type")
		}

		// Detectar se é newsletter para usar upload sem criptografia
		isNewsletter := strings.Contains(data.Number, "@newsletter")

		// Validar se é documento em newsletter (não suportado)
		if isNewsletter && data.Type == "document" {
			return nil, errors.New("documentos não são suportados em canais do WhatsApp. Use imagem, vídeo, áudio ou enquete")
		}

		s.loggerWrapper.GetLogger(instance.Id).LogInfo("[%s] SendMediaFile - Upload iniciado (Newsletter: %v)...", instance.Id, isNewsletter)

		var uploaded whatsmeow.UploadResponse
		if isNewsletter {
			// Newsletter: upload SEM criptografia
			uploaded, err = client.UploadNewsletter(context.Background(), fileData, uploadType)
			s.loggerWrapper.GetLogger(instance.Id).LogInfo("[%s] Newsletter upload - Handle: %s", instance.Id, uploaded.Handle)
		} else {
			// Normal: upload COM criptografia
			uploaded, err = client.Upload(context.Background(), fileData, uploadType)
		}

		if err != nil {
			return nil, err
		}

		s.loggerWrapper.GetLogger(instance.Id).LogInfo("[%s] Media uploaded with size %d", instance.Id, uploaded.FileLength)

		var media *waE2E.Message
		var mediaType string

		switch data.Type {
		case "image":
			if isNewsletter {
				// Newsletter: SEM MediaKey e FileEncSHA256
				media = &waE2E.Message{ImageMessage: &waE2E.ImageMessage{
					Caption:    proto.String(data.Caption),
					URL:        &uploaded.URL,
					DirectPath: &uploaded.DirectPath,
					Mimetype:   proto.String(mimeType),
					FileSHA256: uploaded.FileSHA256,
					FileLength: &uploaded.FileLength,
				}}
			} else {
				// Normal: COM MediaKey e FileEncSHA256
				media = &waE2E.Message{ImageMessage: &waE2E.ImageMessage{
					Caption:       proto.String(data.Caption),
					URL:           proto.String(uploaded.URL),
					DirectPath:    proto.String(uploaded.DirectPath),
					MediaKey:      uploaded.MediaKey,
					Mimetype:      proto.String(mimeType),
					FileEncSHA256: uploaded.FileEncSHA256,
					FileSHA256:    uploaded.FileSHA256,
					FileLength:    proto.Uint64(uint64(len(fileData))),
				}}
			}
			mediaType = "ImageMessage"
		case "video":
			if isNewsletter {
				media = &waE2E.Message{VideoMessage: &waE2E.VideoMessage{
					Caption:    proto.String(data.Caption),
					URL:        &uploaded.URL,
					DirectPath: &uploaded.DirectPath,
					Mimetype:   proto.String(mimeType),
					FileSHA256: uploaded.FileSHA256,
					FileLength: &uploaded.FileLength,
				}}
			} else {
				media = &waE2E.Message{VideoMessage: &waE2E.VideoMessage{
					Caption:       proto.String(data.Caption),
					URL:           proto.String(uploaded.URL),
					DirectPath:    proto.String(uploaded.DirectPath),
					MediaKey:      uploaded.MediaKey,
					Mimetype:      proto.String(mimeType),
					FileEncSHA256: uploaded.FileEncSHA256,
					FileSHA256:    uploaded.FileSHA256,
					FileLength:    proto.Uint64(uint64(len(fileData))),
				}}
			}
			mediaType = "VideoMessage"
		case "ptv":
			if isNewsletter {
				media = &waE2E.Message{PtvMessage: &waE2E.VideoMessage{
					URL:        &uploaded.URL,
					DirectPath: &uploaded.DirectPath,
					Mimetype:   proto.String(mimeType),
					FileSHA256: uploaded.FileSHA256,
					FileLength: &uploaded.FileLength,
				}}
			} else {
				media = &waE2E.Message{PtvMessage: &waE2E.VideoMessage{
					URL:           proto.String(uploaded.URL),
					DirectPath:    proto.String(uploaded.DirectPath),
					MediaKey:      uploaded.MediaKey,
					Mimetype:      proto.String(mimeType),
					FileEncSHA256: uploaded.FileEncSHA256,
					FileSHA256:    uploaded.FileSHA256,
					FileLength:    proto.Uint64(uint64(len(fileData))),
				}}
			}
			mediaType = "PtvMessage"
		case "audio":
			if isNewsletter {
				media = &waE2E.Message{AudioMessage: &waE2E.AudioMessage{
					URL:        &uploaded.URL,
					PTT:        proto.Bool(true),
					DirectPath: &uploaded.DirectPath,
					Mimetype:   proto.String(mimeType),
					FileSHA256: uploaded.FileSHA256,
					FileLength: &uploaded.FileLength,
					Seconds:    proto.Uint32(uint32(duration)),
				}}
			} else {
				media = &waE2E.Message{AudioMessage: &waE2E.AudioMessage{
					URL:           proto.String(uploaded.URL),
					PTT:           proto.Bool(true),
					DirectPath:    proto.String(uploaded.DirectPath),
					MediaKey:      uploaded.MediaKey,
					Mimetype:      proto.String(mimeType),
					FileEncSHA256: uploaded.FileEncSHA256,
					FileSHA256:    uploaded.FileSHA256,
					FileLength:    proto.Uint64(uploaded.FileLength),
					Seconds:       proto.Uint32(uint32(duration)),
				}}
			}
			mediaType = "AudioMessage"
		case "document":
			if isNewsletter {
				media = &waE2E.Message{DocumentMessage: &waE2E.DocumentMessage{
					FileName:   &data.Filename,
					Caption:    proto.String(data.Caption),
					URL:        &uploaded.URL,
					DirectPath: &uploaded.DirectPath,
					Mimetype:   proto.String(mimeType),
					FileSHA256: uploaded.FileSHA256,
					FileLength: &uploaded.FileLength,
				}}
			} else {
				media = &waE2E.Message{DocumentMessage: &waE2E.DocumentMessage{
					FileName:      &data.Filename,
					Caption:       proto.String(data.Caption),
					URL:           proto.String(uploaded.URL),
					DirectPath:    proto.String(uploaded.DirectPath),
					MediaKey:      uploaded.MediaKey,
					Mimetype:      proto.String(mimeType),
					FileEncSHA256: uploaded.FileEncSHA256,
					FileSHA256:    uploaded.FileSHA256,
					FileLength:    proto.Uint64(uint64(len(fileData))),
				}}
			}

			if media.GetDocumentMessage().GetCaption() != "" {
				media.DocumentWithCaptionMessage = &waE2E.FutureProofMessage{
					Message: &waE2E.Message{
						DocumentMessage: media.DocumentMessage,
					},
				}
				media.DocumentMessage = nil
			}

			mediaType = "DocumentMessage"
		default:
			return nil, errors.New("invalid media type")
		}

		message, err := s.SendMessage(instance, media, mediaType, &SendDataStruct{
			Id:           data.Id,
			Number:       data.Number,
			Quoted:       data.Quoted,
			Delay:        data.Delay,
			MentionAll:   data.MentionAll,
			MentionedJID: data.MentionedJID,
			FormatJid:    data.FormatJid,
			MediaHandle:  uploaded.Handle,
		})

		if err != nil {
			// Check if it's a client disconnection error
			if strings.Contains(err.Error(), "client disconnected") || strings.Contains(err.Error(), "no active session") {
				s.loggerWrapper.GetLogger(instance.Id).LogWarn("[%s] SendMediaFile failed due to disconnection on attempt %d/%d: %v", instance.Id, attempt, maxRetries, err)
				if attempt < maxRetries {
					waitTime := time.Duration(attempt) * time.Second
					s.loggerWrapper.GetLogger(instance.Id).LogInfo("[%s] Waiting %v before retry", instance.Id, waitTime)
					time.Sleep(waitTime)
					continue
				}
			}
			return nil, err
		}

		s.loggerWrapper.GetLogger(instance.Id).LogInfo("[%s] SendMediaFile successful on attempt %d", instance.Id, attempt)
		return message, nil
	}

	return nil, fmt.Errorf("failed to send media file after %d attempts", maxRetries)
}

func (s *sendService) SendMediaUrl(data *MediaStruct, instance *instance_model.Instance) (*MessageSendStruct, error) {
	return s.sendMediaUrlWithRetry(data, instance, 3)
}

func (s *sendService) sendMediaUrlWithRetry(data *MediaStruct, instance *instance_model.Instance, maxRetries int) (*MessageSendStruct, error) {
	for attempt := 1; attempt <= maxRetries; attempt++ {
		s.loggerWrapper.GetLogger(instance.Id).LogInfo("[%s] SendMediaUrl attempt %d/%d for URL: %s", instance.Id, attempt, maxRetries, data.Url)
		startTime := time.Now()

		client, err := s.ensureClientConnectedWithRetry(instance.Id, 2)
		if err != nil {
			if attempt == maxRetries {
				return nil, err
			}
			continue
		}

		s.loggerWrapper.GetLogger(instance.Id).LogInfo("[%s] Iniciando download da URL: %s", instance.Id, data.Url)

		resp, err := http.Get(data.Url)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		s.loggerWrapper.GetLogger(instance.Id).LogInfo("[%s] Download concluído em %v. Lendo dados...", instance.Id, time.Since(startTime))

		downloadStart := time.Now()
		fileData, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		s.loggerWrapper.GetLogger(instance.Id).LogInfo("[%s] Leitura dos dados concluída em %v. Tamanho: %d bytes", instance.Id, time.Since(downloadStart), len(fileData))

		mime, _ := mimetype.DetectReader(bytes.NewReader(fileData))
		mimeType := mime.String()
		if strings.HasSuffix(strings.ToLower(data.Url), ".mp4") {
			mimeType = "video/mp4"
		}

		s.loggerWrapper.GetLogger(instance.Id).LogInfo("[%s] Tipo MIME detectado: %s", instance.Id, mimeType)

		var uploadType whatsmeow.MediaType
		var duration int

		processingStart := time.Now()
		switch data.Type {
		case "image":
			if mimeType != "image/jpeg" && mimeType != "image/png" && mimeType != "image/webp" {
				errMsg := fmt.Sprintf("Invalid file format: '%s'. Only 'image/jpeg', 'image/png' and 'image/webp' are accepted", mimeType)
				return nil, errors.New(errMsg)
			}
			if mimeType == "image/webp" {
				mimeType = "image/jpeg"
			}
			uploadType = whatsmeow.MediaImage

		case "video", "ptv":
			if mimeType != "video/mp4" {
				errMsg := fmt.Sprintf("Invalid file format: '%s'. Only 'video/mp4' are accepted", mimeType)
				return nil, errors.New(errMsg)
			}
			uploadType = whatsmeow.MediaVideo
		case "audio":
			s.loggerWrapper.GetLogger(instance.Id).LogInfo("[%s] Iniciando conversão de áudio...", instance.Id)
			converterApiUrl := s.config.ApiAudioConverter
			converterApiKey := s.config.ApiAudioConverterKey
			var convertedData []byte
			var err error
			if converterApiUrl == "" {
				s.loggerWrapper.GetLogger(instance.Id).LogInfo("[%s] Usando conversão local...", instance.Id)
				convertedData, duration, err = convertAudioToOpusWithDuration(fileData)
			} else {
				s.loggerWrapper.GetLogger(instance.Id).LogInfo("[%s] Usando API de conversão...", instance.Id)
				convertedData, duration, err = convertAudioWithApi(converterApiUrl, converterApiKey, ConvertAudio{Base64: base64.StdEncoding.EncodeToString(fileData)})
			}
			if err != nil {
				return nil, err
			}
			fileData = convertedData
			mimeType = "audio/ogg; codecs=opus"
			uploadType = whatsmeow.MediaAudio
			s.loggerWrapper.GetLogger(instance.Id).LogInfo("[%s] Conversão de áudio concluída em %v", instance.Id, time.Since(processingStart))
		case "document":
			uploadType = whatsmeow.MediaDocument
		default:
			return nil, errors.New("invalid media type")
		}

		// Detectar se é newsletter para usar upload sem criptografia
		isNewsletter := strings.Contains(data.Number, "@newsletter")

		// Validar se é documento em newsletter (não suportado)
		if isNewsletter && data.Type == "document" {
			return nil, errors.New("documentos não são suportados em canais do WhatsApp. Use imagem, vídeo, áudio ou enquete")
		}

		s.loggerWrapper.GetLogger(instance.Id).LogInfo("[%s] Iniciando upload para WhatsApp (Newsletter: %v)...", instance.Id, isNewsletter)
		uploadStart := time.Now()

		var uploaded whatsmeow.UploadResponse
		if isNewsletter {
			// Newsletter: upload sem criptografia
			uploaded, err = client.UploadNewsletter(context.Background(), fileData, uploadType)
			s.loggerWrapper.GetLogger(instance.Id).LogInfo("[%s] Newsletter upload - Handle: %s", instance.Id, uploaded.Handle)
		} else {
			// Upload normal com criptografia
			uploaded, err = client.Upload(context.Background(), fileData, uploadType)
		}

		if err != nil {
			return nil, err
		}
		s.loggerWrapper.GetLogger(instance.Id).LogInfo("[%s] Upload concluído em %v. Tamanho: %d", instance.Id, time.Since(uploadStart), uploaded.FileLength)

		var media *waE2E.Message
		var mediaType string

		switch data.Type {
		case "image":
			if isNewsletter {
				// Newsletter: sem criptografia (sem MediaKey e FileEncSHA256)
				media = &waE2E.Message{ImageMessage: &waE2E.ImageMessage{
					Caption:    proto.String(data.Caption),
					URL:        &uploaded.URL,
					DirectPath: &uploaded.DirectPath,
					Mimetype:   proto.String(mimeType),
					FileSHA256: uploaded.FileSHA256,
					FileLength: &uploaded.FileLength,
				}}
			} else {
				// Normal: com criptografia
				media = &waE2E.Message{ImageMessage: &waE2E.ImageMessage{
					Caption:       proto.String(data.Caption),
					URL:           proto.String(uploaded.URL),
					DirectPath:    proto.String(uploaded.DirectPath),
					MediaKey:      uploaded.MediaKey,
					Mimetype:      proto.String(mimeType),
					FileEncSHA256: uploaded.FileEncSHA256,
					FileSHA256:    uploaded.FileSHA256,
					FileLength:    proto.Uint64(uint64(len(fileData))),
				}}
			}
			mediaType = "ImageMessage"
		case "video":
			if isNewsletter {
				media = &waE2E.Message{VideoMessage: &waE2E.VideoMessage{
					Caption:    proto.String(data.Caption),
					URL:        &uploaded.URL,
					DirectPath: &uploaded.DirectPath,
					Mimetype:   proto.String(mimeType),
					FileSHA256: uploaded.FileSHA256,
					FileLength: &uploaded.FileLength,
				}}
			} else {
				media = &waE2E.Message{VideoMessage: &waE2E.VideoMessage{
					Caption:       proto.String(data.Caption),
					URL:           proto.String(uploaded.URL),
					DirectPath:    proto.String(uploaded.DirectPath),
					MediaKey:      uploaded.MediaKey,
					Mimetype:      proto.String(mimeType),
					FileEncSHA256: uploaded.FileEncSHA256,
					FileSHA256:    uploaded.FileSHA256,
					FileLength:    proto.Uint64(uint64(len(fileData))),
				}}
			}
			mediaType = "VideoMessage"
		case "ptv":
			if isNewsletter {
				media = &waE2E.Message{PtvMessage: &waE2E.VideoMessage{
					URL:        &uploaded.URL,
					DirectPath: &uploaded.DirectPath,
					Mimetype:   proto.String(mimeType),
					FileSHA256: uploaded.FileSHA256,
					FileLength: &uploaded.FileLength,
				}}
			} else {
				media = &waE2E.Message{PtvMessage: &waE2E.VideoMessage{
					URL:           proto.String(uploaded.URL),
					DirectPath:    proto.String(uploaded.DirectPath),
					MediaKey:      uploaded.MediaKey,
					Mimetype:      proto.String(mimeType),
					FileEncSHA256: uploaded.FileEncSHA256,
					FileSHA256:    uploaded.FileSHA256,
					FileLength:    proto.Uint64(uint64(len(fileData))),
				}}
			}
			mediaType = "PtvMessage"
		case "audio":
			if isNewsletter {
				media = &waE2E.Message{AudioMessage: &waE2E.AudioMessage{
					URL:              &uploaded.URL,
					PTT:              proto.Bool(true),
					DirectPath:       &uploaded.DirectPath,
					Mimetype:         proto.String(mimeType),
					FileSHA256:       uploaded.FileSHA256,
					FileLength:       &uploaded.FileLength,
					StreamingSidecar: []byte(*proto.String("QpmXDsU7YLagdg==")),
					Waveform:         []byte(*proto.String("OjAnExISDgsKCAkJBwgkHAQEBBEFAwMNAxAcKCgkFzM0QUE4Jh4eKAoKChcLCwkeFgkJCQo3JiQmIiIRPz8/Ow==")),
					Seconds:          proto.Uint32(uint32(duration)),
				}}
			} else {
				media = &waE2E.Message{AudioMessage: &waE2E.AudioMessage{
					URL:              proto.String(uploaded.URL),
					PTT:              proto.Bool(true),
					DirectPath:       proto.String(uploaded.DirectPath),
					MediaKey:         uploaded.MediaKey,
					Mimetype:         proto.String(mimeType),
					FileEncSHA256:    uploaded.FileEncSHA256,
					FileSHA256:       uploaded.FileSHA256,
					FileLength:       proto.Uint64(uploaded.FileLength),
					StreamingSidecar: []byte(*proto.String("QpmXDsU7YLagdg==")),
					Waveform:         []byte(*proto.String("OjAnExISDgsKCAkJBwgkHAQEBBEFAwMNAxAcKCgkFzM0QUE4Jh4eKAoKChcLCwkeFgkJCQo3JiQmIiIRPz8/Ow==")),
					Seconds:          proto.Uint32(uint32(duration)),
				}}
			}
			mediaType = "AudioMessage"
		case "document":
			if isNewsletter {
				media = &waE2E.Message{DocumentMessage: &waE2E.DocumentMessage{
					URL:        &uploaded.URL,
					FileName:   &data.Filename,
					Caption:    proto.String(data.Caption),
					DirectPath: &uploaded.DirectPath,
					Mimetype:   proto.String(mimeType),
					FileSHA256: uploaded.FileSHA256,
					FileLength: &uploaded.FileLength,
				}}
			} else {
				media = &waE2E.Message{DocumentMessage: &waE2E.DocumentMessage{
					URL:           proto.String(uploaded.URL),
					FileName:      &data.Filename,
					Caption:       proto.String(data.Caption),
					DirectPath:    proto.String(uploaded.DirectPath),
					MediaKey:      uploaded.MediaKey,
					Mimetype:      proto.String(mimeType),
					FileEncSHA256: uploaded.FileEncSHA256,
					FileSHA256:    uploaded.FileSHA256,
					FileLength:    proto.Uint64(uint64(len(fileData))),
				}}
			}

			if media.GetDocumentMessage().GetCaption() != "" {
				media.DocumentWithCaptionMessage = &waE2E.FutureProofMessage{
					Message: &waE2E.Message{
						DocumentMessage: media.DocumentMessage,
					},
				}
				media.DocumentMessage = nil
			}

			mediaType = "DocumentMessage"
		default:
			return nil, errors.New("invalid media type")
		}

		messageStart := time.Now()
		message, err := s.SendMessage(instance, media, mediaType, &SendDataStruct{
			Id:           data.Id,
			Number:       data.Number,
			Quoted:       data.Quoted,
			Delay:        data.Delay,
			MentionAll:   data.MentionAll,
			MentionedJID: data.MentionedJID,
			FormatJid:    data.FormatJid,
			MediaHandle:  uploaded.Handle,
		})

		if err != nil {
			// Check if it's a client disconnection error
			if strings.Contains(err.Error(), "client disconnected") || strings.Contains(err.Error(), "no active session") {
				s.loggerWrapper.GetLogger(instance.Id).LogWarn("[%s] SendMediaUrl failed due to disconnection on attempt %d/%d: %v", instance.Id, attempt, maxRetries, err)
				if attempt < maxRetries {
					waitTime := time.Duration(attempt) * time.Second
					s.loggerWrapper.GetLogger(instance.Id).LogInfo("[%s] Waiting %v before retry", instance.Id, waitTime)
					time.Sleep(waitTime)
					continue
				}
			}
			return nil, err
		}

		s.loggerWrapper.GetLogger(instance.Id).LogInfo("[%s] Mensagem enviada em %v", instance.Id, time.Since(messageStart))

		totalTime := time.Since(startTime)
		s.loggerWrapper.GetLogger(instance.Id).LogInfo("[%s] SendMediaUrl successful on attempt %d, processo completo em %v", instance.Id, attempt, totalTime)

		return message, nil
	}

	return nil, fmt.Errorf("failed to send media url after %d attempts", maxRetries)
}

func (s *sendService) SendPoll(data *PollStruct, instance *instance_model.Instance) (*MessageSendStruct, error) {
	return s.sendPollWithRetry(data, instance, 3)
}

func (s *sendService) sendPollWithRetry(data *PollStruct, instance *instance_model.Instance, maxRetries int) (*MessageSendStruct, error) {
	for attempt := 1; attempt <= maxRetries; attempt++ {
		s.loggerWrapper.GetLogger(instance.Id).LogInfo("[%s] SendPoll attempt %d/%d", instance.Id, attempt, maxRetries)

		client, err := s.ensureClientConnectedWithRetry(instance.Id, 2)
		if err != nil {
			if attempt == maxRetries {
				return nil, err
			}
			continue
		}

		msg := client.BuildPollCreation(data.Question, data.Options, data.MaxAnswer)

		message, err := s.SendMessage(instance, msg, "PollCreationMessage", &SendDataStruct{
			Id:           data.Id,
			Number:       data.Number,
			Quoted:       data.Quoted,
			Delay:        data.Delay,
			MentionAll:   data.MentionAll,
			MentionedJID: data.MentionedJID,
			FormatJid:    data.FormatJid,
		})

		if err != nil {
			// Check if it's a client disconnection error
			if strings.Contains(err.Error(), "client disconnected") || strings.Contains(err.Error(), "no active session") {
				s.loggerWrapper.GetLogger(instance.Id).LogWarn("[%s] SendPoll failed due to disconnection on attempt %d/%d: %v", instance.Id, attempt, maxRetries, err)
				if attempt < maxRetries {
					waitTime := time.Duration(attempt) * time.Second
					s.loggerWrapper.GetLogger(instance.Id).LogInfo("[%s] Waiting %v before retry", instance.Id, waitTime)
					time.Sleep(waitTime)
					continue
				}
			}
			return nil, err
		}

		s.loggerWrapper.GetLogger(instance.Id).LogInfo("[%s] SendPoll successful on attempt %d", instance.Id, attempt)
		return message, nil
	}

	return nil, fmt.Errorf("failed to send poll after %d attempts", maxRetries)
}

func convertVideoToWebP(inputData []byte, transparentColor string) ([]byte, error) {
	tmpInput, err := os.CreateTemp("", "sticker-input-*.mp4")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpInput.Name())

	if _, err := tmpInput.Write(inputData); err != nil {
		tmpInput.Close()
		return nil, fmt.Errorf("failed to write to temp file: %v", err)
	}

	if err := tmpInput.Close(); err != nil {
		return nil, fmt.Errorf("failed to close temp file: %v", err)
	}

	tmpOutput := tmpInput.Name() + ".webp"
	defer os.Remove(tmpOutput)

	// Filtros base: scale, pad, fps e loop
	baseFilters := "fps=15,scale=512:512:force_original_aspect_ratio=decrease,pad=512:512:(ow-iw)/2:(oh-ih)/2:color=0x00000000"

	filters := baseFilters
	if transparentColor != "" {
		cleanHex := strings.ReplaceAll(transparentColor, "#", "")
		filters = fmt.Sprintf("colorkey=0x%s:0.1:0.0,%s", cleanHex, baseFilters)
	}

	cmd := exec.Command("ffmpeg",
		"-i", tmpInput.Name(),
		"-vcodec", "libwebp",
		"-filter:v", filters,
		"-lossless", "0",
		"-compression_level", "4",
		"-q:v", "50",
		"-loop", "0",
		"-an",
		"-f", "webp",
		tmpOutput,
	)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffmpeg failed: %v, output: %s", err, stderr.String())
	}

	webpData, err := os.ReadFile(tmpOutput)
	if err != nil {
		return nil, fmt.Errorf("failed to read generated webp: %v", err)
	}

	return webpData, nil
}

func convertToWebP(imageDataURL string, transparentColor string) ([]byte, error) {
	resp, err := http.Get(imageDataURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch from URL: %v", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	mime := mimetype.Detect(data)

	if mime.Is("image/webp") {
		return data, nil
	} else if mime.Is("video/mp4") || mime.Is("image/gif") {
		return convertVideoToWebP(data, transparentColor)
	} else if mime.Is("image/jpeg") || mime.Is("image/png") || mime.Is("image/jpg") {
		img, _, err := image.Decode(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("failed to decode image: %v", err)
		}

		var webpBuffer bytes.Buffer
		err = webp.Encode(&webpBuffer, img, &webp.Options{Lossless: false, Quality: 80})
		if err != nil {
			return nil, fmt.Errorf("failed to encode image to WebP: %v", err)
		}
		return webpBuffer.Bytes(), nil
	}

	return nil, fmt.Errorf("unsupported format: %s", mime.String())
}

func (s *sendService) SendSticker(data *StickerStruct, instance *instance_model.Instance) (*MessageSendStruct, error) {
	client, err := s.ensureClientConnected(instance.Id)
	if err != nil {
		return nil, err
	}

	var uploaded whatsmeow.UploadResponse
	var filedata []byte

	if strings.HasPrefix(data.Sticker, "http") {
		webpData, err := convertToWebP(data.Sticker, data.TransparentColor)
		if err != nil {
			return nil, fmt.Errorf("failed to convert image to WebP: %v", err)
		}

		filedata = webpData

		uploaded, err = client.Upload(context.Background(), filedata, whatsmeow.MediaImage)
		if err != nil {
			return nil, fmt.Errorf("failed to upload sticker: %v", err)
		}
	} else {
		return nil, fmt.Errorf("invalid sticker URL")
	}

	msg := &waE2E.Message{StickerMessage: &waE2E.StickerMessage{
		URL:           proto.String(uploaded.URL),
		DirectPath:    proto.String(uploaded.DirectPath),
		MediaKey:      uploaded.MediaKey,
		Mimetype:      proto.String(http.DetectContentType(filedata)),
		FileEncSHA256: uploaded.FileEncSHA256,
		FileSHA256:    uploaded.FileSHA256,
		FileLength:    proto.Uint64(uint64(len(filedata))),
	}}

	message, err := s.SendMessage(instance, msg, "StickerMessage", &SendDataStruct{
		Id:           data.Id,
		Number:       data.Number,
		Quoted:       data.Quoted,
		Delay:        data.Delay,
		MentionAll:   data.MentionAll,
		MentionedJID: data.MentionedJID,
		FormatJid:    data.FormatJid,
	})
	if err != nil {
		return nil, err
	}

	return message, nil
}

func (s *sendService) SendLocation(data *LocationStruct, instance *instance_model.Instance) (*MessageSendStruct, error) {
	_, err := s.ensureClientConnected(instance.Id)
	if err != nil {
		return nil, err
	}

	msg := &waE2E.Message{LocationMessage: &waE2E.LocationMessage{
		DegreesLatitude:  &data.Latitude,
		DegreesLongitude: &data.Longitude,
		Name:             &data.Name,
		Address:          &data.Address,
	}}

	message, err := s.SendMessage(instance, msg, "LocationMessage", &SendDataStruct{
		Id:           data.Id,
		Number:       data.Number,
		Quoted:       data.Quoted,
		Delay:        data.Delay,
		MentionAll:   data.MentionAll,
		MentionedJID: data.MentionedJID,
		FormatJid:    data.FormatJid,
	})
	if err != nil {
		return nil, err
	}

	return message, nil
}

func (s *sendService) SendContact(data *ContactStruct, instance *instance_model.Instance) (*MessageSendStruct, error) {
	_, err := s.ensureClientConnected(instance.Id)
	if err != nil {
		return nil, err
	}

	VCstring := utils.GenerateVC(utils.VCardStruct{
		FullName:     data.Vcard.FullName,
		Phone:        data.Vcard.Phone,
		Organization: data.Vcard.Organization,
	})

	fmt.Println(VCstring)

	msg := &waE2E.Message{ContactMessage: &waE2E.ContactMessage{
		DisplayName: &data.Vcard.FullName,
		Vcard:       &VCstring,
	}}

	messaged, err := s.SendMessage(instance, msg, "ContactMessage", &SendDataStruct{
		Id:           data.Id,
		Number:       data.Number,
		Quoted:       data.Quoted,
		Delay:        data.Delay,
		MentionAll:   data.MentionAll,
		MentionedJID: data.MentionedJID,
		FormatJid:    data.FormatJid,
	})
	if err != nil {
		return nil, err
	}

	return messaged, nil
}

func mapKeyType(keyType string) string {
	switch keyType {
	case "phone":
		return "PHONE"
	case "email":
		return "EMAIL"
	case "cpf":
		return "CPF"
	case "cnpj":
		return "CNPJ"
	case "random":
		return "EVP"
	default:
		return keyType
	}
}

func (s *sendService) SendButton(data *ButtonStruct, instance *instance_model.Instance) (*MessageSendStruct, error) {
	client, err := s.ensureClientConnected(instance.Id)
	if err != nil {
		return nil, err
	}

	hasReply := false
	hasPix := false
	hasOtherTypes := false
	replyCount := 0

	for _, v := range data.Buttons {
		switch v.Type {
		case "reply":
			hasReply = true
			replyCount++
		case "pix":
			hasPix = true
		default:
			hasOtherTypes = true
		}
	}

	if hasReply {
		if replyCount > 3 {
			return nil, errors.New("máximo de 3 botões do tipo 'reply' permitidos")
		}
		if hasOtherTypes {
			return nil, errors.New("botões do tipo 'reply' não podem ser misturados com outros tipos")
		}
	}

	if hasPix {
		if len(data.Buttons) > 1 {
			return nil, errors.New("botão do tipo 'pix' não pode ser combinado com outros botões")
		}
	}

	buttons := []*waE2E.InteractiveMessage_NativeFlowMessage_NativeFlowButton{}

	for _, v := range data.Buttons {
		var paramsJSON *string

		var name *string

		switch v.Type {
		case "reply":
			name = proto.String("quick_reply")
			paramsJSON = proto.String(`{"display_text":"` + v.DisplayText + `","id":"` + v.Id + `"}`)
		case "copy":
			name = proto.String("cta_copy")
			paramsJSON = proto.String(`{"display_text":"` + v.DisplayText + `","copy_code":"` + v.CopyCode + `"}`)
		case "url":
			name = proto.String("cta_url")
			paramsJSON = proto.String(`{"display_text":"` + v.DisplayText + `","url":"` + v.URL + `","merchant_url":"` + v.URL + `"}`)
		case "call":
			name = proto.String("cta_call")
			paramsJSON = proto.String(`{"display_text":"` + v.DisplayText + `","phone_number":"` + v.PhoneNumber + `"}`)
		case "pix":
			randomId := utils.GenerateRandomString(11)
			name = proto.String("payment_info")
			paramsJSON = proto.String(`{"currency":"` + v.Currency + `","total_amount":{"value":0,"offset":100},"reference_id":"` + randomId + `","type":"physical-goods","order":{"status":"pending","subtotal":{"value":0,"offset":100},"order_type":"ORDER","items":[{"name":"","amount":{"value":0,"offset":100},"quantity":0,"sale_amount":{"value":0,"offset":100}}]},"payment_settings":[{"type":"pix_static_code","pix_static_code":{"merchant_name":"` + v.Name + `","key":"` + v.Key + `","key_type":"` + mapKeyType(v.KeyType) + `"}}],"share_payment_status":false}`)
		}

		buttons = append(buttons, &waE2E.InteractiveMessage_NativeFlowMessage_NativeFlowButton{
			Name:             name,
			ButtonParamsJSON: paramsJSON,
		})
	}

	messageId := client.GenerateMessageID()
	templateId := strconv.FormatInt(time.Now().UnixNano()/1000000, 10)
	messageParamsJSON := `{"from":"api","templateId":` + templateId + `}`

	var msg *waE2E.Message

	if hasPix {
		msg = &waE2E.Message{
			InteractiveMessage: &waE2E.InteractiveMessage{
				InteractiveMessage: &waE2E.InteractiveMessage_NativeFlowMessage_{
					NativeFlowMessage: &waE2E.InteractiveMessage_NativeFlowMessage{
						Buttons:           buttons,
						MessageParamsJSON: &messageParamsJSON,
					},
				},
			},
		}
	} else {
		body := func() string {
			t := "*" + data.Title + "*"
			if data.Description != "" {
				t += "\n\n" + data.Description + "\n"
			}
			return t
		}()

		msg = &waE2E.Message{ViewOnceMessage: &waE2E.FutureProofMessage{
			Message: &waE2E.Message{
				InteractiveMessage: &waE2E.InteractiveMessage{
					Body: &waE2E.InteractiveMessage_Body{
						Text: &body,
					},
					Footer: &waE2E.InteractiveMessage_Footer{
						Text: &data.Footer,
					},
					InteractiveMessage: &waE2E.InteractiveMessage_NativeFlowMessage_{
						NativeFlowMessage: &waE2E.InteractiveMessage_NativeFlowMessage{
							Buttons:           buttons,
							MessageParamsJSON: &messageParamsJSON,
						},
					},
				},
			},
		}}
	}

	recipient, err := s.validateAndCheckUserExists(data.Number, data.FormatJid, &data.Quoted.MessageID, &data.Quoted.MessageID, instance)
	if err != nil {
		s.loggerWrapper.GetLogger(instance.Id).LogError("[%s] Error validating message fields or user check: %v", instance.Id, err)
		return nil, err
	}

	if data.Delay > 0 {
		err := client.SendChatPresence(context.Background(), recipient, types.ChatPresence("composing"), types.ChatPresenceMedia(""))
		if err != nil {
			return nil, err
		}

		time.Sleep(time.Duration(data.Delay) * time.Millisecond)

		err = client.SendChatPresence(context.Background(), recipient, types.ChatPresence("paused"), types.ChatPresenceMedia(""))
		if err != nil {
			return nil, err
		}
	}

	response, err := client.SendMessage(context.Background(), recipient, msg, whatsmeow.SendRequestExtra{ID: messageId})
	if err != nil {
		return nil, err
	}

	messageInfo := types.MessageInfo{
		MessageSource: types.MessageSource{
			Chat:     recipient,
			Sender:   *client.Store.ID,
			IsFromMe: true,
			IsGroup:  false,
		},
		ID:        messageId,
		Timestamp: time.Now(),
		ServerID:  response.ServerID,
		Type:      "ButtonMessage",
	}

	messageSent := &MessageSendStruct{
		Info:    messageInfo,
		Message: msg,
		MessageContextInfo: &waE2E.ContextInfo{
			StanzaID:      proto.String(data.Quoted.MessageID),
			Participant:   proto.String(data.Quoted.Participant),
			QuotedMessage: &waE2E.Message{Conversation: proto.String("")},
		},
	}

	return messageSent, nil
}

func stringPointer(s string) *string {
	return &s
}

func sectionsToString(data *ListStruct) (string, error) {
	type row struct {
		Header      *string `json:"header,omitempty"`
		Title       string  `json:"title"`
		Description *string `json:"description,omitempty"`
		ID          string  `json:"id"`
	}

	type listSection struct {
		Title string `json:"title"`
		Rows  []row  `json:"rows"`
	}

	type list struct {
		Title    string        `json:"title"`
		Sections []listSection `json:"sections"`
	}

	sections := []listSection{}

	for _, s := range data.Sections {
		rows := []row{}

		for _, r := range s.Rows {
			rows = append(rows, row{
				Title:       r.Title,
				Description: stringPointer(r.Description),
				ID:          r.RowId,
			})
		}

		section := listSection{
			Title: s.Title,
			Rows:  rows,
		}

		sections = append(sections, section)
	}

	listData := list{
		Title:    data.ButtonText,
		Sections: sections,
	}

	jsonData, err := json.Marshal(listData)
	if err != nil {
		return "", err
	}

	return string(jsonData), nil
}

func (s *sendService) SendList(data *ListStruct, instance *instance_model.Instance) (*MessageSendStruct, error) {
	client, err := s.ensureClientConnected(instance.Id)
	if err != nil {
		return nil, err
	}

	messageId := client.GenerateMessageID()

	templateId := strconv.FormatInt(time.Now().UnixNano()/1000000, 10)

	messageParamsJSON := `{"from":"api","templateId":` + templateId + `}`

	buttons := []*waE2E.InteractiveMessage_NativeFlowMessage_NativeFlowButton{}

	sectionString, err := sectionsToString(data)
	if err != nil {
		return nil, err
	}

	buttons = append(buttons, &waE2E.InteractiveMessage_NativeFlowMessage_NativeFlowButton{
		Name:             proto.String("single_select"),
		ButtonParamsJSON: proto.String(sectionString),
	})

	body := func() string {
		t := "*" + data.Title + "*"
		if data.Description != "" {
			t += "\n\n" + data.Description + "\n"
		}
		return t
	}()

	msg := &waE2E.Message{ViewOnceMessage: &waE2E.FutureProofMessage{
		Message: &waE2E.Message{
			InteractiveMessage: &waE2E.InteractiveMessage{
				Body: &waE2E.InteractiveMessage_Body{
					Text: &body,
				},
				Footer: &waE2E.InteractiveMessage_Footer{
					Text: &data.FooterText,
				},
				InteractiveMessage: &waE2E.InteractiveMessage_NativeFlowMessage_{
					NativeFlowMessage: &waE2E.InteractiveMessage_NativeFlowMessage{
						Buttons:           buttons,
						MessageParamsJSON: &messageParamsJSON,
					},
				},
			},
		},
	}}

	recipient, err := s.validateAndCheckUserExists(data.Number, data.FormatJid, &data.Quoted.MessageID, &data.Quoted.MessageID, instance)
	if err != nil {
		s.loggerWrapper.GetLogger(instance.Id).LogError("[%s] Error validating message fields or user check: %v", instance.Id, err)
		return nil, err
	}

	if data.Delay > 0 {
		err := client.SendChatPresence(context.Background(), recipient, types.ChatPresence("composing"), types.ChatPresenceMedia(""))
		if err != nil {
			return nil, err
		}

		time.Sleep(time.Duration(data.Delay) * time.Millisecond)

		err = s.clientPointer[instance.Id].SendChatPresence(context.Background(), recipient, types.ChatPresence("paused"), types.ChatPresenceMedia(""))
		if err != nil {
			return nil, err
		}
	}

	response, err := s.clientPointer[instance.Id].SendMessage(context.Background(), recipient, msg, whatsmeow.SendRequestExtra{ID: messageId})
	if err != nil {
		return nil, err
	}

	messageInfo := types.MessageInfo{
		MessageSource: types.MessageSource{
			Chat:     recipient,
			Sender:   *s.clientPointer[instance.Id].Store.ID,
			IsFromMe: true,
			IsGroup:  false,
		},
		ID:        messageId,
		Timestamp: time.Now(),
		ServerID:  response.ServerID,
		Type:      "ListMessage",
	}

	messageSent := &MessageSendStruct{
		Info:    messageInfo,
		Message: msg,
		MessageContextInfo: &waE2E.ContextInfo{
			StanzaID:      proto.String(data.Quoted.MessageID),
			Participant:   proto.String(data.Quoted.Participant),
			QuotedMessage: &waE2E.Message{Conversation: proto.String("")},
		},
	}

	return messageSent, nil
}

func (s *sendService) SendMessage(instance *instance_model.Instance, msg *waE2E.Message, messageType string, data *SendDataStruct) (*MessageSendStruct, error) {
	s.loggerWrapper.GetLogger(instance.Id).LogInfo("[%s] SendMessage called for number: %s, type: %s", instance.Id, data.Number, messageType)

	recipient, err := s.validateAndCheckUserExists(data.Number, data.FormatJid, &data.Quoted.MessageID, &data.Quoted.MessageID, instance)
	if err != nil {
		s.loggerWrapper.GetLogger(instance.Id).LogError("[%s] Error validating message fields or user check: %v", instance.Id, err)
		return nil, err
	}

	s.loggerWrapper.GetLogger(instance.Id).LogInfo("[%s] Recipient validated: %s (Server: %s)", instance.Id, recipient.String(), recipient.Server)

	var message string
	if data.Id == "" {
		message = s.clientPointer[instance.Id].GenerateMessageID()
	} else {
		message = data.Id
	}

	if data.Delay > 0 {
		media := ""
		if messageType == "AudioMessage" {
			media = "audio"
		}

		err := s.clientPointer[instance.Id].SendChatPresence(context.Background(), recipient, types.ChatPresence("composing"), types.ChatPresenceMedia(media))
		if err != nil {
			return nil, err
		}

		time.Sleep(time.Duration(data.Delay) * time.Millisecond)

		err = s.clientPointer[instance.Id].SendChatPresence(context.Background(), recipient, types.ChatPresence("paused"), types.ChatPresenceMedia(media))
		if err != nil {
			return nil, err
		}
	}

	isMedia := false

	if data.Quoted.MessageID != "" {
		switch messageType {
		case "ExtendedTextMessage":
			msg.ExtendedTextMessage.ContextInfo = &waE2E.ContextInfo{
				StanzaID:      proto.String(data.Quoted.MessageID),
				Participant:   proto.String(data.Quoted.Participant),
				QuotedMessage: &waE2E.Message{Conversation: proto.String("")},
			}
		case "ImageMessage":
			msg.ImageMessage.ContextInfo = &waE2E.ContextInfo{
				StanzaID:      proto.String(data.Quoted.MessageID),
				Participant:   proto.String(data.Quoted.Participant),
				QuotedMessage: &waE2E.Message{Conversation: proto.String("")},
			}
			isMedia = true
		case "VideoMessage":
			msg.VideoMessage.ContextInfo = &waE2E.ContextInfo{
				StanzaID:      proto.String(data.Quoted.MessageID),
				Participant:   proto.String(data.Quoted.Participant),
				QuotedMessage: &waE2E.Message{Conversation: proto.String("")},
			}
			isMedia = true
		case "PtvMessage":
			msg.PtvMessage.ContextInfo = &waE2E.ContextInfo{
				StanzaID:      proto.String(data.Quoted.MessageID),
				Participant:   proto.String(data.Quoted.Participant),
				QuotedMessage: &waE2E.Message{Conversation: proto.String("")},
			}
			isMedia = true
		case "AudioMessage":
			msg.AudioMessage.ContextInfo = &waE2E.ContextInfo{
				StanzaID:      proto.String(data.Quoted.MessageID),
				Participant:   proto.String(data.Quoted.Participant),
				QuotedMessage: &waE2E.Message{Conversation: proto.String("")},
			}
			isMedia = true
		case "DocumentMessage":
			if msg.DocumentMessage != nil {
				msg.DocumentMessage.ContextInfo = &waE2E.ContextInfo{
					StanzaID:      proto.String(data.Quoted.MessageID),
					Participant:   proto.String(data.Quoted.Participant),
					QuotedMessage: &waE2E.Message{Conversation: proto.String("")},
				}
			} else if msg.DocumentWithCaptionMessage != nil {
				msg.DocumentWithCaptionMessage.Message.DocumentMessage.ContextInfo = &waE2E.ContextInfo{
					StanzaID:      proto.String(data.Quoted.MessageID),
					Participant:   proto.String(data.Quoted.Participant),
					QuotedMessage: &waE2E.Message{Conversation: proto.String("")},
				}
			}
			isMedia = true
		case "PollCreationMessage":
			msg.PollCreationMessage.ContextInfo = &waE2E.ContextInfo{
				StanzaID:      proto.String(data.Quoted.MessageID),
				Participant:   proto.String(data.Quoted.Participant),
				QuotedMessage: &waE2E.Message{Conversation: proto.String("")},
			}
		case "StickerMessage":
			msg.StickerMessage.ContextInfo = &waE2E.ContextInfo{
				StanzaID:      proto.String(data.Quoted.MessageID),
				Participant:   proto.String(data.Quoted.Participant),
				QuotedMessage: &waE2E.Message{Conversation: proto.String("")},
			}
			isMedia = true
		case "LocationMessage":
			msg.LocationMessage.ContextInfo = &waE2E.ContextInfo{
				StanzaID:      proto.String(data.Quoted.MessageID),
				Participant:   proto.String(data.Quoted.Participant),
				QuotedMessage: &waE2E.Message{Conversation: proto.String("")},
			}
		case "ContactMessage":
			msg.ContactMessage.ContextInfo = &waE2E.ContextInfo{
				StanzaID:      proto.String(data.Quoted.MessageID),
				Participant:   proto.String(data.Quoted.Participant),
				QuotedMessage: &waE2E.Message{Conversation: proto.String("")},
			}
		default:
			return nil, fmt.Errorf("invalid messageType: %s", messageType)
		}
	} else {
		switch messageType {
		case "ExtendedTextMessage":
			msg.ExtendedTextMessage.ContextInfo = &waE2E.ContextInfo{}
		case "ImageMessage":
			msg.ImageMessage.ContextInfo = &waE2E.ContextInfo{}
			isMedia = true
		case "VideoMessage":
			msg.VideoMessage.ContextInfo = &waE2E.ContextInfo{}
			isMedia = true
		case "PtvMessage":
			msg.PtvMessage.ContextInfo = &waE2E.ContextInfo{}
			isMedia = true
		case "AudioMessage":
			msg.AudioMessage.ContextInfo = &waE2E.ContextInfo{}
			isMedia = true
		case "DocumentMessage":
			if msg.DocumentMessage != nil {
				msg.DocumentMessage.ContextInfo = &waE2E.ContextInfo{}
			} else if msg.DocumentWithCaptionMessage != nil {
				msg.DocumentWithCaptionMessage.Message.DocumentMessage.ContextInfo = &waE2E.ContextInfo{}
			}
			isMedia = true
		case "PollCreationMessage":
			msg.PollCreationMessage.ContextInfo = &waE2E.ContextInfo{}
		case "StickerMessage":
			msg.StickerMessage.ContextInfo = &waE2E.ContextInfo{}
		case "LocationMessage":
			msg.LocationMessage.ContextInfo = &waE2E.ContextInfo{}
		case "ContactMessage":
			msg.ContactMessage.ContextInfo = &waE2E.ContextInfo{}
		default:
			return nil, fmt.Errorf("invalid messageType: %s", messageType)
		}
	}

	isGroup := strings.Contains(data.Number, "@g.us")
	isNewsletter := strings.Contains(data.Number, "@newsletter")

	// Only try to get participants for actual groups, not newsletters
	if isGroup && !isNewsletter {
		if data.MentionAll {
			groupInfo, err := s.clientPointer[instance.Id].GetGroupInfo(context.Background(), recipient)
			if err != nil {
				return nil, err
			}

			var mentionedJIDs []string
			for _, participant := range groupInfo.Participants {
				mentionedJIDs = append(mentionedJIDs, participant.JID.String())
			}

			switch messageType {
			case "ExtendedTextMessage":
				if msg.ExtendedTextMessage.ContextInfo == nil {
					msg.ExtendedTextMessage.ContextInfo = &waE2E.ContextInfo{}
				}
				msg.ExtendedTextMessage.ContextInfo.MentionedJID = mentionedJIDs
			case "ImageMessage":
				if msg.ImageMessage.ContextInfo == nil {
					msg.ImageMessage.ContextInfo = &waE2E.ContextInfo{}
				}
				msg.ImageMessage.ContextInfo.MentionedJID = mentionedJIDs
			case "VideoMessage":
				if msg.VideoMessage.ContextInfo == nil {
					msg.VideoMessage.ContextInfo = &waE2E.ContextInfo{}
				}
				msg.VideoMessage.ContextInfo.MentionedJID = mentionedJIDs
			case "PtvMessage":
				if msg.PtvMessage.ContextInfo == nil {
					msg.PtvMessage.ContextInfo = &waE2E.ContextInfo{}
				}
				msg.PtvMessage.ContextInfo.MentionedJID = mentionedJIDs
			case "AudioMessage":
				if msg.AudioMessage.ContextInfo == nil {
					msg.AudioMessage.ContextInfo = &waE2E.ContextInfo{}
				}
				msg.AudioMessage.ContextInfo.MentionedJID = mentionedJIDs
			case "DocumentMessage":
				if msg.DocumentMessage.ContextInfo == nil {
					msg.DocumentMessage.ContextInfo = &waE2E.ContextInfo{}
				}
				msg.DocumentMessage.ContextInfo.MentionedJID = mentionedJIDs
			case "PollCreationMessage":
				if msg.PollCreationMessage.ContextInfo == nil {
					msg.PollCreationMessage.ContextInfo = &waE2E.ContextInfo{}
				}
				msg.PollCreationMessage.ContextInfo.MentionedJID = mentionedJIDs
			case "StickerMessage":
				if msg.StickerMessage.ContextInfo == nil {
					msg.StickerMessage.ContextInfo = &waE2E.ContextInfo{}
				}
				msg.StickerMessage.ContextInfo.MentionedJID = mentionedJIDs
			case "LocationMessage":
				if msg.LocationMessage.ContextInfo == nil {
					msg.LocationMessage.ContextInfo = &waE2E.ContextInfo{}
				}
				msg.LocationMessage.ContextInfo.MentionedJID = mentionedJIDs
			case "ContactMessage":
				if msg.ContactMessage.ContextInfo == nil {
					msg.ContactMessage.ContextInfo = &waE2E.ContextInfo{}
				}
				msg.ContactMessage.ContextInfo.MentionedJID = mentionedJIDs
			}

		}

		if len(data.MentionedJID) > 0 {
			switch messageType {
			case "ExtendedTextMessage":
				if msg.ExtendedTextMessage.ContextInfo == nil {
					msg.ExtendedTextMessage.ContextInfo = &waE2E.ContextInfo{}
				}
				msg.ExtendedTextMessage.ContextInfo.MentionedJID = data.MentionedJID
			case "ImageMessage":
				if msg.ImageMessage.ContextInfo == nil {
					msg.ImageMessage.ContextInfo = &waE2E.ContextInfo{}
				}
				msg.ImageMessage.ContextInfo.MentionedJID = data.MentionedJID
			case "VideoMessage":
				if msg.VideoMessage.ContextInfo == nil {
					msg.VideoMessage.ContextInfo = &waE2E.ContextInfo{}
				}
				msg.VideoMessage.ContextInfo.MentionedJID = data.MentionedJID
			case "PtvMessage":
				if msg.PtvMessage.ContextInfo == nil {
					msg.PtvMessage.ContextInfo = &waE2E.ContextInfo{}
				}
				msg.PtvMessage.ContextInfo.MentionedJID = data.MentionedJID
			case "AudioMessage":
				if msg.AudioMessage.ContextInfo == nil {
					msg.AudioMessage.ContextInfo = &waE2E.ContextInfo{}
				}
				msg.AudioMessage.ContextInfo.MentionedJID = data.MentionedJID
			case "DocumentMessage":
				if msg.DocumentMessage.ContextInfo == nil {
					msg.DocumentMessage.ContextInfo = &waE2E.ContextInfo{}
				}
				msg.DocumentMessage.ContextInfo.MentionedJID = data.MentionedJID
			case "PollCreationMessage":
				if msg.PollCreationMessage.ContextInfo == nil {
					msg.PollCreationMessage.ContextInfo = &waE2E.ContextInfo{}
				}
				msg.PollCreationMessage.ContextInfo.MentionedJID = data.MentionedJID
			case "StickerMessage":
				if msg.StickerMessage.ContextInfo == nil {
					msg.StickerMessage.ContextInfo = &waE2E.ContextInfo{}
				}
				msg.StickerMessage.ContextInfo.MentionedJID = data.MentionedJID
			case "LocationMessage":
				if msg.LocationMessage.ContextInfo == nil {
					msg.LocationMessage.ContextInfo = &waE2E.ContextInfo{}
				}
				msg.LocationMessage.ContextInfo.MentionedJID = data.MentionedJID
			case "ContactMessage":
				if msg.ContactMessage.ContextInfo == nil {
					msg.ContactMessage.ContextInfo = &waE2E.ContextInfo{}
				}
				msg.ContactMessage.ContextInfo.MentionedJID = data.MentionedJID
			}
		}
	}

	recipient.User = strings.ReplaceAll(recipient.User, "+", "")

	s.loggerWrapper.GetLogger(instance.Id).LogInfo("[%s] Sending message to %s with ID %s", instance.Id, recipient.String(), message)

	// Preparar extra parameters para o envio
	sendExtra := whatsmeow.SendRequestExtra{ID: message}

	// Para newsletters/canais, adicionar o MediaHandle se houver mídia
	if recipient.Server == "newsletter" && data.MediaHandle != "" {
		sendExtra.MediaHandle = data.MediaHandle
		s.loggerWrapper.GetLogger(instance.Id).LogInfo("[%s] Newsletter detected, using MediaHandle: %s", instance.Id, data.MediaHandle)
	}

	response, err := s.clientPointer[instance.Id].SendMessage(context.Background(), recipient, msg, sendExtra)
	if err != nil {
		s.loggerWrapper.GetLogger(instance.Id).LogError("[%s] Error sending message: %v", instance.Id, err)
		return nil, err
	}

	s.loggerWrapper.GetLogger(instance.Id).LogInfo("[%s] Message sent successfully! ServerID: %d", instance.Id, response.ServerID)

	messageInfo := types.MessageInfo{
		MessageSource: types.MessageSource{
			Chat:     recipient,
			Sender:   *s.clientPointer[instance.Id].Store.ID,
			IsFromMe: true,
			IsGroup:  isGroup,
		},
		ID:        message,
		Timestamp: time.Now(),
		ServerID:  response.ServerID,
		Type:      messageType,
	}

	messageSent := &MessageSendStruct{
		Info:    messageInfo,
		Message: msg,
		MessageContextInfo: &waE2E.ContextInfo{
			StanzaID:      proto.String(data.Quoted.MessageID),
			Participant:   proto.String(data.Quoted.Participant),
			QuotedMessage: &waE2E.Message{Conversation: proto.String("")},
		},
	}

	postMap := make(map[string]interface{})
	postMap["event"] = "SendMessage"

	// Convertendo o MessageSendStruct para map antes de atribuir
	messageData := make(map[string]interface{})
	messageData["Info"] = messageSent.Info

	// Convertendo a mensagem para map usando json marshal/unmarshal
	msgBytes, err := json.Marshal(messageSent.Message)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal message: %v", err)
	}

	var msgMap map[string]interface{}
	if err := json.Unmarshal(msgBytes, &msgMap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal message: %v", err)
	}

	messageData["Message"] = msgMap
	messageData["MessageContextInfo"] = messageSent.MessageContextInfo

	postMap["data"] = messageData

	if isMedia && s.config.WebhookFiles {
		var data []byte
		var err error

		img := msg.GetImageMessage()
		audio := msg.GetAudioMessage()
		document := msg.GetDocumentMessage()
		video := msg.GetVideoMessage()
		sticker := msg.GetStickerMessage()

		if img != nil {
			data, err = s.clientPointer[instance.Id].Download(context.Background(), img)
		} else if audio != nil {
			data, err = s.clientPointer[instance.Id].Download(context.Background(), audio)
		} else if document != nil {
			data, err = s.clientPointer[instance.Id].Download(context.Background(), document)
		} else if video != nil {
			data, err = s.clientPointer[instance.Id].Download(context.Background(), video)
		} else if sticker != nil {
			data, err = s.clientPointer[instance.Id].Download(context.Background(), sticker)

			webpReader := bytes.NewReader(data)
			img, err := webp.Decode(webpReader)
			if err == nil {
				var pngBuffer bytes.Buffer
				err = png.Encode(&pngBuffer, img)
				if err == nil {
					data = pngBuffer.Bytes()
				}
			}
		}

		if err == nil {
			// Acessando o Message do map já convertido
			messageMap := msgMap
			if messageMap == nil {
				messageMap = make(map[string]interface{})
			}

			encodeData := base64.StdEncoding.EncodeToString(data)
			messageMap["base64"] = encodeData

			messageData["Message"] = messageMap
		}
	}

	postMap["instanceToken"] = instance.Token
	postMap["instanceId"] = instance.Id
	postMap["instanceName"] = instance.Name

	var queueName string

	if _, ok := postMap["event"]; ok {
		queueName = strings.ToLower(fmt.Sprintf("%s.%s", instance.Id, postMap["event"]))
	}

	values, err := json.Marshal(postMap)
	if err != nil {
		s.loggerWrapper.GetLogger(instance.Id).LogError("[%s] Failed to marshal JSON for queue", instance.Id)
		return nil, err
	}

	go s.whatsmeowService.CallWebhook(instance, queueName, values)

	if s.config.AmqpGlobalEnabled || s.config.NatsGlobalEnabled {
		go s.whatsmeowService.SendToGlobalQueues(postMap["event"].(string), values, instance.Id)
	}

	s.loggerWrapper.GetLogger(instance.Id).LogInfo("[%s] Message sent to %s", instance.Id, data.Number)
	return messageSent, nil
}

func (s *sendService) SendCarousel(data *CarouselStruct, instance *instance_model.Instance) (*MessageSendStruct, error) {
	client, err := s.ensureClientConnected(instance.Id)
	if err != nil {
		return nil, err
	}

	formatJid := true
	if data.FormatJid != nil {
		formatJid = *data.FormatJid
	}

	var recipient types.JID
	var ok bool
	recipient, ok = utils.ParseJID(data.Number)
	if !ok && formatJid {
		s.loggerWrapper.GetLogger(instance.Id).LogError("[%s] Error validating message fields", instance.Id)
		return nil, errors.New("invalid phone number")
	} else if !ok && !formatJid {
		recipient = types.JID{
			User:   data.Number,
			Server: types.DefaultUserServer,
		}
	}

	// Build carousel cards
	cards := make([]*waE2E.InteractiveMessage, len(data.Cards))
	messageVersion := int32(1)

	s.loggerWrapper.GetLogger(instance.Id).LogInfo("[%s] Building carousel for %s with %d cards", instance.Id, recipient.String(), len(data.Cards))

	for i, card := range data.Cards {
		// Each card MUST have both header and body for carousel to work
		interactiveCard := &waE2E.InteractiveMessage{
			Body: &waE2E.InteractiveMessage_Body{
				Text: proto.String(card.Body.Text),
			},
			Header: &waE2E.InteractiveMessage_Header{
				Title:              proto.String(card.Header.Title),
				Subtitle:           proto.String(card.Header.Subtitle),
				HasMediaAttachment: proto.Bool(false),
			},
		}

		// Add media to header if URL provided
		if card.Header.ImageUrl != "" || card.Header.VideoUrl != "" {
			header := interactiveCard.Header

			// Add media to header if URL provided
			if card.Header.ImageUrl != "" {
				// Download and upload image
				resp, err := http.Get(card.Header.ImageUrl)
				if err == nil {
					defer resp.Body.Close()
					fileData, err := io.ReadAll(resp.Body)
					if err == nil {
						uploaded, err := client.Upload(context.Background(), fileData, whatsmeow.MediaImage)
						if err == nil {
							header.HasMediaAttachment = proto.Bool(true)
							header.Media = &waE2E.InteractiveMessage_Header_ImageMessage{
								ImageMessage: &waE2E.ImageMessage{
									URL:           proto.String(uploaded.URL),
									DirectPath:    proto.String(uploaded.DirectPath),
									MediaKey:      uploaded.MediaKey,
									Mimetype:      proto.String("image/jpeg"),
									FileEncSHA256: uploaded.FileEncSHA256,
									FileSHA256:    uploaded.FileSHA256,
									FileLength:    proto.Uint64(uint64(len(fileData))),
								},
							}
						}
					}
				}
			} else if card.Header.VideoUrl != "" {
				// Download and upload video
				resp, err := http.Get(card.Header.VideoUrl)
				if err == nil {
					defer resp.Body.Close()
					fileData, err := io.ReadAll(resp.Body)
					if err == nil {
						uploaded, err := client.Upload(context.Background(), fileData, whatsmeow.MediaVideo)
						if err == nil {
							header.HasMediaAttachment = proto.Bool(true)
							header.Media = &waE2E.InteractiveMessage_Header_VideoMessage{
								VideoMessage: &waE2E.VideoMessage{
									URL:           proto.String(uploaded.URL),
									DirectPath:    proto.String(uploaded.DirectPath),
									MediaKey:      uploaded.MediaKey,
									Mimetype:      proto.String("video/mp4"),
									FileEncSHA256: uploaded.FileEncSHA256,
									FileSHA256:    uploaded.FileSHA256,
									FileLength:    proto.Uint64(uint64(len(fileData))),
								},
							}
						}
					}
				}
			}
		}

		// Add footer if exists
		if card.Footer != "" {
			interactiveCard.Footer = &waE2E.InteractiveMessage_Footer{
				Text: proto.String(card.Footer),
			}
		}

		// Add buttons if exist
		if len(card.Buttons) > 0 {
			buttons := make([]*waE2E.InteractiveMessage_NativeFlowMessage_NativeFlowButton, len(card.Buttons))
			for j, btn := range card.Buttons {
				buttonType := strings.ToUpper(btn.Type)
				if buttonType == "" {
					buttonType = "REPLY" // Default type
				}

				var buttonName string
				var buttonParams string

				switch buttonType {
				case "URL":
					// URL button - opens a link
					buttonName = "cta_url"
					buttonParams = fmt.Sprintf(`{"display_text":"%s","url":"%s"}`, btn.DisplayText, btn.Id)
				case "CALL":
					// Call button - initiates a phone call
					buttonName = "cta_call"
					buttonParams = fmt.Sprintf(`{"display_text":"%s","phone_number":"%s"}`, btn.DisplayText, btn.Id)
				case "COPY":
					// Copy button - copies text to clipboard
					buttonName = "cta_copy"
					buttonParams = fmt.Sprintf(`{"display_text":"%s","copy_code":"%s"}`, btn.DisplayText, btn.CopyCode)
				case "REPLY":
					fallthrough
				default:
					// Quick reply button (default)
					buttonName = "quick_reply"
					buttonParams = fmt.Sprintf(`{"display_text":"%s","id":"%s"}`, btn.DisplayText, btn.Id)
				}

				buttons[j] = &waE2E.InteractiveMessage_NativeFlowMessage_NativeFlowButton{
					Name:             proto.String(buttonName),
					ButtonParamsJSON: proto.String(buttonParams),
				}
			}

			// messageParamsJSON is REQUIRED for NativeFlowMessage with buttons
			messageParamsJSON := fmt.Sprintf(`{"from":"api","templateId":"%d"}`, time.Now().UnixNano()/1000000)

			interactiveCard.InteractiveMessage = &waE2E.InteractiveMessage_NativeFlowMessage_{
				NativeFlowMessage: &waE2E.InteractiveMessage_NativeFlowMessage{
					Buttons:           buttons,
					MessageParamsJSON: proto.String(messageParamsJSON),
					MessageVersion:    proto.Int32(1),
				},
			}
		}

		cards[i] = interactiveCard
	}

	// Build carousel message
	carouselCardType := waE2E.InteractiveMessage_CarouselMessage_HSCROLL_CARDS
	interactiveMsg := &waE2E.InteractiveMessage{
		InteractiveMessage: &waE2E.InteractiveMessage_CarouselMessage_{
			CarouselMessage: &waE2E.InteractiveMessage_CarouselMessage{
				Cards:            cards,
				MessageVersion:   &messageVersion,
				CarouselCardType: &carouselCardType,
			},
		},
	}

	// Add body if provided (main message above carousel)
	if data.Body != "" {
		interactiveMsg.Body = &waE2E.InteractiveMessage_Body{
			Text: proto.String(data.Body),
		}
	}

	// Add footer if provided (text below carousel)
	if data.Footer != "" {
		interactiveMsg.Footer = &waE2E.InteractiveMessage_Footer{
			Text: proto.String(data.Footer),
		}
	}

	// ContextInfo is REQUIRED for iOS compatibility
	// Even if empty, iOS requires this field to display carousel
	contextInfo := &waE2E.ContextInfo{}

	// Add quoted message if exists
	if data.Quoted.MessageID != "" {
		contextInfo.StanzaID = proto.String(data.Quoted.MessageID)
		if data.Quoted.Participant != "" {
			participantJID, ok := utils.ParseJID(data.Quoted.Participant)
			if ok {
				contextInfo.Participant = proto.String(participantJID.String())
			}
		}
	}

	// Always set ContextInfo (required for iOS)
	interactiveMsg.ContextInfo = contextInfo

	// Build final message with MessageContextInfo for proper notification delivery
	msg := &waE2E.Message{
		InteractiveMessage: interactiveMsg,
		MessageContextInfo: &waE2E.MessageContextInfo{
			DeviceListMetadata: &waE2E.DeviceListMetadata{},
		},
	}

	message, err := s.SendMessage(instance, msg, "InteractiveMessage", &SendDataStruct{
		Number: data.Number,
		Delay:  data.Delay,
	})

	if err != nil {
		s.loggerWrapper.GetLogger(instance.Id).LogError("[%s] Error sending carousel: %v", instance.Id, err)
		return nil, err
	}

	s.loggerWrapper.GetLogger(instance.Id).LogInfo("[%s] Carousel sent to %s with %d cards", instance.Id, data.Number, len(data.Cards))
	return message, nil
}

func NewSendService(
	clientPointer map[string]*whatsmeow.Client,
	whatsmeowService whatsmeow_service.WhatsmeowService,
	config *config.Config,
	loggerWrapper *logger_wrapper.LoggerManager,
) SendService {
	return &sendService{
		clientPointer:    clientPointer,
		whatsmeowService: whatsmeowService,
		config:           config,
		loggerWrapper:    loggerWrapper,
	}
}
