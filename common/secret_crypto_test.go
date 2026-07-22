package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSecretEncryptionRoundTripAndContextBinding(t *testing.T) {
	originalSecret := CryptoSecret
	CryptoSecret = "test-secret-for-aes-gcm"
	t.Cleanup(func() {
		CryptoSecret = originalSecret
	})

	encrypted, err := EncryptSecret("proxy-password", "browser-proxy-url")
	require.NoError(t, err)
	assert.NotContains(t, encrypted, "proxy-password")

	decrypted, err := DecryptSecret(encrypted, "browser-proxy-url")
	require.NoError(t, err)
	assert.Equal(t, "proxy-password", decrypted)

	_, err = DecryptSecret(encrypted, "codex-oauth-verifier")
	assert.Error(t, err)
}

func TestSecretDecryptionRejectsTampering(t *testing.T) {
	originalSecret := CryptoSecret
	CryptoSecret = "test-secret-for-aes-gcm"
	t.Cleanup(func() {
		CryptoSecret = originalSecret
	})

	encrypted, err := EncryptSecret("refresh-token", "codex-oauth-credentials")
	require.NoError(t, err)

	tamperIndex := len(encrypted) / 2
	replacement := byte('A')
	if encrypted[tamperIndex] == replacement {
		replacement = 'B'
	}
	tampered := encrypted[:tamperIndex] + string(replacement) + encrypted[tamperIndex+1:]
	_, err = DecryptSecret(tampered, "codex-oauth-credentials")
	assert.Error(t, err)
}
