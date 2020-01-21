#!/bin/bash

# {{ .instanceGroup.name }}
# {{ .zoneName }}

swapoff -a
sed -i '/swapfile/d' /etc/fstab
