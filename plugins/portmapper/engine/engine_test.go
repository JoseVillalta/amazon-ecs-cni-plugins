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

package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	eng, err := New()
	require.NoError(t, err)
	assert.NotNil(t, eng)
}

func TestEngine_SetupPortMapping(t *testing.T) {
	eng, err := New()
	require.NoError(t, err)

	// Currently returns not implemented error
	err = eng.SetupPortMapping()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "SetupPortMapping not implemented")
}

func TestEngine_TeardownPortMapping(t *testing.T) {
	eng, err := New()
	require.NoError(t, err)

	// Currently returns not implemented error
	err = eng.TeardownPortMapping()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "TeardownPortMapping not implemented")
}