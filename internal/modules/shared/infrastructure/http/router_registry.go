package http

import "github.com/gin-gonic/gin"

// RouterRegistry defines the contract for registering routes in a module
type RouterRegistry interface {
	RegisterRoutes(router *gin.RouterGroup)
}
