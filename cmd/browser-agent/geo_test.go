package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/QuantumNous/new-api/pkg/browseragentapi"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVerifyProxyGeoIdentityUsesProxyAndMatchesFingerprint(t *testing.T) {
	var directRequests atomic.Int32
	directTarget := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		directRequests.Add(1)
		http.Error(response, "geo endpoint must not be reached directly", http.StatusInternalServerError)
	}))
	t.Cleanup(directTarget.Close)

	var proxyRequests atomic.Int32
	proxyServer := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		proxyRequests.Add(1)
		assert.Equal(t, directTarget.URL+"/", request.URL.String())
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"ip":"8.8.8.8","country_code":"US","timezone":"America/Los_Angeles","languages":"en-US,es-US"}`))
	}))
	t.Cleanup(proxyServer.Close)

	identity, err := verifyProxyGeoIdentity(
		context.Background(),
		proxyServer.URL,
		directTarget.URL,
		browseragentapi.CodexOAuthFingerprint{
			Locale:   "en-US",
			Timezone: "America/Los_Angeles",
		},
	)
	require.NoError(t, err)
	assert.Equal(t, "8.8.8.8", identity.IP)
	assert.Equal(t, "US", identity.CountryCode)
	assert.Equal(t, int32(1), proxyRequests.Load())
	assert.Zero(t, directRequests.Load())
}

func TestVerifyProxyGeoIdentityDoesNotFallBackToDirect(t *testing.T) {
	var directRequests atomic.Int32
	directTarget := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		directRequests.Add(1)
		_, _ = response.Write([]byte(`{"ip":"8.8.8.8","country_code":"US","timezone":"America/Los_Angeles","languages":"en-US"}`))
	}))
	t.Cleanup(directTarget.Close)

	unavailableProxy := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {}))
	unavailableProxyURL := unavailableProxy.URL
	unavailableProxy.Close()

	_, err := verifyProxyGeoIdentity(
		context.Background(),
		unavailableProxyURL,
		directTarget.URL,
		browseragentapi.CodexOAuthFingerprint{Locale: "en-US", Timezone: "America/Los_Angeles"},
	)
	require.Error(t, err)
	assert.Zero(t, directRequests.Load())
}

func TestVerifyProxyGeoIdentityFailsClosed(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		locale     string
		timezone   string
		errorText  string
	}{
		{
			name:       "timezone mismatch",
			statusCode: http.StatusOK,
			body:       `{"ip":"8.8.8.8","country_code":"US","timezone":"America/New_York","languages":"en-US"}`,
			locale:     "en-US",
			timezone:   "America/Los_Angeles",
			errorText:  "timezone",
		},
		{
			name:       "locale mismatch",
			statusCode: http.StatusOK,
			body:       `{"ip":"8.8.8.8","country_code":"US","timezone":"America/Los_Angeles","languages":"es-US"}`,
			locale:     "en-US",
			timezone:   "America/Los_Angeles",
			errorText:  "locale",
		},
		{
			name:       "private IP",
			statusCode: http.StatusOK,
			body:       `{"ip":"10.0.0.1","country_code":"US","timezone":"America/Los_Angeles","languages":"en-US"}`,
			locale:     "en-US",
			timezone:   "America/Los_Angeles",
			errorText:  "public",
		},
		{
			name:       "invalid timezone",
			statusCode: http.StatusOK,
			body:       `{"ip":"8.8.8.8","country_code":"US","timezone":"Mars/Olympus","languages":"en-US"}`,
			locale:     "en-US",
			timezone:   "Mars/Olympus",
			errorText:  "timezone",
		},
		{
			name:       "missing languages",
			statusCode: http.StatusOK,
			body:       `{"ip":"8.8.8.8","country_code":"US","timezone":"America/Los_Angeles","languages":""}`,
			locale:     "en-US",
			timezone:   "America/Los_Angeles",
			errorText:  "languages",
		},
		{
			name:       "service error",
			statusCode: http.StatusServiceUnavailable,
			body:       `{"error":true}`,
			locale:     "en-US",
			timezone:   "America/Los_Angeles",
			errorText:  "HTTP 503",
		},
		{
			name:       "invalid JSON",
			statusCode: http.StatusOK,
			body:       `{`,
			locale:     "en-US",
			timezone:   "America/Los_Angeles",
			errorText:  "decode",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			proxyServer := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
				response.WriteHeader(test.statusCode)
				_, _ = response.Write([]byte(test.body))
			}))
			t.Cleanup(proxyServer.Close)

			_, err := verifyProxyGeoIdentity(
				context.Background(),
				proxyServer.URL,
				"http://127.0.0.1:1/geo",
				browseragentapi.CodexOAuthFingerprint{Locale: test.locale, Timezone: test.timezone},
			)
			require.Error(t, err)
			assert.ErrorContains(t, err, test.errorText)
		})
	}
}

func TestValidateGeoIPURLRequiresHTTPSExceptLoopback(t *testing.T) {
	for _, validURL := range []string{
		"https://ipapi.co/json/",
		"http://127.0.0.1:8080/geo",
		"http://localhost:8080/geo",
	} {
		_, err := validateGeoIPURL(validURL)
		assert.NoError(t, err, validURL)
	}

	for _, invalidURL := range []string{
		"http://geo.example.com/json/",
		"https://user:pass@geo.example.com/json/",
		"https://geo.example.com/json/#fragment",
		"file:///tmp/geo.json",
	} {
		_, err := validateGeoIPURL(invalidURL)
		assert.Error(t, err, invalidURL)
	}
}
