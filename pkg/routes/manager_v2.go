package routes

import "github.com/gin-gonic/gin"

func registerManagerV2Routes(eng *gin.Engine) {
	eng.Static("/manager-v2/assets", "./manager-v2/dist/assets")
	serveIndex := func(c *gin.Context) {
		c.File("manager-v2/dist/index.html")
	}
	eng.GET("/manager-v2", serveIndex)
	eng.GET("/manager-v2/", serveIndex)
	eng.GET("/manager-v2/instances/:instanceId/settings", serveIndex)
}
