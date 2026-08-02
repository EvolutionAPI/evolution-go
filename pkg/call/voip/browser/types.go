package browser

import (
	"context"
	"errors"
	"time"
)

const (
	DataChannelLabel    = "evolution-call-pcm"
	DataChannelProtocol = "evcall.pcm.v1"
	PCMSampleRate       = 16000
	PCMChannels         = 1
	PCMFrameSamples     = 960
)

var (
	ErrWebRTCDisabled    = errors.New("browser WebRTC bridge requires the voip_pion build tag")
	ErrInvalidOffer      = errors.New("invalid WebRTC SDP offer")
	ErrSessionNotFound   = errors.New("browser WebRTC session not found")
	ErrSessionLimit      = errors.New("browser WebRTC session limit reached")
	ErrInvalidPCMMessage = errors.New("invalid browser PCM message")
)

type SDPDescription struct {
	Type string `json:"type" binding:"required"`
	SDP  string `json:"sdp" binding:"required"`
}

type CreateRequest struct {
	Offer SDPDescription `json:"offer" binding:"required"`
}

type ProtocolInfo struct {
	DataChannel  string `json:"dataChannel"`
	Protocol     string `json:"protocol"`
	Format       string `json:"format"`
	SampleRate   int    `json:"sampleRate"`
	Channels     int    `json:"channels"`
	FrameSamples int    `json:"frameSamples"`
}

type CreateResponse struct {
	SessionID string         `json:"sessionId"`
	Answer    SDPDescription `json:"answer"`
	Audio     ProtocolInfo   `json:"audio"`
}

type SessionState string

const (
	SessionStateConnecting SessionState = "connecting"
	SessionStateOpen       SessionState = "open"
	SessionStateClosed     SessionState = "closed"
	SessionStateFailed     SessionState = "failed"
)

type SessionInfo struct {
	SessionID     string       `json:"sessionId"`
	CallID        string       `json:"callId"`
	State         SessionState `json:"state"`
	ChannelOpen   bool         `json:"channelOpen"`
	CreatedAt     time.Time    `json:"createdAt"`
	InputFrames   uint64       `json:"inputFrames"`
	OutputFrames  uint64       `json:"outputFrames"`
	DroppedFrames uint64       `json:"droppedFrames"`
}

type PCMFeeder func(instanceID, callID string, pcm []float32) error

type Manager interface {
	Create(ctx context.Context, instanceID, callID string, request CreateRequest) (CreateResponse, error)
	Sessions(instanceID, callID string) ([]SessionInfo, error)
	CloseSession(instanceID, callID, sessionID string) error
	CloseCall(instanceID, callID string)
	CloseInstance(instanceID string)
	HandlePCM(instanceID, callID string, pcm []float32)
}

func DefaultProtocolInfo() ProtocolInfo {
	return ProtocolInfo{
		DataChannel:  DataChannelLabel,
		Protocol:     DataChannelProtocol,
		Format:       "f32le",
		SampleRate:   PCMSampleRate,
		Channels:     PCMChannels,
		FrameSamples: PCMFrameSamples,
	}
}
