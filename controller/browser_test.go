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

func TestBrowserFingerprintRequiresValidLocaleAndTimezone(t *testing.T) {
	request := browserFingerprintRequest{
		Name:        "managed",
		Locale:      "en-US",
		Timezone:    "America/Los_Angeles",
		ViewportW:   1280,
		ViewportH:   800,
		Payload:     `{}`,
		LaunchArgs:  `[]`,
		Environment: `{}`,
	}

	_, err := normalizeBrowserFingerprintRequest(request, nil)
	require.NoError(t, err)

	request.Locale = "not_a_locale!"
	_, err = normalizeBrowserFingerprintRequest(request, nil)
	assert.ErrorContains(t, err, "locale")

	request.Locale = "en-US"
	request.Timezone = "Mars/Olympus"
	_, err = normalizeBrowserFingerprintRequest(request, nil)
	assert.ErrorContains(t, err, "timezone")
}
