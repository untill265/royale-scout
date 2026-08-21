package netpolicy

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type restrictedTransport struct {
	base         http.RoundTripper
	allowedHosts map[string]struct{}
}

func NewHTTPSClient(timeout time.Duration, hosts ...string) *http.Client {
	allowed := make(map[string]struct{}, len(hosts))
	for _, host := range hosts {
		allowed[strings.ToLower(strings.TrimSpace(host))] = struct{}{}
	}
	return &http.Client{
		Timeout: timeout,
		Transport: &restrictedTransport{
			base:         http.DefaultTransport,
			allowedHosts: allowed,
		},
	}
}

func (transport *restrictedTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	if request == nil || request.URL == nil {
		return nil, errors.New("outbound request URL is missing")
	}
	if request.URL.Scheme != "https" {
		return nil, fmt.Errorf("outbound HTTP scheme %q is not allowed", request.URL.Scheme)
	}
	if request.URL.User != nil {
		return nil, errors.New("credentials in outbound URLs are not allowed")
	}
	if request.URL.Port() != "" && request.URL.Port() != "443" {
		return nil, fmt.Errorf("outbound port %q is not allowed", request.URL.Port())
	}
	host := strings.ToLower(request.URL.Hostname())
	if _, allowed := transport.allowedHosts[host]; !allowed {
		return nil, fmt.Errorf("outbound host %q is not allowed", host)
	}
	return transport.base.RoundTrip(request)
}
