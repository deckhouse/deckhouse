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
set -x

# TODO check for the presence of yq

ARGOCD_VERSION="2.5.3"

VENDOR_ROOT=./hack/vendor
mkdir -p $VENDOR_ROOT
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
TEMPLATES_ROOT=templates
ARGOCD_MANIFESTS_ROOT=$TEMPLATES_ROOT/argocd

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

# Form labels where missing
yq eval-all 'select(.metadata.labels == null) | .metadata.labels.dummy = "true"' argocd-*.yaml

# remove network policies as we don't need them in Deckhouse
# TODO why?
rm *-networkpolicy.yaml

# Move CRDs
mv crd-*.yaml ${CRD_ROOT} &&
  pushd ${CRD_ROOT} &&
  rename 's/^crd-(.*)/argocd-$1/g' * &&
  popd

# Add module namespace
xargs -n 1 -- yq -i '.metadata.namespace="d8-{{ .Chart.Name }}"' <<<$(egrep --files-without-match '^kind: Cluster' argocd-*.yaml)

# Fix default "argocd" namespace where it is specified (ClusterRoleBindings referring Roles).
# https://argo-cd.readthedocs.io/en/stable/getting_started/#1-install-argo-cd
#   - `sed -i` does not work on both MacOS and Linux consistently, so using Perl.
#   - not using `yq` to avoid coupling with manifests paths, we don't know where we can meet the
#     namespace.
xargs -n 1 -- perl -pi -e 's/namespace: argocd/namespace: d8-{{ .Chart.Name }}/' <<<$(egrep --files-with-matches '^\s+namespace: argocd$' argocd-*.yaml)

# Fix manifests
# xargs -n 1 -- perl -pi -e 's/image: .+updater.+/image: {{ include "helm_lib_module_image" (list . "argocdImageUpdater") }}/' <<<$(egrep --files-with-matches '^\s+image:\s+[^{{]+updater.+$' argocd-*.yaml)
# xargs -n 1 -- perl -pi -e 's/image: .+argocd.+/image: {{ include "helm_lib_module_image" (list . "argocd") }}/'              <<<$(egrep --files-with-matches '^\s+image:\s+[^{{]+argocd.+$' argocd-*.yaml)
# xargs -n 1 -- perl -pi -e 's/image: .+redis.+/image: {{ include "helm_lib_module_image" (list . "redis") }}/'                <<<$(egrep --files-with-matches '^\s+image:\s+[^{{]+redis.+$' argocd-*.yaml)

# Sort manifests
COMPONENT_ROOT=${ARGOCD_MANIFESTS_ROOT}/application-controller
mkdir -p $COMPONENT_ROOT
mv argocd-application-controller*.yaml $COMPONENT_ROOT
mv argocd-metrics-*.yaml $COMPONENT_ROOT
yq e $COMPONENT_ROOT/*role* $COMPONENT_ROOT/*account* >$COMPONENT_ROOT/rbac-for-us.yaml
rm $COMPONENT_ROOT/*role* $COMPONENT_ROOT/*account*

COMPONENT_ROOT=${ARGOCD_MANIFESTS_ROOT}/applicationset-controller
mkdir -p $COMPONENT_ROOT
mv argocd-applicationset-controller*.yaml $COMPONENT_ROOT
yq e $COMPONENT_ROOT/*role* $COMPONENT_ROOT/*account* >$COMPONENT_ROOT/rbac-for-us.yaml
rm $COMPONENT_ROOT/*role* $COMPONENT_ROOT/*account*

COMPONENT_ROOT=${ARGOCD_MANIFESTS_ROOT}/notifications-controller
mkdir -p $COMPONENT_ROOT
mv argocd-notifications*.yaml $COMPONENT_ROOT
yq e $COMPONENT_ROOT/*role* $COMPONENT_ROOT/*account* >$COMPONENT_ROOT/rbac-for-us.yaml
rm $COMPONENT_ROOT/*role* $COMPONENT_ROOT/*account*

COMPONENT_ROOT=${ARGOCD_MANIFESTS_ROOT}/repo-server
mkdir -p $COMPONENT_ROOT
mv argocd-repo-server*.yaml $COMPONENT_ROOT
yq e $COMPONENT_ROOT/*account* >$COMPONENT_ROOT/rbac-for-us.yaml
rm $COMPONENT_ROOT/*account*

COMPONENT_ROOT=${ARGOCD_MANIFESTS_ROOT}/server
mkdir -p $COMPONENT_ROOT
mv argocd-server*.yaml $COMPONENT_ROOT
yq e $COMPONENT_ROOT/*role* $COMPONENT_ROOT/*account* >$COMPONENT_ROOT/rbac-for-us.yaml
rm $COMPONENT_ROOT/*role* $COMPONENT_ROOT/*account*

COMPONENT_ROOT=${ARGOCD_MANIFESTS_ROOT}/redis
mkdir -p $COMPONENT_ROOT
mv argocd-redis*.yaml $COMPONENT_ROOT
pushd $COMPONENT_ROOT && rename 's/^(.*)$/ha-$1/g' *-ha* && rename 's/-ha//' *-ha* || true && popd
yq e $COMPONENT_ROOT/*role* $COMPONENT_ROOT/*account* >$COMPONENT_ROOT/rbac-for-us.yaml
rm $COMPONENT_ROOT/*role* $COMPONENT_ROOT/*account*

COMPONENT_ROOT=${ARGOCD_MANIFESTS_ROOT}/image-updater
mkdir -p $COMPONENT_ROOT
mv argocd-image-updater*.yaml $COMPONENT_ROOT
pushd $COMPONENT_ROOT && rename 's/^(.*)$/ha-$1/g' *-ha* && rename 's/-ha//' *-ha* || true && popd
yq e $COMPONENT_ROOT/*role* $COMPONENT_ROOT/*account* >$COMPONENT_ROOT/rbac-for-us.yaml
rm $COMPONENT_ROOT/*role* $COMPONENT_ROOT/*account*

# We use our own dex
rm -rf mv argocd-dex*.yaml

# all other manifests
mv argocd-*.yaml ${ARGOCD_MANIFESTS_ROOT}/
rm .yml # whatever
