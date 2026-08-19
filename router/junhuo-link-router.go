package router

import (
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/gin-gonic/gin"
)

func SetJunhuoLinkInternalRouter(router *gin.Engine) {
	group := router.Group("/v1/internal/junhuo-link")
	group.Use(middleware.RouteTag("junhuo-link-internal"))
	group.Use(middleware.JunhuoLinkInternalAuth())
	group.Use(middleware.DisableCache())
	{
		group.GET("/health", controller.GetJunhuoLinkInternalHealth)
		group.PUT("/users/:userId/mapping", controller.PutJunhuoLinkUserMapping)
		group.DELETE("/users/:userId/mapping", controller.DeleteJunhuoLinkUserMapping)
		group.GET("/users/:userId/usage", controller.GetJunhuoLinkUserUsage)

		group.POST("/device-keys", controller.PostJunhuoLinkDeviceKey)
		group.DELETE("/device-keys/:tokenId", controller.DeleteJunhuoLinkDeviceKey)
		group.GET("/device-keys/:tokenId", controller.GetJunhuoLinkDeviceKey)
		group.GET("/device-keys/:tokenId/usage", controller.GetJunhuoLinkDeviceUsage)
		group.POST("/device-keys/:tokenId/usage", controller.PostJunhuoLinkDeviceUsage)
	}
}
