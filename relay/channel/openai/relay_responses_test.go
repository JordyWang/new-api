package openai

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestOaiResponsesStreamToNonStreamHandlerReturnsCompletedResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	completed := `{"id":"resp_1","object":"response","status":"completed","model":"gpt-5.6-sol","output":[{"type":"message","id":"msg_1","status":"completed","role":"assistant","content":[{"type":"output_text","text":"ok","annotations":[]}]}],"usage":{"input_tokens":3,"output_tokens":2,"total_tokens":5}}`
	streamBody := strings.Join([]string{
		`data: {"type":"response.created","response":{"id":"resp_1","status":"in_progress"}}`,
		"",
		`data: {"type":"response.output_text.delta","delta":"ok"}`,
		"",
		`data: {"type":"response.completed","response":` + completed + `}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")
	upstream := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(streamBody)),
	}

	usage, relayErr := OaiResponsesStreamToNonStreamHandler(ctx, &relaycommon.RelayInfo{}, upstream)

	require.Nil(t, relayErr)
	require.NotNil(t, usage)
	require.Equal(t, 3, usage.PromptTokens)
	require.Equal(t, 2, usage.CompletionTokens)
	require.Equal(t, 5, usage.TotalTokens)
	require.Equal(t, "application/json", recorder.Header().Get("Content-Type"))
	require.JSONEq(t, completed, recorder.Body.String())
}

func TestOaiResponsesStreamToNonStreamHandlerReturnsStreamError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	upstream := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body: io.NopCloser(strings.NewReader(
			`data: {"type":"error","error":{"message":"upstream failed","type":"server_error","code":"server_error"}}` + "\n\n",
		)),
	}

	usage, relayErr := OaiResponsesStreamToNonStreamHandler(ctx, &relaycommon.RelayInfo{}, upstream)

	require.Nil(t, usage)
	require.NotNil(t, relayErr)
	require.Equal(t, http.StatusBadGateway, relayErr.StatusCode)
	require.Equal(t, "upstream failed", relayErr.Error())
	require.Empty(t, recorder.Body.String())
}
