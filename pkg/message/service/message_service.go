package message_service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	instance_model "github.com/EvolutionAPI/evolution-go/pkg/instance/model"
	logger_wrapper "github.com/EvolutionAPI/evolution-go/pkg/logger"
	message_model "github.com/EvolutionAPI/evolution-go/pkg/message/model"
	message_repository "github.com/EvolutionAPI/evolution-go/pkg/message/repository"
	"github.com/EvolutionAPI/evolution-go/pkg/utils"
	whatsmeow_service "github.com/EvolutionAPI/evolution-go/pkg/whatsmeow/service"
	"github.com/vincent-petithory/dataurl"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waCommon"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
)

type MessageService interface {
	React(data *ReactStruct, instance *instance_model.Instance) (*MessageSendStruct, error)
	ChatPresence(data *ChatPresenceStruct, instance *instance_model.Instance) (string, error)
	MarkRead(data *MarkReadStruct, instance *instance_model.Instance) (string, error)
	DownloadMedia(data *DownloadMediaStruct, instance *instance_model.Instance, request *http.Request) (*dataurl.DataURL, string, error)
	GetMessageStatus(data *MessageStatusStruct, instance *instance_model.Instance) (*message_model.Message, string, error)
	DeleteMessageEveryone(data *MessageStruct, instance *instance_model.Instance) (string, string, error)
	EditMessage(data *EditMessageStruct, instance *instance_model.Instance) (string, string, error)
}

type messageService struct {
	clientPointer     map[string]*whatsmeow.Client
	messageRepository message_repository.MessageRepository
	whatsmeowService  whatsmeow_service.WhatsmeowService
	loggerWrapper     *logger_wrapper.LoggerManager
}

type ReactStruct struct {
	Number      string `json:"number"`
	Reaction    string `json:"reaction"`
	Id          string `json:"id"`
	FromMe      bool   `json:"fromMe"`
	Participant string `json:"participant,omitempty"`
}

type ChatPresenceStruct struct {
	Number  string `json:"number"`
	State   string `json:"state"`
	IsAudio bool   `json:"isAudio"`
	// Delay, in milliseconds, keeps the typing/recording indicator alive by
	// re-sending the chat presence every few seconds. Capped at 60s. 0 sends
	// the indicator once.
	Delay int `json:"delay,omitempty"`
}

type MarkReadStruct struct {
	Id     []string `json:"id"`
	Number string   `json:"number"`
}

type DownloadMediaStruct struct {
	Message *waE2E.Message `json:"message"`
}

type MessageStatusStruct struct {
	Id string `json:"id"`
}

type MessageStruct struct {
	Chat      string `json:"chat"`
	MessageID string `json:"messageId"`
}

type EditMessageStruct struct {
	Chat      string `json:"chat"`
	Message   string `json:"message"`
	MessageID string `json:"messageId"`
}

type MessageSendStruct struct {
	Info               types.MessageInfo
	Message            *waE2E.Message
	MessageContextInfo *waE2E.ContextInfo
}

func (m *messageService) ensureClientConnected(instanceId string) (*whatsmeow.Client, error) {
	client := m.clientPointer[instanceId]
	m.loggerWrapper.GetLogger(instanceId).LogInfo("[%s] Checking client connection status - Client exists: %v", instanceId, client != nil)

	if client == nil {
		m.loggerWrapper.GetLogger(instanceId).LogInfo("[%s] No client found, attempting to start new instance", instanceId)
		err := m.whatsmeowService.StartInstance(instanceId)
		if err != nil {
			m.loggerWrapper.GetLogger(instanceId).LogError("[%s] Failed to start instance: %v", instanceId, err)
			return nil, errors.New("no active session found")
		}

		m.loggerWrapper.GetLogger(instanceId).LogInfo("[%s] Instance started, waiting 2 seconds...", instanceId)
		time.Sleep(2 * time.Second)

		client = m.clientPointer[instanceId]
		m.loggerWrapper.GetLogger(instanceId).LogInfo("[%s] Checking new client - Exists: %v, Connected: %v",
			instanceId,
			client != nil,
			client != nil && client.IsConnected())

		if client == nil || !client.IsConnected() {
			m.loggerWrapper.GetLogger(instanceId).LogError("[%s] New client validation failed - Exists: %v, Connected: %v",
				instanceId,
				client != nil,
				client != nil && client.IsConnected())
			return nil, errors.New("no active session found")
		}
	} else if !client.IsConnected() {
		m.loggerWrapper.GetLogger(instanceId).LogError("[%s] Existing client is disconnected - Connected status: %v",
			instanceId,
			client.IsConnected())
		return nil, errors.New("client disconnected")
	}

	m.loggerWrapper.GetLogger(instanceId).LogInfo("[%s] Client successfully validated - Connected: %v", instanceId, client.IsConnected())
	return client, nil
}

