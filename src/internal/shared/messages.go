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

// EtcdMemberRemoveRequestMsg is sent by the etcd tab to request member removal
// via a typed-confirmation modal.
type EtcdMemberRemoveRequestMsg struct {
	MemberID  uint64
	MemberHex string
}

// EtcdMemberRemovedMsg is sent after successful etcd member removal.
type EtcdMemberRemovedMsg struct {
	MemberID uint64
}

// EtcdMemberRemoveErrMsg is sent when etcd member removal fails.
type EtcdMemberRemoveErrMsg struct {
	Err error
}

// ConfigEditRequestMsg is sent to open the config editor for a specific node.
type ConfigEditRequestMsg struct {
	Node string
}

// ConfigApplyRequestMsg is sent when the user applies an edited config.
type ConfigApplyRequestMsg struct {
	Node string
	Data []byte
	Mode int // 0=no-reboot, 1=reboot, 2=staged
}

// ConfigAppliedMsg is sent after successful config apply.
type ConfigAppliedMsg struct {
	Node string
}

// ConfigApplyErrMsg is sent when config apply fails.
type ConfigApplyErrMsg struct {
	Node string
	Err  error
}
