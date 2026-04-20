package middleware

import (
	"github.com/gin-gonic/gin"
)

func AdminRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get(UserRoleKey)
		if !exists || role != "admin" {
			c.JSON(403, gin.H{"code": "FORBIDDEN", "message": "admin access required"})
			c.Abort()
			return
		}
		c.Next()
	}
}
