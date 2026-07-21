package codexoauth

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type roundTripFunc func(request *http.Request) (*http.Response, error)

func (roundTrip roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return roundTrip(request)
}

func TestExchangeAuthorizationCodeUsesManagedPKCEContract(t *testing.T) {
	var submitted url.Values
	client := &http.Client{
		Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			assert.Equal(t, http.MethodPost, request.Method)
			assert.Equal(t, TokenURL, request.URL.String())
			assert.Equal(t, "application/x-www-form-urlencoded", request.Header.Get("Content-Type"))

			body, err := io.ReadAll(request.Body)
			require.NoError(t, err)
			submitted, err = url.ParseQuery(string(body))
			require.NoError(t, err)

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(
					`{"id_token":"id-token","access_token":"access-token","refresh_token":"refresh-token","expires_in":3600}`,
				)),
				Request: request,
			}, nil
		}),
	}

	startedAt := time.Now()
	result, err := ExchangeAuthorizationCode(context.Background(), client, " auth-code ", " verifier ")
	require.NoError(t, err)

	assert.Equal(t, "authorization_code", submitted.Get("grant_type"))
	assert.Equal(t, ClientID, submitted.Get("client_id"))
	assert.Equal(t, "auth-code", submitted.Get("code"))
	assert.Equal(t, "verifier", submitted.Get("code_verifier"))
	assert.Equal(t, RedirectURI, submitted.Get("redirect_uri"))
	assert.Equal(t, "id-token", result.IDToken)
	assert.Equal(t, "access-token", result.AccessToken)
	assert.Equal(t, "refresh-token", result.RefreshToken)
	assert.WithinDuration(t, startedAt.Add(time.Hour), result.ExpiresAt, time.Second)
}
