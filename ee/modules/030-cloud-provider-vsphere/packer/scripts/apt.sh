#!/bin/bash

set -e
set -x

# Prevents popup questions
export DEBIAN_FRONTEND="noninteractive"

sudo apt-get update
sudo apt-get upgrade -y
sudo apt-get dist-upgrade -y

# Need gnupg for fish
sudo apt-get install -y gnupg curl cloud-init python3-distutils
