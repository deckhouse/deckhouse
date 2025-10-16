#!/bin/bash

# Copyright 2025 Flant JSC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -e

#SCAN_TARGET description:
#  only_main - Scan only main branch. executes on push to main.
#  pr - Scan image from PR tag. executes on PRs.
#  regular - Scan main and latest 3 minor releases. executes on schedule and manual run.


case ${SCAN_TARGET} in
  only_main | pr | regular )
    echo "SCAN_TARGET: ${SCAN_TARGET}"
    ;;
  *)
    echo "SCAN_TARGET is not valid"
    exit 1
esac

PROD_REGISTRY_DECKHOUSE_IMAGE="${PROD_REGISTRY}/deckhouse/fe"
DEV_REGISTRY_DECKHOUSE_IMAGE="${DEV_REGISTRY}/sys/deckhouse-oss"

login_prod_registry() {
  echo "Log in to PROD registry"
  docker login "${PROD_REGISTRY}"
}
login_dev_registry() {
  echo "Log in to DEV registry"
  docker login "${DEV_REGISTRY}"
}

trivy_scan() {
  ${WORKDIR}/bin/trivy i --policy "${TRIVY_POLICY_URL}" --cache-dir "${WORKDIR}/bin/trivy_cache" --skip-db-update --skip-java-db-update --exit-code 0 --severity "${SEVERITY}" --format json ${1} --output ${2} --quiet ${3} --username "${trivy_registry_user}" --password "${trivy_registry_pass}" --image-src remote
}

send_report() {
  echo ""
  echo " Uploading trivy ${1} report for image \"${IMAGE_NAME}\" of \"${MODULE_NAME}\" module"
  echo ""
  curl -s -S -o /dev/null --fail-with-body -X POST \
    --retry 5 \
    --retry-delay 10 \
    --retry-all-errors \
    https://${DEFECTDOJO_HOST}/api/v2/reimport-scan/ \
    -H "accept: application/json" \
    -H "Authorization: Token ${DEFECTDOJO_API_TOKEN}" \
    -F "auto_create_context=True" \
    -F "minimum_severity=Info" \
    -F "active=true" \
    -F "verified=true" \
    -F "scan_type=Trivy Scan" \
    -F "close_old_findings=true" \
    -F "do_not_reactivate=false" \
    -F "push_to_jira=false" \
    -F "file=@${2}" \
    -F "product_type_name=DKP" \
    -F "product_name=${3}" \
    -F "scan_date=${date_iso}" \
    -F "engagement_name=${1}" \
    -F "service=${MODULE_NAME} / ${IMAGE_NAME}" \
    -F "group_by=component_name+component_version" \
    -F "deduplication_on_engagement=false" \
    -F "tags=deckhouse_image,module:${MODULE_NAME},image:${IMAGE_NAME},branch:${dd_branch}${codeowner_tags},${dd_short_release_tag},${dd_full_release_tag},${dd_default_branch_tag}" \
    -F "test_title=[${MODULE_NAME}]: ${IMAGE_NAME}:${dd_image_version}" \
    -F "version=${dd_image_version}" \
    -F "build_id=${IMAGE_HASH}" \
    -F "commit_hash=${GITHUB_SHA}" \
    -F "branch_tag=${d8_tag}" \
    -F "apply_tags_to_findings=true"
}

# Create docker config file to use during this CI Job
echo "----------------------------------------------"
echo ""
echo "Preparing DOCKER_CONFIG"
mkdir -p "${WORKDIR}/docker"
cat > "${WORKDIR}/docker/config.json" << EOL
{
        "auths": {
                "${PROD_REGISTRY}": {
                        "auth": "$(echo -n "${PROD_REGISTRY_USER}:${PROD_REGISTRY_PASSWORD}" | base64)"
                },
                "${DEV_REGISTRY}": {
                        "auth": "$(echo -n "${DEV_REGISTRY_USER}:${DEV_REGISTRY_PASSWORD}" | base64)"
                }
        }
}
EOL
export DOCKER_CONFIG="${WORKDIR}/docker"

echo "----------------------------------------------"
echo ""
echo "Getting Trivy"
mkdir -p "${WORKDIR}/bin/trivy-${TRIVY_BIN_VERSION}"
curl -L --fail-with-body "https://${DECKHOUSE_PRIVATE_REPO}/api/v4/projects/${TRIVY_PROJECT_ID}/packages/generic/trivy-${TRIVY_BIN_VERSION}/${TRIVY_BIN_VERSION}/trivy" -o ${WORKDIR}/bin/trivy-${TRIVY_BIN_VERSION}/trivy
chmod u+x ${WORKDIR}/bin/trivy-${TRIVY_BIN_VERSION}/trivy
rm -rf ${WORKDIR}/bin/trivy
ln -s ${PWD}/${WORKDIR}/bin/trivy-${TRIVY_BIN_VERSION}/trivy ${WORKDIR}/bin/trivy

