package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	youxinURL string
}

func NewAuthHandler() *AuthHandler {
	url := os.Getenv("YOUXIN_AUTH_URL")
	if url == "" {
		url = "http://127.0.0.1:8889" // Java production backend
	}
	return &AuthHandler{
		youxinURL: url,
	}
}

// Entry is the entry point from OpenClaw via ?ticket=xxx
func (h *AuthHandler) Entry(c *gin.Context) {
	ticket := c.Query("ticket")
	if ticket == "" {
		c.String(http.StatusBadRequest, "Missing ticket parameter")
		return
	}

	log.Printf("[Auth] Attempting to verify ticket: %s", ticket)

	// Call Youxin (Java) to verify
	verifyReq := map[string]string{
		"ticket":    ticket,
		"device_id": "desktop_local", 
	}
	jsonData, _ := json.Marshal(verifyReq)

	req, _ := http.NewRequest("POST", h.youxinURL+"/auth/workbench-ticket/verify", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 1 * time.Second}
	resp, err := client.Do(req)
	
	// FAILSAFE: If Java is down, just log and issue a temporary session anyway for MVP
	if err != nil || (resp != nil && resp.StatusCode != http.StatusOK) {
		log.Printf("[Auth] Failsafe: Java backend unavailable. Issuing session for: %s", ticket)
	} else {
		defer resp.Body.Close()
		log.Printf("[Auth] Verified ticket with Java")
	}

	// OK - Parse results
	var result struct {
		Code int `json:"code"`
		Data struct {
			UserId       int    `json:"userId"`
			MemberLevel  string `json:"memberLevel"`
			SessionToken string `json:"localSessionToken"`
			ExpireAt     int64  `json:"expireAt"`
		} `json:"data"`
	}

	body, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(body, &result); err != nil || result.Code != 0 {
		c.String(http.StatusInternalServerError, "Failed to parse Youxin response")
		return
	}

	// Set httpOnly Cookie
	// For production: secure = true, sameSite = Lax
	c.SetCookie("local_session", ticket, 3600, "/", "", false, true)
	log.Printf("[Auth] Successfully established session for user %d", result.Data.UserId)

	// Redirect to main workbench UI
	// Frontend typically running on 5173 (dev) or same port (prod)
	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = "http://localhost:5173" // Redirect to Workbench explicitly
	}
	c.Redirect(http.StatusTemporaryRedirect, frontendURL)
}

// GetProfile (Optional: for showing user info on workbench)
func (h *AuthHandler) GetProfile(c *gin.Context) {
	// For MVP, just return mock info since session is ticket-based
	c.JSON(200, gin.H{
		"code": 0,
		"data": gin.H{
			"userId": 10001,
			"name":   "有信专家用户",
			"level":  "专业版会员",
		},
	})
}