func (m *messageService) React(data *ReactStruct, instance *instance_model.Instance) (*MessageSendStruct, error) {
	client, err := m.ensureClientConnected(instance.Id)
	if err != nil {
		return nil, err
	}

	msgId := ""

	recipient, ok := utils.ParseJID(data.Number)
	if !ok {
		m.loggerWrapper.GetLogger(instance.Id).LogError("[%s] Error validating message fields", instance.Id)
		return nil, errors.New("invalid phone number")
	}
	// Reactions travel as raw protocol nodes, so the target JID must be
	// canonical (digits only, no leading "+") or WhatsApp drops them silently.
	recipient = utils.CanonicalJID(recipient)

	if data.Id == "" {
		m.loggerWrapper.GetLogger(instance.Id).LogError("[%s] Missing Id in Payload", instance.Id)
		return nil, errors.New("missing id in payload")
	} else {
		msgId = data.Id
	}

	fromMe := data.FromMe
	reaction := data.Reaction
	if reaction == "remove" {
		reaction = ""
	}

	// Create MessageKey
	messageKey := &waCommon.MessageKey{
		RemoteJID: proto.String(recipient.String()),
		FromMe:    proto.Bool(fromMe),
		ID:        proto.String(msgId),
	}

	// Add participant if provided (for group messages)
	if data.Participant != "" {
		participantJID, ok := utils.ParseJID(data.Participant)
		if ok {
			messageKey.Participant = proto.String(utils.CanonicalJID(participantJID).String())
		}
	}

	msg := &waE2E.Message{
		ReactionMessage: &waE2E.ReactionMessage{
			Key:  messageKey,
			Text: proto.String(reaction),
			// GroupingKey:       proto.String(reaction),
			SenderTimestampMS: proto.Int64(time.Now().UnixMilli()),
		},
	}

	response, err := client.SendMessage(context.Background(), recipient, msg, whatsmeow.SendRequestExtra{
		ID: msgId,
	})
	if err != nil {
		return nil, err
	}

	isGroup := strings.Contains(data.Number, "@g.us")
	messageType := "ReactionMessage"

	messageInfo := types.MessageInfo{
		MessageSource: types.MessageSource{
			Chat:     recipient,
			Sender:   *client.Store.ID,
			IsFromMe: true,
			IsGroup:  isGroup,
		},
		ID:        msgId,
		Timestamp: time.Now(),
		ServerID:  response.ServerID,
		Type:      messageType,
	}

	messageSent := &MessageSendStruct{
		Info:    messageInfo,
		Message: msg,
	}

	return messageSent, nil
}

