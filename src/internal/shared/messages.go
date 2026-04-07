package shared

import "github.com/larkly/lazytalos/internal/talos"

// ContextSelectedMsg is sent when a context is selected from the picker.
type ContextSelectedMsg struct {
	ContextName string
}

// ClientConnectedMsg is sent after successful connection to a Talos cluster.
type ClientConnectedMsg struct {
	Client      *talos.Client
	ContextName string
}

// ClientConnectErrMsg is sent when connecting to a Talos cluster fails.
type ClientConnectErrMsg struct {
	Err error
}

// TickMsg is sent by the auto-refresh ticker.
type TickMsg struct{}

// NodeActionMsg is sent to trigger a write action on one or more nodes.
type NodeActionMsg struct {
	Action string
	Nodes  []string
}

// NodeActionErrMsg is sent when a node write action fails.
type NodeActionErrMsg struct {
	Action string
	Err    error
}

// LogLineMsg is sent for each streamed log line in the Logs tab.
type LogLineMsg struct {
	NodeID  string
	Service string
	Line    string
	IsErr   bool
}

// LogStreamEndedMsg is sent when a log stream closes (cleanly or with error).
type LogStreamEndedMsg struct {
	NodeID  string
	Service string
	Err     error // nil on clean close
}

// NodeActionRequestMsg is sent by a child view when it wants the root app
// to show a confirm modal for a node action.
type NodeActionRequestMsg struct {
	Action         string
	NodeHostnames  []string
	NodeNames      []string // display names (may differ from hostnames)
	IsControlPlane []bool   // per-node control plane flag
}

// ServiceRestartRequestMsg is sent by a child view to request a service restart confirmation.
type ServiceRestartRequestMsg struct {
	Node      string
	ServiceID string
}

// ViewChangeMsg is sent by a child view when it wants to exit (e.g., Esc from nodes tab).
type ViewChangeMsg struct{}

// ContainerLogsRequestMsg is emitted by the Containers tab when the user
// wants to view logs for a specific container (Ctrl+L).
type ContainerLogsRequestMsg struct {
	Node        string
	Namespace   string
	ContainerID string
}

// ConfigEditRequestMsg is emitted by the node detail view when the user
// wants to open the inline config editor (Ctrl+E).
type ConfigEditRequestMsg struct {
	Node string
}

// EtcdMemberRemoveRequestMsg is emitted by the etcd tab when the user
// initiates member removal (Ctrl+M).
type EtcdMemberRemoveRequestMsg struct {
	Node     string // CP node to target for the removal RPC
	MemberID uint64
}
