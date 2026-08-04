package auth_middleware

import (
	"net/http"

	"github.com/evolution-foundation/evolution-go/pkg/config"
	instance_service "github.com/evolution-foundation/evolution-go/pkg/instance/service"
	managerauth "github.com/evolution-foundation/evolution-go/pkg/managerauth"
	"github.com/gin-gonic/gin"
)

type Middleware interface {
	Auth(ctx *gin.Context)
	AuthAdmin(ctx *gin.Context)
}

type middleware struct {
	config          *config.Config
	instanceService instance_service.InstanceService
	managerAuth     *managerauth.Service
}

func (m middleware) Auth(ctx *gin.Context) {
	token := ctx.GetHeader("apikey")
	if token == "" {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "not authorized"})
		return
	}

	instance, err := m.instanceService.GetInstanceByToken(token)
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "not authorized"})
		return
	}

	ctx.Set("instance", instance)

	ctx.Next()
}

func (m middleware) AuthAdmin(ctx *gin.Context) {
	if m.managerAuth != nil && m.managerAuth.IsAuthorized(ctx.Request.Context(), ctx.Request) {
		ctx.Next()
		return
	}

	token := ctx.GetHeader("apikey")
	if token == "" {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "not authorized"})
		return
	}

	if token != m.config.GlobalApiKey {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "not authorized"})
		return
	}

	ctx.Next()
}

func NewMiddleware(config *config.Config, instanceService instance_service.InstanceService, managerAuth *managerauth.Service) *middleware {
	return &middleware{config: config, instanceService: instanceService, managerAuth: managerAuth}
}
