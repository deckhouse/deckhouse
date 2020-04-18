#!/bin/bash

if lsb_release -a | grep -iq 'ubuntu.*18\.04' ; then
  echo ubuntu-18.04
elif cat /etc/redhat-release | grep -iq 'centos.* 7\.'; then
  echo centos-7
else
  >&2 echo "ERROR: Can't determine OS!"
  exit 1
fi
