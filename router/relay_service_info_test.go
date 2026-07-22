package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRelayServiceInfoRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	SetRelayRouter(engine)

	for _, method := range []string{http.MethodGet, http.MethodHead} {
		t.Run(method, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(method, "/v1", nil)

			engine.ServeHTTP(recorder, request)

			assert.Equal(t, http.StatusOK, recorder.Code)
			assert.Equal(t, "application/json; charset=utf-8", recorder.Header().Get("Content-Type"))
			if method == http.MethodHead {
				assert.Empty(t, recorder.Body.String())
				return
			}

			var response struct {
				Status   string `json:"status"`
				Service  string `json:"service"`
				Protocol string `json:"protocol"`
			}
			require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
			assert.Equal(t, "ok", response.Status)
			assert.Equal(t, "new-api", response.Service)
			assert.Equal(t, "openai-compatible", response.Protocol)
			assert.NotContains(t, recorder.Body.String(), "token")
			assert.NotContains(t, recorder.Body.String(), "channel")
			assert.NotContains(t, recorder.Body.String(), "model_mapping")
		})
	}
}
