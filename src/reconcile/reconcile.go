// Package reconcile drives the /ipHost allowlist toward the current public IP.
package reconcile

import (
	"context"
	"fmt"

	"github.com/PrPlanIT/bulkvs-ip-sync/src/bulkvs"
)

// Result summarises one reconcile pass.
type Result struct {
	CurrentIP string
	Added     bool     // the current IP was not previously present (a new entry)
	Wrote     bool     // a PUT was issued this pass (entry created or a drifted field corrected)
	Pruned    []string // stale IPs we removed
	Unchanged bool     // allowlist already converged: no write issued, nothing pruned
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

// Run reconciles the /ipHost allowlist toward ip. It GETs the current entries
// every pass — that read is the drift detector — but writes only when reality
// diverges from desired: PUT when our entry is absent or a tracked field has
// drifted, DELETE the stale ones we own when Prune is set. A converged allowlist
// is left completely untouched, so a manual edit is corrected on the next pass
// while a steady state costs zero writes (no Last-Modification churn, no needless
// load on the API).
func (r *Reconciler) Run(ctx context.Context, ip string) (Result, error) {
	res := Result{CurrentIP: ip}

	hosts, err := r.Client.ListHosts(ctx)
	if err != nil {
		return res, fmt.Errorf("list hosts: %w", err)
	}

	// The entries we own are those carrying our exact Description. Among them,
	// find the one already asserting the current IP, if any.
	var ours []bulkvs.Host
	var current *bulkvs.Host
	for i := range hosts {
		if hosts[i].Description != r.Description {
			continue
		}
		ours = append(ours, hosts[i])
		if hosts[i].IP == ip {
			current = &hosts[i]
		}
	}
	res.Added = current == nil

	// Write only on drift: the desired entry is missing, or its MaxOut differs
	// from what we'd stamp. An unset MaxOut means "don't manage it", so a value
	// already on the entry is left as-is rather than blanked.
	needWrite := current == nil || (r.MaxOut != "" && current.MaxOut != r.MaxOut)
	res.Wrote = needWrite
	if needWrite && !r.DryRun {
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

	res.Unchanged = !needWrite && len(res.Pruned) == 0
	return res, nil
}
