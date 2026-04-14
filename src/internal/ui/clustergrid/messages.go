package clustergrid

import (
	"time"

	"github.com/larkly/lazytalos/internal/cluster"
	"github.com/larkly/lazytalos/internal/shared"
	"github.com/larkly/lazytalos/internal/talos"
)

// ClosedMsg is emitted when the grid overlay is dismissed. If Selected is
// non-empty, the parent app should drill down into that context (reconnect
// and open the Dashboard tab).
type ClosedMsg struct {
	Selected string
}

// clusterSummary is the per-cluster data a card needs to render.
type clusterSummary struct {
	ContextName      string
	TalosVersion     string
	CPCount          int
	WorkerCount      int
	Nodes            []cluster.NodeInfo
	ServicesByNode   map[string][]shared.ServiceRow
	FailedCount      int
	UnreachableCount int
	FetchedAt        time.Time
}

// CardReadyMsg is emitted when a per-cluster fetch completes successfully.
// The client is handed back so the main goroutine owns its lifecycle and we
// avoid a race between Update and Close on an in-flight fetch. Exported so
// the parent app can route late messages to the grid after dismiss for
// resource cleanup.
type CardReadyMsg struct {
	contextName string
	summary     clusterSummary
	client      *talos.Client
	reused      bool
	gen         int
}

// CardErrMsg is emitted when a per-cluster fetch fails. The client may be
// non-nil when the dial succeeded but a subsequent RPC failed. Exported so
// the parent app can route late messages to the grid after dismiss for
// resource cleanup.
type CardErrMsg struct {
	contextName string
	err         error
	client      *talos.Client
	reused      bool
	gen         int
}

// refreshAllMsg triggers a re-fetch of every card (bound to 'r').
type refreshAllMsg struct{}
