{{- define "resources" }}
resources:
  requests:
    memory: {{ pluck .Values.web.env .Values.resources.requests.memory._default | first | default .Values.resources.requests.memory._default._default }}
{{- end }}

{{- define "resources-registry-modules-watcher" }}
resources:
  requests:
    memory: {{ pluck .Values.web.env .Values.resources.requests.memory.registryModulesWatcher._default | first | default .Values.resources.requests.memory.registryModulesWatcher._default._default }}
{{- end }}

{{- define "resources-moduleslibrary-builder" }}
resources:
  requests:
    memory: {{ pluck .Values.web.env .Values.resources.requests.memory.moduleslibrary.builder | first | default .Values.resources.requests.memory.moduleslibrary.builder._default }}
  limits:
    memory: {{ pluck .Values.web.env .Values.resources.limits.memory.moduleslibrary.builder | first | default .Values.resources.limits.memory.moduleslibrary.builder._default }}
{{- end}}

{{- define "resources-moduleslibrary-web" }}
resources:
  requests:
    memory: {{ pluck .Values.web.env .Values.resources.requests.memory.moduleslibrary.web | first | default .Values.resources.requests.memory.moduleslibrary.web._default }}
  limits:
    memory: {{ pluck .Values.web.env .Values.resources.limits.memory.moduleslibrary.web | first | default .Values.resources.limits.memory.moduleslibrary.web._default }}
{{- end}}

{{- define "vrouter_envs" }}
- name: VROUTER_DEFAULT_GROUP
  value: {{ .Values.vrouter.defaultGroup | quote }}
- name: VROUTER_DEFAULT_CHANNEL
  value: {{ pluck .Values.web.env .Values.vrouter.defaultChannel | first | default .Values.vrouter.defaultChannel._default | quote }}
- name: VROUTER_SHOW_LATEST_CHANNEL
  value: {{ .Values.vrouter.showLatestChannel | quote }}
- name: VROUTER_LISTEN_PORT
  value: "8082"
- name: VROUTER_LOG_LEVEL
  value: {{ pluck .Values.web.env .Values.vrouter.logLevel | first | default .Values.vrouter.logLevel._default | quote }}
- name: VROUTER_PATH_STATIC
  value: {{ pluck .Values.web.env .Values.vrouter.pathStatic | first | default .Values.vrouter.pathStatic._default | quote }}
- name: VROUTER_LOCATION_VERSIONS
  value: {{ .Values.vrouter.locationVersions | quote }}
- name: VROUTER_PATH_CHANNELS_FILE
  value: {{ pluck .Values.web.env .Values.vrouter.pathChannelsFile | first | default .Values.vrouter.pathChannelsFile._default | quote }}
- name: VROUTER_PATH_TPLS
  value: {{ pluck .Values.web.env .Values.vrouter.pathTpls | first | default .Values.vrouter.pathTpls._default | quote }}
- name: VROUTER_I18N_TYPE
  value: {{ .Values.vrouter.i18nType | quote }}
- name: VROUTER_URL_VALIDATION
  value: {{ pluck .Values.web.env .Values.vrouter.urlValidation | first | default .Values.vrouter.urlValidation._default | quote }}
{{- end }}

{{- define "readiness_probe" }}
failureThreshold: 5
periodSeconds: 10
timeoutSeconds: 5
{{- end }}
{{- define "liveness_probe" }}
failureThreshold: 10
periodSeconds: 10
timeoutSeconds: 5
{{- end }}
{{- define "startup_probe" }}
failureThreshold: 10
periodSeconds: 10
timeoutSeconds: 5
{{- end }}

{{- define "embedded_modules_history_sync_script" }}
#!/bin/sh

set -eu

BOOTSTRAP_MODE="${BOOTSTRAP_MODE:-false}"
CONFIGMAP="embedded-modules-history-data"
CONFIGMAP_KEY="embedded_modules_list.json"
SOURCE_FILE="/data/embedded_modules_list.json"
DOWNLOAD_STATUS_FILE="/data/download.status"
DOWNLOAD_ERROR_FILE="/data/download.error"
CURRENT_FILE="/tmp/current-embedded_modules_list.json"
EMPTY_FILE="/tmp/empty-embedded_modules_list.json"
MANIFEST_FILE="/tmp/embedded-modules-history-data.yaml"
MAX_CONFIGMAP_FILE_SIZE=900000
MAX_CONFIGMAP_MANIFEST_SIZE=1048576

fail() {
  echo "ERROR: $*" >&2
  exit 1
}

configmap_exists() {
  kubectl get configmap "${CONFIGMAP}" -n "${NAMESPACE}" >/dev/null 2>&1
}

