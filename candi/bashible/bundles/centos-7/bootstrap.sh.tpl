#!/bin/bash

. /etc/os-release

epel_package="epel-release"
if [[ "${ID}" == "rhel" ]]; then
  epel_package="https://dl.fedoraproject.org/pub/epel/epel-release-latest-7.noarch.rpm"
fi

until yum install "$epel_package" -y; do
  echo "Error installing $epel_package"
  sleep 10
done
until yum install jq nc curl wget -y; do
  echo "Error installing packages"
  sleep 10
done

mkdir -p /var/lib/bashible/
