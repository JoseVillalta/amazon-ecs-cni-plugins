// Copyright 2017 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

package commands

import (
	"encoding/json"
	"fmt"
	"net"

	"github.com/aws/amazon-ecs-cni-plugins/plugins/portmapper/engine"

	log "github.com/cihub/seelog"
	"github.com/containernetworking/cni/pkg/skel"
	cnitypes "github.com/containernetworking/cni/pkg/types"
	"github.com/containernetworking/cni/pkg/types/current"
)

// Add invokes the command to add port mappings
func Add(args *skel.CmdArgs) error {
	defer log.Flush()

	eng, err := engine.New()
	if err != nil {
		return err
	}

	return add(args, eng)
}

// Del invokes the command to remove port mappings
func Del(args *skel.CmdArgs) error {
	defer log.Flush()

	eng, err := engine.New()
	if err != nil {
		return err
	}

	return del(args, eng)
}

func add(args *skel.CmdArgs, engine engine.Engine) error {
	netConf, _, err := parseConfig(args.StdinData, args.IfName)
	if err != nil {
		return fmt.Errorf("failed to parse config: %v", err)
	}

	if netConf.PrevResult == nil {
		return fmt.Errorf("must be called as chained plugin")
	}

	if len(netConf.RuntimeConfig.PortMaps) == 0 {
		return cnitypes.PrintResult(netConf.PrevResult, netConf.CNIVersion)
	}

	netConf.ContainerID = args.ContainerID

	log.Infof("Adding port mappings for container %s", args.ContainerID)

	// Pass through the previous result
	return cnitypes.PrintResult(netConf.PrevResult, netConf.CNIVersion)
}

func del(args *skel.CmdArgs, engine engine.Engine) error {
	netConf, _, err := parseConfig(args.StdinData, args.IfName)
	if err != nil {
		return fmt.Errorf("failed to parse config: %v", err)
	}

	if len(netConf.RuntimeConfig.PortMaps) == 0 {
		return nil
	}

	netConf.ContainerID = args.ContainerID

	log.Infof("Removing port mappings for container %s", args.ContainerID)

	return nil
}

// PortMapEntry corresponds to a single entry in the port_mappings argument
type PortMapEntry struct {
	HostPort      int    `json:"hostPort"`
	ContainerPort int    `json:"containerPort"`
	Protocol      string `json:"protocol"`
	HostIP        string `json:"hostIP,omitempty"`
}

type PortMapConf struct {
	cnitypes.NetConf
	RawPrevResult *map[string]interface{} `json:"prevResult"`
	PrevResult    cnitypes.Result         `json:"-"`

	RuntimeConfig struct {
		PortMaps []PortMapEntry `json:"portMappings,omitempty"`
	} `json:"runtimeConfig,omitempty"`

	ContainerID string    `json:"-"`
	ContIPv4    net.IPNet `json:"-"`
	ContIPv6    net.IPNet `json:"-"`
}

// parseConfig parses the supplied configuration (and prevResult) from stdin.
func parseConfig(stdin []byte, ifName string) (*PortMapConf, *current.Result, error) {
	conf := PortMapConf{}

	if err := json.Unmarshal(stdin, &conf); err != nil {
		return nil, nil, fmt.Errorf("failed to parse network configuration: %v", err)
	}

	// Parse previous result.
	var result *current.Result
	if conf.RawPrevResult != nil {
		resultBytes, err := json.Marshal(conf.RawPrevResult)
		if err != nil {
			return nil, nil, fmt.Errorf("could not serialize prevResult: %v", err)
		}
		tmpResult, err := current.NewResult(resultBytes)
		if err != nil {
			return nil, nil, fmt.Errorf("could not parse prevResult: %v", err)
		}
		result, ok := tmpResult.(*current.Result)
		if !ok {
			return nil, nil, fmt.Errorf("could not convert result to current version")
		}
		conf.PrevResult = result
	}

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
		currentResult, ok := conf.PrevResult.(*current.Result)
		if !ok {
			return nil, nil, fmt.Errorf("could not convert prevResult to current version")
		}
		for _, ip := range currentResult.IPs {
			isIPv4 := ip.Address.IP.To4() != nil
			if !isIPv4 && conf.ContIPv6.IP != nil {
				continue
			} else if isIPv4 && conf.ContIPv4.IP != nil {
				continue
			}

			// Skip known non-sandbox interfaces
			if ip.Interface >= 0 &&
				ip.Interface < len(currentResult.Interfaces) &&
				(currentResult.Interfaces[ip.Interface].Name != ifName ||
					currentResult.Interfaces[ip.Interface].Sandbox == "") {
				continue
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
