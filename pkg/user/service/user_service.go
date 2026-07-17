package user_service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	instance_model "github.com/evolution-foundation/evolution-go/pkg/instance/model"
	logger_wrapper "github.com/evolution-foundation/evolution-go/pkg/logger"
	"github.com/evolution-foundation/evolution-go/pkg/utils"
	whatsmeow_service "github.com/evolution-foundation/evolution-go/pkg/whatsmeow/service"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/appstate"
	"go.mau.fi/whatsmeow/proto/waSyncAction"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

type UserService interface {
	GetUser(data *CheckUserStruct, instance *instance_model.Instance) (*UserCollection, error)
	CheckUser(data *CheckUserStruct, instance *instance_model.Instance) (*CheckUserCollection, error)
	GetAvatar(data *GetAvatarStruct, instance *instance_model.Instance) (*types.ProfilePictureInfo, error)
	GetContacts(instance *instance_model.Instance) ([]ContactInfo, error)
	SaveContact(data *SaveContactStruct, instance *instance_model.Instance) error
	GetPrivacy(instance *instance_model.Instance) (types.PrivacySettings, error)
	SetPrivacy(data *PrivacyStruct, instance *instance_model.Instance) (*types.PrivacySettings, error)
	BlockContact(data *BlockStruct, instance *instance_model.Instance) (*types.Blocklist, error)
	UnlockContact(data *BlockStruct, instance *instance_model.Instance) (*types.Blocklist, error)
	GetBlockList(instance *instance_model.Instance) (*types.Blocklist, error)
	SetProfilePicture(data *SetProfilePictureStruct, instance *instance_model.Instance) (bool, error)
	SetProfileName(data *SetProfileNameStruct, instance *instance_model.Instance) (bool, error)
	SetProfileStatus(data *SetProfileStatusStruct, instance *instance_model.Instance) (bool, error)
}

type userService struct {
	clientPointer    map[string]*whatsmeow.Client
	whatsmeowService whatsmeow_service.WhatsmeowService
	loggerWrapper    *logger_wrapper.LoggerManager
}

type ContactInfo struct {
	Jid          string `json:"Jid"`
	Found        bool   `json:"Found"`
	FirstName    string `json:"FirstName"`
	FullName     string `json:"FullName"`
	PushName     string `json:"PushName"`
	BusinessName string `json:"BusinessName"`
}

type UserInfo struct {
	VerifiedName *types.VerifiedName
	Status       string
	PictureID    string
	Devices      []types.JID
	LID          *string // The local ID (if available)
}

type UserCollection struct {
	Users map[types.JID]UserInfo
}

type User struct {
	Query        string
	IsInWhatsapp bool
	JID          string
	RemoteJID    string
	LID          *string
	VerifiedName string
}

type CheckUserCollection struct {
	Users []User
}

type CheckUserStruct struct {
	Number    []string `json:"number"`
	FormatJid *bool    `json:"formatJid,omitempty"`
}

type GetAvatarStruct struct {
	Number  string `json:"number"`
	Preview bool   `json:"preview"`
}

type BlockStruct struct {
	Number string `json:"number"`
}

// SaveContactStruct é o body de POST /user/savecontact.
type SaveContactStruct struct {
	// Número de destino (com DDI).
	Number string `json:"number" example:"5582988898565"`
	// Nome completo salvo para o contato.
	FullName string `json:"fullName" example:"Fulano de Tal"`
	// Primeiro nome (opcional). Se vazio, usa a primeira palavra de fullName.
	FirstName string `json:"firstName,omitempty" example:"Fulano"`
	// Se true (padrão), sincroniza com a agenda do celular primário
	// (equivale ao toggle "Sincronizar contato com celular" do WhatsApp Web).
	SaveOnPhone *bool `json:"saveOnPhone,omitempty"`
}

type SetProfilePictureStruct struct {
	Image string `json:"image"`
}

type SetProfileNameStruct struct {
	Name string `json:"name"`
}

type SetProfileStatusStruct struct {
	Status string `json:"status"`
}

type PrivacyStruct struct {
	GroupAdd     types.PrivacySetting `json:"groupAdd"`
	LastSeen     types.PrivacySetting `json:"lastSeen"`
	Status       types.PrivacySetting `json:"status"`
	Profile      types.PrivacySetting `json:"profile"`
	ReadReceipts types.PrivacySetting `json:"readReceipts"`
	CallAdd      types.PrivacySetting `json:"callAdd"`
	Online       types.PrivacySetting `json:"online"`
}

