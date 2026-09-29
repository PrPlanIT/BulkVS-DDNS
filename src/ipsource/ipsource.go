// Package ipsource reads the site's current public IP.
package ipsource

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
)

// Fetch reads the public IPv4 from a Cloudflare-trace-style endpoint
// (https://1.1.1.1/cdn-cgi/trace) — the same source cloudflare-ddns uses. The
// body is line-oriented key=value; we take the value of the `ip=` line.
func Fetch(ctx context.Context, hc *http.Client, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := hc.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ip provider %s: status %d", url, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(body), "\n") {
		if v, ok := strings.CutPrefix(strings.TrimSpace(line), "ip="); ok {
			ip := strings.TrimSpace(v)
			if net.ParseIP(ip) == nil {
				return "", fmt.Errorf("ip provider returned an unparseable address %q", ip)
			}
			return ip, nil
		}
	}
	return "", fmt.Errorf("ip provider %s: no ip= line in response", url)
}
