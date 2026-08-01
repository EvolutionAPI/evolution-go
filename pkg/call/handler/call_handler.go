package call_handler

import (
	"net/http"

	call_service "github.com/evolution-foundation/evolution-go/pkg/call/service"
	instance_model "github.com/evolution-foundation/evolution-go/pkg/instance/model"
	"github.com/gin-gonic/gin"
)

type CallHandler interface {
	StartCall(ctx *gin.Context)
	TerminateCall(ctx *gin.Context)
	RejectCall(ctx *gin.Context)
	Status(ctx *gin.Context)
}

type callHandler struct {
	callService call_service.CallService
}

func instanceFromContext(ctx *gin.Context) (*instance_model.Instance, bool) {
	value, exists := ctx.Get("instance")
	if !exists {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "instance not found"})
		return nil, false
	}
	instance, ok := value.(*instance_model.Instance)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "instance not found"})
		return nil, false
	}
	return instance, true
}

// Start call
// @Summary Start an experimental signaling-only WhatsApp call
// @Description Sends a real WhatsApp call offer. Audio transport is not implemented yet.
// @Tags Call
// @Accept json
// @Produce json
// @Param message body call_service.StartCallStruct true "Call data"
// @Success 201 {object} gin.H "Call created"
// @Failure 400 {object} gin.H "Invalid request"
// @Failure 500 {object} gin.H "Call signaling failed"
// @Router /call/start [post]
func (g *callHandler) StartCall(ctx *gin.Context) {
	instance, ok := instanceFromContext(ctx)
	if !ok {
		return
	}

	var data call_service.StartCallStruct
	if err := ctx.ShouldBindJSON(&data); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	call, err := g.callService.StartCall(&data, instance)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusCreated, call)
}

// Terminate call
// @Summary Terminate an outgoing WhatsApp call
// @Description Sends a terminate stanza for an outgoing call tracked by this instance
// @Tags Call
// @Produce json
// @Param callId path string true "Call ID"
// @Success 200 {object} gin.H "Call terminated"
// @Failure 404 {object} gin.H "Call not found"
// @Failure 500 {object} gin.H "Call signaling failed"
// @Router /call/{callId} [delete]
func (g *callHandler) TerminateCall(ctx *gin.Context) {
	instance, ok := instanceFromContext(ctx)
	if !ok {
		return
	}
	callID := ctx.Param("callId")
	if callID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "callId is required"})
		return
	}

	call, err := g.callService.TerminateCall(callID, instance)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, call)
}

// Reject call
// @Summary Reject call
// @Description Reject call
// @Tags Call
// @Accept json
// @Produce json
// @Param message body call_service.RejectCallStruct true "Call data"
// @Success 200 {object} gin.H "success"
// @Failure 500 {object} gin.H "Internal server error"
// @Router /call/reject [post]
func (g *callHandler) RejectCall(ctx *gin.Context) {
	instance, ok := instanceFromContext(ctx)
	if !ok {
		return
	}

	var data call_service.RejectCallStruct
	if err := ctx.ShouldBindJSON(&data); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := g.callService.RejectCall(&data, instance); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "success"})
}

// Runtime status
// @Summary Get VoIP runtime status
// @Description Returns the VoIP runtime attached to the authenticated Evolution instance
// @Tags Call
// @Produce json
// @Success 200 {object} gin.H "VoIP runtime status"
// @Failure 500 {object} gin.H "Internal server error"
// @Router /call/status [get]
func (g *callHandler) Status(ctx *gin.Context) {
	instance, ok := instanceFromContext(ctx)
	if !ok {
		return
	}

	status, err := g.callService.RuntimeStatus(instance)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, status)
}

func NewCallHandler(callService call_service.CallService) CallHandler {
	return &callHandler{callService: callService}
}
