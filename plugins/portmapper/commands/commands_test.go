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
	"fmt"
	"testing"

	"github.com/containernetworking/cni/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseConfig(t *testing.T) {
	for _, ver := range []string{"0.3.0", "0.3.1", "0.4.0", "1.0.0"} {
		t.Run(fmt.Sprintf("version_%s", ver), func(t *testing.T) {
			t.Run("correctly_parses_ADD_config", func(t *testing.T) {
				configBytes := []byte(fmt.Sprintf(`{
					"name": "test",
					"type": "portmap",
					"cniVersion": "%s",
					"runtimeConfig": {
						"portMappings": [
							{ "hostPort": 8080, "containerPort": 80, "protocol": "tcp"},
							{ "hostPort": 8081, "containerPort": 81, "protocol": "udp"}
						]
					},
					"prevResult": {
						"interfaces": [
							{"name": "host"},
							{"name": "container", "sandbox":"netns"}
						],
						"ips": [
							{
								"version": "4",
								"address": "10.0.0.1/24",
								"gateway": "10.0.0.1",
								"interface": 0
							},
							{
								"version": "6",
								"address": "2001:db8:1::2/64",
								"gateway": "2001:db8:1::1",
								"interface": 1
							},
							{
								"version": "4",
								"address": "10.0.0.2/24",
								"gateway": "10.0.0.1",
								"interface": 1
							}
						]
					}
				}`, ver))

				c, _, err := parseConfig(configBytes, "container")
				require.NoError(t, err)
				assert.Equal(t, ver, c.CNIVersion)
				assert.Equal(t, "test", c.Name)
				assert.Len(t, c.RuntimeConfig.PortMaps, 2)
				assert.Equal(t, 8080, c.RuntimeConfig.PortMaps[0].HostPort)
				assert.Equal(t, 80, c.RuntimeConfig.PortMaps[0].ContainerPort)
				assert.Equal(t, "tcp", c.RuntimeConfig.PortMaps[0].Protocol)

				n, err := types.ParseCIDR("10.0.0.2/24")
				require.NoError(t, err)
				assert.Equal(t, *n, c.ContIPv4)

				n, err = types.ParseCIDR("2001:db8:1::2/64")
				require.NoError(t, err)
				assert.Equal(t, *n, c.ContIPv6)
			})

			t.Run("correctly_parses_DEL_config", func(t *testing.T) {
				configBytes := []byte(fmt.Sprintf(`{
					"name": "test",
					"type": "portmap",
					"cniVersion": "%s"
				}`, ver))

				c, _, err := parseConfig(configBytes, "container")
				require.NoError(t, err)
				assert.Equal(t, ver, c.CNIVersion)
				assert.Equal(t, "test", c.Name)
				assert.Len(t, c.RuntimeConfig.PortMaps, 0)
			})

			t.Run("fails_with_invalid_mappings", func(t *testing.T) {
				configBytes := []byte(fmt.Sprintf(`{
					"name": "test",
					"type": "portmap",
					"cniVersion": "%s",
					"runtimeConfig": {
						"portMappings": [
							{ "hostPort": 0, "containerPort": 80, "protocol": "tcp"}
						]
					}
				}`, ver))

				_, _, err := parseConfig(configBytes, "container")
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "Invalid host port number: 0")
			})

			t.Run("fails_with_invalid_container_port", func(t *testing.T) {
				configBytes := []byte(fmt.Sprintf(`{
					"name": "test",
					"type": "portmap",
					"cniVersion": "%s",
					"runtimeConfig": {
						"portMappings": [
							{ "hostPort": 8080, "containerPort": 0, "protocol": "tcp"}
						]
					}
				}`, ver))

				_, _, err := parseConfig(configBytes, "container")
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "Invalid container port number: 0")
			})

			t.Run("does_not_fail_on_missing_prevResult_interface_index", func(t *testing.T) {
				configBytes := []byte(fmt.Sprintf(`{
					"name": "test",
					"type": "portmap",
					"cniVersion": "%s",
					"runtimeConfig": {
						"portMappings": [
							{ "hostPort": 8080, "containerPort": 80, "protocol": "tcp"}
						]
					},
					"prevResult": {
						"interfaces": [
							{"name": "host"}
						],
						"ips": [
							{
								"version": "4",
								"address": "10.0.0.1/24",
								"gateway": "10.0.0.1"
							}
						]
					}
				}`, ver))

				_, _, err := parseConfig(configBytes, "container")
				assert.NoError(t, err)
			})
		})
	}
}

func TestPortMapEntry(t *testing.T) {
	entry := PortMapEntry{
		HostPort:      8080,
		ContainerPort: 80,
		Protocol:      "tcp",
		HostIP:        "192.168.1.1",
	}

	assert.Equal(t, 8080, entry.HostPort)
	assert.Equal(t, 80, entry.ContainerPort)
	assert.Equal(t, "tcp", entry.Protocol)
	assert.Equal(t, "192.168.1.1", entry.HostIP)
}