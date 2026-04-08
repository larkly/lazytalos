package resources

import (
	"testing"
)

// TestNetworkStructZeroValues verifies that the network struct types are well
// formed and can be created with zero values.
func TestNetworkStructZeroValues(t *testing.T) {
	_ = AddressStatus{}
	_ = RouteStatus{}
	_ = HostnameStatus{}
	_ = DNSUpstream{}

	a := AddressStatus{
		NodeHostname: "node1",
		Interface:    "eth0",
		Address:      "10.0.0.1/24",
		Scope:        "global",
		Flags:        "",
	}
	if a.NodeHostname != "node1" {
		t.Fatalf("expected node1 got %s", a.NodeHostname)
	}

	r := RouteStatus{
		NodeHostname: "node1",
		Destination:  "0.0.0.0/0",
		Gateway:      "10.0.0.254",
		Interface:    "eth0",
		Metric:       100,
	}
	if r.Metric != 100 {
		t.Fatalf("expected 100 got %d", r.Metric)
	}

	h := HostnameStatus{NodeHostname: "10.0.0.1", Hostname: "talos-node1"}
	if h.Hostname == "" {
		t.Fatal("hostname should not be empty")
	}

	d := DNSUpstream{NodeHostname: "node1", Address: "8.8.8.8"}
	if d.Address == "" {
		t.Fatal("address should not be empty")
	}
}

// TestStorageStructZeroValues verifies that the storage struct types are well
// formed and can be created with zero values.
func TestStorageStructZeroValues(t *testing.T) {
	_ = BlockDevice{}
	_ = DiscoveredVolume{}
	_ = VolumeStatus{}

	bd := BlockDevice{
		NodeHostname: "node1",
		Name:         "sda",
		DevType:      "sata",
		BusPath:      "/pci0000:00/0000:00:1f.2",
		Size:         500107862016,
	}
	if bd.Size == 0 {
		t.Fatal("size should not be zero")
	}

	dv := DiscoveredVolume{
		NodeHostname: "node1",
		Name:         "sda1",
		FSType:       "ext4",
		Label:        "BOOT",
		UUID:         "abc-123",
		Size:         536870912,
	}
	if dv.FSType == "" {
		t.Fatal("fstype should not be empty")
	}

	vs := VolumeStatus{
		NodeHostname: "node1",
		Name:         "STATE",
		Phase:        "ready",
		MountSpec:    "/var/lib/talos",
	}
	if vs.Phase == "" {
		t.Fatal("phase should not be empty")
	}
}

// TestNodeResourceZeroValues verifies the generic NodeResource wrapper type.
func TestNodeResourceZeroValues(t *testing.T) {
	nr := NodeResource[string]{
		NodeHostname: "node1",
		Resource:     "some-value",
	}
	if nr.NodeHostname != "node1" {
		t.Fatalf("expected node1 got %s", nr.NodeHostname)
	}
	if nr.Resource != "some-value" {
		t.Fatalf("expected some-value got %s", nr.Resource)
	}
}

// TestDiagnosticStructZeroValues verifies that DiagnosticEntry fields exist.
func TestDiagnosticStructZeroValues(t *testing.T) {
	d := DiagnosticEntry{
		NodeHostname: "node1",
		ID:           "address-overlap",
		Severity:     "warning",
		Message:      "address overlap detected",
		Details:      "10.0.0.1 overlaps with 10.0.0.0/24",
	}
	if d.NodeHostname != "node1" {
		t.Fatalf("expected node1 got %s", d.NodeHostname)
	}
	if d.Severity != "warning" {
		t.Fatalf("expected warning got %s", d.Severity)
	}
	if d.Message == "" {
		t.Fatal("message should not be empty")
	}
}

// TestListDiagnosticsNilClient verifies nil client returns empty without error.
func TestListDiagnosticsNilClient(t *testing.T) {
	ctx := t.Context()
	diags, err := ListDiagnostics(ctx, nil)
	if err != nil {
		t.Fatalf("ListDiagnostics nil client: unexpected error: %v", err)
	}
	if diags != nil {
		t.Fatal("ListDiagnostics nil client: expected nil result")
	}
}

// TestListFunctionsNilClient verifies that list functions return nil/empty when
// the client is nil, without panicking.
func TestListFunctionsNilClient(t *testing.T) {
	ctx := t.Context()

	addrs, err := ListAddresses(ctx, nil)
	if err != nil {
		t.Fatalf("ListAddresses nil client: unexpected error: %v", err)
	}
	if addrs != nil {
		t.Fatal("ListAddresses nil client: expected nil result")
	}

	routes, err := ListRoutes(ctx, nil)
	if err != nil {
		t.Fatalf("ListRoutes nil client: unexpected error: %v", err)
	}
	if routes != nil {
		t.Fatal("ListRoutes nil client: expected nil result")
	}

	hostnames, err := ListHostnames(ctx, nil)
	if err != nil {
		t.Fatalf("ListHostnames nil client: unexpected error: %v", err)
	}
	if hostnames != nil {
		t.Fatal("ListHostnames nil client: expected nil result")
	}

	dns, err := ListDNSUpstreams(ctx, nil)
	if err != nil {
		t.Fatalf("ListDNSUpstreams nil client: unexpected error: %v", err)
	}
	if dns == nil {
		// graceful empty slice is also acceptable
		dns = []DNSUpstream{}
	}

	bds, err := ListBlockDevices(ctx, nil)
	if err != nil {
		t.Fatalf("ListBlockDevices nil client: unexpected error: %v", err)
	}
	if bds != nil {
		t.Fatal("ListBlockDevices nil client: expected nil result")
	}

	dvs, err := ListDiscoveredVolumes(ctx, nil)
	if err != nil {
		t.Fatalf("ListDiscoveredVolumes nil client: unexpected error: %v", err)
	}
	if dvs != nil {
		t.Fatal("ListDiscoveredVolumes nil client: expected nil result")
	}

	vss, err := ListVolumeStatuses(ctx, nil)
	if err != nil {
		t.Fatalf("ListVolumeStatuses nil client: unexpected error: %v", err)
	}
	if vss != nil {
		t.Fatal("ListVolumeStatuses nil client: expected nil result")
	}

	_ = dns
}
