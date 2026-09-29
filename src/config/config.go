// Package config loads runtime settings from the environment, mirroring the
// env-driven style of cloudflare-ddns so this deploys the same way.
package config

import (
	"encoding/base64"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config is the fully-resolved runtime configuration.
type Config struct {
	APIBase         string        // BulkVS API base URL
	APIUser         string        // Basic-auth username / account email (secret)
	APIKey          string        // Basic-auth password/token (secret)
	BasicAuth       string        // optional pre-computed "Basic <b64>" header (portal-provided); overrides the pair
	HostDescription string        // Description tag that marks the /ipHost entries WE own
	IPProviderURL   string        // where the current public IP is read from
	MaxOut          string        // optional MaxOut value stamped on the host entry
	Interval        time.Duration // reconcile cadence when looping
	HTTPTimeout     time.Duration // per-request timeout
	Prune           bool          // delete our stale entries (old IPs)
	RunOnce         bool          // reconcile once and exit (CronJob mode)
	DryRun          bool          // log intended changes without calling the API
}

// Load reads and validates configuration from the environment.
func Load() (Config, error) {
	c := Config{
		APIBase:         env("BULKVS_API_BASE", "https://portal.bulkvs.com/api/v1.0"),
		APIUser:         os.Getenv("BULKVS_API_USER"),
		APIKey:          os.Getenv("BULKVS_API_KEY"),
		BasicAuth:       strings.TrimSpace(os.Getenv("BULKVS_BASIC_AUTH")),
		HostDescription: env("BULKVS_HOST_DESCRIPTION", "pbx-host"),
		IPProviderURL:   resolveProvider(),
		MaxOut:          os.Getenv("BULKVS_MAX_OUT"),
		Interval:        envDuration("INTERVAL", 5*time.Minute),
		HTTPTimeout:     envDuration("HTTP_TIMEOUT", 15*time.Second),
		Prune:           envBool("PRUNE", true),
		RunOnce:         envBool("RUN_ONCE", false),
		DryRun:          envBool("DRY_RUN", false),
	}

	// Auth is the user/password(token) pair (the common case) OR the portal's
	// pre-computed Basic Auth header — provide one.
	if c.BasicAuth == "" && (c.APIUser == "" || c.APIKey == "") {
		return c, fmt.Errorf("provide BULKVS_API_USER + BULKVS_API_KEY, or BULKVS_BASIC_AUTH")
	}
	// No guard on HostDescription: EffectiveDescription always appends the marker,
	// so the prune ownership boundary is non-empty even when the label is blank.
	return c, nil
}

// AuthorizationHeader returns the ready-to-send Authorization value. A supplied
// Basic Auth header is used verbatim (with or without the "Basic " prefix);
// otherwise it's computed from the pair — it's just base64(user:key).
func (c Config) AuthorizationHeader() string {
	if c.BasicAuth != "" {
		if strings.HasPrefix(strings.ToLower(c.BasicAuth), "basic ") {
			return c.BasicAuth
		}
		return "Basic " + c.BasicAuth
	}
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(c.APIUser+":"+c.APIKey))
}

// descriptionMarker is appended to the operator's label so every entry this tool
// writes carries a machine-identifiable "managed by us" stamp. The friendly label
// stays human-readable; the marker makes ownership unambiguous — a human-created
// entry sharing the label won't carry the trailer, so it's never mistaken for ours.
const descriptionMarker = "bulkvs-ddns"

// EffectiveDescription is the Description actually written to — and matched in —
// BulkVS: the operator's label with the marker appended, e.g. "pbx-host (bulkvs-ddns)".
// It is always non-empty (the marker is always present), so it is safe as the prune
// ownership boundary even when BULKVS_HOST_DESCRIPTION is left blank.
func (c Config) EffectiveDescription() string {
	if label := strings.TrimSpace(c.HostDescription); label != "" {
		return label + " (" + descriptionMarker + ")"
	}
	return descriptionMarker
}

// resolveProvider maps the cloudflare-ddns-style IP_PROVIDER onto a concrete URL.
// IP_PROVIDER_URL wins if set; otherwise cloudflare.trace resolves to the same
// endpoint cloudflare-ddns uses (https://1.1.1.1/cdn-cgi/trace).
func resolveProvider() string {
	if u := os.Getenv("IP_PROVIDER_URL"); u != "" {
		return u
	}
	switch strings.ToLower(os.Getenv("IP_PROVIDER")) {
	case "", "cloudflare.trace", "cloudflare":
		return "https://1.1.1.1/cdn-cgi/trace"
	default:
		// Unknown token: assume the operator passed a trace-style URL directly.
		return os.Getenv("IP_PROVIDER")
	}
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envBool(key string, def bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}

func envDuration(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	// Accept the cloudflare-ddns "@every 5m" spelling as well as a bare duration.
	v = strings.TrimPrefix(strings.TrimSpace(v), "@every ")
	d, err := time.ParseDuration(strings.TrimSpace(v))
	if err != nil || d <= 0 {
		return def
	}
	return d
}
