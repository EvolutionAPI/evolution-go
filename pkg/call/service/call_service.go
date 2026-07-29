package call_service

import (
	"errors"

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
