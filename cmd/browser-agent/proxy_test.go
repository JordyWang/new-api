package main

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type proxyHandshakeResult struct {
	method        string
	host          string
	authorization string
	err           error
}

func TestDialTunnelUsesAuthenticatedManagedHTTPProxy(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if errors.Is(err, syscall.EPERM) {
		t.Skip("local sockets are disabled by the test sandbox")
	}
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = listener.Close()
	})

	handshake := make(chan proxyHandshakeResult, 1)
	go func() {
		connection, acceptErr := listener.Accept()
		if acceptErr != nil {
			handshake <- proxyHandshakeResult{err: acceptErr}
			return
		}
		defer connection.Close()

		reader := bufio.NewReader(connection)
		request, readErr := http.ReadRequest(reader)
		if readErr != nil {
			handshake <- proxyHandshakeResult{err: readErr}
			return
		}
		handshake <- proxyHandshakeResult{
			method:        request.Method,
			host:          request.Host,
			authorization: request.Header.Get("Proxy-Authorization"),
		}

		_, _ = connection.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))
		payload := make([]byte, 4)
		if _, readErr := io.ReadFull(reader, payload); readErr != nil {
			return
		}
		if string(payload) == "ping" {
			_, _ = connection.Write([]byte("pong"))
		}
	}()

	upstream := &url.URL{
		Scheme: "http",
		Host:   listener.Addr().String(),
		User:   url.UserPassword("managed-user", "managed-password"),
	}
	forward := &localForwardProxy{upstream: upstream}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tunnel, err := forward.dialTunnel(ctx, "auth.openai.com:443")
	require.NoError(t, err)
	defer tunnel.Close()

	_, err = tunnel.Write([]byte("ping"))
	require.NoError(t, err)
	response := make([]byte, 4)
	_, err = io.ReadFull(tunnel, response)
	require.NoError(t, err)
	assert.Equal(t, "pong", string(response))

	result := <-handshake
	require.NoError(t, result.err)
	assert.Equal(t, http.MethodConnect, result.method)
	assert.Equal(t, "auth.openai.com:443", result.host)
	assert.Equal(t, "Basic bWFuYWdlZC11c2VyOm1hbmFnZWQtcGFzc3dvcmQ=", result.authorization)
}
