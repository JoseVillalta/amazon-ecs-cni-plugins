# ECS Portmapper Plugin

## Overview

The ECS Portmapper plugin manages port mappings for containers, allowing external traffic to reach containerized applications through host port forwarding. This plugin forwards traffic from one or more ports on the host to the container and expects to be run as a chained plugin. An example configuration for invoking this plugin is listed next:

```json
{
    "type": "ecs-portmapper",
    "cniVersion": "0.3.0",
    "portMappings": [
        {
            "hostPort": 8080,
            "containerPort": 80,
            "protocol": "tcp"
        },
        {
            "hostPort": 8443,
            "containerPort": 443,
            "protocol": "tcp"
        }
    ]
}
```

## Configuration Options

You should use this plugin as part of a network configuration list. It accepts the following configuration options:

* `portMappings` (array, optional): list of port mappings to configure
  * `hostPort` (int, required): port on the host to bind to
  * `containerPort` (int, required): port in the container to forward to
  * `protocol` (string, optional): protocol to use (tcp/udp), defaults to tcp
* `snat` (boolean, default true): If true or omitted, set up the SNAT chains
* `masqAll` (boolean, default false): If false or omitted, the `snat` rule set up on loopback & hairpin traffic, else will `snat` all source traffic
* `markMasqBit` (int, 0-31, default 13): The mark bit to use for masquerading (see section SNAT). Cannot be set when `externalSetMarkChain` is used. (Only used by the "iptables" backend.)
* `externalSetMarkChain` (string, default nil): If you already have a Masquerade mark chain (e.g. Kubernetes), specify it here. This will use that instead of creating a separate chain. When this is set, `markMasqBit` must be unspecified. (Only used by the "iptables" backend.)
* `conditionsV4`, `conditionsV6` (array of strings): A list of arbitrary `iptables` or `nft` matches to add to the per-container rule. This may be useful if you wish to exclude specific IPs from port-mapping
* `backend` (string): The backend ("iptables" or "nftables") to use for rules. Defaults to "iptables", unless iptables is unavailable, or nftables-specific configuration is provided

The plugin expects to receive the actual list of port mappings via the `portMappings` [capability argument](https://github.com/containernetworking/cni/blob/master/CONVENTIONS.md).

## Usage Examples

### Sample Configuration List

A sample standalone config list for Kubernetes (with the file extension .conflist) might look like:

```json
{
    "cniVersion": "0.3.1",
    "name": "mynet",
    "plugins": [
        {
            "type": "ptp",
            "ipMasq": true,
            "ipam": {
                "type": "host-local",
                "subnet": "172.16.30.0/24",
                "routes": [
                    {
                        "dst": "0.0.0.0/0"
                    }
                ]
            }
        },
        {
            "type": "portmap",
            "capabilities": {"portMappings": true}
        }
    ]
}
```

### Configuration with All Options (iptables)

```json
{
    "type": "portmap",
    "backend": "iptables",
    "capabilities": {"portMappings": true},
    "snat": true,
    "markMasqBit": 13,
    "externalSetMarkChain": "CNI-HOSTPORT-SETMARK",
    "conditionsV4": ["!", "-d", "192.0.2.0/24"],
    "conditionsV6": ["!", "-d", "fc00::/7"]
}
```


## Running the Plugin

### Using CNI Environment Variables

Please ensure that the environment variables needed for running any CNI plugins are appropriately configured:
* `CNI_COMMAND`: Command to execute eg: ADD.
* `CNI_PATH`: Plugin binary path eg: `pwd`/bin.
* `CNI_IFNAME`: Interface name inside the container

#### Add:
```
export CNI_COMMAND=ADD && cat mynet.conf | ../bin/ecs-portmapper
```

#### Del:
```
export CNI_COMMAND=DEL && cat mynet.conf | ../bin/ecs-portmapper
```

`mynet.conf` is the configuration file for the plugin, it's the same as described in the overview above.

### Using cnitool (Recommended for Testing)

For easier testing and debugging, you can use `cnitool` to run the plugin:

#### Prerequisites
```bash
# Install cnitool
go install github.com/containernetworking/cni/cnitool@latest

# Set environment variables
export CNI_PATH=/opt/cni/bin/
export NETCONFPATH=/opt/cni/netconfs
```

#### Set Port Mappings
```bash
export CAP_ARGS='{
    "portMappings": [
        {
            "hostPort":      9090,
            "containerPort": 80,
            "protocol":      "tcp"
        }
    ]
}'
```

#### Execute Plugin
```bash
# Add port mapping
cnitool add ecs-portmapper /run/netns/cake3

# Remove port mapping
cnitool del ecs-portmapper /run/netns/cake3

# Verify iptables rules
iptables -t nat -L
```

**Note**: The plugin requires a `prevResult` from a previous CNI plugin execution (like ecs-bridge).

## Rule Structure

#### DNAT
The DNAT rule rewrites the destination port and address of new connections. There is a top-level chain, `CNI-HOSTPORT-DNAT` which is always created and never deleted. Each plugin execution creates an additional chain for ease of cleanup.

#### SNAT (Masquerade)
Some packets also need to have the source address rewritten:
* connections from localhost
* Hairpin traffic back to the container
* Plugins whose traffic does not go through the default net namespace e.g., ipvlan, macvlan, etc. (need `masqAll` option)

## Known Issues

### Efficiency
Each new connection to the host will have to traverse every rule in the chain, so large numbers of port forwards may have a performance impact. (This won't affect established connections, just the first packet.)

### Localhost hostports
Because MASQUERADE happens in POSTROUTING, packets with source ip 127.0.0.1 need to first pass a routing boundary before being masqueraded. The plugin needs to enable the sysctl `net.ipv4.conf.IFNAME.route_localnet`, where IFNAME is the name of the host-side interface that routes traffic to the container.

There is no equivalent to `route_localnet` for ipv6, so connections to ::1 will not be portmapped for ipv6. If you need port forwarding from localhost, your container must have an ipv4 address.

## Testing

### End-to-end Tests

The end-to-end test suite for this package makes the following assumptions:
1. The `ecs-portmapper` plugin executable has been built
2. The `CNI_PATH` environment variable points to the location of these plugins
3. The test is being executed with `root` user privileges

Please refer the [Makefile](../../Makefile) for an example of the command line required to run end-to-end tests (under the `e2e-test` target).