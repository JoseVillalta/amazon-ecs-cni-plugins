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

package types

import (
	"encoding/json"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/pkg/errors"
)

// PortMapping represents a port mapping configuration
type PortMapping struct {
	HostPort      int    `json:"hostPort"`
	ContainerPort int    `json:"containerPort"`
	Protocol      string `json:"protocol"`
}

// NetConf represents the network configuration for the portmapper plugin
type NetConf struct {
	CNIVersion   string        `json:"cniVersion,omitempty"`
	Name         string        `json:"name,omitempty"`
	Type         string        `json:"type,omitempty"`
	PortMappings []PortMapping `json:"portMappings,omitempty"`
}

// NewConf creates a new NetConf object by parsing the configuration from stdin
func NewConf(args *skel.CmdArgs) (*NetConf, error) {
	conf := &NetConf{}
	if err := json.Unmarshal(args.StdinData, conf); err != nil {
		return nil, errors.Wrap(err, "NewConf types: failed to parse network configuration")
	}

	return conf, nil
}