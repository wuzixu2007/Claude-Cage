package managedproxy

import (
	"reflect"
	"testing"
)

func TestWindowsUplinkAdaptersAcceptsHyperVManagementTwin(t *testing.T) {
	nic := []byte{0x00, 0x15, 0x5d, 0x01, 0x02, 0x03}
	list := []windowsAdapterInfo{
		{index: 4, name: "Ethernet", hardware: true, ipv4: false, mac: nic},
		{index: 12, name: "vEthernet (External)", metric: 15, hardware: false, ipv4: true, mac: nic},
		{index: 20, name: "sing-box", hardware: false, ipv4: true, mac: []byte{0x02, 0, 0, 0, 0, 1}},
		{index: 21, name: "VMware Network Adapter VMnet8", hardware: false, ipv4: true, mac: []byte{0x00, 0x50, 0x56, 0xc0, 0, 8}},
		{index: 22, name: "aTrustVNIC", hardware: false, ipv4: true},
	}
	want := map[uint32]windowsPhysicalAdapter{12: {name: "vEthernet (External)", metric: 15}}
	if got := windowsUplinkAdapters(list); !reflect.DeepEqual(got, want) {
		t.Fatalf("uplinks = %+v, want %+v", got, want)
	}
}

func TestWindowsUplinkAdaptersKeepsHardwareAndRejectsUntwinnedVirtual(t *testing.T) {
	wifi := []byte{0x3c, 0x22, 0xfb, 0x01, 0x02, 0x03}
	list := []windowsAdapterInfo{
		{index: 7, name: "Wi-Fi", metric: 25, hardware: true, ipv4: true, mac: wifi},
		// A virtual adapter copying the MAC of a hardware adapter that still
		// has its own IPv4 is not a bridged management adapter.
		{index: 30, name: "Clone", hardware: false, ipv4: true, mac: wifi},
		{index: 31, name: "Zero MAC", hardware: false, ipv4: true, mac: make([]byte, 6)},
	}
	want := map[uint32]windowsPhysicalAdapter{7: {name: "Wi-Fi", metric: 25}}
	if got := windowsUplinkAdapters(list); !reflect.DeepEqual(got, want) {
		t.Fatalf("uplinks = %+v, want %+v", got, want)
	}
	// Two bridged hardware adapters sharing one MAC are ambiguous.
	dup := []byte{0x00, 0x15, 0x5d, 9, 9, 9}
	ambiguous := []windowsAdapterInfo{
		{index: 1, name: "A", hardware: true, mac: dup},
		{index: 2, name: "B", hardware: true, mac: dup},
		{index: 3, name: "vEthernet", ipv4: true, mac: dup},
	}
	if got := windowsUplinkAdapters(ambiguous); len(got) != 0 {
		t.Fatalf("ambiguous twin accepted: %+v", got)
	}
}

func TestWindowsUplinkAdaptersAcceptsRouterVMUplink(t *testing.T) {
	// The whole uplink sits behind a router VM: the physical port belongs to
	// the VM and carries no IPv4, and the host's only address is a Hyper-V
	// vNIC with its own MAC, so no bridged twin exists.
	nic := []byte{0x1c, 0x83, 0x41, 0xc5, 0xcf, 0xd7}
	list := []windowsAdapterInfo{
		{index: 14, name: "Ethernet", description: "Realtek PCIe GbE Family Controller", hardware: true, ipv4: false, mac: nic},
		{index: 82, name: "vEthernet (OpenWrt-LAN)", description: "Hyper-V Virtual Ethernet Adapter #2", metric: 25, ipv4: true, mac: []byte{0x00, 0x15, 0x5d, 0x00, 0x01, 0x00}},
		{index: 59, name: "vEthernet (Default Switch)", description: "Hyper-V Virtual Ethernet Adapter", metric: 5, ipv4: true, mac: []byte{0x00, 0x15, 0x5d, 0x00, 0x51, 0x01}},
		{index: 66, name: "Mihomo", description: "Meta Tunnel", ipv4: true},
		{index: 16, name: "Radmin VPN", description: "Famatech Radmin VPN Ethernet Adapter", metric: 35, ipv4: true, mac: []byte{0x02, 0x50, 0x38, 0xea, 0x7b, 0x93}},
	}
	want := map[uint32]windowsPhysicalAdapter{
		82: {name: "vEthernet (OpenWrt-LAN)", metric: 25},
		59: {name: "vEthernet (Default Switch)", metric: 5},
	}
	if got := windowsUplinkAdapters(list); !reflect.DeepEqual(got, want) {
		t.Fatalf("uplinks = %+v, want %+v", got, want)
	}
}

func TestWindowsUplinkAdaptersRejectsVirtualUplinksBesideHardware(t *testing.T) {
	// The router-VM fallback stays off whenever a hardware adapter already
	// carries IPv4, so an extra vNIC cannot win the forward-table ranking.
	nic := []byte{0x3c, 0x22, 0xfb, 0x01, 0x02, 0x03}
	list := []windowsAdapterInfo{
		{index: 7, name: "Wi-Fi", description: "Intel(R) Wi-Fi 6E AX211 160MHz", metric: 35, hardware: true, ipv4: true, mac: nic},
		{index: 82, name: "vEthernet (OpenWrt-LAN)", description: "Hyper-V Virtual Ethernet Adapter #2", metric: 5, ipv4: true, mac: []byte{0x00, 0x15, 0x5d, 0x00, 0x01, 0x00}},
	}
	want := map[uint32]windowsPhysicalAdapter{7: {name: "Wi-Fi", metric: 35}}
	if got := windowsUplinkAdapters(list); !reflect.DeepEqual(got, want) {
		t.Fatalf("uplinks = %+v, want %+v", got, want)
	}
}

func TestWindowsUplinkAdaptersRejectsUntwinnedNonHyperVVirtual(t *testing.T) {
	// Vendor VPN, TAP and container virtual NICs must stay out even when no
	// hardware adapter carries IPv4 at all.
	list := []windowsAdapterInfo{
		{index: 4, name: "Ethernet", description: "Realtek PCIe GbE Family Controller", hardware: true, mac: []byte{0x1c, 0x83, 0x41, 0xc5, 0xcf, 0xd7}},
		{index: 20, name: "aTrustVNIC", description: "Sangfor aTrust Virtual NIC", ipv4: true, mac: []byte{0x02, 0x00, 0x00, 0x00, 0x00, 0x01}},
		{index: 21, name: "cfw-tap", description: "TAP-Windows Adapter V9", ipv4: true, mac: []byte{0x00, 0xff, 0x1e, 0xf5, 0xa0, 0x81}},
		{index: 22, name: "vEthernet (Default Switch)", description: "Hyper-V Virtual Ethernet Adapter", ipv4: false, mac: []byte{0x00, 0x15, 0x5d, 0x00, 0x51, 0x01}},
		{index: 66, name: "sing-box", description: "sing-box Tunnel", ipv4: true, mac: make([]byte, 6)},
	}
	if got := windowsUplinkAdapters(list); len(got) != 0 {
		t.Fatalf("virtual uplinks accepted without a Hyper-V router host: %+v", got)
	}
}
