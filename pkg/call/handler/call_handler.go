package call_handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	call_service "github.com/evolution-foundation/evolution-go/pkg/call/service"
	call_browser "github.com/evolution-foundation/evolution-go/pkg/call/voip/browser"
	instance_model "github.com/evolution-foundation/evolution-go/pkg/instance/model"
	managerauth "github.com/evolution-foundation/evolution-go/pkg/managerauth"
	"github.com/gin-gonic/gin"
)

type CallHandler interface {
	StartCall(ctx *gin.Context)
	AcceptCall(ctx *gin.Context)
	TerminateCall(ctx *gin.Context)
	RejectCall(ctx *gin.Context)
	Status(ctx *gin.Context)
	History(ctx *gin.Context)
	CreateWebRTC(ctx *gin.Context)
	ListWebRTC(ctx *gin.Context)
	CloseWebRTC(ctx *gin.Context)
}

type callHandler struct {
	callService call_service.CallService
	managerAuth *managerauth.Service
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
// @Summary Start an experimental WhatsApp call
// @Description Sends a real WhatsApp call offer and prepares the experimental media pipeline.
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

// Accept call
// @Summary Accept an incoming WhatsApp call
// @Description Sends preaccept and accept signaling for a prepared incoming call.
// @Tags Call
// @Produce json
// @Param callId path string true "Call ID"
// @Success 200 {object} gin.H "Call accepted"
// @Failure 400 {object} gin.H "Invalid request"
// @Failure 500 {object} gin.H "Call signaling failed"
// @Router /call/{callId}/accept [post]
func (g *callHandler) AcceptCall(ctx *gin.Context) {
	instance, ok := instanceFromContext(ctx)
	if !ok {
		return
	}
	callID := ctx.Param("callId")
	if callID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "callId is required"})
		return
	}

	call, err := g.callService.AcceptCall(callID, instance, g.answerActor(ctx))
	if err != nil {
		if errors.Is(err, call_service.ErrIncomingCallNotReady) {
			ctx.JSON(http.StatusConflict, gin.H{"error": "incoming call is still being prepared"})
			return
		}
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, call)
}

// Terminate call
// @Summary Terminate a WhatsApp call
// @Description Sends a terminate stanza for a call tracked by this instance.
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

// History returns durable, public call records. Unlike /call/status, it does
// not depend on an active WhatsApp client and survives process restarts.
// @Summary Get persisted WhatsApp call history
// @Tags Call
// @Produce json
// @Param limit query int false "Maximum entries (1-500)"
// @Success 200 {array} gin.H
// @Router /call/history [get]
func (g *callHandler) History(ctx *gin.Context) {
	instance, ok := instanceFromContext(ctx)
	if !ok {
		return
	}
	limit, err := strconv.Atoi(ctx.DefaultQuery("limit", "100"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "limit must be a number"})
		return
	}
	entries, err := g.callService.History(instance, limit)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, entries)
}

func (g *callHandler) answerActor(ctx *gin.Context) string {
	if g == nil || g.managerAuth == nil || ctx == nil {
		return ""
	}
	admin, err := g.managerAuth.AdministratorFromRequest(ctx.Request.Context(), ctx.Request)
	if err != nil || admin == nil {
		return ""
	}
	if name := strings.TrimSpace(admin.Name); name != "" {
		return name
	}
	return strings.TrimSpace(admin.Email)
}

func browserHTTPStatus(err error) int {
	switch {
	case errors.Is(err, call_browser.ErrWebRTCDisabled):
		return http.StatusNotImplemented
	case errors.Is(err, call_browser.ErrInvalidOffer), errors.Is(err, call_browser.ErrInvalidPCMMessage):
		return http.StatusBadRequest
	case errors.Is(err, call_browser.ErrSessionNotFound):
		return http.StatusNotFound
	case errors.Is(err, call_browser.ErrSessionLimit), errors.Is(err, call_service.ErrCallNotActive):
		return http.StatusConflict
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		return http.StatusGatewayTimeout
	default:
		return http.StatusInternalServerError
	}
}

// Create browser WebRTC PCM session
// @Summary Create an experimental browser PCM bridge
// @Description Exchanges a complete SDP offer and answer. Requires the voip_pion build and an active WhatsApp call.
// @Tags Call
// @Accept json
// @Produce json
// @Param callId path string true "Call ID"
// @Param offer body call_browser.CreateRequest true "Browser SDP offer"
// @Success 201 {object} call_browser.CreateResponse
// @Router /call/{callId}/webrtc [post]
func (g *callHandler) CreateWebRTC(ctx *gin.Context) {
	instance, ok := instanceFromContext(ctx)
	if !ok {
		return
	}
	callID := ctx.Param("callId")
	if callID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "callId is required"})
		return
	}
	var request call_browser.CreateRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	requestContext, cancel := context.WithTimeout(ctx.Request.Context(), 30*time.Second)
	defer cancel()
	response, err := g.callService.CreateWebRTC(requestContext, callID, request, instance)
	if err != nil {
		ctx.JSON(browserHTTPStatus(err), gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusCreated, response)
}

// List browser WebRTC PCM sessions
// @Summary List browser PCM bridge sessions
// @Tags Call
// @Produce json
// @Param callId path string true "Call ID"
// @Success 200 {object} gin.H
// @Router /call/{callId}/webrtc [get]
func (g *callHandler) ListWebRTC(ctx *gin.Context) {
	instance, ok := instanceFromContext(ctx)
	if !ok {
		return
	}
	sessions, err := g.callService.WebRTCSessions(ctx.Param("callId"), instance)
	if err != nil {
		ctx.JSON(browserHTTPStatus(err), gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"sessions": sessions})
}

// Close browser WebRTC PCM session
// @Summary Close a browser PCM bridge session
// @Tags Call
// @Produce json
// @Param callId path string true "Call ID"
// @Param sessionId path string true "WebRTC session ID"
// @Success 200 {object} gin.H
// @Router /call/{callId}/webrtc/{sessionId} [delete]
func (g *callHandler) CloseWebRTC(ctx *gin.Context) {
	instance, ok := instanceFromContext(ctx)
	if !ok {
		return
	}
	if err := g.callService.CloseWebRTC(ctx.Param("callId"), ctx.Param("sessionId"), instance); err != nil {
		ctx.JSON(browserHTTPStatus(err), gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"message": "browser media session closed"})
}

func NewCallHandler(callService call_service.CallService, managerAuth *managerauth.Service) CallHandler {
	return &callHandler{callService: callService, managerAuth: managerAuth}
}