func (m *messageService) ChatPresence(data *ChatPresenceStruct, instance *instance_model.Instance) (string, error) {
	client, err := m.ensureClientConnected(instance.Id)
	if err != nil {
		return "", err
	}

	var ts time.Time

	recipient, ok := utils.ParseJID(data.Number)
	if !ok {
		m.loggerWrapper.GetLogger(instance.Id).LogError("[%s] Error validating message fields", instance.Id)
		return "", errors.New("invalid phone number")
	}
	// Chat presence is delivered as a raw protocol node, so the target JID must
	// be canonical (digits only, no leading "+") or WhatsApp drops it silently.
	recipient = utils.CanonicalJID(recipient)

	media := ""

	if data.IsAudio {
		media = "audio"
	}

	// WhatsApp only forwards chat presence (typing/recording) while the sender
	// is marked online, so announce availability before sending the indicator.
	if err = client.SendPresence(context.Background(), types.PresenceAvailable); err != nil {
		m.loggerWrapper.GetLogger(instance.Id).LogWarn("[%s] failed to send available presence: %v", instance.Id, err)
	}

	presence := types.ChatPresence(data.State)
	presenceMedia := types.ChatPresenceMedia(media)

	if err = client.SendChatPresence(context.Background(), recipient, presence, presenceMedia); err != nil {
		return "", err
	}

	// Optionally sustain the indicator by re-sending it periodically, since a
	// single chat presence node expires after a few seconds on the client.
	if data.Delay > 0 {
		const refreshInterval = 5 * time.Second
		const maxDelay = 60 * time.Second

		remaining := time.Duration(data.Delay) * time.Millisecond
		if remaining > maxDelay {
			remaining = maxDelay
		}

		for remaining > refreshInterval {
			time.Sleep(refreshInterval)
			remaining -= refreshInterval
			if err = client.SendChatPresence(context.Background(), recipient, presence, presenceMedia); err != nil {
				m.loggerWrapper.GetLogger(instance.Id).LogWarn("[%s] failed to refresh chat presence: %v", instance.Id, err)
				break
			}
		}
		if remaining > 0 {
			time.Sleep(remaining)
		}
	}

	m.loggerWrapper.GetLogger(instance.Id).LogInfo("Message sent to %s", data.Number)

	return ts.String(), nil
}

func (m *messageService) MarkRead(data *MarkReadStruct, instance *instance_model.Instance) (string, error) {
	client, err := m.ensureClientConnected(instance.Id)
	if err != nil {
		return "", err
	}

	var ts time.Time

	jid, ok := utils.ParseJID(data.Number)
	if !ok {
		m.loggerWrapper.GetLogger(instance.Id).LogError("[%s] Error validating message fields", instance.Id)
		return "", errors.New("invalid phone number")
	}
	// Read receipts are sent as raw protocol nodes, so the target JID must be
	// canonical (digits only, no leading "+") or WhatsApp drops them silently.
	jid = utils.CanonicalJID(jid)

	err = client.MarkRead(context.Background(), data.Id, time.Now(), jid, jid)
	if err != nil {
		m.loggerWrapper.GetLogger(instance.Id).LogError("[%s] error marking message as read: %v", instance.Id, err)
		return "", errors.New("error marking message as read")
	}

	return ts.String(), nil
}

func (m *messageService) DownloadMedia(data *DownloadMediaStruct, instance *instance_model.Instance, request *http.Request) (*dataurl.DataURL, string, error) {
	client, err := m.ensureClientConnected(instance.Id)
	if err != nil {
		return nil, "", err
	}

	var ts time.Time

	msg := data.Message

	mimetype := ""
	var mediaData []byte

	img := msg.GetImageMessage()
	audio := msg.GetAudioMessage()
	document := msg.GetDocumentMessage()
	video := msg.GetVideoMessage()
	sticker := msg.GetStickerMessage()

	if img == nil && audio == nil && document == nil && video == nil && sticker == nil {
		return nil, "", errors.New("invalid media type")
	}

	userDirectory := fmt.Sprintf(`files/user_%s`, instance.Id)
	_, err = os.Stat(userDirectory)
	if os.IsNotExist(err) {
		errDir := os.MkdirAll(userDirectory, 0751)
		if errDir != nil {
			m.loggerWrapper.GetLogger(instance.Id).LogError("[%s] Could not create user directory (%s)", instance.Id, userDirectory)
			return nil, "", errDir
		}
	}

	if img != nil {
		mediaData, err = client.Download(context.Background(), img)
		if err != nil {
			m.loggerWrapper.GetLogger(instance.Id).LogError("[%s] Failed to download image", instance.Id)
			msg := fmt.Sprintf("Failed to download image %v", err)
			return nil, "", errors.New(msg)
		}
		mimetype = img.GetMimetype()
	}

	if audio != nil {
		mediaData, err = client.Download(context.Background(), audio)
		if err != nil {
			m.loggerWrapper.GetLogger(instance.Id).LogError("[%s] Failed to download audio", instance.Id)
			msg := fmt.Sprintf("Failed to download audio %v", err)
			return nil, "", errors.New(msg)
		}
		mimetype = audio.GetMimetype()
	}

	if document != nil {
		mediaData, err = client.Download(context.Background(), document)
		if err != nil {
			m.loggerWrapper.GetLogger(instance.Id).LogError("[%s] Failed to download document", instance.Id)
			msg := fmt.Sprintf("Failed to download document %v", err)
			return nil, "", errors.New(msg)
		}
		mimetype = document.GetMimetype()
	}

	if video != nil {
		mediaData, err = client.Download(context.Background(), video)
		if err != nil {
			m.loggerWrapper.GetLogger(instance.Id).LogError("[%s] Failed to download video", instance.Id)
			msg := fmt.Sprintf("Failed to download video %v", err)
			return nil, "", errors.New(msg)
		}
		mimetype = video.GetMimetype()
	}

	if sticker != nil {
		mediaData, err = client.Download(context.Background(), sticker)
		if err != nil {
			m.loggerWrapper.GetLogger(instance.Id).LogError("[%s] Failed to download sticker", instance.Id)
			msg := fmt.Sprintf("Failed to download sticker %v", err)
			return nil, "", errors.New(msg)
		}
		mimetype = sticker.GetMimetype()
	}

	dataURL := dataurl.New(mediaData, mimetype)

	return dataURL, ts.String(), nil
}

