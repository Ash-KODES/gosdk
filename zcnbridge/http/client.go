package http

import (
	"net"
	"net/http"
	"time"

	"github.com/hashicorp/go-retryablehttp"
)

const (
	// clientTimeout represents default http.Client timeout.
	clientTimeout = 60 * time.Second

	// tlsHandshakeTimeout represents default http.Transport TLS handshake timeout.
	tlsHandshakeTimeout = 10 * time.Second

	// dialTimeout represents default net.Dialer timeout.
	dialTimeout = 5 * time.Second
)

// NewClient creates default http.Client with timeouts.
func NewClient() *http.Client {
	d := &net.Dialer{
		Timeout: dialTimeout,
	}

	transport := &http.Transport{
		TLSHandshakeTimeout: tlsHandshakeTimeout,
		DialContext:         d.DialContext,
	}

	return &http.Client{
		Timeout:   clientTimeout,
		Transport: transport,
	}
}

// NewRetryableClient creates default retryablehttp.Client with timeouts and embedded NewClient result.
func NewRetryableClient(retryMax int) *retryablehttp.Client {
	client := retryablehttp.NewClient()
	client.HTTPClient = NewClient()
	client.RetryWaitMax = clientTimeout
	client.RetryMax = retryMax
	client.Logger = nil

	return client
}
