#!/bin/bash

if [ ! -f /var/lib/bashible/hosname-set-as-in-aws ]; then
  wget -O /usr/local/bin/ec2_describe_tags https://github.com/flant/go-ec2-describe-tags/releases/download/v0.0.1-flant.1/ec2_describe_tags
  chmod +x /usr/local/bin/ec2_describe_tags
  instance_name=$(/usr/local/bin/ec2_describe_tags -query_meta | grep -Po '(?<=Name=).+')
  hostnamectl set-hostname "$instance_name"
  rm /usr/local/bin/ec2_describe_tags
  touch /var/lib/bashible/hosname-set-as-in-aws
fi
