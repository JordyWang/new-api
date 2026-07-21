package codex

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	appconstant "github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestConvertOpenAIResponsesRequestAdaptsCodexUpstreamRequirements(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name          string
		input         string
		streamSet     bool
		stream        bool
		expectedInput string
	}{
		{
			name:          "string non-stream",
			input:         `"Hello"`,
			streamSet:     true,
			stream:        false,
			expectedInput: `[{"role":"user","content":[{"type":"input_text","text":"Hello"}]}]`,
		},
		{
			name:          "string omitted stream",
			input:         `"Hello"`,
			expectedInput: `[{"role":"user","content":[{"type":"input_text","text":"Hello"}]}]`,
		},
		{
			name:          "string stream",
			input:         `"Hello"`,
			streamSet:     true,
			stream:        true,
			expectedInput: `[{"role":"user","content":[{"type":"input_text","text":"Hello"}]}]`,
		},
		{
			name:          "array non-stream",
			input:         `[{"role":"user","content":"Hello"}]`,
			streamSet:     true,
			stream:        false,
			expectedInput: `[{"role":"user","content":"Hello"}]`,
		},
		{
			name:          "array stream",
			input:         `[{"role":"user","content":"Hello"}]`,
			streamSet:     true,
			stream:        true,
			expectedInput: `[{"role":"user","content":"Hello"}]`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, _ := gin.CreateTestContext(nil)
			var stream *bool
			if test.streamSet {
				stream = &test.stream
			}
			request := dto.OpenAIResponsesRequest{
				Model:  "gpt-5.6-sol",
				Input:  []byte(test.input),
				Stream: stream,
			}

			converted, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(ctx, &relaycommon.RelayInfo{
				RelayMode:   relayconstant.RelayModeResponses,
				ChannelMeta: &relaycommon.ChannelMeta{},
			}, request)

			require.NoError(t, err)
			actual, ok := converted.(dto.OpenAIResponsesRequest)
			require.True(t, ok)
			require.NotNil(t, actual.Stream)
			require.True(t, *actual.Stream)
			require.JSONEq(t, test.expectedInput, string(actual.Input))
		})
	}
}

func TestConvertOpenAIResponsesRequestDoesNotForceCompactionStream(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(nil)
	stream := false

	converted, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(ctx, &relaycommon.RelayInfo{
		RelayMode:   relayconstant.RelayModeResponsesCompact,
		ChannelMeta: &relaycommon.ChannelMeta{},
	}, dto.OpenAIResponsesRequest{
		Model:  "gpt-5.6-sol",
		Input:  []byte(`"Hello"`),
		Stream: &stream,
	})

	require.NoError(t, err)
	actual, ok := converted.(dto.OpenAIResponsesRequest)
	require.True(t, ok)
	require.NotNil(t, actual.Stream)
	require.False(t, *actual.Stream)
	require.JSONEq(t, `"Hello"`, string(actual.Input))
}

func TestDoResponseAggregatesCodexStreamForNonStreamClient(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	completed := `{"id":"resp_1","object":"response","status":"completed","model":"gpt-5.6-sol","output":[],"usage":{"input_tokens":3,"output_tokens":2,"total_tokens":5}}`
	upstream := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body: io.NopCloser(strings.NewReader(
			`data: {"type":"response.completed","response":` + completed + "}\n\n" +
				"data: [DONE]\n\n",
		)),
	}

	usage, relayErr := (&Adaptor{}).DoResponse(ctx, upstream, &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeResponses,
		IsStream:  false,
	})

	require.Nil(t, relayErr)
	require.Equal(t, "application/json", recorder.Header().Get("Content-Type"))
	require.JSONEq(t, completed, recorder.Body.String())
	actualUsage, ok := usage.(*dto.Usage)
	require.True(t, ok)
	require.Equal(t, 5, actualUsage.TotalTokens)
}

func TestDoResponsePreservesCodexStreamForStreamClient(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previousStreamingTimeout := appconstant.StreamingTimeout
	appconstant.StreamingTimeout = 1
	t.Cleanup(func() {
		appconstant.StreamingTimeout = previousStreamingTimeout
	})
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	completedEvent := `{"type":"response.completed","response":{"id":"resp_1","object":"response","status":"completed","model":"gpt-5.6-sol","output":[],"usage":{"input_tokens":3,"output_tokens":2,"total_tokens":5}}}`
	upstream := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body: io.NopCloser(strings.NewReader(
			"data: " + completedEvent + "\n\n" + "data: [DONE]\n\n",
		)),
	}

	usage, relayErr := (&Adaptor{}).DoResponse(ctx, upstream, &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeResponses,
		IsStream:  true,
	})

	require.Nil(t, relayErr)
	require.Contains(t, recorder.Header().Get("Content-Type"), "text/event-stream")
	require.Contains(t, recorder.Body.String(), "data: "+completedEvent)
	actualUsage, ok := usage.(*dto.Usage)
	require.True(t, ok)
	require.Equal(t, 5, actualUsage.TotalTokens)
}

func TestSetupRequestHeaderRequestsSSEForNonStreamClient(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	ctx.Request.Header.Set("Content-Type", "application/json")
	headers := http.Header{}

	err := (&Adaptor{}).SetupRequestHeader(ctx, &headers, &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeResponses,
		IsStream:  false,
		ChannelMeta: &relaycommon.ChannelMeta{
			ApiKey: `{"access_token":"test-token","account_id":"test-account"}`,
		},
	})

	require.NoError(t, err)
	require.Equal(t, "text/event-stream", headers.Get("Accept"))
}
