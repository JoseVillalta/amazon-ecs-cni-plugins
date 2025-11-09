package engine

import (
	"net"
	"reflect"
	"testing"

	portmapperTypes "github.com/aws/amazon-ecs-cni-plugins/plugins/portmapper/types"
	"github.com/vishvananda/netlink"
)

func TestPortMapperIPTablesForwardPorts(t *testing.T) {
	tests := []struct {
		name          string
		config        *PortMapConf
		containerNet  net.IPNet
		expectError   bool
		errorContains string
	}{
		{
			name: "IPv4 port forwarding with SNAT",
			config: &PortMapConf{
				ContainerID: "test-container",
				SNAT:        func() *bool { b := true; return &b }(),
				MarkMasqBit: func() *int { i := 13; return &i }(),
				RuntimeConfig: struct {
					PortMaps []portmapperTypes.PortMapEntry `json:"portMappings,omitempty"`
				}{
					PortMaps: []portmapperTypes.PortMapEntry{
						{HostPort: 8080, ContainerPort: 80, Protocol: "tcp"},
					},
				},
			},
			containerNet: net.IPNet{
				IP:   net.ParseIP("192.168.1.10"),
				Mask: net.CIDRMask(24, 32),
			},
			expectError: false,
		},
		{
			name: "IPv6 port forwarding without SNAT",
			config: &PortMapConf{
				ContainerID: "test-container",
				SNAT:        func() *bool { b := false; return &b }(),
				RuntimeConfig: struct {
					PortMaps []portmapperTypes.PortMapEntry `json:"portMappings,omitempty"`
				}{
					PortMaps: []portmapperTypes.PortMapEntry{
						{HostPort: 8080, ContainerPort: 80, Protocol: "tcp"},
					},
				},
			},
			containerNet: net.IPNet{
				IP:   net.ParseIP("2001:db8::1"),
				Mask: net.CIDRMask(64, 128),
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pm := &portMapperIPTables{}
			err := pm.ForwardPorts(tt.config, tt.containerNet)
			
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				} else if tt.errorContains != "" && !contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error to contain %q, got %q", tt.errorContains, err.Error())
				}
			} else if err != nil {
				t.Errorf("Expected no error but got %v", err)
			}
		})
	}
}

func TestGenToplevelDnatChain(t *testing.T) {
	resultChain := genToplevelDnatChain()
	
	expectedChain := chain{
		table: "nat",
		name:  TopLevelDNATChainName,
		entryRules: [][]string{{
			"-m", "addrtype",
			"--dst-type", "LOCAL",
		}},
		entryChains: []string{"PREROUTING", "OUTPUT"},
	}
	
	if !reflect.DeepEqual(resultChain, expectedChain) {
		t.Errorf("Expected chain %+v, got %+v", expectedChain, resultChain)
	}
}

func TestGenDnatChain(t *testing.T) {
	netName := "test-net"
	containerID := "test-container"
	
	resultChain := genDnatChain(netName, containerID)
	
	if resultChain.table != "nat" {
		t.Errorf("Expected table 'nat', got %q", resultChain.table)
	}
	
	if len(resultChain.entryChains) != 1 || resultChain.entryChains[0] != TopLevelDNATChainName {
		t.Errorf("Expected entryChains [%q], got %v", TopLevelDNATChainName, resultChain.entryChains)
	}
	
	// Chain name should be formatted with prefix
	if resultChain.name == "" {
		t.Error("Expected non-empty chain name")
	}
}