echo "Updating Trivy Data Bases"
mkdir -p "${WORKDIR}/bin/trivy_cache"
${WORKDIR}/bin/trivy image --username "${DEV_REGISTRY_USER}" --password "${DEV_REGISTRY_PASSWORD}" --download-db-only --db-repository "${TRIVY_DB_URL}" --cache-dir "${WORKDIR}/bin/trivy_cache"
${WORKDIR}/bin/trivy image --username "${DEV_REGISTRY_USER}" --password "${DEV_REGISTRY_PASSWORD}" --download-java-db-only --java-db-repository "${TRIVY_JAVA_DB_URL}" --cache-dir "${WORKDIR}/bin/trivy_cache"

echo "----------------------------------------------"
echo ""
echo "Getting tags to scan"
d8_tags=("${TAG}")
if [ "${SCAN_TARGET}" == "regular" ]; then
  login_prod_registry
  if [ "${TAG}" != "main" ]; then
    # if some specific release is defined - scan only it
    if echo "${TAG}"|grep -q "^[0-9]\.[0-9]*$"; then
      d8_tags=($(crane ls "${PROD_REGISTRY_DECKHOUSE_IMAGE}" | grep "^v${TAG}\.[0-9]*$" | sort -V -r | head -n 1))
    else
      echo "ERROR: Please specify required release in the following format: [0-9]\.[0-9]*"
      exit 1
    fi
  else
    # Get release tags by regexp, sort by sevmer desc, cut to get minor version, uniq and get 3 latest
    releases=($(crane ls "${PROD_REGISTRY_DECKHOUSE_IMAGE}" | grep "^v[0-9]*\.[0-9]*\.[0-9]*$" | sort -V -r))
    latest_minor_releases=($(printf '%s\n' "${releases[@]}"| cut -d "." -f -2 | uniq | head -n 3))
    for r in "${latest_minor_releases[@]}"; do
      d8_tags+=($(printf '%s\n' "${releases[@]}" | grep "${r}" | sort -V -r|head -n 1))
    done
  fi
# else - this is push to main or PR, so scan only them.
fi
echo "CVE Scan will be applied to the following tags of Deckhouse"
echo "${d8_tags[@]}"

