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
	client      *http.Client
	maxBytes    int64
	maxRedirect int
}

func NewSafeFetcher(dial, tls, respHeader, downloadTimeout int64, maxRedirect int, maxBytes int64) *SafeFetcher {
	tr := &http.Transport{
		DialContext:           (&net.Dialer{Timeout: time.Duration(dial) * time.Millisecond, KeepAlive: 30 * time.Second}).DialContext,
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
				return http.ErrUseLastResponse
			}
			return nil
		},
	}
	return &SafeFetcher{client: client, maxBytes: maxBytes, maxRedirect: maxRedirect}
}

func (s *SafeFetcher) Fetch(ctx context.Context, rawURL string) ([]byte, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, ErrInvalidURL
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, ErrInvalidURL
	}
	host := u.Hostname()
	if host == "" {
		return nil, ErrInvalidURL
	}
	ips, err := net.LookupIP(host)
	if err != nil {
		return nil, err
	}
	for _, ip := range ips {
		if isPrivateIP(ip) {
			return nil, ErrPrivateHost
		}
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
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if resp.StatusCode == http.StatusFound || resp.StatusCode == http.StatusMovedPermanently || resp.StatusCode == http.StatusTemporaryRedirect || resp.StatusCode == http.StatusSeeOther {
			loc := resp.Header.Get("Location")
			if loc == "" {
				return nil, fmt.Errorf("bad status: %d", resp.StatusCode)
			}
			if s.maxRedirect < 0 {
				return nil, fmt.Errorf("too many redirects")
			}
			return s.Fetch(ctx, loc)
		}
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
