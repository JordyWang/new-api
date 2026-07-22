package main

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/pkg/browserproxy"

	xproxy "golang.org/x/net/proxy"
)

type localForwardProxy struct {
	upstream  *url.URL
	listener  net.Listener
	server    *http.Server
	transport *http.Transport
	socks     xproxy.Dialer
}

type bufferedConn struct {
	net.Conn
	reader *bufio.Reader
}

func (connection *bufferedConn) Read(buffer []byte) (int, error) {
	return connection.reader.Read(buffer)
}

func startLocalForwardProxy(upstreamURL string) (*localForwardProxy, error) {
	normalized, _, err := browserproxy.ValidateURL(upstreamURL)
	if err != nil {
		return nil, err
	}
	upstream, err := url.Parse(normalized)
	if err != nil {
		return nil, err
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}

	forward := &localForwardProxy{upstream: upstream, listener: listener}
	transport := &http.Transport{
		MaxIdleConns:        32,
		MaxIdleConnsPerHost: 8,
		IdleConnTimeout:     60 * time.Second,
		ForceAttemptHTTP2:   true,
	}
	switch upstream.Scheme {
	case "http", "https":
		transport.Proxy = http.ProxyURL(upstream)
	case "socks5", "socks5h":
		var authentication *xproxy.Auth
		if upstream.User != nil {
			password, _ := upstream.User.Password()
			authentication = &xproxy.Auth{User: upstream.User.Username(), Password: password}
		}
		forwardDialer := &net.Dialer{Timeout: 20 * time.Second, KeepAlive: 30 * time.Second}
		forward.socks, err = xproxy.SOCKS5("tcp", proxyEndpoint(upstream), authentication, forwardDialer)
		if err != nil {
			_ = listener.Close()
			return nil, err
		}
		if contextDialer, ok := forward.socks.(xproxy.ContextDialer); ok {
			transport.DialContext = contextDialer.DialContext
		} else {
			transport.DialContext = func(_ context.Context, network string, address string) (net.Conn, error) {
				return forward.socks.Dial(network, address)
			}
		}
	}
	forward.transport = transport
	forward.server = &http.Server{
		Handler:           forward,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	go func() {
		_ = forward.server.Serve(listener)
	}()
	return forward, nil
}

func (forward *localForwardProxy) URL() string {
	return "http://" + forward.listener.Addr().String()
}

func (forward *localForwardProxy) Close() {
	forward.transport.CloseIdleConnections()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = forward.server.Shutdown(ctx)
	_ = forward.listener.Close()
}

func (forward *localForwardProxy) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	if request.Method == http.MethodConnect {
		forward.serveConnect(response, request)
		return
	}
	forward.serveHTTP(response, request)
}

func (forward *localForwardProxy) serveHTTP(response http.ResponseWriter, request *http.Request) {
	outbound := request.Clone(request.Context())
	outbound.RequestURI = ""
	outbound.Header = request.Header.Clone()
	removeHopByHopHeaders(outbound.Header)
	outbound.Header.Del("Proxy-Authorization")

	upstreamResponse, err := forward.transport.RoundTrip(outbound)
	if err != nil {
		http.Error(response, "proxy request failed", http.StatusBadGateway)
		return
	}
	defer upstreamResponse.Body.Close()
	removeHopByHopHeaders(upstreamResponse.Header)
	for key, values := range upstreamResponse.Header {
		for _, value := range values {
			response.Header().Add(key, value)
		}
	}
	response.WriteHeader(upstreamResponse.StatusCode)
	_, _ = io.Copy(response, upstreamResponse.Body)
}

