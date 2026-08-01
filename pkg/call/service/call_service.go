package call_service

import (
	"context"
	"errors"
	"fmt"
	"time"

	call_lifecycle "github.com/evolution-foundation/evolution-go/pkg/call/lifecycle"
	call_runtime "github.com/evolution-foundation/evolution-go/pkg/call/runtime"
	call_driver "github.com/evolution-foundation/evolution-go/pkg/call/voip/driver"
	instance_model "github.com/evolution-foundation/evolution-go/pkg/instance/model"
	logger_wrapper "github.com/evolution-foundation/evolution-go/pkg/logger"
	"github.com/evolution-foundation/evolution-go/pkg/utils"
	whatsmeow_service "github.com/evolution-foundation/evolution-go/pkg/whatsmeow/service"
	"github.com/gomessguii/logger"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
)

const signalingTimeout = 30 * time.Second

type CallService interface {
	StartCall(data *StartCallStruct, instance *instance_model.Instance) (call_runtime.Call, error)
	AcceptCall(callID string, instance *instance_model.Instance) (call_runtime.Call, error)
	TerminateCall(callID string, instance *instance_model.Instance) (call_runtime.Call, error)
	RejectCall(data *RejectCallStruct, instance *instance_model.Instance) error
	RuntimeStatus(instance *instance_model.Instance) (call_runtime.Snapshot, error)
}

type callService struct {
	clientPointer    map[string]*whatsmeow.Client
	whatsmeowService whatsmeow_service.WhatsmeowService
	loggerWrapper    *logger_wrapper.LoggerManager
	coordinator      *call_lifecycle.Coordinator
}

type StartCallStruct struct {
	Number string `json:"number" binding:"required"`
	Video  bool   `json:"video"`
}

type RejectCallStruct struct {
	CallCreator types.JID `json:"callCreator"`
	CallID      string    `json:"callId" binding:"required"`
}

func (c *callService) ensureClientConnected(instanceID string) (*whatsmeow.Client, error) {
	client := c.clientPointer[instanceID]
	c.loggerWrapper.GetLogger(instanceID).LogInfo("[%s] Checking client connection status - Client exists: %v", instanceID, client != nil)

	if client == nil {
		c.loggerWrapper.GetLogger(instanceID).LogInfo("[%s] No client found, attempting to start new instance", instanceID)
		if err := c.whatsmeowService.StartInstance(instanceID); err != nil {
			c.loggerWrapper.GetLogger(instanceID).LogError("[%s] Failed to start instance: %v", instanceID, err)
			return nil, errors.New("no active session found")
		}

		c.loggerWrapper.GetLogger(instanceID).LogInfo("[%s] Instance started, waiting 2 seconds...", instanceID)
		time.Sleep(2 * time.Second)

		client = c.clientPointer[instanceID]
		c.loggerWrapper.GetLogger(instanceID).LogInfo("[%s] Checking new client - Exists: %v, Connected: %v",
			instanceID,
			client != nil,
			client != nil && client.IsConnected())

		if client == nil || !client.IsConnected() {
			c.loggerWrapper.GetLogger(instanceID).LogError("[%s] New client validation failed - Exists: %v, Connected: %v",
				instanceID,
				client != nil,
				client != nil && client.IsConnected())
			return nil, errors.New("no active session found")
		}
	} else if !client.IsConnected() {
		c.loggerWrapper.GetLogger(instanceID).LogError("[%s] Existing client is disconnected - Connected status: %v",
			instanceID,
			client.IsConnected())
		return nil, errors.New("client disconnected")
	}

	// Calls and messaging share the same authenticated client. Public state and
	// private call negotiation are attached idempotently to that client.
	c.coordinator.Attach(instanceID, client)

	c.loggerWrapper.GetLogger(instanceID).LogInfo("[%s] Client successfully validated - Connected: %v", instanceID, client.IsConnected())
	return client, nil
}

func (c *callService) StartCall(data *StartCallStruct, instance *instance_model.Instance) (call_runtime.Call, error) {
	client, err := c.ensureClientConnected(instance.Id)
	if err != nil {
		return call_runtime.Call{}, err
	}

	peer, ok := utils.ParseJID(data.Number)
	if !ok {
		return call_runtime.Call{}, fmt.Errorf("invalid WhatsApp number: %s", data.Number)
	}
	peer = utils.CanonicalJID(peer)
	if peer.Server != types.DefaultUserServer && peer.Server != types.HiddenUserServer {
		return call_runtime.Call{}, fmt.Errorf("calls only support individual WhatsApp users")
	}

	ctx, cancel := context.WithTimeout(context.Background(), signalingTimeout)
	defer cancel()

	result, err := call_driver.NewSignalingDriver(client).Start(ctx, peer, data.Video)
	if err != nil {
		return call_runtime.Call{}, err
	}
	defer result.Wipe()

	if err := c.coordinator.StoreOutgoing(
		instance.Id,
		result.CallID,
		result.CallKey,
		result.Peer,
		result.Creator,
		data.Video,
		result.RelayData,
	); err != nil {
		return call_runtime.Call{}, err
	}

	runtime := c.coordinator.RuntimeFor(instance.Id, client)
	video := data.Video
	runtime.Transition(
		result.CallID,
		result.Peer.String(),
		call_runtime.DirectionOutgoing,
		call_runtime.StateRinging,
		&video,
		"",
	)
	call, _ := runtime.Call(result.CallID)
	relayCount := 0
	if result.RelayData != nil {
		relayCount = len(result.RelayData.Endpoints)
	}
	c.loggerWrapper.GetLogger(instance.Id).LogInfo(
		"[%s] Call offer sent - CallID: %s, Peer: %s, Video: %v, Relays: %d",
		instance.Id,
		result.CallID,
		result.Peer.String(),
		data.Video,
		relayCount,
	)
	return call, nil
}

