package utils

import (
	"crypto/tls"
	"net/http"
	"time"
)

// NewHTTPClient creates a configured HTTP client for external requests
func NewHTTPClient(timeout time.Duration) *http.Client {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
	}

	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}
}

// UserAgentMiddleware adds a user agent header to requests
func UserAgentMiddleware(next http.RoundTripper) http.RoundTripper {
	return &userAgentTransport{next: next}
}

type userAgentTransport struct {
	next http.RoundTripper
}

func (t *userAgentTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", "MyJobMatch/1.0 (Cloud Run)")
	}
	return t.next.RoundTrip(req)
}
