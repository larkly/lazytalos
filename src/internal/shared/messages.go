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
