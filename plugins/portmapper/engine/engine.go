// Copyright 2025 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may
// not use this file except in compliance with the License. A copy of the
// License is located at
//
//     http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either
// express or implied. See the License for the specific language governing
// permissions and limitations under the License.

package engine

import (
	"encoding/json"
	"fmt"

	// TODO clean this up
	"github.com/containernetworking/cni/pkg/skel"
	t "github.com/containernetworking/cni/pkg/types"
	"github.com/containernetworking/cni/pkg/types/current"

	portmapperTypes "github.com/aws/amazon-ecs-cni-plugins/plugins/portmapper/types"
	"github.com/containernetworking/plugins/pkg/utils"
)

// These are vars rather than consts so we can "&" them
var (
	iptablesBackend = "iptables"
	nftablesBackend = "nftables"
)

type NetConf = portmapperTypes.PluginConf
type PluginConf = portmapperTypes.PluginConf

// PluginConf describes a plugin configuration for a specific network.

type PortMapper = portmapperTypes.PortMapper
type PortMapConf = portmapperTypes.PortMapConf
type PortMapEntry = portmapperTypes.PortMapEntry

// The default mark bit to signal that masquerading is required
// Kubernetes uses 14 and 15, Calico uses 20-31.
const DefaultMarkBit = 13

type ResultFactoryFunc func([]byte) (t.Result, error)

type creator struct {
	// CNI Result spec versions that createFn can create a Result for
	versions []string
	createFn ResultFactoryFunc
}

var creators []*creator

func findCreator(version string) *creator {
	for _, c := range creators {
		for _, v := range c.versions {
			if v == version {
				return c
			}
		}
	}
	return nil
}

// Engine represents the execution engine for the portmapper plugin. It defines all the
// operations performed by the plugin
type Engine interface {
	ForwardPorts() error
	GetConfig() *PortMapConf
}

type engine struct {
	config PortMapConf
}

// New creates a new Engine object
func New(args *skel.CmdArgs) (Engine, error) {
	e := &engine{}
	conf, _, err := e.parseConfig(args.StdinData, args.IfName)
	e.config = *conf
	return e, err
}

func (e *engine) ForwardPorts() error {
	if e.config.PrevResult == nil {
		return fmt.Errorf("must be called as chained plugin")
	}

	if len(e.config.RuntimeConfig.PortMaps) == 0 {
		return nil // No port mappings to configure
	}

	if e.config.ContIPv4.IP != nil {
		if err := e.config.Mapper.ForwardPorts(&e.config, e.config.ContIPv4); err != nil {
			return err
		}
		// Delete conntrack entries for UDP to avoid conntrack blackholing traffic
		if err := deletePortmapStaleConnections(e.config.RuntimeConfig.PortMaps, 2); err != nil { // AF_INET = 2
			// Log error but don't fail
		}

		if *e.config.SNAT {
			// Set the route_localnet bit on the host interface
			hostIfName := getRoutableHostIF(e.config.ContIPv4.IP)
			if hostIfName != "" {
				if err := enableLocalnetRouting(hostIfName); err != nil {
					return fmt.Errorf("unable to enable route_localnet: %v", err)
				}
			}
		}
	}

	if e.config.ContIPv6.IP != nil {
		if err := e.config.Mapper.ForwardPorts(&e.config, e.config.ContIPv6); err != nil {
			return err
		}
		// Delete conntrack entries for UDP to avoid conntrack blackholing traffic
		if err := deletePortmapStaleConnections(e.config.RuntimeConfig.PortMaps, 10); err != nil { // AF_INET6 = 10
			// Log error but don't fail
		}
	}

	return nil
}

func (e *engine) GetConfig() *PortMapConf {
	return &e.config
}

