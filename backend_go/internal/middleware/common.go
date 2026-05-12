package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		if origin != "" {
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
		} else {
			c.Writer.Header().Set("Access-Control-Allow-Origin", "http://localhost:5173")
		}
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With, X-Company-Id, X-Skeleton-Parent-Id")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, PATCH, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

func CompanyID() gin.HandlerFunc {
	return func(c *gin.Context) {
		companyID := c.GetHeader("X-Company-Id")
		// Also support query param for iframe requests or secondary loaders
		if companyID == "" || companyID == "undefined" || companyID == "null" {
			companyID = c.Query("companyId")
		}

		if companyID == "" || companyID == "undefined" || companyID == "null" {
			companyID = "c1" // Default for development
		}
		c.Set("companyID", companyID)

		requestID := c.GetHeader("X-Request-Id")
		if requestID == "" {
			requestID = uuid.New().String()
		}
		c.Set("requestID", requestID)
		c.Writer.Header().Set("X-Request-ID", requestID)

		c.Next()
	}
}
