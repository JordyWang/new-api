package relay

import (
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/channel"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type headerlessCodexResponsesAdaptor struct {
	channel.Adaptor
	responseBody      string
	convertedRequest  dto.OpenAIResponsesRequest
	requestInfoStream bool
}

func (a *headerlessCodexResponsesAdaptor) ConvertOpenAIResponsesRequest(_ *gin.Context, _ *relaycommon.RelayInfo, request dto.OpenAIResponsesRequest) (any, error) {
	a.convertedRequest = request
	return request, nil
}

func (a *headerlessCodexResponsesAdaptor) DoRequest(_ *gin.Context, info *relaycommon.RelayInfo, _ io.Reader) (any, error) {
	a.requestInfoStream = info.IsStream
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(a.responseBody)),
		Header:     make(http.Header),
	}, nil
}

func TestIsResponsesEventStreamContentType(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		want        bool
	}{
		{name: "plain", contentType: "text/event-stream", want: true},
		{name: "mixed case with charset", contentType: "Text/Event-Stream; charset=utf-8", want: true},
		{name: "json", contentType: "application/json", want: false},
		{name: "empty", contentType: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isResponsesEventStreamContentType(tt.contentType))
		})
	}
}

func TestChatCompletionsViaResponsesHandlesHeaderlessCodexStream(t *testing.T) {
	oldMode := gin.Mode()
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() { gin.SetMode(oldMode) })
	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldTimeout })

	responseBody := strings.Join([]string{
		`event: response.created`,
		`data: {"type":"response.created","response":{"id":"resp_1","model":"gpt-5.6-luna","created_at":1710000000}}`,
		`event: response.output_text.delta`,
		`data: {"type":"response.output_text.delta","delta":"OK"}`,
		`event: response.completed`,
		`data: {"type":"response.completed","response":{"model":"gpt-5.6-luna","status":"completed","usage":{"input_tokens":2,"output_tokens":1,"total_tokens":3}}}`,
		`data: [DONE]`,
		``,
	}, "\n")

	for _, clientStream := range []bool{false, true} {
		t.Run(map[bool]string{false: "buffered", true: "streaming"}[clientStream], func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
			c.Set(common.RequestIdKey, "codex-headerless-stream-test")

			info := &relaycommon.RelayInfo{
				ChannelMeta: &relaycommon.ChannelMeta{
					ChannelType:       constant.ChannelTypeCodex,
					UpstreamModelName: "gpt-5.6-luna",
				},
				IsStream:           clientStream,
				RelayMode:          relayconstant.RelayModeChatCompletions,
				RelayFormat:        types.RelayFormatOpenAI,
				OriginModelName:    "gpt-5.6-luna",
				ShouldIncludeUsage: true,
				DisablePing:        true,
			}
			adaptor := &headerlessCodexResponsesAdaptor{responseBody: responseBody}
			request := &dto.GeneralOpenAIRequest{
				Model:    "gpt-5.6-luna",
				Messages: []dto.Message{{Role: "user", Content: "Reply with exactly OK."}},
				Stream:   &clientStream,
			}

			usage, newAPIError := chatCompletionsViaResponses(c, info, adaptor, request)

			require.Nil(t, newAPIError)
			require.NotNil(t, usage)
			assert.Equal(t, 3, usage.TotalTokens)
			require.NotNil(t, adaptor.convertedRequest.Stream)
			assert.True(t, *adaptor.convertedRequest.Stream)
			assert.True(t, adaptor.requestInfoStream)
			assert.Equal(t, clientStream, info.IsStream)
			if clientStream {
				assert.Contains(t, recorder.Body.String(), `"object":"chat.completion.chunk"`)
				assert.Contains(t, recorder.Body.String(), `data: [DONE]`)
				return
			}
			assert.Contains(t, recorder.Body.String(), `"object":"chat.completion"`)
			assert.NotContains(t, recorder.Body.String(), `data:`)
		})
	}
}

func TestRecalcQuotaFromRatiosIgnoresInvalidMultipliers(t *testing.T) {
	info := &relaycommon.RelayInfo{
		PriceData: types.PriceData{
			Quota: 100,
		},
	}
	info.PriceData.AddOtherRatio("duration", 2)

	quota, ok := recalcQuotaFromRatios(info, map[string]float64{
		"duration": 3,
		"zero":     0,
		"negative": -1,
		"nan":      math.NaN(),
		"inf":      math.Inf(1),
	})

	require.True(t, ok)
	assert.Equal(t, 150, quota)
	assert.True(t, info.PriceData.HasOtherRatio("duration"))
}

func TestRecalcQuotaFromRatiosRejectsAllInvalidAdjustedRatios(t *testing.T) {
	info := &relaycommon.RelayInfo{
		PriceData: types.PriceData{
			Quota: 100,
		},
	}
	info.PriceData.AddOtherRatio("duration", 2)

	quota, ok := recalcQuotaFromRatios(info, map[string]float64{
		"zero":     0,
		"negative": -1,
		"nan":      math.NaN(),
		"inf":      math.Inf(1),
	})

	require.False(t, ok)
	assert.Equal(t, 0, quota)
	assert.True(t, info.PriceData.HasOtherRatio("duration"))
}