// parseConfig parses the supplied configuration (and prevResult) from stdin.
func (e *engine) parseConfig(stdin []byte, ifName string) (*PortMapConf, *current.Result, error) {
	conf := PortMapConf{}

	if err := json.Unmarshal(stdin, &conf); err != nil {
		return nil, nil, fmt.Errorf("failed to parse network configuration: %v", err)
	}

	// Parse previous result.
	var result *current.Result

	if conf.RawPrevResult != nil {
		var err error
		if err = e.parsePrevResult(&conf.PluginConf); err != nil {
			return nil, nil, fmt.Errorf("could not parse prevResult: %v", err)
		}

		result, err = current.NewResultFromResult(conf.PrevResult)
		if err != nil {
			return nil, nil, fmt.Errorf("could not convert result to current version: %v", err)
		}
	}

	conf.Mapper = &portMapperIPTables{}

	if conf.SNAT == nil {
		tvar := true
		conf.SNAT = &tvar
	}

	if conf.MarkMasqBit != nil && conf.ExternalSetMarkChain != nil {
		return nil, nil, fmt.Errorf("Cannot specify externalSetMarkChain and markMasqBit")
	}

	if conf.MarkMasqBit == nil {
		bvar := DefaultMarkBit // go constants are "special"
		conf.MarkMasqBit = &bvar
	}

	if *conf.MarkMasqBit < 0 || *conf.MarkMasqBit > 31 {
		return nil, nil, fmt.Errorf("MasqMarkBit must be between 0 and 31")
	}

	err := ensureBackend(&conf)
	if err != nil {
		return nil, nil, err
	}

	conf.Mapper = &portMapperIPTables{}

	// Reject invalid port numbers
	for _, pm := range conf.RuntimeConfig.PortMaps {
		if pm.ContainerPort <= 0 {
			return nil, nil, fmt.Errorf("Invalid container port number: %d", pm.ContainerPort)
		}
		if pm.HostPort <= 0 {
			return nil, nil, fmt.Errorf("Invalid host port number: %d", pm.HostPort)
		}
	}

	if conf.PrevResult != nil {
		for _, ip := range result.IPs {
			isIPv4 := ip.Address.IP.To4() != nil
			if !isIPv4 && conf.ContIPv6.IP != nil {
				continue
			} else if isIPv4 && conf.ContIPv4.IP != nil {
				continue
			}

			// Skip known non-sandbox interfaces
			if ip.Interface >= 0 {
				intIdx := ip.Interface
				if intIdx < len(result.Interfaces) &&
					(result.Interfaces[intIdx].Name != ifName ||
						result.Interfaces[intIdx].Sandbox == "") {
					continue
				}
			}
			if ip.Address.IP.To4() != nil {
				conf.ContIPv4 = ip.Address
			} else {
				conf.ContIPv6 = ip.Address
			}
		}
	}

	return &conf, result, nil
}

func (e *engine) parsePrevResult(conf *PluginConf) error {
	if conf.RawPrevResult == nil {
		return nil
	}

	// Prior to 1.0.0, Result types may not marshal a CNIVersion. Since the
	// result version must match the config version, if the Result's version
	// is empty, inject the config version.
	if ver, ok := conf.RawPrevResult["CNIVersion"]; !ok || ver == "" {
		conf.RawPrevResult["CNIVersion"] = conf.CNIVersion
	}

	resultBytes, err := json.Marshal(conf.RawPrevResult)
	if err != nil {
		return fmt.Errorf("could not serialize prevResult: %w", err)
	}

	conf.RawPrevResult = nil
	conf.PrevResult, err = e.create(conf.CNIVersion, resultBytes)
	if err != nil {
		return fmt.Errorf("could not parse prevResult: %w", err)
	}

	return nil
}

// Create creates a CNI Result using the given JSON with the expected
// version, or an error if the creation could not be performed
func (e *engine) create(version string, bytes []byte) (t.Result, error) {
	if c := findCreator(version); c != nil {
		return c.createFn(bytes)
	}
	return nil, fmt.Errorf("unsupported CNI result version %q", version)
}

// ensureBackend validates and/or sets conf.Backend
func ensureBackend(conf *PortMapConf) error {
	backendConfig := make(map[string][]string)

	if conf.ExternalSetMarkChain != nil {
		backendConfig["iptables"] = append(backendConfig["iptables"], "externalSetMarkChain")
	}
	if conditionsBackend := detectBackendOfConditions(conf.ConditionsV4); conditionsBackend != "" {
		backendConfig[conditionsBackend] = append(backendConfig[conditionsBackend], "conditionsV4")
	}
	if conditionsBackend := detectBackendOfConditions(conf.ConditionsV6); conditionsBackend != "" {
		backendConfig[conditionsBackend] = append(backendConfig[conditionsBackend], "conditionsV6")
	}

	// If backend wasn't requested explicitly, default to iptables, unless it is not
	// available (and nftables is). FIXME: flip this default at some point.
	if conf.Backend == nil {
		if !utils.SupportsIPTables() && utils.SupportsNFTables() {
			nftablesBackend := "nftables"
			conf.Backend = &nftablesBackend
		} else {
			iptablesBackend := "iptables"
			conf.Backend = &iptablesBackend
		}
	}

	// Make sure we dont have config for the wrong backend
	var wrongBackend string
	if *conf.Backend == "iptables" {
		wrongBackend = "nftables"
	} else {
		wrongBackend = "iptables"
	}
	if len(backendConfig[wrongBackend]) > 0 {
		return fmt.Errorf("cannot use %s-specific config with %s backend", wrongBackend, *conf.Backend)
	}

	return nil
}

// detectBackendOfConditions returns "iptables" if conditions contains iptables
// conditions, "nftables" if it contains nftables conditions, and "" if it is empty.
func detectBackendOfConditions(conditions *[]string) string {
	if conditions == nil || len(*conditions) == 0 || (*conditions)[0] == "" {
		return ""
	}

	// The first character of any iptables condition would either be an hyphen
	// (e.g. "-d", "--sport", "-m") or an exclamation mark.
	// No nftables condition would start that way. (An nftables condition might
	// include a negative number, but not as the first token.)
	if (*conditions)[0][0] == '-' || (*conditions)[0][0] == '!' {
		return iptablesBackend
	}
	return nftablesBackend
}
