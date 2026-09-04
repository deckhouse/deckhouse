#!/bin/bash

# Copyright 2026 Flant JSC
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

# This script has just one purpose:
# Fetch inputs for validation script:
#   - title and description of pull request
#   - diff content

echo "COMMIT_REF_NAME: ${COMMIT_REF_NAME}"
echo "COMMIT_HASH: ${COMMIT_HASH}"

set -e

# Define anlysis options
SKIP_UNREACHABLE_PROCEDURE_ANALYSIS=${SKIP_UNREACHABLE_PROCEDURE_ANALYSIS:-"deps"}

# Define import options
PROJECT_GROUP=${PROJECT_GROUP:-"Unspecified"}
IF_NO_GROUP=${IF_NO_GROUP:-"add"}

# Define color codes
RED="\033[0;31m"   # Red for errors
YELLOW="\033[0;33m" # Yellow for warnings
GREEN="\033[0;32m"  # Green for success
BLUE="\033[0;34m"   # Blue for info
CYAN="\033[0;36m"   # Cyan fot datetime
NC="\033[0m"        # No color (reset)

error() { echo -e "${CYAN}$(date '+%Y-%m-%d %H:%M:%S') ${RED}ERROR: ${1}${NC}" >&2; }
warning() { echo -e "${CYAN}$(date '+%Y-%m-%d %H:%M:%S') ${YELLOW}WARNING: ${1}${NC}"; }
info() { echo -e "${CYAN}$(date '+%Y-%m-%d %H:%M:%S') ${BLUE}INFO: ${1}${NC}"; }
success() { echo -e "${CYAN}$(date '+%Y-%m-%d %H:%M:%S') ${GREEN}SUCCESS: ${1}${NC}"; }

SSH="ssh -o UserKnownHostsFile=${ISOLATE_KNOWN_HOSTS} -o ConnectTimeout=10 -o ServerAliveInterval=10 -o ServerAliveCountMax=12"


send() {
    # $1 - command to send
    # $2 - retries count
    local command="$1"
    local retries="${2:-3}"
    local attempt=0
    local success=0
    result=""

    while [ "$attempt" -lt "$retries" ]; do
        result=$($SSH ${SVACE_ANALYZE_SSH_USER}@${SVACE_ANALYZE_HOST} "${command}") && success=1 && break
        error "Command failed. Retrying..."
        attempt=$((attempt + 1))
        sleep 2
    done

    if [ "$success" -eq 0 ]; then
        error "All $retries attempts failed!"
        return 1
    else
        echo "${result}"
        return 0
    fi
}

send_request () {
    # $1 - request
    # $2 - retries count
    local request="${1}"
    local expected_code="200"
    local retries="${2:-3}"
    local attempt=0
    local success=0
    local sleep_timeout=2
    local max_sleep=500
    local svacer_user=${SVACER_IMPORT_USER}
    local svace_password=${SVACER_IMPORT_PASSWORD}
    local token_response_code=""
    local token_response=""
    result=""

    get_token="curl --write-out \"\\n%{http_code}\" -sSL --request POST \
    --url ${url}/api/public/login \
    --header 'User-Agent: Curl' \
    --header 'content-type: application/json' \
    --data '{ \
    \"auth_type\": \"svacer\", \
    \"login\": \"${svacer_user}\", \
    \"password\": \"${svace_password}\" \
    }'"

    while [ "$attempt" -lt "$retries" ]; do
        response=$($SSH ${SVACE_ANALYZE_SSH_USER}@${SVACE_ANALYZE_HOST} "${get_token}") && success=1 || success=0

        if [[ $success == 1 ]]; then
            token_response_code=$(echo "$response" | tail -n1)
            token_response=$(echo "${response}" | sed '$d')
            if [[ "${token_response_code}" == "${expected_code}" && -n "${token_response}" ]]; then
                token=$(jq -r '.token' <<< "${token_response}") && success=1 || success=0
                if [[ $success == 1 && -n "${token}" ]]; then
                    request="${request} --header 'Authorization: Bearer ${token}'"
                    response=$($SSH ${SVACE_ANALYZE_SSH_USER}@${SVACE_ANALYZE_HOST} "${request}") && success=1 || success=0

                    if [[ $success == 1 ]]; then
                        result=$(echo "${response}" | sed '$d')
                        response_code=$(echo "$response" | tail -n1)
                        [[ "${response_code}" == "${expected_code}" && -n "${result}" ]] || success=0 && success=1 && break
                    fi
                fi
            fi
        fi

        error "Request failed with code: ${response_code}. Retrying..."
        error "Sleeping for ${sleep_timeout} sec ..."
        sleep $sleep_timeout
        sleep_timeout=$((sleep_timeout*2))
        ((sleep_timeout > max_sleep)) && sleep_timeout=$max_sleep
        attempt=$((attempt + 1))
    done

    if [ "$success" -eq 0 ]; then
        error "All $retries attempts failed!"
        return 1
    else
        echo "${result}"
        return 0
    fi
}

