//go:build !voip_pion

package browser

import "context"

type disabledManager struct{}

func NewManager(PCMFeeder) Manager {
	return &disabledManager{}
}

func (*disabledManager) Create(context.Context, string, string, CreateRequest) (CreateResponse, error) {
	return CreateResponse{}, ErrWebRTCDisabled
}

func (*disabledManager) Sessions(string, string) ([]SessionInfo, error) {
	return nil, ErrWebRTCDisabled
}

func (*disabledManager) CloseSession(string, string, string) error {
	return ErrWebRTCDisabled
}

func (*disabledManager) CloseCall(string, string)               {}
func (*disabledManager) CloseInstance(string)                   {}
func (*disabledManager) HandlePCM(string, string, []float32)    {}
