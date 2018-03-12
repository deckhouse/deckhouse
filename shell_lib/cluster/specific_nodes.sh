#!/bin/bash -e

# "Специфичными" мы считаем узлы, на которых есть одни из следующих taint'ов:
#  * dedicated=<XXX>
#  * node-role/*
#  * node-role.kubernetes.io/*

function cluster::specific_nodes() {
  kubectl get nodes -o json | jq '.items[]
    | select(.spec.taints)
    | select(
      .spec.taints[] | select(
        .key == "dedicated" or (.key | startswith("node-role/")) or (.key | startswith("node-role.kubernetes.io/"))
      )
    )
    | .metadata.name' -r
}

function cluster::nonspecific_nodes() {
  kubectl get nodes -o json | jq '.items[]
    | select(
      .spec.taints == null or (.spec.taints[] | select(
        .key != "dedicated" and (.key | startswith("node-role/") | not) and (.key | startswith("node-role.kubernetes.io/") | not)
      ))
    )
    | .metadata.name' -r
}
