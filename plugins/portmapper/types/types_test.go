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
	"testing"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewConf(t *testing.T) {
	configBytes := []byte(`{
		"name": "test-network",
		"type": "portmapper",
		"cniVersion": "0.3.0",
		"portMappings": [
			{
				"hostPort": 8080,
				"containerPort": 80,
				"protocol": "tcp"
			}
		]
	}`)

	args := &skel.CmdArgs{
		StdinData: configBytes,
	}

	conf, err := NewConf(args)
	require.NoError(t, err)
	assert.Equal(t, "test-network", conf.Name)
	assert.Equal(t, "portmapper", conf.Type)
	assert.Equal(t, "0.3.0", conf.CNIVersion)
	assert.Len(t, conf.PortMappings, 1)
	assert.Equal(t, 8080, conf.PortMappings[0].HostPort)
	assert.Equal(t, 80, conf.PortMappings[0].ContainerPort)
	assert.Equal(t, "tcp", conf.PortMappings[0].Protocol)
}

func TestNewConf_InvalidJSON(t *testing.T) {
	configBytes := []byte(`{invalid json}`)

	args := &skel.CmdArgs{
		StdinData: configBytes,
	}

	_, err := NewConf(args)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse network configuration")
}

func TestPortMapping(t *testing.T) {
	pm := PortMapping{
		HostPort:      8080,
		ContainerPort: 80,
		Protocol:      "tcp",
	}

	assert.Equal(t, 8080, pm.HostPort)
	assert.Equal(t, 80, pm.ContainerPort)
	assert.Equal(t, "tcp", pm.Protocol)
}