# Scan in loop for provided list of tags
for d8_tag in "${d8_tags[@]}"; do
  dd_default_branch_tag=""
  dd_short_release_tag=""
  dd_full_release_tag=""
  dd_image_version="${d8_tag}"
  dd_branch="${d8_tag}"
  date_iso=$(date -I)
  d8_image="${DEV_REGISTRY_DECKHOUSE_IMAGE}"
  trivy_registry_user="${DEV_REGISTRY_USER}"
  trivy_registry_pass="${DEV_REGISTRY_PASSWORD}"
  module_reports="${WORKDIR}/deckhouse/${d8_tag}/reports"
  mkdir -p {"${module_reports}","${WORKDIR}/artifacts"}

  # Log in to registry before pulling each deckhouse image to avoid registry session end
  login_dev_registry
  if [ "${SCAN_TARGET}" == "regular" ]; then
    login_prod_registry
  fi

  # set cpecial tag for DD if images from main
  if [ "${d8_tag}" == "main" ]; then
    dd_default_branch_tag="default_branch"
  fi
  # if d8_tag is for release - we need to take it from prod registry
  if echo "${d8_tag}"|grep -q "^v[0-9]\.[0-9]*\.[0-9]*$"; then
    d8_image="${PROD_REGISTRY_DECKHOUSE_IMAGE}"
    dd_short_release_tag="release:$(echo ${d8_tag} | cut -d '.' -f -2 | sed 's/^v//')"
    dd_full_release_tag="image_release_tag:${d8_tag}"
    dd_image_version="$(echo ${dd_short_release_tag} | sed 's/^release\://')"
    dd_branch="$(echo ${dd_short_release_tag} | sed 's/\:/\-/')"
    trivy_registry_user="${PROD_REGISTRY_USER}"
    trivy_registry_pass="${PROD_REGISTRY_PASSWORD}"
  fi

  echo "Deckhouse image to check: ${d8_image}:${d8_tag}"
  echo "Severity: ${SEVERITY}"
  echo "----------------------------------------------"
  echo ""
  docker pull "${d8_image}:${d8_tag}"
  digests=$(docker run --rm "${d8_image}:${d8_tag}" cat /deckhouse/modules/images_digests.json)

  # Additional images to scan
  declare -a additional_images=("${d8_image}"
                "${d8_image}/install"
                )
  for additional_image in "${additional_images[@]}"; do
    additional_image_name=$(echo "${additional_image}" | grep -o '[^/]*$')
    # if it is deckhouse-oss - add it as deckhouse-controller module
    if [ "${additional_image_name}" == "deckhouse-oss" ]; then
      digests=$(echo "${digests}"|jq --arg i "${additional_image_name}" --arg s "${d8_tag}" '."deckhouse-controller" += { ($i): ($s) }')
    elif [ "${additional_image_name}" == "install" ]; then
      digests=$(echo "${digests}"|jq --arg i "${additional_image_name}" --arg s "${d8_tag}" '.dhctl += { ($i): ($s) }')
    fi
  done

  echo "=============================================="
  echo "The following images will be scanned:"
  echo "${digests}"

  for module in $(jq -rc 'to_entries[]' <<< "${digests}"); do
    MODULE_NAME=$(jq -rc '.key' <<< "${module}")
    echo "=============================================="
    echo "🛰 Module: ${MODULE_NAME}"

    # Get codeowners to fill defectDojo tags
    CODEOWNERS_MODULE_NAME="$(echo $MODULE_NAME|sed -s 's/[A-Z]/-&/g')"
    codeowner_tags=""
    # Search module number if any
    if ls -1 modules/ |grep -i "^[0-9]*-${CODEOWNERS_MODULE_NAME}$"; then
      # As we know module number - lets search with it
      CODEOWNERS_MODULE_NAME=$(ls -1 modules/ |grep -i "^[0-9]*-${CODEOWNERS_MODULE_NAME}$")
      while IFS="\n" read -r line; do
        search_pattern=$(echo "$line"| sed 's/^\///'|cut -d '/' -f 1)
        if echo ${CODEOWNERS_MODULE_NAME} | grep -i -q "$search_pattern"; then
          for owner_name in $(echo "${line#*@}"); do
            codeowner_tags="${codeowner_tags},codeowner:${owner_name#*@}"
          done
          break
        fi
      done < .github/CODEOWNERS
    else
      # As we dont have module number - also cut it from search pattern
      while IFS="\n" read -r line; do
        # 'sed' will cut "/" before folder name if exist, 'cut' will get dirname that will be used as regexp for current module_name, then cut digits from module name
        search_pattern=$(echo "$line"| sed 's/^\///'|cut -d '/' -f 1|sed 's/^[0-9]*-//')
        if echo ${CODEOWNERS_MODULE_NAME} | grep -i -q "$search_pattern"; then
          for owner_name in $(echo "${line#*@}"); do
            codeowner_tags="${codeowner_tags},codeowner:${owner_name#*@}"
          done
          break
        fi
      done < .github/CODEOWNERS
    fi
    # Set default codeowner in case if not found in CODEOWNERS file
    if [ -z "${codeowner_tags}" ]; then
      codeowner_tags=",codeowner:RomanenkoDenys"
    fi

    for module_image in $(jq -rc '.value | to_entries[]' <<<"${module}"); do
      IMAGE_NAME="$(jq -rc '.key' <<< ${module_image})"
      IMAGE_HASH="$(jq -rc '.value' <<< ${module_image})"
      # Set flag if additional image to use tag instead of hash
      additional_image_detected=false
      for image_item in "${additional_images[@]}"; do
        if [ "${IMAGE_NAME}" == $(echo "${image_item}"| grep -o '[^/]*$') ]; then
          additional_image_detected=true
          break
        fi
      done

      echo "----------------------------------------------"
      echo "👾 Scaning Deckhouse image \"${IMAGE_NAME}\" of module \"${MODULE_NAME}\" for tag \"${d8_tag}\""
      echo ""
      if [ "${additional_image_detected}" == true ]; then
        # CVE Scan
        image_to_scan="${d8_image}${IMAGE_NAME/deckhouse-oss/""}:${d8_tag}" # replace deckhouse-oss with an empty string, but keep any other image_name for additional_image.
        trivy_scan "--scanners vuln" "${module_reports}/d8_${MODULE_NAME}_${IMAGE_NAME}_report.json" "${image_to_scan/"/:"/:}"
        # License scan
        trivy_scan "--scanners license --license-full" "${module_reports}/d8_${MODULE_NAME}_${IMAGE_NAME}_report_license.json" "${image_to_scan/"/:"/:}"
      else
        # CVE Scan
        trivy_scan "--scanners vuln" "${module_reports}/d8_${MODULE_NAME}_${IMAGE_NAME}_report.json" "${d8_image}@${IMAGE_HASH}"
        # License scan
        trivy_scan "--scanners license --license-full" "${module_reports}/d8_${MODULE_NAME}_${IMAGE_NAME}_report_license.json" "${d8_image}@${IMAGE_HASH}"
      fi

      send_report "CVE" "${module_reports}/d8_${MODULE_NAME}_${IMAGE_NAME}_report.json" "${MODULE_NAME}"
      send_report "License" "${module_reports}/d8_${MODULE_NAME}_${IMAGE_NAME}_report_license.json" "${MODULE_NAME}"
    done
  done
done
