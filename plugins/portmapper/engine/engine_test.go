package engine

import (
	"net"
	"testing"

	t "github.com/containernetworking/cni/pkg/types"
	portmapperTypes "github.com/aws/amazon-ecs-cni-plugins/plugins/portmapper/types"
)

// mockPortMapper implements PortMapper interface for testing
type mockPortMapper struct {
	forwardPortsCalled bool
	forwardPortsError  error
}

func (m *mockPortMapper) ForwardPorts(config *PortMapConf, containerNet net.IPNet) error {
	m.forwardPortsCalled = true
	return m.forwardPortsError
}

func (m *mockPortMapper) CheckPorts(config *PortMapConf, containerNet net.IPNet) error {
	return nil
}

func (m *mockPortMapper) UnforwardPorts(config *PortMapConf) error {
	return nil
}

// mockResult implements t.Result interface for testing
type mockResult struct{}

func (m *mockResult) Version() string { return "0.3.0" }
func (m *mockResult) GetAsVersion(version string) (t.Result, error) { return m, nil }
func (m *mockResult) Print() error { return nil }
func (m *mockResult) String() string { return "mockResult" }

func TestForwardPorts_NoPrevResult(t *testing.T) {
	e := &engine{
		config: PortMapConf{},
	}
	
	err := e.ForwardPorts()
	if err == nil {
		t.Error("Expected error when PrevResult is nil")
	}
	if err.Error() != "must be called as chained plugin" {
		t.Errorf("Expected 'must be called as chained plugin', got %v", err)
	}
}

func TestForwardPorts_NoPortMaps(t *testing.T) {
	e := &engine{
		config: PortMapConf{
			PluginConf: portmapperTypes.PluginConf{
				PrevResult: &mockResult{},
			},
			RuntimeConfig: struct {
				PortMaps []portmapperTypes.PortMapEntry `json:"portMappings,omitempty"`
			}{
				PortMaps: []portmapperTypes.PortMapEntry{},
			},
		},
	}
	
	err := e.ForwardPorts()
	if err != nil {
		t.Errorf("Expected no error when no port mappings, got %v", err)
	}
}

func TestForwardPorts_IPv4(t *testing.T) {
	mockMapper := &mockPortMapper{}
	snat := true
	
	e := &engine{
		config: PortMapConf{
			PluginConf: portmapperTypes.PluginConf{
				PrevResult: &mockResult{},
			},
			Mapper: mockMapper,
			SNAT:   &snat,
			ContIPv4: net.IPNet{
				IP:   net.ParseIP("192.168.1.10"),
				Mask: net.CIDRMask(24, 32),
			},
			RuntimeConfig: struct {
				PortMaps []portmapperTypes.PortMapEntry `json:"portMappings,omitempty"`
			}{
				PortMaps: []portmapperTypes.PortMapEntry{
					{HostPort: 8080, ContainerPort: 80, Protocol: "tcp"},
				},
			},
		},
	}
	
	err := e.ForwardPorts()
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	
	if !mockMapper.forwardPortsCalled {
		t.Error("Expected ForwardPorts to be called on mapper")
	}
}

func TestForwardPorts_IPv6(t *testing.T) {
	mockMapper := &mockPortMapper{}
	snat := false
	
	e := &engine{
		config: PortMapConf{
			PluginConf: portmapperTypes.PluginConf{
				PrevResult: &mockResult{},
			},
			Mapper: mockMapper,
			SNAT:   &snat,
			ContIPv6: net.IPNet{
				IP:   net.ParseIP("2001:db8::1"),
				Mask: net.CIDRMask(64, 128),
			},
			RuntimeConfig: struct {
				PortMaps []portmapperTypes.PortMapEntry `json:"portMappings,omitempty"`
			}{
				PortMaps: []portmapperTypes.PortMapEntry{
					{HostPort: 8080, ContainerPort: 80, Protocol: "tcp"},
				},
			},
		},
	}
	
	err := e.ForwardPorts()
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	
	if !mockMapper.forwardPortsCalled {
		t.Error("Expected ForwardPorts to be called on mapper")
	}
}

func TestDetectBackendOfConditions(t *testing.T) {
	tests := []struct {
		name       string
		conditions *[]string
		expected   string
	}{
		{
			name:       "nil conditions",
			conditions: nil,
			expected:   "",
		},
		{
			name:       "empty conditions",
			conditions: &[]string{},
			expected:   "",
		},
		{
			name:       "iptables conditions with dash",
			conditions: &[]string{"-d", "192.168.1.1"},
			expected:   "iptables",
		},
		{
			name:       "iptables conditions with exclamation",
			conditions: &[]string{"!", "-d", "192.168.1.1"},
			expected:   "iptables",
		},
		{
			name:       "nftables conditions",
			conditions: &[]string{"ip", "daddr", "192.168.1.1"},
			expected:   "nftables",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectBackendOfConditions(tt.conditions)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestEnsureBackend(t *testing.T) {
	tests := []struct {
		name        string
		config      *PortMapConf
		expectError bool
	}{
		{
			name: "default backend selection",
			config: &PortMapConf{
				Backend: nil,
			},
			expectError: false,
		},
		{
			name: "valid iptables backend",
			config: &PortMapConf{
				Backend: func() *string { s := "iptables"; return &s }(),
			},
			expectError: false,
		},
		{
			name: "valid nftables backend",
			config: &PortMapConf{
				Backend: func() *string { s := "nftables"; return &s }(),
			},
			expectError: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ensureBackend(tt.config)
			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got %v", err)
			}
		})
	}
}