func (m *messageService) GetMessageStatus(data *MessageStatusStruct, instance *instance_model.Instance) (*message_model.Message, string, error) {
	_, err := m.ensureClientConnected(instance.Id)
	if err != nil {
		return nil, "", err
	}

	var ts time.Time

	result, err := m.messageRepository.GetMessageByID(data.Id)
	if err != nil {
		return nil, "", err
	}

	return result, ts.String(), nil
}

func (m *messageService) DeleteMessageEveryone(data *MessageStruct, instance *instance_model.Instance) (string, string, error) {
	client, err := m.ensureClientConnected(instance.Id)
	if err != nil {
		return "", "", err
	}

	var ts time.Time

	recipient, ok := utils.ParseJID(data.Chat)
	if !ok {
		m.loggerWrapper.GetLogger(instance.Id).LogError("[%s] Error validating message fields", instance.Id)
		return "", "", errors.New("invalid phone number")
	}

	m.loggerWrapper.GetLogger(instance.Id).LogInfo("Revoking message %s from %s", data.MessageID, recipient)

	resp, err := client.SendMessage(
		context.Background(),
		recipient,
		client.BuildRevoke(recipient, types.EmptyJID, data.MessageID))
	if err != nil {
		m.loggerWrapper.GetLogger(instance.Id).LogError("[%s] error revoking message: %v", instance.Id, err)
		return "", "", err
	}

	response := resp.ID

	return response, ts.String(), nil
}

func (m *messageService) EditMessage(data *EditMessageStruct, instance *instance_model.Instance) (string, string, error) {
	client, err := m.ensureClientConnected(instance.Id)
	if err != nil {
		return "", "", err
	}

	recipient, ok := utils.ParseJID(data.Chat)
	if !ok {
		m.loggerWrapper.GetLogger(instance.Id).LogError("[%s] Error validating message fields", instance.Id)
		return "", "", errors.New("invalid phone number")
	}

	resp, err := client.SendMessage(
		context.Background(),
		recipient,
		client.BuildEdit(
			recipient,
			data.MessageID,
			&waE2E.Message{
				ExtendedTextMessage: &waE2E.ExtendedTextMessage{
					Text: &data.Message,
				},
			}))
	if err != nil {
		m.loggerWrapper.GetLogger(instance.Id).LogError("[%s] error revoking message: %v", instance.Id, err)
		return "", "", err
	}

	return resp.ID, resp.Timestamp.String(), nil
}

func NewMessageService(
	clientPointer map[string]*whatsmeow.Client,
	messageRepository message_repository.MessageRepository,
	whatsmeowService whatsmeow_service.WhatsmeowService,
	loggerWrapper *logger_wrapper.LoggerManager,
) MessageService {
	return &messageService{
		clientPointer:     clientPointer,
		messageRepository: messageRepository,
		whatsmeowService:  whatsmeowService,
		loggerWrapper:     loggerWrapper,
	}
}
