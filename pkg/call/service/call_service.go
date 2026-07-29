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

func (c *callService) ensureClientConnected(instanceId string) (*whatsmeow.Client, error) {
	client := c.clientPointer[instanceId]
	c.loggerWrapper.GetLogger(instanceId).LogInfo("[%s] Checking client connection status - Client exists: %v", instanceId, client != nil)

	if client == nil {
		c.loggerWrapper.GetLogger(instanceId).LogInfo("[%s] No client found, attempting to start new instance", instanceId)
		err := c.whatsmeowService.StartInstance(instanceId)
		if err != nil {
			c.loggerWrapper.GetLogger(instanceId).LogError("[%s] Failed to start instance: %v", instanceId, err)
			return nil, errors.New("no active session found")
		}

		c.loggerWrapper.GetLogger(instanceId).LogInfo("[%s] Instance started, waiting 2 seconds...", instanceId)
		time.Sleep(2 * time.Second)

		client = c.clientPointer[instanceId]
		c.loggerWrapper.GetLogger(instanceId).LogInfo("[%s] Checking new client - Exists: %v, Connected: %v",
			instanceId,
			client != nil,
			client != nil && client.IsConnected())

		if client == nil || !client.IsConnected() {
			c.loggerWrapper.GetLogger(instanceId).LogError("[%s] New client validation failed - Exists: %v, Connected: %v",
				instanceId,
				client != nil,
				client != nil && client.IsConnected())
			return nil, errors.New("no active session found")
		}
	} else if !client.IsConnected() {
		c.loggerWrapper.GetLogger(instanceId).LogError("[%s] Existing client is disconnected - Connected status: %v",
			instanceId,
			client.IsConnected())
		return nil, errors.New("client disconnected")
	}

	c.loggerWrapper.GetLogger(instanceId).LogInfo("[%s] Client successfully validated - Connected: %v", instanceId, client.IsConnected())
	return client, nil
}

func (c *callService) RejectCall(data *RejectCallStruct, instance *instance_model.Instance) error {
	client, err := c.ensureClientConnected(instance.Id)
	if err != nil {
		return err
	}

	err = client.RejectCall(context.Background(), data.CallCreator, data.CallID)
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

	if err := call.Answer(); err != nil {
		logger.LogError("[%s] error answering call: %v", instance.Id, err)
		return nil, err
	}

	if call.IsVideo() {
		if err := call.AcceptVideo(); err != nil {
			c.loggerWrapper.GetLogger(instance.Id).LogError("[%s] answered call but failed to accept video: %v", instance.Id, err)
		}
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
