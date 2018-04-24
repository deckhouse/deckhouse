#!/bin/bash

function kubectl::apply_jq() {
  local namespace=$1
  local resource=$2
  local filter=$3

  local a=$(mktemp)
  local b=$(mktemp)
  local tmp=$(mktemp)

  local cleanup_filter='. | del(
    .metadata.annotations."deployment.kubernetes.io/revision",
    .metadata.annotations."kubectl.kubernetes.io/last-applied-configuration",
    .metadata.creationTimestamp,
    .metadata.generation,
    .metadata.resourceVersion,
    .metadata.selfLink,
    .metadata.uid,
    .status
  )'

  if ! kubectl -n $namespace get $resource -o json > $tmp ||
     ! jq "$cleanup_filter" $tmp > $a ||
     ! jq "$filter" $a > $b ||
     ! kubectl apply -f $b ;
  then
    echo FILTER: "$filter"

    echo "Before JQ"
    cat $a
    echo "After JQ"
    cat $b

    rm $a $b $tmp
    return 1
  fi

  diff -u $a $b || true
  rm $a $b $tmp
  return 0
}
