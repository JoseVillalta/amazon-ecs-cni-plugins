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
	"testing"

	portmapperTypes "github.com/aws/amazon-ecs-cni-plugins/plugins/portmapper/types"
	"github.com/stretchr/testify/assert"
)

func TestParseConfig(t *testing.T) {
	t.Skip("Skipping parseConfig tests - engine.New() has runtime dependencies")
}

func TestPortMapEntry(t *testing.T) {
	entry := portmapperTypes.PortMapEntry{
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