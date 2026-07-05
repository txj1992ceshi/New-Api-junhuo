package router

import (
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/gin-gonic/gin"
)

func SetClientRouter(router *gin.Engine) {
	clientRoute := router.Group("/v1/client")
	clientRoute.Use(middleware.RouteTag("client"))
	{
		clientRoute.POST("/auth/login", controller.JunhuoCodexClientLogin)
		clientRoute.POST("/activation/activate", controller.JunhuoCodexEntitlementVerify)
		clientRoute.POST("/entitlements/verify", controller.JunhuoCodexEntitlementVerify)
	}
}
