package handler

import (
	"github.com/gin-gonic/gin"
)

type Response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Message string      `json:"message,omitempty"`
}

type ErrorResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error"`
}

func Success(c *gin.Context, data interface{}, message ...string) {
	msg := "ok"
	if len(message) > 0 {
		msg = message[0]
	}
	c.JSON(200, Response{
		Success: true,
		Data:    data,
		Message: msg,
	})
}

func Error(c *gin.Context, code int, msg string) {
	c.JSON(code, ErrorResponse{
		Success: false,
		Error:   msg,
	})
}

func HealthCheck(c *gin.Context) {
	c.JSON(200, gin.H{"status": "up"})
}