func (u *userService) ensureClientConnected(instanceId string) (*whatsmeow.Client, error) {
	client := u.clientPointer[instanceId]
	u.loggerWrapper.GetLogger(instanceId).LogInfo("[%s] Checking client connection status - Client exists: %v", instanceId, client != nil)

	if client == nil {
		u.loggerWrapper.GetLogger(instanceId).LogInfo("[%s] No client found, attempting to start new instance", instanceId)
		err := u.whatsmeowService.StartInstance(instanceId)
		if err != nil {
			u.loggerWrapper.GetLogger(instanceId).LogError("[%s] Failed to start instance: %v", instanceId, err)
			return nil, errors.New("no active session found")
		}

		u.loggerWrapper.GetLogger(instanceId).LogInfo("[%s] Instance started, waiting 2 seconds...", instanceId)
		time.Sleep(2 * time.Second)

		client = u.clientPointer[instanceId]
		u.loggerWrapper.GetLogger(instanceId).LogInfo("[%s] Checking new client - Exists: %v, Connected: %v",
			instanceId,
			client != nil,
			client != nil && client.IsConnected())

		if client == nil || !client.IsConnected() {
			u.loggerWrapper.GetLogger(instanceId).LogError("[%s] New client validation failed - Exists: %v, Connected: %v",
				instanceId,
				client != nil,
				client != nil && client.IsConnected())
			return nil, errors.New("no active session found")
		}
	} else if !client.IsConnected() {
		u.loggerWrapper.GetLogger(instanceId).LogError("[%s] Existing client is disconnected - Connected status: %v",
			instanceId,
			client.IsConnected())
		return nil, errors.New("client disconnected")
	}

	u.loggerWrapper.GetLogger(instanceId).LogInfo("[%s] Client successfully validated - Connected: %v", instanceId, client.IsConnected())
	return client, nil
}

func (u *userService) GetUser(data *CheckUserStruct, instance *instance_model.Instance) (*UserCollection, error) {
	client, err := u.ensureClientConnected(instance.Id)
	if err != nil {
		return nil, err
	}

	var jids []types.JID
	for _, arg := range data.Number {
		jid, ok := utils.ParseJID(arg)
		if !ok {
			return nil, errors.New("invalid phone number")
		}
		jids = append(jids, jid)
	}
	resp, err := client.GetUserInfo(context.Background(), jids)
	if err != nil {
		return nil, err
	}

	uc := new(UserCollection)
	uc.Users = make(map[types.JID]UserInfo)

	for jid, whatsmeowInfo := range resp {
		// Consultar LID Store para obter LID associado ao JID
		var lidStr *string
		if client.Store.LIDs != nil {
			if lid, err := client.Store.LIDs.GetLIDForPN(context.TODO(), jid); err == nil && !lid.IsEmpty() {
				lidString := fmt.Sprintf("%v", lid)
				lidStr = &lidString
			}
		}

		// Converter para nossa estrutura UserInfo que inclui LID
		info := UserInfo{
			VerifiedName: whatsmeowInfo.VerifiedName,
			Status:       whatsmeowInfo.Status,
			PictureID:    whatsmeowInfo.PictureID,
			Devices:      whatsmeowInfo.Devices,
			LID:          lidStr,
		}
		uc.Users[jid] = info
	}

	return uc, nil
}

func (u *userService) CheckUser(data *CheckUserStruct, instance *instance_model.Instance) (*CheckUserCollection, error) {
	client, err := u.ensureClientConnected(instance.Id)
	if err != nil {
		return nil, err
	}

	// Set formatJid to false by default for CheckUser
	formatJid := false
	if data.FormatJid != nil {
		formatJid = *data.FormatJid
	}

	// First attempt with the requested formatJid setting
	uc, shouldRetry := u.performCheckUser(client, data.Number, formatJid, instance.Id)
	if !shouldRetry {
		return uc, nil
	}

	// If formatJid was true and we got false results, retry with formatJid=false
	if formatJid {
		u.loggerWrapper.GetLogger(instance.Id).LogInfo("[%s] Some users not found with formatJid=true, retrying with formatJid=false", instance.Id)
		ucRetry, _ := u.performCheckUser(client, data.Number, false, instance.Id)

		// Merge results: use retry results for users that weren't found in first attempt
		return u.mergeCheckUserResults(uc, ucRetry), nil
	}

	return uc, nil
}

