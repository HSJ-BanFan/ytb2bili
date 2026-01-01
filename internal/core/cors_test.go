package core_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/difyz9/ytb2bili/internal/core"
	"github.com/difyz9/ytb2bili/internal/core/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestCORSMiddleware_AllowedOrigin(t *testing.T) {
	gin.SetMode(gin.TestMode)

	config := &types.AppConfig{
		Environment: "production",
		Security: types.SecurityConfig{
			CORSAllowedOrigins: []string{
				"http://localhost:3000",
				"https://example.com",
			},
		},
	}

	logger, _ := zap.NewDevelopment()
	server := core.NewServer(config, logger.Sugar())
	server.Init(nil)

	// Add a test endpoint
	server.Engine.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	tests := []struct {
		name           string
		origin         string
		expectedStatus int
		expectAllow    bool
	}{
		{
			name:           "Allowed origin - localhost",
			origin:         "http://localhost:3000",
			expectedStatus: http.StatusOK,
			expectAllow:    true,
		},
		{
			name:           "Allowed origin - example.com",
			origin:         "https://example.com",
			expectedStatus: http.StatusOK,
			expectAllow:    true,
		},
		{
			name:           "Disallowed origin",
			origin:         "http://evil-site.com",
			expectedStatus: http.StatusForbidden,
			expectAllow:    false,
		},
		{
			name:           "No origin header",
			origin:         "",
			expectedStatus: http.StatusOK,
			expectAllow:    true, // Same-origin requests don't have Origin header
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			w := httptest.NewRecorder()

			server.Engine.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectAllow && tt.origin != "" {
				assert.Equal(t, tt.origin, w.Header().Get("Access-Control-Allow-Origin"))
			}
		})
	}
}

func TestCORSMiddleware_PreflightRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	config := &types.AppConfig{
		Environment: "production",
		Security: types.SecurityConfig{
			CORSAllowedOrigins: []string{"http://localhost:3000"},
		},
	}

	logger, _ := zap.NewDevelopment()
	server := core.NewServer(config, logger.Sugar())
	server.Init(nil)

	// Preflight request (OPTIONS)
	req := httptest.NewRequest("OPTIONS", "/api/test", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "Content-Type, Authorization")
	w := httptest.NewRecorder()

	server.Engine.ServeHTTP(w, req)

	// Should return 204 for preflight
	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, "http://localhost:3000", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Contains(t, w.Header().Get("Access-Control-Allow-Methods"), "POST")
}

func TestCORSMiddleware_DevelopmentMode(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Development mode without configured origins
	config := &types.AppConfig{
		Environment: "development",
		Security: types.SecurityConfig{
			CORSAllowedOrigins: []string{}, // Empty in dev mode
		},
	}

	logger, _ := zap.NewDevelopment()
	server := core.NewServer(config, logger.Sugar())
	server.Init(nil)

	server.Engine.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// Any origin should be allowed in dev mode without configured origins
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "http://any-origin.com")
	w := httptest.NewRecorder()

	server.Engine.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