upload_to_svacer() {
    # $1 - svacer project name
    # $2 - branch name
    # $3 - path to archive
    # $4 - waiting timeout
    # $5 - polling interval
    # $6 - request attempts number

    local url="${SVACER_URL}"
    local project_name="${1}"
    local branch_name="${2}"
    local archive_name="${3}"
    local timeout=${4:-1800}
    local interval=${5:-10}
    local retries=$((timeout/interval))
    local request_attempts=${6:-10}
    local import_task_id=""
    local import_task_status=""
    local success=0
    local response=""

    svacer_import="curl --write-out \"\\n%{http_code}\" -sSL --request POST \
    --url ${url}/api/public/svace/import\\?async\\=true \
    --header 'User-Agent: Curl' \
    --header 'content-type: multipart/form-data' \
    --form project=\"${project_name}\" \
    --form branch=\"${branch_name}\" \
    --form file=@\"${archive_name}\" \
    --form options='{\"values\":[ \
    {\"option\":\"project-group\",\"value\":\"${PROJECT_GROUP}\"}, \
    {\"option\":\"if-no-group\",\"value\":\"${IF_NO_GROUP}\"}, \
    {\"option\":\"field\",\"value\":\"CI_COMMIT_HASH:${COMMIT_HASH}\"}, \
    {\"option\":\"field\",\"value\":\"CI_COMMIT_REF_NAME:${COMMIT_REF_NAME}\"} \
    ]}'"

    info "Importing \"${project_name}\"..."
    response=$(send_request "${svacer_import}" $request_attempts)
    read import_task_id import_task_status <<< $(jq -r '(.task_id|tostring)+" "+.status' <<< ${response})
    if [[ -z "${import_task_id}" ]]; then
        error "Import task failed. Response: ${response}"
        exit 12
    fi
    info "Import task scheduled succesfully. Task id: ${import_task_id}"

    get_import_log="curl --write-out \"\\n%{http_code}\" -sSL --request GET \
    --url ${url}/api/public/svace/import/logs/${import_task_id} \
    --header 'User-Agent: Curl'"

    info "Waiting for import to complete..."
    for ((i = 0; i < retries; i++)); do
        info "Checking import task status..."
        response=$(send_request "${get_import_log}" $request_attempts)

        if grep -Eqi '^## END OF TASK' <<< "${response}"; then
            if grep -Eqi 'Upload SUCCESS' <<< "${response}"; then
                success=1
                break
            else
                success=0
                break
            fi
        fi


        info "Import of ${project_name} in progress. Sleeping for $interval sec"
        sleep $interval
    done

    if [ "$success" -eq 0 ]; then
        error "All attempts failed!"
        error "${response}"
        return 1
    else
        return 0
    fi
}

