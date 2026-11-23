package managedproxy

import "bytes"

// windowsHyperVVNIC marks a Hyper-V virtual NIC on the management OS. The
// string comes from the driver, not a localized resource, so it is stable
// across display languages.
const windowsHyperVVNIC = "Hyper-V Virtual Ethernet Adapter"

type windowsPhysicalAdapter struct {
	name   string
	metric uint32
}

// windowsAdapterInfo is what the IP Helper reports for one Up adapter.
type windowsAdapterInfo struct {
	index       uint32
	name        string
	description string
	metric      uint32
	hardware    bool   // MIB_IF_ROW2 HardwareInterface
	ipv4        bool   // has a preferred, routable IPv4 address
	mac         []byte // permanent or current link-layer address
}

// windowsUplinkAdapters keeps hardware adapters that carry IPv4. When a
// hardware adapter is bound to a Hyper-V external switch it has no IPv4 of its
// own; the host's address sits on the management vEthernet adapter, which
// reuses the hardware adapter's MAC. Only such an exact twin of an Up hardware
// adapter without IPv4 is accepted. VPN, TUN, NAT and other virtual adapters
// have no hardware twin and stay excluded.
//
// A host that runs its whole uplink behind a router VM (OpenWrt, pfSense and
// similar) has neither shape: the physical port belongs to the VM, and the
// host's only IPv4 address is a Hyper-V vNIC with its own MAC, so no twin
// exists. When nothing else qualifies, those vNICs become the candidates; the
// forward table still decides which one reaches the internet.
func windowsUplinkAdapters(list []windowsAdapterInfo) map[uint32]windowsPhysicalAdapter {
	var bridged [][]byte
	for _, a := range list {
		if a.hardware && !a.ipv4 && validMAC(a.mac) {
			bridged = append(bridged, a.mac)
		}
	}
	out := map[uint32]windowsPhysicalAdapter{}
	for _, a := range list {
		if a.index == 0 || a.name == "" || !a.ipv4 {
			continue
		}
		accept := a.hardware
		if !accept && validMAC(a.mac) {
			matches := 0
			for _, mac := range bridged {
				if bytes.Equal(mac, a.mac) {
					matches++
				}
			}
			accept = matches == 1
		}
		if accept {
			out[a.index] = windowsPhysicalAdapter{name: a.name, metric: a.metric}
		}
	}
	if len(out) == 0 {
		for _, a := range list {
			// A valid unicast MAC keeps TUN adapters out; the Hyper-V
			// description keeps VPN, TAP and vendor virtual NICs out.
			if a.index == 0 || a.name == "" || a.hardware || !a.ipv4 || !validMAC(a.mac) {
				continue
			}
			if !bytes.Contains([]byte(a.description), []byte(windowsHyperVVNIC)) {
				continue
			}
			out[a.index] = windowsPhysicalAdapter{name: a.name, metric: a.metric}
		}
	}
	return out
}

func validMAC(mac []byte) bool {
	return len(mac) == 6 && !bytes.Equal(mac, make([]byte, 6)) && mac[0]&1 == 0
}
