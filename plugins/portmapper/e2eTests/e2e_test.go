//go:build e2e
// +build e2e

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

package e2eTests

import (
	"fmt"
	"io/ioutil"
	"os"
	"runtime"
	"strconv"
	"testing"

	"github.com/containernetworking/cni/pkg/invoke"
	"github.com/containernetworking/cni/pkg/ns"
	"github.com/stretchr/testify/require"
)

const (
	ifName      = "eth0"
	containerID = "test-container"
	netConf     = `{
		"cniVersion": "0.3.0",
		"name": "portmap-test",
		"type": "portmap",
		"runtimeConfig": {
			"portMappings": [
				{
					"hostPort": 8080,
					"containerPort": 80,
					"protocol": "tcp"
				}
			]
		},
		"prevResult": {
			"interfaces": [
				{"name": "host"},
				{"name": "container", "sandbox": "netns"}
			],
			"ips": [
				{
					"version": "4",
					"address": "10.0.0.2/24",
					"gateway": "10.0.0.1",
					"interface": 1
				}
			]
		}
	}`
)

func init() {
	runtime.LockOSThread()
}

func TestPortmapperAddDel(t *testing.T) {
	// Ensure that the portmapper plugin exists
	pluginPath, err := invoke.FindInPath("ecs-portmapper", []string{os.Getenv("CNI_PATH")})
	require.NoError(t, err, "Unable to find portmap plugin in path")

	// Create a directory for storing test logs
	testLogDir, err := ioutil.TempDir("", "portmap-e2e-test-")
	require.NoError(t, err, "Unable to create directory for storing test logs")

	// Configure the env var to use the test logs directory
	os.Setenv("ECS_CNI_LOG_FILE", fmt.Sprintf("%s/portmap.log", testLogDir))
	t.Logf("Using %s for test logs", testLogDir)
	defer os.Unsetenv("ECS_CNI_LOG_FILE")

	// Handle deletion of test logs at the end of the test execution
	ok, err := strconv.ParseBool(getEnvOrDefault("ECS_PRESERVE_E2E_TEST_LOGS", "false"))
	require.NoError(t, err, "Unable to parse ECS_PRESERVE_E2E_TEST_LOGS env var")
	defer func(preserve bool) {
		if !t.Failed() && !preserve {
			os.RemoveAll(testLogDir)
		}
	}(ok)

	// Create a network namespace to mimic the container's network namespace
	targetNS, err := ns.NewNS()
	require.NoError(t, err, "Unable to create the network namespace for the container")
	defer targetNS.Close()

	// Construct args to invoke the CNI plugin with
	execInvokeArgs := &invoke.Args{
		ContainerID: containerID,
		NetNS:       targetNS.Path(),
		IfName:      ifName,
		Path:        os.Getenv("CNI_PATH"),
	}

	// Execute the "ADD" command for the plugin
	execInvokeArgs.Command = "ADD"
	result, err := invoke.ExecPluginWithResult(
		pluginPath,
		[]byte(netConf),
		execInvokeArgs)
	require.NoError(t, err, "Unable to execute ADD command for portmap plugin")
	require.NotNil(t, result, "ADD command should return a result")

	// Execute the "DEL" command for the plugin
	execInvokeArgs.Command = "DEL"
	err = invoke.ExecPluginWithoutResult(
		pluginPath,
		[]byte(netConf),
		execInvokeArgs)
	require.NoError(t, err, "Unable to execute DEL command for portmap plugin")
}

// getEnvOrDefault gets the value of an env var. It returns the fallback value
// if the env var is not set
func getEnvOrDefault(name string, fallback string) string {
	val := os.Getenv(name)
	if val == "" {
		return fallback
	}
	return val
}
