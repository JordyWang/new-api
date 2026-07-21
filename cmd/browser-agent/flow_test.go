package main

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAgentRejectsServerControlledNetworkAndProfileOverrides(t *testing.T) {
	for _, argument := range []string{
		"--proxy-server=http://other-proxy.example:8080",
		"--proxy-pac-url=https://example.com/proxy.pac",
		"--proxy-bypass-list=*",
		"--user-data-dir=/tmp/unmanaged",
		"--remote-debugging-port=9222",
	} {
		assert.True(t, forbiddenBrowserArgument(argument), argument)
	}
	assert.False(t, forbiddenBrowserArgument("--fingerprint-config={fingerprint_file}"))
}

func TestAgentRejectsDangerousBrowserEnvironmentVariables(t *testing.T) {
	for _, key := range []string{"LD_PRELOAD", "PATH", "HOME", "DYLD_INSERT_LIBRARIES", "NODE_OPTIONS"} {
		assert.True(t, forbiddenBrowserEnvironmentKey(key), key)
	}
	assert.False(t, forbiddenBrowserEnvironmentKey("FINGERPRINT_PROFILE"))
}

func TestProxyEndpointAddsSchemeDefaultPort(t *testing.T) {
	httpsProxy, _ := url.Parse("https://proxy.example.com")
	socksProxy, _ := url.Parse("socks5://proxy.example.com")
	explicitProxy, _ := url.Parse("http://proxy.example.com:3128")

	assert.Equal(t, "proxy.example.com:443", proxyEndpoint(httpsProxy))
	assert.Equal(t, "proxy.example.com:1080", proxyEndpoint(socksProxy))
	assert.Equal(t, "proxy.example.com:3128", proxyEndpoint(explicitProxy))
}
