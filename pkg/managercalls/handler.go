package managercalls

import (
	"net/http"
	"strings"
	"time"

	managerauth "github.com/evolution-foundation/evolution-go/pkg/managerauth"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

const (
	pingPeriod = 25 * time.Second
	pongWait   = 60 * time.Second
	writeWait  = 10 * time.Second
)

// Leaving CheckOrigin unset keeps Gorilla's same-origin validation in place.
// The session cookie is also SameSite=Strict, so this endpoint can only be
// opened from the authenticated Manager origin.
var managerUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type Handler struct {
	hub  *Hub
	auth *managerauth.Service
}

func NewHandler(hub *Hub, auth *managerauth.Service) *Handler {
	if hub == nil {
		hub = NewHub()
	}
	return &Handler{hub: hub, auth: auth}
}

func (h *Handler) RegisterRoutes(engine *gin.Engine) {
	group := engine.Group("/manager-v2/calls")
	group.Use(h.requireManager)
	group.GET("/events", h.Events)
}

func (h *Handler) requireManager(ctx *gin.Context) {
	if h.auth == nil || !h.auth.IsAuthorized(ctx.Request.Context(), ctx.Request) {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	ctx.Next()
}

// Events upgrades an authenticated Manager V2 request and streams only the
// selected instance's public call lifecycle changes.
func (h *Handler) Events(ctx *gin.Context) {
	instanceID := strings.TrimSpace(ctx.Query("instanceId"))
	if instanceID == "" || len(instanceID) > 200 {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "instanceId is required"})
		return
	}

	connection, err := managerUpgrader.Upgrade(ctx.Writer, ctx.Request, nil)
	if err != nil {
		return
	}
	defer connection.Close()

	subscription := h.hub.Subscribe(instanceID)
	defer subscription.Cancel()

	connection.SetReadLimit(1024)
	_ = connection.SetReadDeadline(time.Now().Add(pongWait))
	connection.SetPongHandler(func(string) error {
		return connection.SetReadDeadline(time.Now().Add(pongWait))
	})

	readerDone := make(chan struct{})
	go func() {
		defer close(readerDone)
		for {
			if _, _, readErr := connection.ReadMessage(); readErr != nil {
				return
			}
		}
	}()

	pings := time.NewTicker(pingPeriod)
	defer pings.Stop()
	for {
		select {
		case <-readerDone:
			return
		case <-subscription.Done:
			return
		case event := <-subscription.Events:
			_ = connection.SetWriteDeadline(time.Now().Add(writeWait))
			if writeErr := connection.WriteJSON(event); writeErr != nil {
				return
			}
		case <-pings.C:
			if pingErr := connection.WriteControl(websocket.PingMessage, nil, time.Now().Add(writeWait)); pingErr != nil {
				return
			}
		}
	}
}
