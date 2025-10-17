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
  ${WORKDIR}/bin/trivy i --vex oci --show-suppressed --policy "${TRIVY_POLICY_URL}" --cache-dir "${WORKDIR}/bin/trivy_cache" --skip-db-update --skip-java-db-update --exit-code 0 --severity "${SEVERITY}" --format json ${1} --output ${2} --quiet ${3} --username "${trivy_registry_user}" --password "${trivy_registry_pass}" --image-src remote
}

function send_report() {
  dd_scan_type="${1}"
  dd_report_file_path="${2}"
  dd_module_name="${3}"
  dd_image_name="${4}"

  dd_engagement_name="[$(echo "${dd_scan_type}" | tr '[:lower:]' '[:upper:]')] [IMAGES] [${dd_branch}]"

  tags_string="\"test_new_struct\",\"dkp\",\"images\",\"${dd_scan_type}\",\"${dd_release_or_dev_tag}\",\"${dd_image_version}\""
  if [[ -n "${dd_short_release_tag}" && -n "${dd_full_release_tag}" ]]; then
    tags_string+=",\"${dd_short_release_tag}\",\"${dd_full_release_tag}\""
  fi

  echo ""
  echo " Uploading trivy ${dd_branch} report for image \"${dd_image_name}\" of \"${dd_module_name}\" module"
  echo ""
  dd_upload_response=$(curl -sw "%{http_code}" -X POST \
    --retry 10 \
    --retry-delay 20 \
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
    -F "file=@${dd_report_file_path}" \
    -F "product_type_name=DKP" \
    -F "product_name=${dd_module_name}" \
    -F "scan_date=${date_iso}" \
    -F "engagement_name=${dd_engagement_name}" \
    -F "service=${dd_module_name} / ${dd_image_name}" \
    -F "group_by=component_name+component_version" \
    -F "deduplication_on_engagement=false" \
    -F "tags=dkp,module:${MODULE_NAME},image:${IMAGE_NAME},branch:${dd_branch}${codeowner_tags},${dd_short_release_tag},${dd_full_release_tag},${dd_default_branch_tag},${dd_release_or_dev_tag}" \
    -F "test_title=[${dd_module_name}]: ${dd_image_name}:${dd_image_version}" \
    -F "version=${dd_image_version}" \
    -F "build_id=${IMAGE_HASH}" \
    -F "commit_hash=${GITHUB_SHA}" \
    -F "branch_tag=${d8_tag}" \
    -F "apply_tags_to_findings=true")

  dd_return_code="${dd_upload_response: -3}"
  dd_return_body="${dd_upload_response:0: -3}"
  if [ ${dd_return_code} -eq 201 ]; then
    dd_engagement_id=$(echo ${dd_return_body} | jq ".engagement_id" )
    echo "dd_engagement_id: ${dd_engagement_id}"
    echo "Update with tags: ${tags_string}"
    # Updating engagement
    dd_eng_patch_response=$(curl -sw "%{http_code}" -X "PATCH" \
      --retry 10 \
      --retry-delay 20 \
      --retry-all-errors \
      "https://${DEFECTDOJO_HOST}/api/v2/engagements/${dd_engagement_id}/" \
      -H "accept: application/json" \
      -H "Authorization: Token ${DEFECTDOJO_API_TOKEN}" \
      -H "Content-Type: application/json" \
      -d "{
      \"tags\": ["${tags_string}"],
      \"version\": \"${dd_image_version}\",
      \"branch_tag\": \"${d8_tag}\"
    }")
    if [ ${dd_eng_patch_response: -3} -eq 200 ]; then
      echo "Engagemet \"${dd_engagement_name}\" updated successfully"
    else
      echo "!!!WARNING!!!"
      echo "Engagemet \"${dd_engagement_name}\" WAS NOT UPDATED"
      echo "HTTP_CODE: ${dd_eng_patch_response: -3}"
      echo "DD_RESPONSE: ${dd_eng_patch_response:0: -3}"
    fi
  else
    echo "!!!WARNING!!!"
    echo "Report for image \"${dd_image_name}\" of \"${dd_module_name}\" module WAS NOT UPLOADED"
    echo "HTTP_CODE: ${dd_return_code}"
    echo "DD_RESPONSE: ${dd_return_body}"
  fi
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
  dd_release_or_dev_tag="dev"
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
    dd_release_or_dev_tag="release"
    dd_image_version="$(echo ${dd_short_release_tag} | sed 's/^release\://')"
    dd_branch="$(echo ${dd_short_release_tag} | sed 's/\:/\-/')"
    trivy_registry_user="${PROD_REGISTRY_USER}"
    trivy_registry_pass="${PROD_REGISTRY_PASSWORD}"
  fi
  echo "=============================================="
  echo "Scanning additional images:"
  # Additional images to scan
  declare -a additional_images=("${d8_image}"
                "${d8_image}/install"
                )

  for additional_image in "${additional_images[@]}"; do
    a_module_name=""
    a_image_name=$(echo "${additional_image}" | grep -o '[^/]*$')
    # if it is deckhouse-oss - add it as deckhouse-controller module
    if [ "${a_image_name}" == "deckhouse-oss" ]; then
      a_module_name="deckhouse-controller"
    elif [ "${a_image_name}" == "install" ]; then
      a_module_name="dhctl"
    fi
    echo "----------------------------------------------"
    echo "👾 Scaning Deckhouse image \"${a_image_name}\" of module \"${a_module_name}\" for tag \"${d8_tag}\""
    echo ""
    # CVE Scan
    trivy_scan "--scanners vuln" "${module_reports}/d8_${a_module_name}_${a_image_name}_report.json" "${additional_image}:${d8_tag}"
    # License scan
    trivy_scan "--scanners license --license-full" "${module_reports}/d8_${a_module_name}_${a_image_name}_report_license.json" "${additional_image}:${d8_tag}"
    send_report "CVE" "${module_reports}/d8_${a_module_name}_${a_image_name}_report.json" "${a_module_name}" "${a_image_name}"
    send_report "License" "${module_reports}/d8_${a_module_name}_${a_image_name}_report_license.json" "${a_module_name}" "${a_image_name}"
  done

  echo "Deckhouse image to check: ${d8_image}:${d8_tag}"
  echo "Severity: ${SEVERITY}"
  echo "----------------------------------------------"
  echo ""
  docker pull "${d8_image}:${d8_tag}"
  digests=$(docker run --rm "${d8_image}:${d8_tag}" cat /deckhouse/modules/images_digests.json)

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

      echo "----------------------------------------------"
      echo "👾 Scaning Deckhouse image \"${IMAGE_NAME}\" of module \"${MODULE_NAME}\" for tag \"${d8_tag}\""
      echo ""
      # CVE Scan
      trivy_scan "--scanners vuln" "${module_reports}/d8_${MODULE_NAME}_${IMAGE_NAME}_report.json" "${d8_image}@${IMAGE_HASH}"
      # License scan
      trivy_scan "--scanners license --license-full" "${module_reports}/d8_${MODULE_NAME}_${IMAGE_NAME}_report_license.json" "${d8_image}@${IMAGE_HASH}"
      send_report "CVE" "${module_reports}/d8_${MODULE_NAME}_${IMAGE_NAME}_report.json" "${MODULE_NAME}" "${IMAGE_NAME}"
      send_report "License" "${module_reports}/d8_${MODULE_NAME}_${IMAGE_NAME}_report_license.json" "${MODULE_NAME}" "${IMAGE_NAME}"
    done
  done
done
