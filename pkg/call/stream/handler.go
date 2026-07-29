// pkg/call/stream/handler.go
package call_stream

import (
	"net/http"

	call_service "github.com/evolution-foundation/evolution-go/pkg/call/service"
	instance_service "github.com/evolution-foundation/evolution-go/pkg/instance/service"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// RegisterRoutes mounts GET /call/stream/:callId directly on the engine, bypassing the
// header-based authMiddleware chain used by pkg/routes: WebSocket clients (especially
// browser-based ones) can't always set a custom apikey header on the upgrade request,
// so auth here is a query parameter instead, resolved the same way authMiddleware.Auth
// resolves it. This mirrors how pkg/passkey/handler.RegisterRoutes already registers
// its own routes directly on *gin.Engine for the same "doesn't fit the standard
// middleware chain" reason.
func RegisterRoutes(r *gin.Engine, callService call_service.CallService, instanceService instance_service.InstanceService) {
	r.GET("/call/stream/:callId", serveStream(callService, instanceService))
}

func serveStream(callService call_service.CallService, instanceService instance_service.InstanceService) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		instance, err := instanceService.GetInstanceByToken(ctx.Query("apikey"))
		if err != nil {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "not authorized"})
			return
		}

		callID := ctx.Param("callId")
		call, err := callService.GetActiveCall(instance.Id, callID)
		if err != nil {
			ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}

		conn, err := upgrader.Upgrade(ctx.Writer, ctx.Request, nil)
		if err != nil {
			return
		}

		b := newBridge(conn)
		b.sendVideo = call.SendVideo
		// StartVideo requests an audio->video upgrade. Calling it on a call that
		// already has video (declared in the original offer) doesn't just no-op --
		// live testing showed it can drop the call entirely. Only wire it up when
		// there's an actual upgrade to request.
		if !call.IsVideo() {
			b.startVideo = call.StartVideo
		}
		call.OnEnd(func(reason string) { b.Close() })
		call.Receive(b)
		call.ReceiveVideo(b)
		call.Play(b)

		b.writeStart(callID, call.IsVideo())
		b.readLoop() // blocks until the socket closes, from either end

		_ = call.Hangup()
	}
}