func (c *callService) AcceptCall(callID string, instance *instance_model.Instance) (call_runtime.Call, error) {
	client, err := c.ensureClientConnected(instance.Id)
	if err != nil {
		return call_runtime.Call{}, err
	}

	runtime := c.coordinator.RuntimeFor(instance.Id, client)
	call, ok := runtime.Call(callID)
	if !ok {
		return call_runtime.Call{}, fmt.Errorf("call %s not found", callID)
	}
	if call.Direction != call_runtime.DirectionIncoming {
		return call_runtime.Call{}, fmt.Errorf("call %s is not incoming", callID)
	}
	if call.State == call_runtime.StateEnded || call.State == call_runtime.StateFailed {
		return call_runtime.Call{}, fmt.Errorf("call %s cannot be accepted in state %s", callID, call.State)
	}

	ctx, cancel := context.WithTimeout(context.Background(), signalingTimeout)
	defer cancel()
	if err := c.coordinator.AcceptIncoming(ctx, instance.Id, callID); err != nil {
		return call_runtime.Call{}, err
	}

	runtime.Transition(callID, "", call_runtime.DirectionIncoming, call_runtime.StateConnecting, nil, "")
	call, _ = runtime.Call(callID)
	c.loggerWrapper.GetLogger(instance.Id).LogInfo("[%s] Incoming call accepted - CallID: %s", instance.Id, callID)
	return call, nil
}

func (c *callService) TerminateCall(callID string, instance *instance_model.Instance) (call_runtime.Call, error) {
	client, err := c.ensureClientConnected(instance.Id)
	if err != nil {
		return call_runtime.Call{}, err
	}

	runtime := c.coordinator.RuntimeFor(instance.Id, client)
	call, ok := runtime.Call(callID)
	if !ok {
		return call_runtime.Call{}, fmt.Errorf("call %s not found", callID)
	}
	if call.State == call_runtime.StateEnded || call.State == call_runtime.StateFailed {
		c.coordinator.RemovePrivate(instance.Id, callID)
		return call, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), signalingTimeout)
	defer cancel()

	if call.Direction == call_runtime.DirectionIncoming {
		if err := c.coordinator.TerminateIncoming(ctx, instance.Id, callID); err != nil {
			return call_runtime.Call{}, err
		}
	} else {
		peer, parseErr := types.ParseJID(call.Peer)
		if parseErr != nil || peer.IsEmpty() {
			return call_runtime.Call{}, fmt.Errorf("invalid call peer: %s", call.Peer)
		}
		if err := call_driver.NewSignalingDriver(client).EndOutgoing(ctx, callID, peer); err != nil {
			return call_runtime.Call{}, err
		}
	}

	c.coordinator.RemovePrivate(instance.Id, callID)
	runtime.Transition(callID, "", "", call_runtime.StateEnded, nil, "user_ended")
	call, _ = runtime.Call(callID)
	return call, nil
}

func (c *callService) RejectCall(data *RejectCallStruct, instance *instance_model.Instance) error {
	client, err := c.ensureClientConnected(instance.Id)
	if err != nil {
		return err
	}

	if err = client.RejectCall(context.Background(), data.CallCreator, data.CallID); err != nil {
		logger.LogError("[%s] error reject call: %v", instance.Id, err)
		return err
	}

	c.coordinator.RemovePrivate(instance.Id, data.CallID)
	runtime := c.coordinator.RuntimeFor(instance.Id, client)
	runtime.Transition(data.CallID, data.CallCreator.String(), call_runtime.DirectionIncoming, call_runtime.StateEnded, nil, "rejected")
	return nil
}

func (c *callService) RuntimeStatus(instance *instance_model.Instance) (call_runtime.Snapshot, error) {
	client, err := c.ensureClientConnected(instance.Id)
	if err != nil {
		return call_runtime.Snapshot{InstanceID: instance.Id}, err
	}

	runtime := c.coordinator.RuntimeFor(instance.Id, client)
	return runtime.Snapshot(), nil
}

func NewCallService(
	clientPointer map[string]*whatsmeow.Client,
	whatsmeowService whatsmeow_service.WhatsmeowService,
	loggerWrapper *logger_wrapper.LoggerManager,
	coordinator *call_lifecycle.Coordinator,
) CallService {
	if coordinator == nil {
		coordinator = call_lifecycle.NewCoordinator()
	}
	return &callService{
		clientPointer:    clientPointer,
		whatsmeowService: whatsmeowService,
		loggerWrapper:    loggerWrapper,
		coordinator:      coordinator,
	}
}