create_configmap_from_file() {
  src_file="$1"

  kubectl create configmap "${CONFIGMAP}" \
    -n "${NAMESPACE}" \
    --from-file="${CONFIGMAP_KEY}=${src_file}" \
    --dry-run=client \
    -o yaml > "${MANIFEST_FILE}" || fail "Failed to render ConfigMap manifest for '${CONFIGMAP}'."

  MANIFEST_SIZE=$(wc -c < "${MANIFEST_FILE}" | tr -d ' ')
  if [ "${MANIFEST_SIZE}" -gt "${MAX_CONFIGMAP_MANIFEST_SIZE}" ]; then
    fail "Rendered ConfigMap manifest is too large (${MANIFEST_SIZE} bytes > ${MAX_CONFIGMAP_MANIFEST_SIZE} bytes). The existing ConfigMap '${CONFIGMAP}' was left unchanged."
  fi

  kubectl apply \
    --dry-run=server \
    -f "${MANIFEST_FILE}" \
    >/dev/null || fail "Server-side validation failed for ConfigMap '${CONFIGMAP}'. The existing ConfigMap was left unchanged."

  kubectl apply \
    -f "${MANIFEST_FILE}" \
    >/dev/null || fail "Failed to update ConfigMap '${CONFIGMAP}'."
}

handle_bootstrap_source_error() {
  message="$1"

  if configmap_exists; then
    echo "WARNING: ${message}" >&2
    echo "ConfigMap '${CONFIGMAP}' already exists. Leaving it unchanged." >&2
    exit 0
  fi

  printf '{}' > "${EMPTY_FILE}"
  create_configmap_from_file "${EMPTY_FILE}"
  echo "WARNING: ${message}" >&2
  echo "ConfigMap '${CONFIGMAP}' did not exist, so an empty placeholder was created to keep builder mounts valid." >&2
  exit 0
}

handle_source_error() {
  message="$1"

  if [ "${BOOTSTRAP_MODE}" = "true" ]; then
    handle_bootstrap_source_error "${message}"
  fi

  fail "${message} The existing ConfigMap '${CONFIGMAP}' was left unchanged."
}

if [ ! -f "${DOWNLOAD_STATUS_FILE}" ]; then
  handle_source_error "Download status file '${DOWNLOAD_STATUS_FILE}' was not found."
fi

DOWNLOAD_STATUS=$(cat "${DOWNLOAD_STATUS_FILE}")
if [ "${DOWNLOAD_STATUS}" != "ok" ]; then
  DOWNLOAD_ERROR="$(cat "${DOWNLOAD_ERROR_FILE}" 2>/dev/null || true)"
  if [ -z "${DOWNLOAD_ERROR}" ]; then
    DOWNLOAD_ERROR="Failed to download '${SOURCE_FILE}' from S3."
  fi
  handle_source_error "${DOWNLOAD_ERROR}"
fi

if [ ! -f "${SOURCE_FILE}" ]; then
  handle_source_error "Downloaded file '${SOURCE_FILE}' was not found."
fi

if [ ! -s "${SOURCE_FILE}" ]; then
  handle_source_error "Downloaded file '${SOURCE_FILE}' is empty."
fi

FILE_SIZE=$(wc -c < "${SOURCE_FILE}" | tr -d ' ')
if [ "${FILE_SIZE}" -gt "${MAX_CONFIGMAP_FILE_SIZE}" ]; then
  handle_source_error "Downloaded file is too large for ConfigMap update (${FILE_SIZE} bytes > ${MAX_CONFIGMAP_FILE_SIZE} bytes)."
fi

FIRST_CHAR=$(tr -d '\n\r\t ' < "${SOURCE_FILE}" | cut -c1)
LAST_CHAR=$(tr -d '\n\r\t ' < "${SOURCE_FILE}" | sed 's/.*\(.\)$/\1/')

if [ "${FIRST_CHAR}" != "{" ] || [ "${LAST_CHAR}" != "}" ]; then
  handle_source_error "Downloaded file '${SOURCE_FILE}' does not look like a JSON object."
fi

kubectl get configmap "${CONFIGMAP}" \
  -n "${NAMESPACE}" \
  -o jsonpath='{.data.embedded_modules_list\.json}' > "${CURRENT_FILE}" 2>/dev/null || true

if [ -s "${CURRENT_FILE}" ] && cmp -s "${CURRENT_FILE}" "${SOURCE_FILE}"; then
  echo "ConfigMap '${CONFIGMAP}' is already up to date."
  exit 0
fi

create_configmap_from_file "${SOURCE_FILE}"

echo "ConfigMap '${CONFIGMAP}' has been updated successfully."
{{- end }}
