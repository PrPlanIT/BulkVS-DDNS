// Command bulkvs-ddns keeps a BulkVS IP-based-auth host (/ipHost) in sync with the
// site's current public IP — DDNS for a BulkVS SIP trunk.
package main

import (
	"context"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/PrPlanIT/bulkvs-ddns/src/bulkvs"
	"github.com/PrPlanIT/bulkvs-ddns/src/config"
	"github.com/PrPlanIT/bulkvs-ddns/src/ipsource"
	"github.com/PrPlanIT/bulkvs-ddns/src/reconcile"
	"github.com/PrPlanIT/bulkvs-ddns/src/version"
)

func main() {
	log.SetFlags(log.LstdFlags | log.LUTC)
	log.Printf("bulkvs-ddns %s starting", version.String())

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	hc := &http.Client{Timeout: cfg.HTTPTimeout}
	rec := &reconcile.Reconciler{
		Client:      bulkvs.New(cfg.APIBase, cfg.AuthorizationHeader(), hc),
		Description: cfg.EffectiveDescription(),
		MaxOut:      cfg.MaxOut,
		Prune:       cfg.Prune,
		DryRun:      cfg.DryRun,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pass := func() {
		cctx, cancel := context.WithTimeout(ctx, 4*cfg.HTTPTimeout)
		defer cancel()

		ip, err := ipsource.Fetch(cctx, hc, cfg.IPProviderURL)
		if err != nil {
			log.Printf("detect IP: %v", err)
			return
		}
		res, err := rec.Run(cctx, ip)
		if err != nil {
			log.Printf("reconcile (ip=%s): %v", ip, err)
			return
		}
		switch {
		case cfg.DryRun:
			log.Printf("[dry-run] ip=%s would-put=%v would-prune=%v", res.CurrentIP, res.Added, res.Pruned)
		case res.Unchanged:
			log.Printf("ip=%s already current; nothing to do", res.CurrentIP)
		default:
			log.Printf("ip=%s asserted (new=%v) pruned=%v", res.CurrentIP, res.Added, res.Pruned)
		}
	}

	pass()
	if cfg.RunOnce {
		return
	}

	log.Printf("watching every %s (description=%q, prune=%v)", cfg.Interval, cfg.EffectiveDescription(), cfg.Prune)
	ticker := time.NewTicker(cfg.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Printf("shutting down")
			return
		case <-ticker.C:
			pass()
		}
	}
}
