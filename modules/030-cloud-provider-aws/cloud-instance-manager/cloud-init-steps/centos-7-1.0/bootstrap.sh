#!/bin/bash

# {{ .instanceGroup.name }}
# {{ .zoneName }}

hostnamectl set-hostname "$(curl -s http://169.254.169.254/latest/meta-data/local-hostname)"
