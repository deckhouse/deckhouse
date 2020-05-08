#!/bin/bash

until yum install epel-release -y; do
  echo "Error installing epel-release"
  sleep 10
done
until yum install jq nc curl -y; do
  echo "Error installing packages"
  sleep 10
done

touch /var/lib/bashible/first_run
