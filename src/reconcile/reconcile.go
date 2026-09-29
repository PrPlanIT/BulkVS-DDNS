// Package reconcile drives the /ipHost allowlist toward the current public IP.
package reconcile

import (
	"context"
	"fmt"

	"github.com/PrPlanIT/bulkvs-ddns/src/bulkvs"
)

// Result summarises one reconcile pass.
type Result struct {
	CurrentIP string
	Added     bool     // the current IP was not previously present
	Pruned    []string // stale IPs we removed
	Unchanged bool     // current IP already present and nothing pruned
}

// Reconciler asserts a single public IP as the sole /ipHost entry carrying our
// Description. Entries with any other Description are never read as "ours" and
// never touched — Description is the ownership boundary.
type Reconciler struct {
	Client      *bulkvs.Client
	Description string
	MaxOut      string
	Prune       bool
	DryRun      bool
}

// Run: PUT the current IP (upsert, so Description/MaxOut converge even if the IP
// already exists), then — when Prune is set — DELETE every other entry we own
// (old rotated IPs, manual leftovers).
func (r *Reconciler) Run(ctx context.Context, ip string) (Result, error) {
	res := Result{CurrentIP: ip}

	hosts, err := r.Client.ListHosts(ctx)
	if err != nil {
		return res, fmt.Errorf("list hosts: %w", err)
	}

	var ours []bulkvs.Host
	currentPresent := false
	for _, h := range hosts {
		if h.Description != r.Description {
			continue
		}
		ours = append(ours, h)
		if h.IP == ip {
			currentPresent = true
		}
	}
	res.Added = !currentPresent

	if !r.DryRun {
		if err := r.Client.PutHost(ctx, bulkvs.Host{IP: ip, Description: r.Description, MaxOut: r.MaxOut}); err != nil {
			return res, fmt.Errorf("put host %s: %w", ip, err)
		}
	}

	if r.Prune {
		for _, h := range ours {
			if h.IP == ip {
				continue
			}
			if !r.DryRun {
				if err := r.Client.DeleteHost(ctx, h.IP); err != nil {
					return res, fmt.Errorf("delete stale host %s: %w", h.IP, err)
				}
			}
			res.Pruned = append(res.Pruned, h.IP)
		}
	}

	res.Unchanged = currentPresent && len(res.Pruned) == 0
	return res, nil
}
