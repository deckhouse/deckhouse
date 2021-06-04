#!/bin/bash

set -e
set -x

passwd -d ubuntu
rm -rf /etc/sudoers.d/ubuntu
