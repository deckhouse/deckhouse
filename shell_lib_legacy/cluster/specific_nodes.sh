#!/bin/bash -e

# "Специфичными" мы считаем узлы, на которых есть одни из следующих taint'ов:
#  * dedicated=<XXX>
#  * node-role.kubernetes.io/*

function cluster::specific_nodes() {
  kubectl get nodes -o json | jq '.items[]
    | select(.spec.taints)
    | select(
      .spec.taints[] | select(.key == "dedicated.flant.com" or .key == "node-role.kubernetes.io/master")
      )
    | .metadata.name' -r
}

function cluster::nonspecific_nodes() {
  kubectl get nodes -o json | jq '.items[]
    | select(
      .spec.taints == null or (.spec.taints[] | select(.key != "dedicated.flant.com" and .key != "node-role.kubernetes.io/master"))
      )
    | .metadata.name' -r
}