// performCheckUser executes the actual user check with specified formatJid
func (u *userService) performCheckUser(client *whatsmeow.Client, numbers []string, formatJid bool, instanceId string) (*CheckUserCollection, bool) {
	// Use centralized function to prepare numbers for WhatsApp check
	phoneNumbers, err := utils.PrepareNumbersForWhatsAppCheck(numbers, &formatJid)
	if err != nil {
		u.loggerWrapper.GetLogger(instanceId).LogWarn("[%s] Failed to prepare numbers for WhatsApp check: %v", instanceId, err)
		return nil, false
	}

	resp, err := client.IsOnWhatsApp(context.Background(), phoneNumbers)
	if err != nil {
		u.loggerWrapper.GetLogger(instanceId).LogError("[%s] Failed to check users on WhatsApp: %v", instanceId, err)
		return nil, false
	}

	uc := new(CheckUserCollection)
	shouldRetry := false

	for _, item := range resp {
		// Consultar LID Store para obter LID associado ao JID
		var lidStr *string
		if client.Store.LIDs != nil {
			if lid, err := client.Store.LIDs.GetLIDForPN(context.TODO(), item.JID); err == nil && !lid.IsEmpty() {
				lidString := fmt.Sprintf("%v", lid)
				lidStr = &lidString
			}
		}

		// Determine the RemoteJID to use for messaging
		remoteJID := item.Query // Default to original query
		if item.IsIn {
			// When user exists on WhatsApp, use the JID returned by WhatsApp
			remoteJID = fmt.Sprintf("%v", item.JID)
		} else if formatJid {
			// If user not found and we used formatJid=true, we should retry with formatJid=false
			shouldRetry = true
		}

		if item.VerifiedName != nil {
			var msg = User{
				Query:        item.Query,
				IsInWhatsapp: item.IsIn,
				JID:          fmt.Sprintf("%v", item.JID),
				RemoteJID:    remoteJID,
				LID:          lidStr,
				VerifiedName: item.VerifiedName.Details.GetVerifiedName(),
			}
			uc.Users = append(uc.Users, msg)
		} else {
			var msg = User{
				Query:        item.Query,
				IsInWhatsapp: item.IsIn,
				JID:          fmt.Sprintf("%v", item.JID),
				RemoteJID:    remoteJID,
				LID:          lidStr,
				VerifiedName: "",
			}
			uc.Users = append(uc.Users, msg)
		}
	}

	return uc, shouldRetry
}

// mergeCheckUserResults merges results from two CheckUser attempts
// Priority: if a user is found in retry (formatJid=false), use that result
func (u *userService) mergeCheckUserResults(original, retry *CheckUserCollection) *CheckUserCollection {
	if retry == nil {
		return original
	}

	// Create a map of retry results by original query for quick lookup
	retryMap := make(map[string]User)
	for _, user := range retry.Users {
		retryMap[user.Query] = user
	}

	// Merge results
	merged := &CheckUserCollection{}
	for _, originalUser := range original.Users {
		if retryUser, exists := retryMap[originalUser.Query]; exists && retryUser.IsInWhatsapp && !originalUser.IsInWhatsapp {
			// Use retry result if it found the user and original didn't
			merged.Users = append(merged.Users, retryUser)
		} else {
			// Use original result
			merged.Users = append(merged.Users, originalUser)
		}
	}

	return merged
}

func (u *userService) GetAvatar(data *GetAvatarStruct, instance *instance_model.Instance) (*types.ProfilePictureInfo, error) {
	client, err := u.ensureClientConnected(instance.Id)
	if err != nil {
		return nil, err
	}

	// 🔒 FIX: Verificar se o cliente está conectado antes de fazer a requisição
	if !client.IsConnected() {
		return nil, errors.New("client is not connected to WhatsApp")
	}

	// 🔒 FIX: Verificar se o cliente está autenticado
	if !client.IsLoggedIn() {
		return nil, errors.New("client is not logged in to WhatsApp")
	}

	jid, ok := utils.ParseJID(data.Number)
	if !ok {
		return nil, errors.New("invalid phone number")
	}

	u.loggerWrapper.GetLogger(instance.Id).LogInfo("[%s] Requesting avatar for JID: %s, Preview: %v", instance.Id, jid, data.Preview)

	var pic *types.ProfilePictureInfo

	// 🔒 FIX: Adicionar timeout ao contexto para evitar que a requisição trave indefinidamente
	// Usar timeout maior que o padrão do sendIQ (75s) para dar tempo suficiente
	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Second)
	defer cancel()

	u.loggerWrapper.GetLogger(instance.Id).LogInfo("[%s] Starting GetProfilePictureInfo request...", instance.Id)
	pic, err = client.GetProfilePictureInfo(ctx, jid, &whatsmeow.GetProfilePictureParams{
		Preview: data.Preview,
	})
	if err != nil {
		u.loggerWrapper.GetLogger(instance.Id).LogError("[%s] GetProfilePictureInfo failed: %v", instance.Id, err)
		return nil, err
	}

	if pic == nil {
		return nil, errors.New("no profile picture found")
	}

	u.loggerWrapper.GetLogger(instance.Id).LogInfo("[%s] Got avatar %s", instance.Id, pic.URL)

	return pic, nil
}

