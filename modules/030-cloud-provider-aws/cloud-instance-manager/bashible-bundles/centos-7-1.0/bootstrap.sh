#!/bin/bash

set -Eeuxo pipefail

# {{ .instanceGroup.name }}
# {{ .zoneName }}

wget https://github.com/flant/go-ec2-describe-tags/releases/download/v0.0.1-flant.1/ec2_describe_tags
chmod +x ec2_describe_tags
instance_name=$(./ec2_describe_tags -query_meta | grep -Po '(?<=Name=).+')
hostnamectl set-hostname "$instance_name"
