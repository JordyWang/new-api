package controller

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBrowserAgentIdentifiersRespectStorageBounds(t *testing.T) {
	assert.True(t, validBrowserAgentInstanceId(strings.Repeat("a", 64)))
	assert.False(t, validBrowserAgentInstanceId(strings.Repeat("a", 65)))

	runtimes := make([]string, 65)
	for index := range runtimes {
		runtimes[index] = "runtime-" + string(rune('A'+index%26)) + strings.Repeat("x", index/26)
	}
	_, err := normalizeBrowserRuntimeKeys(runtimes)
	assert.Error(t, err)
}

func TestFingerprintEnvironmentRejectsEncodedPayloadOverLimit(t *testing.T) {
	environment := make(map[string]string, 16)
	for index := 0; index < 16; index++ {
		environment["VARIABLE_"+string(rune('A'+index))] = strings.Repeat("x", 4096)
	}
	encoded, err := common.Marshal(environment)
	require.NoError(t, err)

	_, err = normalizeStringMap(string(encoded), "fingerprint environment")
	assert.Error(t, err)
}

func TestBrowserFingerprintStoresRegionNeutralCorePayload(t *testing.T) {
	request := browserFingerprintRequest{
		Name:      "managed",
		ViewportW: 1280,
		ViewportH: 800,
		Payload: `{
			"timezone":"America/Los_Angeles",
			"geo_overlay":{"country_code":"US"},
			"fingerprint":{
				"accept_language":"en-US,en",
				"timezone":"America/Los_Angeles",
				"canvas":{"noise_seed":"fixed-core"},
				"navigator":{"language":"en-US","languages":["en-US","en"],"platform":"Linux x86_64"}
			}
		}`,
		LaunchArgs:  `[]`,
		Environment: `{}`,
	}

	fingerprint, err := normalizeBrowserFingerprintRequest(request, nil)
	require.NoError(t, err)

	var payload map[string]any
	require.NoError(t, common.UnmarshalJsonStr(fingerprint.Payload, &payload))
	assert.NotContains(t, payload, "timezone")
	assert.NotContains(t, payload, "geo_overlay")
	core, ok := payload["fingerprint"].(map[string]any)
	require.True(t, ok)
	assert.NotContains(t, core, "accept_language")
	assert.NotContains(t, core, "timezone")
	assert.Equal(t, map[string]any{"noise_seed": "fixed-core"}, core["canvas"])
	navigator, ok := core["navigator"].(map[string]any)
	require.True(t, ok)
	assert.NotContains(t, navigator, "language")
	assert.NotContains(t, navigator, "languages")
	assert.Equal(t, "Linux x86_64", navigator["platform"])
}
