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

	"github.com/aws/amazon-ecs-cni-plugins/plugins/portmapper/engine"

	log "github.com/cihub/seelog"
	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
)

// Add invokes the command to add port mappings
func Add(args *skel.CmdArgs) error {
	defer log.Flush()

	eng, err := engine.New(args)
	if err != nil {
		return err
	}

	return add(args, eng)
}

// Del invokes the command to remove port mappings
func Del(args *skel.CmdArgs) error {
	defer log.Flush()

	eng, err := engine.New(args)
	if err != nil {
		return err
	}

	return del(args, eng)
}

func add(args *skel.CmdArgs, eng engine.Engine) error {
	config := eng.GetConfig()
	
	if config.PrevResult == nil {
		return fmt.Errorf("must be called as chained plugin")
	}

	if len(config.RuntimeConfig.PortMaps) == 0 {
		return types.PrintResult(config.PrevResult, config.CNIVersion)
	}

	if err := eng.ForwardPorts(); err != nil {
		return err
	}

	return types.PrintResult(config.PrevResult, config.CNIVersion)
}

func del(args *skel.CmdArgs, engine engine.Engine) error {
	/*
		netConf, _, err := parseConfig(args.StdinData, args.IfName)
		if err != nil {
			return fmt.Errorf("failed to parse config: %v", err)
		}

		if len(netConf.RuntimeConfig.PortMaps) == 0 {
			return nil
		}

		netConf.ContainerID = args.ContainerID

		log.Infof("Removing port mappings for container %s", args.ContainerID)
	*/
	return nil
}


