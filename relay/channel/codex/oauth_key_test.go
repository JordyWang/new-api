package codex

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseOAuthKeyPreservesManagedProxyBinding(t *testing.T) {
	key, err := ParseOAuthKey(`{"access_token":"token","managed_proxy_id":42}`)
	require.NoError(t, err)
	assert.Equal(t, 42, key.ManagedProxyID)
}
