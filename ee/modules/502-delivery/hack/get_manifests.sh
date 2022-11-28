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
#
#   argocd repo         https://github.com/argoproj/argo-cd
#                      (checkout specific tag)
#
#   rename             famous perl script
#

set -euo pipefail
set -x

VERSION="v2.4.16"

# TODO check for the presence of git and yq

# First, clone the repo
ARGOCD_REPO="${HOME}/dev/flant/argoproj/argo-cd"
ARGO_MANIFESTS="${ARGOCD_REPO}/manifests/install.yaml"
# HA:
# ARGO_MANIFESTS="${ARGOCD_REPO}/manifests/ha/install.yaml"

mkdir -p "${ARGOCD_REPO}"
git clone git@github.com:argoproj/argo-cd.git "${ARGOCD_REPO}" || true
pushd $ARGOCD_REPO &&
  git clean -df &&
  git reset --hard &&
  git fetch --all --prune &&
  git checkout $VERSION &&
  popd

# NOTE we are on master branch
mkdir -p "${HOME}/dev/flant/werf"
IMAGE_UPDATER_REPO="${HOME}/dev/flant/werf/3p-argocd-image-updater"
IMAGE_UPDATER_MANIFESTS="${IMAGE_UPDATER_REPO}/manifests/install.yaml"
git clone git@github.com:werf/3p-argocd-image-updater.git "${IMAGE_UPDATER_REPO}" || true
pushd $IMAGE_UPDATER_REPO &&
  git clean -df &&
  git reset --hard &&
  git fetch --all --prune &&
  git checkout master &&
  git pull &&
  popd

split_manifests() {
  MANIFESTS=$1

  yq eval-all 'select(.kind == "CustomResourceDefinition") | .' $MANIFESTS |
    yq e --no-doc -s '"crd-" + .spec.names.singular' -

  yq eval-all 'select(.kind != "CustomResourceDefinition") | .' $MANIFESTS |
    yq e --no-doc -s '.metadata.name + "-" + (.kind | downcase)' -

  # .yml -> .yaml
  rename -s yml yaml *.yml
}

# target dirs
CRD_ROOT=crds
ARGOCD_MANIFESTS_ROOT=templates/argocd

# clean existing manifests
mkdir -p $CRD_ROOT
mkdir -p $ARGOCD_MANIFESTS_ROOT
rm -rf ${ARGOCD_MANIFESTS_ROOT}/argocd-* ${ARGOCD_MANIFESTS_ROOT}/*/argocd-* crds/argocd-*

# pull fresh manifests
split_manifests "${ARGO_MANIFESTS}"
split_manifests "${IMAGE_UPDATER_MANIFESTS}"

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
mv argocd-repo-server*.yaml ${ARGOCD_MANIFESTS_ROOT}/repo-server

mkdir -p ${ARGOCD_MANIFESTS_ROOT}/server
mv argocd-server*.yaml ${ARGOCD_MANIFESTS_ROOT}/server

mkdir -p ${ARGOCD_MANIFESTS_ROOT}/dex
mv argocd-dex*.yaml ${ARGOCD_MANIFESTS_ROOT}/dex
# We use our own dex
rm -rf ${ARGOCD_MANIFESTS_ROOT}/dex

mkdir -p ${ARGOCD_MANIFESTS_ROOT}/redis
mv argocd-redis*.yaml ${ARGOCD_MANIFESTS_ROOT}/redis
pushd ${ARGOCD_MANIFESTS_ROOT}/redis && rename 's/^(.*)$/ha-$1/g' *-ha* && rename 's/-ha//' *-ha* || true && popd

mkdir -p ${ARGOCD_MANIFESTS_ROOT}/image-updater
mv argocd-image-updater*.yaml ${ARGOCD_MANIFESTS_ROOT}/image-updater
pushd ${ARGOCD_MANIFESTS_ROOT}/image-updater && rename 's/^(.*)$/ha-$1/g' *-ha* && rename 's/-ha//' *-ha* || true && popd

# all other manifests
mv argocd-*.yaml ${ARGOCD_MANIFESTS_ROOT}/
rm .yml # whatever
