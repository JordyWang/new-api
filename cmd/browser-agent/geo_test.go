package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"
)

func TestVerifyProxyGeoIdentityUsesProxyAndDerivesRegionalOverlay(t *testing.T) {
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
		_, _ = response.Write([]byte(`{"ip":"8.8.8.8","country_code":"SG","timezone":"Asia/Singapore","languages":"cmn,en-SG,ms-SG,ta-SG,zh-SG"}`))
	}))
	t.Cleanup(proxyServer.Close)

	identity, err := verifyProxyGeoIdentity(
		context.Background(),
		proxyServer.URL,
		directTarget.URL,
	)
	require.NoError(t, err)
	assert.Equal(t, "8.8.8.8", identity.IP)
	assert.Equal(t, "SG", identity.CountryCode)
	assert.Equal(t, "en-SG", identity.Locale)
	assert.Equal(t, "Asia/Singapore", identity.Timezone)
	assert.Equal(t, []string{"en-SG", "en", "cmn", "ms-SG", "ta-SG", "zh-SG"}, identity.Languages)
	assert.Equal(t, "en-SG,en,cmn,ms-SG,ta-SG,zh-SG", identity.AcceptLanguage)
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
	)
	require.Error(t, err)
	assert.Zero(t, directRequests.Load())
}

func TestDeriveProxyGeoLanguagesRegionalizesBareLanguage(t *testing.T) {
	locale, languages, err := deriveProxyGeoLanguages("JP", []language.Tag{language.Japanese})

	require.NoError(t, err)
	assert.Equal(t, "ja-JP", locale)
	assert.Equal(t, []string{"ja-JP", "ja"}, languages)
}

func TestDeriveProxyGeoLanguagesRejectsDifferentRegion(t *testing.T) {
	_, _, err := deriveProxyGeoLanguages("US", []language.Tag{language.MustParse("en-GB")})

	assert.ErrorContains(t, err, "country_code")
}

func TestVerifyProxyGeoIdentityFailsClosed(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		errorText  string
	}{
		{
			name:       "private IP",
			statusCode: http.StatusOK,
			body:       `{"ip":"10.0.0.1","country_code":"US","timezone":"America/Los_Angeles","languages":"en-US"}`,
			errorText:  "public",
		},
		{
			name:       "invalid timezone",
			statusCode: http.StatusOK,
			body:       `{"ip":"8.8.8.8","country_code":"US","timezone":"Mars/Olympus","languages":"en-US"}`,
			errorText:  "timezone",
		},
		{
			name:       "missing languages",
			statusCode: http.StatusOK,
			body:       `{"ip":"8.8.8.8","country_code":"US","timezone":"America/Los_Angeles","languages":""}`,
			errorText:  "languages",
		},
		{
			name:       "service error",
			statusCode: http.StatusServiceUnavailable,
			body:       `{"error":true}`,
			errorText:  "HTTP 503",
		},
		{
			name:       "invalid JSON",
			statusCode: http.StatusOK,
			body:       `{`,
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
