package call_service

import (
	"context"
	"errors"
	"time"

	call_registry "github.com/evolution-foundation/evolution-go/pkg/call/registry"
	instance_model "github.com/evolution-foundation/evolution-go/pkg/instance/model"
	logger_wrapper "github.com/evolution-foundation/evolution-go/pkg/logger"
	whatsmeow_service "github.com/evolution-foundation/evolution-go/pkg/whatsmeow/service"
	"github.com/gomessguii/logger"
	"github.com/purpshell/meowcaller"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
)

type CallService interface {
	RejectCall(data *RejectCallStruct, instance *instance_model.Instance) error
	AnswerCall(data *AnswerCallStruct, instance *instance_model.Instance) (*meowcaller.Call, error)
	HangupCall(data *HangupCallStruct, instance *instance_model.Instance) error
	GetActiveCall(instanceId, callId string) (*meowcaller.Call, error)
	DialCall(data *DialCallStruct, instance *instance_model.Instance) (*meowcaller.Call, error)
	ReactCall(data *ReactCallStruct, instance *instance_model.Instance) error
	AddParticipant(data *AddParticipantStruct, instance *instance_model.Instance) error
	ScreenShareCall(data *ScreenShareStruct, instance *instance_model.Instance) error
	HandRaiseCall(data *HandRaiseStruct, instance *instance_model.Instance) error
}

type callService struct {
	clientPointer    map[string]*whatsmeow.Client
	whatsmeowService whatsmeow_service.WhatsmeowService
	callRegistry     *call_registry.CallRegistry
	loggerWrapper    *logger_wrapper.LoggerManager
}

type RejectCallStruct struct {
	CallCreator types.JID `json:"callCreator"`
	CallID      string    `json:"callId"`
}

type AnswerCallStruct struct {
	CallCreator types.JID `json:"callCreator"`
	CallID      string    `json:"callId"`
}

type HangupCallStruct struct {
	CallID string `json:"callId"`
}

// DialCallStruct is a spike/experimental addition, outside the reviewed
// answer-call plan: it places an OUTBOUND call rather than answering an
// inbound one. Number accepts anything meowcaller.Client.Call accepts (a
// phone number, a phone JID, or an @lid JID).
type DialCallStruct struct {
	Number string `json:"number"`
	Video  bool   `json:"video"`
}

// The following structs are all spike/experimental additions, outside the reviewed
// answer-call plan, for call-control features requested during live manual testing.

type ReactCallStruct struct {
	CallID string `json:"callId"`
	Emoji  string `json:"emoji"`
}

type AddParticipantStruct struct {
	CallID string `json:"callId"`
	Number string `json:"number"`
}

type ScreenShareStruct struct {
	CallID string `json:"callId"`
	Start  bool   `json:"start"`
}

type HandRaiseStruct struct {
	CallID string `json:"callId"`
	Raised bool   `json:"raised"`
}

func (c *callService) RejectCall(data *RejectCallStruct, instance *instance_model.Instance) error {
	call, ok := c.callRegistry.Get(instance.Id, data.CallID)
	if !ok {
		return errors.New("no pending call with that id")
	}

	err := call.Reject()
	c.callRegistry.Delete(data.CallID)
	if err != nil {
		logger.LogError("[%s] error reject call: %v", instance.Id, err)
		return err
	}

	return nil
}

func (c *callService) AnswerCall(data *AnswerCallStruct, instance *instance_model.Instance) (*meowcaller.Call, error) {
	call, ok := c.callRegistry.Get(instance.Id, data.CallID)
	if !ok {
		return nil, errors.New("no pending call with that id")
	}

	// Answer negotiates media for whatever the offer already declared (audio, or
	// audio+video if the call started as a video call) — no separate step needed.
	// AcceptVideo is for a different case entirely: accepting a peer's request to
	// upgrade an in-progress audio call to video (Call.StartVideo on their side),
	// which isn't wired up here.
	if err := call.Answer(); err != nil {
		logger.LogError("[%s] error answering call: %v", instance.Id, err)
		return nil, err
	}

	return call, nil
}

