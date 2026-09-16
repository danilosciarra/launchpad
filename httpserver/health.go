package httpserver

import "github.com/gin-gonic/gin"

// healthCheck is the default handler mounted at GET /health. It always
// returns 200 with a plain-text "ok" body, which is sufficient for
// container orchestrator liveness/readiness probes. Register your own
// handler at the same path via RegisterRoutes to override it.
func healthCheck(c *gin.Context) {
	c.String(200, "ok")
}
