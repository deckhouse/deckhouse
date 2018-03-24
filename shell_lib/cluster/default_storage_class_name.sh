#!/bin/bash -e

function cluster::default_storage_class_name() {
  kubectl get storageclass -o json | jq '.items[] | select (.metadata.annotations."storageclass.beta.kubernetes.io/is-default-class" == "true") | .metadata.name' -r | head -n 1
}
