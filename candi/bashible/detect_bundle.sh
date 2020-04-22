#!/bin/bash

if lsb_release -a 2>/dev/null | grep -iq 'ubuntu.*18\.04' ; then
  echo ubuntu-18.04
elif cat /etc/redhat-release 2>/dev/null | grep -iq 'centos.* 7\.' ; then
  echo centos-7
else
  >&2 echo "ERROR: Can't determine OS!"
  exit 1
fi
