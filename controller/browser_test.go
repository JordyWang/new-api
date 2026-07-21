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