func (u *userService) GetContacts(instance *instance_model.Instance) ([]ContactInfo, error) {
	client, err := u.ensureClientConnected(instance.Id)
	if err != nil {
		return nil, err
	}

	contacts, err := client.Store.Contacts.GetAllContacts(context.Background())
	if err != nil {
		return nil, err
	}

	var contactsArray []ContactInfo

	for jid, contact := range contacts {
		contactsArray = append(contactsArray, ContactInfo{
			Jid:          jid.String(),
			Found:        contact.Found,
			FirstName:    contact.FirstName,
			FullName:     contact.FullName,
			PushName:     contact.PushName,
			BusinessName: contact.BusinessName,
		})
	}

	return contactsArray, nil

}

// SaveContact adiciona/atualiza um contato na lista de contatos do WhatsApp da
// instância, via app state patch (coleção critical_unblock_low, índice "contact").
// É o mesmo mecanismo que o WhatsApp Web usa na tela "Novo contato".
//
// Quando SaveOnPhone=true, seta ContactAction.SaveOnPrimaryAddressbook, que faz o
// dispositivo primário (celular) gravar o contato também na agenda do sistema —
// equivale ao toggle "Sincronizar contato com celular".
//
// Requisitos: as app state keys precisam já estar sincronizadas (acontece após o
// pareamento) e o celular primário precisa estar online para propagar a mudança.
func (u *userService) SaveContact(data *SaveContactStruct, instance *instance_model.Instance) error {
	client, err := u.ensureClientConnected(instance.Id)
	if err != nil {
		return err
	}

	jid, ok := utils.ParseJID(data.Number)
	if !ok {
		return errors.New("invalid phone number")
	}
	// Normaliza o JID removendo o '+' inicial para ficar idêntico ao padrão dos
	// demais contatos (556284875027@s.whatsapp.net). Com o '+' o WhatsApp aceita
	// o app state mas o dispositivo primário não grava na agenda do sistema.
	jid.User = strings.ReplaceAll(jid.User, "+", "")

	if data.FullName == "" {
		return errors.New("fullName is required")
	}

	firstName := data.FirstName
	if firstName == "" {
		// Usa a primeira palavra do nome completo como fallback.
		if idx := strings.IndexByte(data.FullName, ' '); idx > 0 {
			firstName = data.FullName[:idx]
		} else {
			firstName = data.FullName
		}
	}

	// Por padrão sincroniza com a agenda do celular (toggle do WhatsApp Web).
	saveOnPhone := true
	if data.SaveOnPhone != nil {
		saveOnPhone = *data.SaveOnPhone
	}

	patch := appstate.PatchInfo{
		Type: appstate.WAPatchCriticalUnblockLow,
		Mutations: []appstate.MutationInfo{{
			Index:   []string{appstate.IndexContact, jid.String()},
			Version: 2,
			Value: &waSyncAction.SyncActionValue{
				ContactAction: &waSyncAction.ContactAction{
					FullName:                 proto.String(data.FullName),
					FirstName:                proto.String(firstName),
					SaveOnPrimaryAddressbook: proto.Bool(saveOnPhone),
				},
			},
		}},
	}

	if err := client.SendAppState(context.Background(), patch); err != nil {
		u.loggerWrapper.GetLogger(instance.Id).LogError("[%s] Error saving contact %s: %v", instance.Id, jid.String(), err)
		return err
	}

	u.loggerWrapper.GetLogger(instance.Id).LogInfo("[%s] Contact saved: %s (%s)", instance.Id, data.FullName, jid.String())
	return nil
}

