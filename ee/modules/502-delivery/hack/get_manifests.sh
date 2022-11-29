#!/bin/bash
#
# Copyright 2022 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
#
#
#
###########################################################################
# CALL THIS SCRIPT FROM THE MODULE DIRECTORY
#  ./hack/get_manifests.sh
#
# Dependencies:
#   yq                  https://github.com/mikefarah/yq
#   curl
#   rename
#   perl

set -euo pipefail
# set -x

# TODO check for the presence of yq

ARGOCD_VERSION="2.5.3"

VENDOR_ROOT=./hack/vendor
pushd $VENDOR_ROOT
# the path in the arhived repo
ARGO_MANIFESTS="argo-cd-${ARGOCD_VERSION}/manifests/install.yaml"
# ARGO_MANIFESTS="argo-cd-${ARGOCD_VERSION}/manifests/ha/install.yaml" # HA
curl -LfsS "https://github.com/argoproj/argo-cd/archive/refs/tags/v${ARGOCD_VERSION}.tar.gz" | tar -xzvf - "${ARGO_MANIFESTS}"

# NOTE we are on master branch
IMAGE_UPDATER_MANIFESTS="3p-argocd-image-updater-master/manifests/install.yaml"
curl -LfsS https://github.com/werf/3p-argocd-image-updater/archive/refs/heads/master.tar.gz | tar -xzvf - "${IMAGE_UPDATER_MANIFESTS}"
popd

# target dirs
CRD_ROOT=crds
ARGOCD_MANIFESTS_ROOT=templates/argocd

# clean existing manifests
mkdir -p $CRD_ROOT
mkdir -p $ARGOCD_MANIFESTS_ROOT
rm -rf ${ARGOCD_MANIFESTS_ROOT}/argocd-* ${ARGOCD_MANIFESTS_ROOT}/*/argocd-* crds/argocd-*

# extract manifests
split_manifests() {
  MANIFESTS=$1

  yq eval-all 'select(.kind == "CustomResourceDefinition") | .' $MANIFESTS |
    yq e --no-doc -s '"crd-" + .spec.names.singular' -

  yq eval-all 'select(.kind != "CustomResourceDefinition") | .' $MANIFESTS |
    yq e --no-doc -s '.metadata.name + "-" + (.kind | downcase)' -

  # .yml -> .yaml
  rename -s yml yaml *.yml
}
split_manifests "${VENDOR_ROOT}/${ARGO_MANIFESTS}"
split_manifests "${VENDOR_ROOT}/${IMAGE_UPDATER_MANIFESTS}"

# remove network policies as we don't need them in Deckhouse
rm *-networkpolicy.yaml

# Move CRDs
mv crd-*.yaml ${CRD_ROOT} &&
  pushd ${CRD_ROOT} &&
  rename 's/^crd-(.*)/argocd-$1/g' * &&
  popd

# Add module namespace
xargs -n 1 -- yq -i '.metadata.namespace="d8-{{ .Chart.Name }}"' <<<$(egrep --files-without-match '^kind: Cluster' argocd-*.yaml)

# Fix default "argocd" namespace where it is specified (ClusterRoleBindings).
# https://argo-cd.readthedocs.io/en/stable/getting_started/#1-install-argo-cd
#   - `sed -i` does not work on both MacOS and Linux consistently, so using Perl.
#   - not using `yq` to avoid coupling with manifests paths, we don't know where we can meet the
#     namespace.
xargs -n 1 -- perl -pi -e 's/namespace: argocd/namespace: d8-{{ .Chart.Name }}/' <<<$(egrep --files-with-matches '^\s+namespace: argocd$' argocd-*.yaml)

# Sort manifests
mkdir -p ${ARGOCD_MANIFESTS_ROOT}/application-controller
mv argocd-application-controller*.yaml ${ARGOCD_MANIFESTS_ROOT}/application-controller
mv argocd-metrics-*.yaml ${ARGOCD_MANIFESTS_ROOT}/application-controller

mkdir -p ${ARGOCD_MANIFESTS_ROOT}/applicationset-controller
mv argocd-applicationset-controller*.yaml ${ARGOCD_MANIFESTS_ROOT}/applicationset-controller

mkdir -p ${ARGOCD_MANIFESTS_ROOT}/notifications-controller
mv argocd-notifications*.yaml ${ARGOCD_MANIFESTS_ROOT}/notifications-controller

mkdir -p ${ARGOCD_MANIFESTS_ROOT}/repo-server
# No need to keep this SA, see https://github.com/argoproj/argo-cd/pull/9301
rm argocd-repo-server-serviceaccount.yaml
mv argocd-repo-server*.yaml ${ARGOCD_MANIFESTS_ROOT}/repo-server

mkdir -p ${ARGOCD_MANIFESTS_ROOT}/server
mv argocd-server*.yaml ${ARGOCD_MANIFESTS_ROOT}/server

mkdir -p ${ARGOCD_MANIFESTS_ROOT}/redis
mv argocd-redis*.yaml ${ARGOCD_MANIFESTS_ROOT}/redis
pushd ${ARGOCD_MANIFESTS_ROOT}/redis && rename 's/^(.*)$/ha-$1/g' *-ha* && rename 's/-ha//' *-ha* || true && popd

mkdir -p ${ARGOCD_MANIFESTS_ROOT}/image-updater
mv argocd-image-updater*.yaml ${ARGOCD_MANIFESTS_ROOT}/image-updater
pushd ${ARGOCD_MANIFESTS_ROOT}/image-updater && rename 's/^(.*)$/ha-$1/g' *-ha* && rename 's/-ha//' *-ha* || true && popd

# We use our own dex
rm -rf mv argocd-dex*.yaml

# all other manifests
mv argocd-*.yaml ${ARGOCD_MANIFESTS_ROOT}/
rm .yml # whatever