func (c *callService) HangupCall(data *HangupCallStruct, instance *instance_model.Instance) error {
	call, ok := c.callRegistry.Get(instance.Id, data.CallID)
	if !ok {
		return errors.New("no active call with that id")
	}

	err := call.Hangup()
	c.callRegistry.Delete(data.CallID)
	if err != nil {
		logger.LogError("[%s] error hanging up call: %v", instance.Id, err)
		return err
	}
	return nil
}

func (c *callService) GetActiveCall(instanceId, callId string) (*meowcaller.Call, error) {
	call, ok := c.callRegistry.Get(instanceId, callId)
	if !ok {
		return nil, errors.New("no active call with that id")
	}
	return call, nil
}

// DialCall places an outbound call and registers it so /call/stream and
// /call/hangup can act on it just like an answered inbound call.
func (c *callService) DialCall(data *DialCallStruct, instance *instance_model.Instance) (*meowcaller.Call, error) {
	meowcallerClient, err := c.whatsmeowService.GetMeowcallerClient(instance.Id)
	if err != nil {
		return nil, err
	}

	call, err := meowcallerClient.CallWithOptions(context.Background(), data.Number, meowcaller.CallOptions{Video: data.Video})
	if err != nil {
		logger.LogError("[%s] error dialing call: %v", instance.Id, err)
		return nil, err
	}

	c.callRegistry.Store(instance.Id, call)

	return call, nil
}

func (c *callService) ReactCall(data *ReactCallStruct, instance *instance_model.Instance) error {
	call, ok := c.callRegistry.Get(instance.Id, data.CallID)
	if !ok {
		return errors.New("no active call with that id")
	}
	if err := call.SendReaction(data.Emoji); err != nil {
		logger.LogError("[%s] error sending call reaction: %v", instance.Id, err)
		return err
	}
	return nil
}

func (c *callService) AddParticipant(data *AddParticipantStruct, instance *instance_model.Instance) error {
	call, ok := c.callRegistry.Get(instance.Id, data.CallID)
	if !ok {
		return errors.New("no active call with that id")
	}
	// Spike/experimental: live testing showed the usync lookup AddParticipant does
	// internally reliably times out while a call is already active (it works fine
	// with no call in progress), even for numbers resolved moments earlier in the
	// same session. Retrying with a short backoff to see whether it's transient
	// network contention rather than a hard block.
	var err error
	for attempt := 1; attempt <= 3; attempt++ {
		err = call.AddParticipant(context.Background(), data.Number)
		if err == nil {
			return nil
		}
		c.loggerWrapper.GetLogger(instance.Id).LogError("[%s] add participant attempt %d/3 failed: %v", instance.Id, attempt, err)
		if attempt < 3 {
			time.Sleep(2 * time.Second)
		}
	}
	logger.LogError("[%s] error adding call participant after 3 attempts: %v", instance.Id, err)
	return err
}

func (c *callService) ScreenShareCall(data *ScreenShareStruct, instance *instance_model.Instance) error {
	call, ok := c.callRegistry.Get(instance.Id, data.CallID)
	if !ok {
		return errors.New("no active call with that id")
	}
	var err error
	if data.Start {
		err = call.StartScreenShare(nil)
	} else {
		err = call.StopScreenShare()
	}
	if err != nil {
		logger.LogError("[%s] error toggling call screen share: %v", instance.Id, err)
		return err
	}
	return nil
}

func (c *callService) HandRaiseCall(data *HandRaiseStruct, instance *instance_model.Instance) error {
	call, ok := c.callRegistry.Get(instance.Id, data.CallID)
	if !ok {
		return errors.New("no active call with that id")
	}
	if err := call.SetHandRaised(data.Raised); err != nil {
		logger.LogError("[%s] error setting call hand-raise state: %v", instance.Id, err)
		return err
	}
	return nil
}

func NewCallService(
	clientPointer map[string]*whatsmeow.Client,
	whatsmeowService whatsmeow_service.WhatsmeowService,
	callRegistry *call_registry.CallRegistry,
	loggerWrapper *logger_wrapper.LoggerManager,
) CallService {
	return &callService{
		clientPointer:    clientPointer,
		whatsmeowService: whatsmeowService,
		callRegistry:     callRegistry,
		loggerWrapper:    loggerWrapper,
	}
}
