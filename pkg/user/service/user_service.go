package user_service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	instance_model "github.com/evolution-foundation/evolution-go/pkg/instance/model"
	logger_wrapper "github.com/evolution-foundation/evolution-go/pkg/logger"
	"github.com/evolution-foundation/evolution-go/pkg/utils"
	whatsmeow_service "github.com/evolution-foundation/evolution-go/pkg/whatsmeow/service"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

// avatarRequestTimeout bounds POST /user/avatar so clients (e.g. Chatwoot at 12s)
// get a clear HTTP error instead of a hung connection waiting for the ~75s IQ default.
const avatarRequestTimeout = 8 * time.Second

// clientReadyWait is the max time to wait after StartInstance before failing.
const clientReadyWait = 2 * time.Second

type UserService interface {
	GetUser(data *CheckUserStruct, instance *instance_model.Instance) (*UserCollection, error)
	CheckUser(data *CheckUserStruct, instance *instance_model.Instance) (*CheckUserCollection, error)
	GetAvatar(ctx context.Context, data *GetAvatarStruct, instance *instance_model.Instance) (*types.ProfilePictureInfo, error)
	GetContacts(instance *instance_model.Instance) ([]ContactInfo, error)
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
	PictureURL   string
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
	return u.ensureClientConnectedCtx(context.Background(), instanceId)
}

func (u *userService) ensureClientConnectedCtx(ctx context.Context, instanceId string) (*whatsmeow.Client, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	client := u.clientPointer[instanceId]
	u.loggerWrapper.GetLogger(instanceId).LogInfo("[%s] Checking client connection status - Client exists: %v", instanceId, client != nil)

	if client == nil {
		u.loggerWrapper.GetLogger(instanceId).LogInfo("[%s] No client found, attempting to start new instance", instanceId)
		err := u.whatsmeowService.StartInstance(instanceId)
		if err != nil {
			u.loggerWrapper.GetLogger(instanceId).LogError("[%s] Failed to start instance: %v", instanceId, err)
			return nil, errors.New("no active session found")
		}

		u.loggerWrapper.GetLogger(instanceId).LogInfo("[%s] Instance started, waiting up to %s for connection...", instanceId, clientReadyWait)
		client, err = u.waitForClientReady(ctx, instanceId, clientReadyWait)
		if err != nil {
			u.loggerWrapper.GetLogger(instanceId).LogError("[%s] New client validation failed: %v", instanceId, err)
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

func (u *userService) waitForClientReady(ctx context.Context, instanceId string, maxWait time.Duration) (*whatsmeow.Client, error) {
	deadline := time.Now().Add(maxWait)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		client := u.clientPointer[instanceId]
		if client != nil && client.IsConnected() {
			return client, nil
		}
		if time.Now().After(deadline) {
			return nil, errors.New("client not ready within wait window")
		}
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("waiting for client: %w", ctx.Err())
		case <-ticker.C:
		}
	}
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
		// usync IQ also requires a digits-only user JID (no "+" prefix).
		jids = append(jids, utils.CanonicalJID(jid).ToNonAD())
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

		pictureURL := ""
		pic, picErr := u.fetchProfilePicture(context.Background(), client, jid, true, 25*time.Second)
		if picErr != nil {
			u.loggerWrapper.GetLogger(instance.Id).LogWarn("[%s] Failed to enrich PictureURL for %s: %v", instance.Id, jid, picErr)
		} else if pic != nil {
			pictureURL = pic.URL
		}

		// Converter para nossa estrutura UserInfo que inclui LID
		info := UserInfo{
			VerifiedName: whatsmeowInfo.VerifiedName,
			Status:       whatsmeowInfo.Status,
			PictureID:    whatsmeowInfo.PictureID,
			PictureURL:   pictureURL,
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

// fetchProfilePicture requests a profile picture URL for jid.
// Never pass ExistingID here: when the picture is unchanged whatsmeow returns
// nil with no error and no URL.
func (u *userService) fetchProfilePicture(parent context.Context, client *whatsmeow.Client, jid types.JID, preview bool, timeout time.Duration) (*types.ProfilePictureInfo, error) {
	jid = utils.CanonicalJID(jid).ToNonAD()
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()

	pic, err := client.GetProfilePictureInfo(ctx, jid, &whatsmeow.GetProfilePictureParams{
		Preview: preview,
	})
	if err != nil {
		return nil, fmt.Errorf("get profile picture for %s: %w", jid, err)
	}
	return pic, nil
}

func (u *userService) GetAvatar(ctx context.Context, data *GetAvatarStruct, instance *instance_model.Instance) (*types.ProfilePictureInfo, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	client, err := u.ensureClientConnectedCtx(ctx, instance.Id)
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
	// Profile picture IQ is a RAW node (Target=jid). CreateJID/ParseJID may
	// prefix "+" which WhatsApp does not accept on this path — same class of
	// bug as typing/receipts (see utils.CanonicalJID).
	jid = utils.CanonicalJID(jid).ToNonAD()
	// Prefer PN JID when the store knows the mapping for @lid.
	if jid.Server == types.HiddenUserServer && client.Store.LIDs != nil {
		if pn, lidErr := client.Store.LIDs.GetPNForLID(ctx, jid); lidErr == nil && !pn.IsEmpty() {
			u.loggerWrapper.GetLogger(instance.Id).LogInfo("[%s] Resolved LID %s to PN %s for avatar", instance.Id, jid, pn)
			jid = utils.CanonicalJID(pn).ToNonAD()
		}
	}

	u.loggerWrapper.GetLogger(instance.Id).LogInfo("[%s] Requesting avatar for JID: %s, Preview: %v", instance.Id, jid, data.Preview)
	u.loggerWrapper.GetLogger(instance.Id).LogInfo("[%s] Starting GetProfilePictureInfo request...", instance.Id)

	pic, err := u.fetchProfilePicture(ctx, client, jid, data.Preview, avatarRequestTimeout)
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
