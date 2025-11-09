package types

import (
	"net"

	"github.com/containernetworking/cni/pkg/types"
	"github.com/containernetworking/cni/pkg/types/current"
)

type PortMapper interface {
	ForwardPorts(config *PortMapConf, containerNet net.IPNet) error
	CheckPorts(config *PortMapConf, containerNet net.IPNet) error
	UnforwardPorts(config *PortMapConf) error
}

// These are vars rather than consts so we can "&" them
var (
	iptablesBackend = "iptables"
	nftablesBackend = "nftables"
)

// Use PluginConf instead of NetConf, the NetConf
// backwards-compat alias will be removed in a future release.
type NetConf = PluginConf

// PluginConf describes a plugin configuration for a specific network.
type PluginConf struct {
	CNIVersion string `json:"cniVersion,omitempty"`

	Name         string          `json:"name,omitempty"`
	Type         string          `json:"type,omitempty"`
	Capabilities map[string]bool `json:"capabilities,omitempty"`

	RawPrevResult map[string]interface{} `json:"prevResult,omitempty"`
	PrevResult    types.Result           `json:"-"`
}

type PortMapConf struct {
	PluginConf

	Mapper PortMapper `json:"-"`

	// Generic config
	Backend       *string                `json:"backend,omitempty"`
	SNAT          *bool                  `json:"snat,omitempty"`
	ConditionsV4  *[]string              `json:"conditionsV4"`
	ConditionsV6  *[]string              `json:"conditionsV6"`
	MasqAll       bool                   `json:"masqAll,omitempty"`
	MarkMasqBit   *int                   `json:"markMasqBit"`
	RuntimeConfig struct {
		PortMaps []PortMapEntry `json:"portMappings,omitempty"`
	} `json:"runtimeConfig,omitempty"`

	// iptables-backend-specific config
	ExternalSetMarkChain *string `json:"externalSetMarkChain"`

	// These are fields parsed out of the config or the environment;
	// included here for convenience
	ContainerID string    `json:"-"`
	ContIPv4    net.IPNet `json:"-"`
	ContIPv6    net.IPNet `json:"-"`
}

// PortMapEntry corresponds to a single entry in the port_mappings argument,
// see CONVENTIONS.md
type PortMapEntry struct {
	HostPort      int    `json:"hostPort"`
	ContainerPort int    `json:"containerPort"`
	Protocol      string `json:"protocol"`
	HostIP        string `json:"hostIP,omitempty"`
}

// Result is an interface that provides the result of plugin execution
type Result struct {
	CNIVersion string               `json:"cniVersion,omitempty"`
	Interfaces []*current.Interface `json:"interfaces,omitempty"`
	IPs        []*current.IPConfig  `json:"ips,omitempty"`
	Routes     []*types.Route       `json:"routes,omitempty"`
	DNS        types.DNS            `json:"dns,omitempty"`
}