define_import_params() {
    local -n project="${1}"
    local -n branch="${2}"
    if [[ $(send "[[ -f ${proj}/.svace-dir/import-settings ]] && echo true || echo false") == true ]]; then
        import_settings=$(send "cat /${proj}/.svace-dir/import-settings")
        while IFS='=' read -r key val; do
            case "$key" in
            ProjectName)
                project=$val
                ;;
            Branch)
                branch=$val
                ;;
            *)
                warning "Undefined import setting \"${key}=${val}\" will be ommited!"
                ;;
            esac
        done <<< "${import_settings}"
    fi
}

get_svace_bin() {
    proj="${1}"
    svace_version=$(send "cat ${proj}/.svace-dir/svace-dir.version | awk 'FNR==3{print}'")

    svace_bin="/opt/svace-${svace_version}/bin/svace"
    if [[ $(send "[[ -x ${svace_bin} ]] && echo true || echo false") == true ]]; then
        echo "${svace_bin}"
    else
        echo "svace"
        error "\"${svace_bin}\" is not executable on analyze server. Using default."
    fi
}

echo "Searching for current build artifacts on server by path: /${SVACE_ANALYZE_DIR}/${COMMIT_HASH}"
if [[ $(send "[[ -d /${SVACE_ANALYZE_DIR}/${COMMIT_HASH} ]] && echo true || echo false") == false ]]; then
warning "Specified commit directory doesn't exists on analyze server." && exit 11
fi

projects=$(send "find /${SVACE_ANALYZE_DIR}/${COMMIT_HASH} \\( -type d -iname .svace-dir -o -iname *.tar.gz \\) -exec dirname {} \\;")
[[ -z $projects ]] && warning "Nothing to analyze in ${proj}" && echo "::warning file=$(realpath "$0")::Nothing to analyze" && exit 0
info "${projects}"

info "Starting svace analyze..."
for proj in $projects; do
    svacer_proj=${proj#/${SVACE_ANALYZE_DIR}/${COMMIT_HASH}/}
    build_item=${svacer_proj##*/}

    import_project=${svacer_proj}
    import_branch=${COMMIT_REF_NAME}

    if [[ $(send "[[ -d ${proj}/.svace-dir ]] && echo true || echo false") == true ]]; then
        define_import_params import_project import_branch
        svace_bin=$(get_svace_bin "${proj}")
        info "Using svace binary: $svace_bin"

        info "Start analyzing project \"${svacer_proj}\" ..."
        send "${svace_bin} config --svace-dir ${proj} THREAD_NUMBER auto"
        send "${svace_bin} analyze --set-config SKIP_UNREACHABLE_PROCEDURE_ANALYSIS=${SKIP_UNREACHABLE_PROCEDURE_ANALYSIS} --quiet --svace-dir ${proj}"
        success "Analysis completed successfully!"

        info "Start archiving project \"${svacer_proj}\" ..."
        send "cd ${proj} && tar -czf ${build_item}.tar.gz .svace-dir && rm -rf .svace-dir"
        info "Archiving completed successfuly!"
    else
        warning "Nothing to analyze! Couldn't find \".svace-dir\" in \"${proj}\""
    fi

    if [[ $(send "[[ -f ${proj}/${build_item}.tar.gz ]] && echo true || echo false") == true ]]; then
        info "Start importing project \"${import_project}\" with branch \"${import_branch}\" ..."
        upload_to_svacer "${import_project}" "${import_branch}" "${proj}/${build_item}.tar.gz"
        success "Import completed successfuly!"
    else
        warning "Nothing to import! Couldn't find \"${build_item}.tar.gz\" in \"${proj}\""
    fi

    info "Cleaning up artifacts..."
    send "rm -rf ${proj}"
    send "find /${SVACE_ANALYZE_DIR}/${COMMIT_HASH} -maxdepth 2 -type d -empty -delete"
    info "Cleanup completed successfully"
done