package routes

import (
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRegisterManagerV2Routes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	registerManagerV2Routes(engine)

	routes := make(map[string]bool)
	for _, route := range engine.Routes() {
		routes[route.Method+" "+route.Path] = true
	}

	for _, expected := range []string{
		"GET /manager-v2",
		"GET /manager-v2/",
		"GET /manager-v2/assets/*filepath",
		"HEAD /manager-v2/assets/*filepath",
	} {
		if !routes[expected] {
			t.Fatalf("missing route %s; registered routes: %#v", expected, routes)
		}
	}
}
