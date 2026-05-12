package middleware

import (
	"github.com/gin-gonic/gin"
	"net/http/httputil"
	"net/url"
)

func ReverseProxy(target string) gin.HandlerFunc {
	url, _ := url.Parse(target)
	proxy := httputil.NewSingleHostReverseProxy(url)

	return func(c *gin.Context) {
		// Only proxy if it's an API call that we haven't implemented
		// Actually, we'll use this as a NoRoute handler
		proxy.ServeHTTP(c.Writer, c.Request)
	}
}

func NoRouteProxy(target string) gin.HandlerFunc {
	targetURL, _ := url.Parse(target)
	proxy := httputil.NewSingleHostReverseProxy(targetURL)

	return func(c *gin.Context) {
		c.Request.URL.Host = targetURL.Host
		c.Request.URL.Scheme = targetURL.Scheme
		c.Request.Header.Set("X-Forwarded-Host", c.Request.Header.Get("Host"))
		proxy.ServeHTTP(c.Writer, c.Request)
	}
}
