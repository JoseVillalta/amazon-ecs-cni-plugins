# Amazon ECS CNI Plugins

[![Build Status](https://travis-ci.org/aws/amazon-ecs-cni-plugins.svg?branch=master)](https://travis-ci.org/aws/amazon-ecs-cni-plugins)
## Description

Amazon ECS CNI Plugins is a collection of Container Network Interface([CNI](https://github.com/containernetworking/cni)) Plugins used by the [Amazon ECS Agent](https://github.com/aws/amazon-ecs-agent) to configure network namespace of containers with Elastic Network Interfaces ([ENIs](http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/using-eni.html))

For more information about Amazon ECS, see the [Amazon ECS Developer Guide](http://docs.aws.amazon.com/AmazonECS/latest/developerguide/Welcome.html).

For more information about Plugins in this project, see the individual READMEs.

## Plugins
* [ECS ENI Plugin](plugins/eni/README.md): configures the network namespace of the container with an ENI device
* [ECS Bridge Plugin](plugins/ecs-bridge/README.md): configures the network namespace of the container to be able to communicate with the credentials endpoint of the ECS Agent
* [ECS IPAM Plugin](plugins/ipam/README.md): allocates an IP address and constructs Gateway and Route structures used by the ECS Bridge plugin to configure the bridge and veth pair in the container network namespace
* [ECS Port Mapper Plugin](plugins/portmapper/README.md): configures port forwarding from host to container using iptables rules

## Running Plugins via CLI

You can test the ECS CNI plugins manually using the `cnitool` command. Here's how to set up and run the plugins:

### Prerequisites

```bash
sudo su
cd /home/ssm-user

# Install golang
export PATH=$PATH:$HOME/go/bin

# Install cnitool
go install github.com/containernetworking/cni/cnitool@latest
cnitool # Should output usage instructions
```

### Setup Test Environment

```bash
# Create a container with none network mode
docker run --network none -d amazonlinux sleep infinity

# Create named network namespace
cid=`docker ps | grep amazonlinux | awk '{print $1}'`
pid=`docker inspect -f '{{.State.Pid}}' $cid`
touch /var/run/netns/cake3
mount -o bind /proc/${pid}/ns/net /var/run/netns/cake3

# Verify namespace is visible
ip netns list
```

### Install ECS CNI Plugins

```bash
# Set environment variables
export NETCONFPATH=/opt/cni/netconfs
export CNI_PATH=/opt/cni/bin/
export IPAM_DB_PATH=/var/lib/ecs/data/eni-ipam.db
export CNI_IFNAME=ecs-eth1

# Build and install plugins
cd /home/ssm-user
export GOPATH=`pwd`
mkdir -p "$GOPATH/src/github.com/aws"
cd "$GOPATH/src/github.com/aws"
git clone https://github.com/aws/amazon-ecs-cni-plugins.git
cd amazon-ecs-cni-plugins/
export GO111MODULE=off
make plugins
cp bin/plugins/* $CNI_PATH
```

### Configure and Run ECS Bridge Plugin

```bash
# Create bridge configuration
cat > $NETCONFPATH/ecs-bridge.conflist <<EOF
{
  "cniVersion": "0.3.0",
  "name": "ecs-bridge",
  "plugins": [
     {
        "type": "ecs-bridge",
        "bridge": "ecs-bridge",
        "ipam":{
            "type":"ecs-ipam",
            "id":"test",
            "cniVersion":"0.3.0",
            "ipv4-subnet":"169.254.172.0/22",
            "ipv4-routes":[
                {
                "dst":"0.0.0.0/0"
                },
                {
                "dst":"169.254.172.0/22"
                }
            ]
        }
    }
  ]  
}
EOF

# Execute bridge plugin
cnitool add ecs-bridge /run/netns/cake3

# Verify bridge and interface creation
ip address list
ip -n cake3 address list
```

### Configure and Run Port Mapper Plugin

```bash
# Create port mapper configuration (replace prevResult with output from bridge plugin)
cat > $NETCONFPATH/portmapping.conflist <<EOF
{
  "cniVersion": "0.3.0",
  "name": "ecs-portmapper",
  "plugins": [ 
    {
      "type": "ecs-portmapper",
      "capabilities": {"portMappings": true},
      "snat": true,
      "prevResult": {
        "cniVersion": "0.3.0",
        "interfaces": [
          {"name": "ecs-bridge", "mac": "52:f9:59:11:33:06"},
          {"name": "vethd47f388e", "mac": "5a:d7:5a:0a:81:38"},
          {"name": "ecs-eth1", "mac": "0a:58:a9:fe:ac:02", "sandbox": "/run/netns/cake3"}
        ],
        "ips": [
          {
            "version": "4",
            "interface": 2,
            "address": "169.254.172.3/22",
            "gateway": "169.254.172.1"
          }
        ],
        "routes": [
          {"dst": "0.0.0.0/0"},
          {"dst": "169.254.172.0/22"},
          {"dst": "169.254.172.1/32"}
        ],
        "dns": {}
      }
    }
  ]
}
EOF

# Set port mapping configuration
export CAP_ARGS='{
    "portMappings": [
        {
            "hostPort":      9090,
            "containerPort": 80,
            "protocol":      "tcp"
        }
    ]
}'

# Execute port mapper plugin
cnitool add ecs-portmapper /run/netns/cake3

# Verify iptables rules were created
iptables -t nat -L
```

**Note**: Replace the `prevResult` in the port mapper configuration with the actual output from the bridge plugin execution, ensuring the interface includes both `mac` and `sandbox` fields.

## Security disclosures
If you think you’ve found a potential security issue, please do not post it in the Issues.  Instead, please follow the instructions [here](https://aws.amazon.com/security/vulnerability-reporting/) or [email AWS security directly](mailto:aws-security@amazon.com).
