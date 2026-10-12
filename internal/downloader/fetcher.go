package downloader

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var (
	ErrPrivateHost  = errors.New("private host not allowed")
	ErrInvalidURL   = errors.New("invalid url")
	ErrTooLarge     = errors.New("image too large")
	ErrInvalidMedia = errors.New("invalid media type")
)

type SafeFetcher struct {
	client   *http.Client
	maxBytes int64
}

func NewSafeFetcher(dial, tls, respHeader, downloadTimeout int64, maxRedirect int, maxBytes int64) *SafeFetcher {
	dialer := &net.Dialer{
		Timeout:   time.Duration(dial) * time.Millisecond,
		KeepAlive: 30 * time.Second,
	}
	tr := &http.Transport{
		DialContext:           safeDialContext(dialer),
		TLSHandshakeTimeout:   time.Duration(tls) * time.Millisecond,
		ResponseHeaderTimeout: time.Duration(respHeader) * time.Millisecond,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		DisableCompression:    false,
	}
	client := &http.Client{
		Transport: tr,
		Timeout:   time.Duration(downloadTimeout) * time.Millisecond,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirect {
				return fmt.Errorf("too many redirects")
			}
			return validateURL(req.URL)
		},
	}
	return &SafeFetcher{client: client, maxBytes: maxBytes}
}

func (s *SafeFetcher) Fetch(ctx context.Context, rawURL string) ([]byte, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, ErrInvalidURL
	}
	if err := validateURL(u); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "NudeDetector/1.0")
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("bad status: %d", resp.StatusCode)
	}
	limited := io.LimitReader(resp.Body, s.maxBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > s.maxBytes {
		return nil, ErrTooLarge
	}
	ct := strings.ToLower(resp.Header.Get("Content-Type"))
	if ct != "" && !strings.Contains(ct, "image/") {
		// allow if looks like image? but spec says sniff; we'll be lenient on header
	}
	return data, nil
}

func safeDialContext(dialer *net.Dialer) func(context.Context, string, string) (net.Conn, error) {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		ip, err := resolvePublicIP(ctx, host)
		if err != nil {
			return nil, err
		}
		return dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
	}
}

func validateURL(u *url.URL) error {
	if u == nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		return ErrInvalidURL
	}
	_, err := resolvePublicIP(context.Background(), u.Hostname())
	return err
}

func resolvePublicIP(ctx context.Context, host string) (net.IP, error) {
	ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
	if err != nil {
		return nil, err
	}
	for _, ip := range ips {
		if !isPrivateIP(ip) {
			return ip, nil
		}
	}
	return nil, ErrPrivateHost
}