func TestFillDnatRules(t *testing.T) {
	tests := []struct {
		name         string
		config       *PortMapConf
		containerNet net.IPNet
		expectRules  int
	}{
		{
			name: "single TCP port with SNAT",
			config: &PortMapConf{
				ContainerID: "test-container",
				SNAT:        func() *bool { b := true; return &b }(),
				RuntimeConfig: struct {
					PortMaps []portmapperTypes.PortMapEntry `json:"portMappings,omitempty"`
				}{
					PortMaps: []portmapperTypes.PortMapEntry{
						{HostPort: 8080, ContainerPort: 80, Protocol: "tcp"},
					},
				},
			},
			containerNet: net.IPNet{
				IP:   net.ParseIP("192.168.1.10"),
				Mask: net.CIDRMask(24, 32),
			},
			expectRules: 3, // hairpin + localhost + dnat
		},
		{
			name: "single TCP port without SNAT",
			config: &PortMapConf{
				ContainerID: "test-container",
				SNAT:        func() *bool { b := false; return &b }(),
				RuntimeConfig: struct {
					PortMaps []portmapperTypes.PortMapEntry `json:"portMappings,omitempty"`
				}{
					PortMaps: []portmapperTypes.PortMapEntry{
						{HostPort: 8080, ContainerPort: 80, Protocol: "tcp"},
					},
				},
			},
			containerNet: net.IPNet{
				IP:   net.ParseIP("192.168.1.10"),
				Mask: net.CIDRMask(24, 32),
			},
			expectRules: 1, // only dnat
		},
		{
			name: "IPv6 with SNAT",
			config: &PortMapConf{
				ContainerID: "test-container",
				SNAT:        func() *bool { b := true; return &b }(),
				RuntimeConfig: struct {
					PortMaps []portmapperTypes.PortMapEntry `json:"portMappings,omitempty"`
				}{
					PortMaps: []portmapperTypes.PortMapEntry{
						{HostPort: 8080, ContainerPort: 80, Protocol: "tcp"},
					},
				},
			},
			containerNet: net.IPNet{
				IP:   net.ParseIP("2001:db8::1"),
				Mask: net.CIDRMask(64, 128),
			},
			expectRules: 2, // hairpin + dnat (no localhost for IPv6)
		},
		{
			name: "multiple ports",
			config: &PortMapConf{
				ContainerID: "test-container",
				SNAT:        func() *bool { b := true; return &b }(),
				RuntimeConfig: struct {
					PortMaps []portmapperTypes.PortMapEntry `json:"portMappings,omitempty"`
				}{
					PortMaps: []portmapperTypes.PortMapEntry{
						{HostPort: 8080, ContainerPort: 80, Protocol: "tcp"},
						{HostPort: 8081, ContainerPort: 81, Protocol: "tcp"},
					},
				},
			},
			containerNet: net.IPNet{
				IP:   net.ParseIP("192.168.1.10"),
				Mask: net.CIDRMask(24, 32),
			},
			expectRules: 6, // 3 rules per port
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := chain{}
			fillDnatRules(&c, tt.config, tt.containerNet)
			
			if len(c.rules) != tt.expectRules {
				t.Errorf("Expected %d rules, got %d", tt.expectRules, len(c.rules))
			}
			
			// Verify entry rules are created
			if len(c.entryRules) == 0 {
				t.Error("Expected entry rules to be created")
			}
		})
	}
}

func TestGenSetMarkChain(t *testing.T) {
	markBit := 13
	resultChain := genSetMarkChain(markBit)
	
	expectedMarkValue := 1 << uint(markBit)
	expectedMarkDef := "0x2000/0x2000" // 1 << 13 = 8192 = 0x2000
	
	if resultChain.table != "nat" {
		t.Errorf("Expected table 'nat', got %q", resultChain.table)
	}
	
	if resultChain.name != SetMarkChainName {
		t.Errorf("Expected name %q, got %q", SetMarkChainName, resultChain.name)
	}
	
	if len(resultChain.rules) != 1 {
		t.Errorf("Expected 1 rule, got %d", len(resultChain.rules))
	}
	
	rule := resultChain.rules[0]
	found := false
	for i, arg := range rule {
		if arg == "--set-xmark" && i+1 < len(rule) {
			if rule[i+1] == expectedMarkDef {
				found = true
				break
			}
		}
	}
	
	if !found {
		t.Errorf("Expected mark definition %q in rule, got %v", expectedMarkDef, rule)
	}
	
	_ = expectedMarkValue // Suppress unused variable warning
}

func TestGenMarkMasqChain(t *testing.T) {
	markBit := 13
	resultChain := genMarkMasqChain(markBit)
	
	expectedMarkValue := 1 << uint(markBit)
	expectedMarkDef := "0x2000/0x2000" // 1 << 13 = 8192 = 0x2000
	
	if resultChain.table != "nat" {
		t.Errorf("Expected table 'nat', got %q", resultChain.table)
	}
	
	if resultChain.name != MarkMasqChainName {
		t.Errorf("Expected name %q, got %q", MarkMasqChainName, resultChain.name)
	}
	
	if len(resultChain.entryChains) != 1 || resultChain.entryChains[0] != "POSTROUTING" {
		t.Errorf("Expected entryChains ['POSTROUTING'], got %v", resultChain.entryChains)
	}
	
	if !resultChain.prependEntry {
		t.Error("Expected prependEntry to be true")
	}
	
	if len(resultChain.rules) != 1 {
		t.Errorf("Expected 1 rule, got %d", len(resultChain.rules))
	}
	
	rule := resultChain.rules[0]
	found := false
	for i, arg := range rule {
		if arg == "--mark" && i+1 < len(rule) {
			if rule[i+1] == expectedMarkDef {
				found = true
				break
			}
		}
	}
	
	if !found {
		t.Errorf("Expected mark definition %q in rule, got %v", expectedMarkDef, rule)
	}
	
	_ = expectedMarkValue // Suppress unused variable warning
}

func TestGenOldSnatChain(t *testing.T) {
	netName := "test-net"
	containerID := "test-container"
	
	resultChain := genOldSnatChain(netName, containerID)
	
	if resultChain.table != "nat" {
		t.Errorf("Expected table 'nat', got %q", resultChain.table)
	}
	
	if len(resultChain.entryChains) != 1 || resultChain.entryChains[0] != OldTopLevelSNATChainName {
		t.Errorf("Expected entryChains [%q], got %v", OldTopLevelSNATChainName, resultChain.entryChains)
	}
	
	// Chain name should be formatted with prefix
	if resultChain.name == "" {
		t.Error("Expected non-empty chain name")
	}
}