func (u *userService) GetPrivacy(instance *instance_model.Instance) (types.PrivacySettings, error) {
	client, err := u.ensureClientConnected(instance.Id)
	if err != nil {
		return types.PrivacySettings{}, err
	}

	privacy := client.GetPrivacySettings(context.Background())

	return privacy, nil
}

func (u *userService) SetPrivacy(data *PrivacyStruct, instance *instance_model.Instance) (*types.PrivacySettings, error) {
	client, err := u.ensureClientConnected(instance.Id)
	if err != nil {
		return nil, err
	}

	privacySettings := []struct {
		name  types.PrivacySettingType
		value types.PrivacySetting
	}{
		{types.PrivacySettingTypeGroupAdd, data.GroupAdd},
		{types.PrivacySettingTypeLastSeen, data.LastSeen},
		{types.PrivacySettingTypeStatus, data.Status},
		{types.PrivacySettingTypeProfile, data.Profile},
		{types.PrivacySettingTypeReadReceipts, data.ReadReceipts},
		{types.PrivacySettingTypeCallAdd, data.CallAdd},
		{types.PrivacySettingTypeOnline, data.Online},
	}

	for _, setting := range privacySettings {
		_, err := client.SetPrivacySetting(context.Background(), setting.name, setting.value)
		if err != nil {
			return nil, err
		}
	}

	privacy := client.GetPrivacySettings(context.Background())

	return &privacy, nil
}

func (u *userService) BlockContact(data *BlockStruct, instance *instance_model.Instance) (*types.Blocklist, error) {
	client, err := u.ensureClientConnected(instance.Id)
	if err != nil {
		return nil, err
	}

	jid, ok := utils.ParseJID(data.Number)
	if !ok {
		return nil, errors.New("invalid phone number")
	}

	resp, err := client.UpdateBlocklist(context.Background(), jid, events.BlocklistChangeActionBlock)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (u *userService) UnlockContact(data *BlockStruct, instance *instance_model.Instance) (*types.Blocklist, error) {
	client, err := u.ensureClientConnected(instance.Id)
	if err != nil {
		return nil, err
	}

	jid, ok := utils.ParseJID(data.Number)
	if !ok {
		return nil, errors.New("invalid phone number")
	}

	resp, err := client.UpdateBlocklist(context.Background(), jid, events.BlocklistChangeActionUnblock)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (u *userService) GetBlockList(instance *instance_model.Instance) (*types.Blocklist, error) {
	client, err := u.ensureClientConnected(instance.Id)
	if err != nil {
		return nil, err
	}

	resp, err := client.GetBlocklist(context.Background())
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (u *userService) SetProfilePicture(data *SetProfilePictureStruct, instance *instance_model.Instance) (bool, error) {
	client, err := u.ensureClientConnected(instance.Id)
	if err != nil {
		return false, err
	}

	var filedata []byte

	resp, err := http.Get(data.Image)
	if err != nil {
		return false, fmt.Errorf("failed to fetch image from URL: %v", err)
	}
	defer resp.Body.Close()

	filedata, err = io.ReadAll(resp.Body)
	if err != nil {
		return false, fmt.Errorf("failed to read image data: %v", err)
	}

	_, err = client.SetGroupPhoto(context.Background(), types.EmptyJID, filedata)
	if err != nil {
		return false, err
	}

	return true, nil
}

func (u *userService) SetProfileName(data *SetProfileNameStruct, instance *instance_model.Instance) (bool, error) {
	client, err := u.ensureClientConnected(instance.Id)
	if err != nil {
		return false, err
	}

	err = client.SetGroupName(context.Background(), types.EmptyJID, data.Name)
	if err != nil {
		return false, err
	}

	return true, nil
}

func (u *userService) SetProfileStatus(data *SetProfileStatusStruct, instance *instance_model.Instance) (bool, error) {
	client, err := u.ensureClientConnected(instance.Id)
	if err != nil {
		return false, err
	}

	err = client.SetStatusMessage(context.Background(), data.Status)
	if err != nil {
		return false, err
	}

	return true, nil
}

func NewUserService(
	clientPointer map[string]*whatsmeow.Client,
	whatsmeowService whatsmeow_service.WhatsmeowService,
	loggerWrapper *logger_wrapper.LoggerManager,
) UserService {
	return &userService{
		clientPointer:    clientPointer,
		whatsmeowService: whatsmeowService,
		loggerWrapper:    loggerWrapper,
	}
}
