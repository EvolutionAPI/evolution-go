// Package wa adapts the Evolution whatsmeow client to the VoIP socket interface.
// Portions are derived from JotaDev66/WaCalls under the MIT license in ../LICENSE-WACALLS.
package wa

import (
	"context"
	"fmt"
	"time"

	"github.com/evolution-foundation/evolution-go/pkg/call/voip/core"
	"github.com/evolution-foundation/evolution-go/pkg/call/voip/signaling"
	"go.mau.fi/whatsmeow"
	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/types"
)

const queryTimeout = 15 * time.Second

type Socket struct {
	client *whatsmeow.Client
}

func NewSocket(client *whatsmeow.Client) *Socket {
	return &Socket{client: client}
}

var _ core.VoipSocket = (*Socket)(nil)

func (s *Socket) dangerous() *whatsmeow.DangerousInternalClient {
	return s.client.DangerousInternals()
}

func (s *Socket) OwnPN() types.JID { return s.dangerous().GetOwnID() }
func (s *Socket) OwnLID() types.JID { return s.dangerous().GetOwnLID() }

func (s *Socket) AccountDeviceIdentityNode() (waBinary.Node, bool) {
	if s.client == nil || s.client.Store == nil || s.client.Store.Account == nil {
		return waBinary.Node{}, false
	}
	return s.dangerous().MakeDeviceIdentityNode(), true
}

func (s *Socket) SendNode(ctx context.Context, node waBinary.Node) error {
	if s.client == nil {
		return fmt.Errorf("nil whatsmeow client")
	}
	return s.dangerous().SendNode(ctx, node)
}

func (s *Socket) Query(ctx context.Context, node waBinary.Node) (*waBinary.Node, error) {
	id, _ := node.Attrs["id"].(string)
	if id == "" {
		return nil, s.SendNode(ctx, node)
	}

	dangerous := s.dangerous()
	responseChannel := dangerous.WaitResponse(id)
	if err := dangerous.SendNode(ctx, node); err != nil {
		dangerous.CancelResponse(id, responseChannel)
		return nil, err
	}

	timer := time.NewTimer(queryTimeout)
	defer timer.Stop()
	select {
	case response := <-responseChannel:
		return response, nil
	case <-timer.C:
		dangerous.CancelResponse(id, responseChannel)
		return nil, nil
	case <-ctx.Done():
		dangerous.CancelResponse(id, responseChannel)
		return nil, ctx.Err()
	}
}

func (s *Socket) GetUSyncDevices(ctx context.Context, jids []types.JID) ([]types.JID, error) {
	return s.client.GetUserDevices(ctx, jids)
}

func (s *Socket) AssertSessions(context.Context, []types.JID, bool) error {
	// whatsmeow ensures Signal sessions while encrypting for the target devices.
	return nil
}

func (s *Socket) CreateParticipantNodes(ctx context.Context, devices []types.JID, callKey []byte, attrs waBinary.Attrs) ([]waBinary.Node, bool, error) {
	plaintext, err := signaling.EncodeCallKeyMessage(callKey)
	if err != nil {
		return nil, false, err
	}
	messageID := s.client.GenerateMessageID()
	return s.dangerous().EncryptMessageForDevices(ctx, devices, messageID, plaintext, plaintext, attrs)
}

func (s *Socket) DecryptCallKey(ctx context.Context, from types.JID, encrypted *waBinary.Node) ([]byte, error) {
	typeValue, _ := encrypted.Attrs["type"].(string)
	plaintext, _, err := s.dangerous().DecryptDM(ctx, encrypted, from, typeValue == "pkmsg", time.Now())
	if err != nil {
		return nil, err
	}
	return signaling.DecodeCallKeyPlaintext(plaintext)
}

func (s *Socket) GetTCToken(ctx context.Context, jid types.JID) ([]byte, error) {
	if s.client.Store == nil || s.client.Store.PrivacyTokens == nil {
		return nil, nil
	}
	candidates := []types.JID{s.ResolveLIDForPN(ctx, jid).ToNonAD(), jid.ToNonAD()}
	for _, candidate := range candidates {
		if candidate.IsEmpty() {
			continue
		}
		token, err := s.client.Store.PrivacyTokens.GetPrivacyToken(ctx, candidate)
		if err != nil {
			return nil, err
		}
		if token != nil && len(token.Token) > 0 {
			return token.Token, nil
		}
	}
	return nil, nil
}

func (s *Socket) ResolveLIDForPN(ctx context.Context, phoneNumber types.JID) types.JID {
	if phoneNumber.Server == types.HiddenUserServer {
		return phoneNumber
	}
	if s.client.Store != nil && s.client.Store.LIDs != nil {
		if lid, err := s.client.Store.LIDs.GetLIDForPN(ctx, phoneNumber); err == nil && !lid.IsEmpty() {
			return lid
		}
	}
	if userInfo, err := s.client.GetUserInfo(ctx, []types.JID{phoneNumber}); err == nil {
		if lid := userInfo[phoneNumber].LID; !lid.IsEmpty() {
			return lid
		}
	}
	return phoneNumber
}