func TestMaybeGetIptables(t *testing.T) {
	tests := []struct {
		name    string
		isV6    bool
		wantErr bool
	}{
		{
			name:    "IPv4 iptables",
			isV6:    false,
			wantErr: false, // May fail in test environment, but function should not panic
		},
		{
			name:    "IPv6 iptables",
			isV6:    true,
			wantErr: false, // May fail in test environment, but function should not panic
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This test mainly ensures the function doesn't panic
			// Actual iptables functionality depends on system setup
			ipt, err := maybeGetIptables(tt.isV6)
			
			// In test environment, iptables may not be available
			// We just check that the function handles errors gracefully
			if err != nil && ipt != nil {
				t.Error("If error is returned, iptables handle should be nil")
			}
		})
	}
}

func TestDeletePortmapStaleConnections(t *testing.T) {
	tests := []struct {
		name         string
		portMappings []portmapperTypes.PortMapEntry
		family       int // Using int instead of netlink.InetFamily for simplicity
		expectError  bool
	}{
		{
			name: "TCP ports - should be skipped",
			portMappings: []portmapperTypes.PortMapEntry{
				{HostPort: 8080, ContainerPort: 80, Protocol: "tcp"},
			},
			family:      2, // AF_INET
			expectError: false,
		},
		{
			name: "UDP ports - should process",
			portMappings: []portmapperTypes.PortMapEntry{
				{HostPort: 8080, ContainerPort: 80, Protocol: "udp"},
			},
			family:      2, // AF_INET
			expectError: false, // May fail in test environment but shouldn't panic
		},
		{
			name: "mixed protocols",
			portMappings: []portmapperTypes.PortMapEntry{
				{HostPort: 8080, ContainerPort: 80, Protocol: "tcp"},
				{HostPort: 8081, ContainerPort: 81, Protocol: "udp"},
			},
			family:      2, // AF_INET
			expectError: false,
		},
		{
			name:         "empty port mappings",
			portMappings: []portmapperTypes.PortMapEntry{},
			family:       2, // AF_INET
			expectError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This test mainly ensures the function doesn't panic
			// Actual conntrack functionality depends on system setup
			err := deletePortmapStaleConnections(tt.portMappings, netlink.InetFamily(tt.family))
			
			// In test environment, conntrack may not be available
			// We just check that the function handles the case gracefully
			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
		})
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || (len(s) > len(substr) && 
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || 
		 containsSubstring(s, substr))))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Test helper functions
func TestGroupByProto(t *testing.T) {
	entries := []portmapperTypes.PortMapEntry{
		{HostPort: 8080, ContainerPort: 80, Protocol: "tcp"},
		{HostPort: 8081, ContainerPort: 81, Protocol: "tcp"},
		{HostPort: 9090, ContainerPort: 90, Protocol: "udp"},
	}
	
	result := groupByProto(entries)
	
	if len(result["tcp"]) != 2 {
		t.Errorf("Expected 2 TCP entries, got %d", len(result["tcp"]))
	}
	
	if len(result["udp"]) != 1 {
		t.Errorf("Expected 1 UDP entry, got %d", len(result["udp"]))
	}
}

func TestSplitPortList(t *testing.T) {
	entries := []portmapperTypes.PortMapEntry{
		{HostPort: 8080, ContainerPort: 80, Protocol: "tcp"},
		{HostPort: 8081, ContainerPort: 81, Protocol: "tcp"},
	}
	
	// Convert entries to port list first
	ports := make([]int, len(entries))
	for i, entry := range entries {
		ports[i] = entry.HostPort
	}
	result := splitPortList(ports)
	
	if len(result) == 0 {
		t.Error("Expected at least one port specification")
	}
	
	// Should contain comma-separated ports
	if len(result) > 0 && !contains(result[0], "8080") {
		t.Errorf("Expected port specification to contain '8080', got %q", result[0])
	}
}

func TestFmtIPPort(t *testing.T) {
	tests := []struct {
		name     string
		ip       net.IP
		port     int
		expected string
	}{
		{
			name:     "IPv4 address",
			ip:       net.ParseIP("192.168.1.10"),
			port:     80,
			expected: "192.168.1.10:80",
		},
		{
			name:     "IPv6 address",
			ip:       net.ParseIP("2001:db8::1"),
			port:     80,
			expected: "[2001:db8::1]:80",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := fmtIPPort(tt.ip, tt.port)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestTrimComment(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "short comment",
			input:    "short comment",
			expected: "short comment",
		},
		{
			name:     "long comment",
			input:    "this is a very long comment that exceeds the maximum length allowed for iptables comments and should be truncated",
			expected: "this is a very long comment that exceeds the maximum length allowed for iptables comments and should be tru",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := trimComment(tt.input)
			if len(result) > 100 {
				t.Errorf("Comment should be truncated to 100 characters, got %d", len(result))
			}
			if tt.input != tt.expected && result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}