func (forward *localForwardProxy) serveConnect(response http.ResponseWriter, request *http.Request) {
	target := strings.TrimSpace(request.Host)
	if target == "" {
		http.Error(response, "CONNECT target is required", http.StatusBadRequest)
		return
	}
	upstreamConnection, err := forward.dialTunnel(request.Context(), target)
	if err != nil {
		http.Error(response, "proxy tunnel failed", http.StatusBadGateway)
		return
	}

	hijacker, ok := response.(http.Hijacker)
	if !ok {
		_ = upstreamConnection.Close()
		http.Error(response, "proxy hijacking is unavailable", http.StatusInternalServerError)
		return
	}
	clientConnection, buffered, err := hijacker.Hijack()
	if err != nil {
		_ = upstreamConnection.Close()
		return
	}
	if _, err := buffered.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
		_ = clientConnection.Close()
		_ = upstreamConnection.Close()
		return
	}
	if err := buffered.Flush(); err != nil {
		_ = clientConnection.Close()
		_ = upstreamConnection.Close()
		return
	}

	copyDone := make(chan struct{}, 2)
	go tunnelCopy(upstreamConnection, clientConnection, copyDone)
	go tunnelCopy(clientConnection, upstreamConnection, copyDone)
	<-copyDone
	_ = clientConnection.Close()
	_ = upstreamConnection.Close()
}

func (forward *localForwardProxy) dialTunnel(ctx context.Context, target string) (net.Conn, error) {
	if forward.socks != nil {
		if contextDialer, ok := forward.socks.(xproxy.ContextDialer); ok {
			return contextDialer.DialContext(ctx, "tcp", target)
		}
		return forward.socks.Dial("tcp", target)
	}

	dialer := &net.Dialer{Timeout: 20 * time.Second, KeepAlive: 30 * time.Second}
	var connection net.Conn
	var err error
	if forward.upstream.Scheme == "https" {
		connection, err = tls.DialWithDialer(dialer, "tcp", proxyEndpoint(forward.upstream), &tls.Config{
			MinVersion: tls.VersionTLS12,
			ServerName: forward.upstream.Hostname(),
		})
	} else {
		connection, err = dialer.DialContext(ctx, "tcp", proxyEndpoint(forward.upstream))
	}
	if err != nil {
		return nil, err
	}

	connectRequest := &http.Request{
		Method: http.MethodConnect,
		URL:    &url.URL{Opaque: target},
		Host:   target,
		Header: make(http.Header),
	}
	if forward.upstream.User != nil {
		password, _ := forward.upstream.User.Password()
		credentials := forward.upstream.User.Username() + ":" + password
		connectRequest.Header.Set("Proxy-Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(credentials)))
	}
	if err := connectRequest.WriteProxy(connection); err != nil {
		_ = connection.Close()
		return nil, err
	}

	reader := bufio.NewReader(connection)
	proxyResponse, err := http.ReadResponse(reader, connectRequest)
	if err != nil {
		_ = connection.Close()
		return nil, err
	}
	if proxyResponse.Body != nil {
		_ = proxyResponse.Body.Close()
	}
	if proxyResponse.StatusCode != http.StatusOK {
		_ = connection.Close()
		return nil, fmt.Errorf("upstream proxy CONNECT returned HTTP %d", proxyResponse.StatusCode)
	}
	if reader.Buffered() > 0 {
		return &bufferedConn{Conn: connection, reader: reader}, nil
	}
	return connection, nil
}

func tunnelCopy(destination net.Conn, source net.Conn, done chan<- struct{}) {
	_, _ = io.Copy(destination, source)
	if closeWriter, ok := destination.(interface{ CloseWrite() error }); ok {
		_ = closeWriter.CloseWrite()
	}
	select {
	case done <- struct{}{}:
	default:
	}
}

func removeHopByHopHeaders(header http.Header) {
	for _, value := range header.Values("Connection") {
		for _, token := range strings.Split(value, ",") {
			header.Del(strings.TrimSpace(token))
		}
	}
	for _, key := range []string{
		"Connection",
		"Proxy-Connection",
		"Keep-Alive",
		"Proxy-Authenticate",
		"Proxy-Authorization",
		"Te",
		"Trailer",
		"Transfer-Encoding",
		"Upgrade",
	} {
		header.Del(key)
	}
}

func proxyEndpoint(proxyURL *url.URL) string {
	if proxyURL.Port() != "" {
		return proxyURL.Host
	}
	port := "80"
	if proxyURL.Scheme == "https" {
		port = "443"
	} else if proxyURL.Scheme == "socks5" || proxyURL.Scheme == "socks5h" {
		port = "1080"
	}
	return net.JoinHostPort(proxyURL.Hostname(), port)
}
