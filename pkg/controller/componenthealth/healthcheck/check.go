// Package healthcheck holds what every ComponentHealth check shares: the check contract, the writer
// that publishes results, the ignore rules and the helpers that build check results.
package healthcheck

import (
	"context"

	"k8s.io/client-go/tools/cache"

	"github.com/harvester/resource-health-controller/pkg/config"
)

// Check is a unit that owns one ComponentHealth and recomputes all of its checks.
type Check interface {
	// Name is the name of the ComponentHealth the check owns.
	Name() string
	Reconcile() error
}

// Registration builds a check with its caches, registers its change handlers and returns its HasSynced.
type Registration func(ctx context.Context, writer Writer, management *config.Management) (Check, cache.InformerSynced)
