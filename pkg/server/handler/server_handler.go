package server_handler

import "github.com/gin-gonic/gin"

type ServerHandler interface {
	ServerOk(ctx *gin.Context)
}

type serverHandler struct {
}

type ServerStatus struct {
	Status string `json:"status" example:"ok"`
}

// ServerOk implements ServerHandler.
// @Summary Health check
// @Description Returns server status
// @Tags Server
// @Produce json
// @Success 200 {object} ServerStatus "Server is healthy"
// @Router /server/ok [get]
func (s *serverHandler) ServerOk(ctx *gin.Context) {
	ctx.JSON(200, ServerStatus{Status: "ok"})
}

func NewServerHandler() ServerHandler {
	return &serverHandler{}
}
