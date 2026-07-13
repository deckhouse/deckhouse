# Feature-gates test helpers. Source after ./scripts/functions.sh.

CPM_E2E_FG_STATE_DIR="${CPM_E2E_FG_STATE_DIR:-${TMPDIR:-/tmp}/cpm-e2e-feature-gates}"
CPM_E2E_FEATURE_GATES_MAP="${CPM_E2E_FEATURE_GATES_MAP:-/deckhouse/candi/feature_gates_map.yml}"
CPM_E2E_FG_MODULECONFIG="${CPM_E2E_FG_STATE_DIR}/moduleconfig-target.yaml"

CPM_E2E_FG_COMPONENT_KEYS="${CPM_E2E_FG_COMPONENT_KEYS:-apiserver kubeControllerManager kubeScheduler}"
CPM_E2E_FG_POD_COMPONENTS="${CPM_E2E_FG_POD_COMPONENTS:-kube-apiserver kube-controller-manager kube-scheduler}"

_feature_gates_map_json() {
  map_file="$1"
  if [ ! -f "$map_file" ]; then
    e2e_log "feature gates map not found: $map_file"
    return 1
  fi
  if ! command -v yq >/dev/null 2>&1; then
    e2e_log "yq is required to read $map_file"
    return 1
  fi
  yq -o=json '.' "$map_file"
}

_component_map_key() {
  case "$1" in
    kube-apiserver) printf '%s' apiserver ;;
    kube-controller-manager) printf '%s' kubeControllerManager ;;
    kube-scheduler) printf '%s' kubeScheduler ;;
    *)
      e2e_log "_component_map_key: unknown pod component $1"
      return 1
      ;;
  esac
}

# feature_gates_enabled_for_version prints a JSON array of enabled feature gates for a version.
feature_gates_enabled_for_version() {
  version="$1"
  map_file="$2"
  map_json=$(_feature_gates_map_json "$map_file") || return 1
  printf '%s' "$map_json" | jq --arg ver "$version" '
    .[$ver] as $v
    | if $v == null then error("kubernetes version \($ver) not found in feature gates map") else $v end
    | (($v.deprecated // []) + ($v.forbidden // [])) as $exclude
    | [($v.apiserver // []), ($v.kubeControllerManager // []), ($v.kubeScheduler // []), ($v.kubelet // [])]
    | add | unique
    | map(select(. as $g | ($exclude | index($g) | not)))
  '
}

# feature_gates_for_component prints newline-separated gate names for a component map key.
feature_gates_for_component() {
  version="$1"
  component_key="$2"
  map_file="$3"
  map_json=$(_feature_gates_map_json "$map_file") || return 1
  printf '%s' "$map_json" | jq -r --arg ver "$version" --arg component "$component_key" '
    .[$ver] as $v
    | if $v == null then error("kubernetes version \($ver) not found in feature gates map") else $v end
    | (($v.deprecated // []) + ($v.forbidden // [])) as $exclude
    | ($v[$component] // [])
    | map(select(. as $g | ($exclude | index($g) | not)))
    | .[]
  '
}

write_feature_gates_moduleconfig() {
  output_file="$1"
  gates_json="$2"
  mkdir -p "$(dirname "$output_file")"
  {
    printf '%s\n' 'apiVersion: deckhouse.io/v1alpha1'
    printf '%s\n' 'kind: ModuleConfig'
    printf '%s\n' 'metadata:'
    printf '%s\n' '  name: control-plane-manager'
    printf '%s\n' 'spec:'
    printf '%s\n' '  enabled: true'
    printf '%s\n' '  settings:'
    printf '%s\n' '    enabledFeatureGates:'
    printf '%s' "$gates_json" | jq -r '.[] | "    - " + .'
    printf '%s\n' '  version: 3'
  } > "$output_file"
  gate_count=$(printf '%s' "$gates_json" | jq 'length')
  e2e_log "wrote ModuleConfig with ${gate_count} enabledFeatureGates to $output_file"
}

# prepare_feature_gates_test backs up ModuleConfig, snapshots CPOs, and builds target manifest.
prepare_feature_gates_test() {
  state_dir="$1"
  map_file="${2:-$CPM_E2E_FEATURE_GATES_MAP}"
  if [ -z "$state_dir" ]; then
    e2e_log "prepare_feature_gates_test: state directory is required"
    return 1
  fi
  mkdir -p "$state_dir"

  version=$(kubernetes_version)
  e2e_log "detected Kubernetes version $version"

  enabled_gates=$(feature_gates_enabled_for_version "$version" "$map_file") || return 1
  printf '%s' "$enabled_gates" > "$state_dir/enabled-feature-gates.json"
  gate_count=$(printf '%s' "$enabled_gates" | jq 'length')
  e2e_log "selected ${gate_count} enabled feature gates for version $version"

  for pod_component in $CPM_E2E_FG_POD_COMPONENTS; do
    component_key=$(_component_map_key "$pod_component")
    feature_gates_for_component "$version" "$component_key" "$map_file" \
      > "$state_dir/component-gates-${pod_component}.txt"
    component_count=$(wc -l < "$state_dir/component-gates-${pod_component}.txt" | tr -d ' ')
    e2e_log "$pod_component: ${component_count} feature gates to verify"
  done

  write_feature_gates_moduleconfig "$state_dir/moduleconfig-target.yaml" "$enabled_gates"
}

# assert_control_plane_feature_gates checks each component manifest contains its feature gates.
assert_control_plane_feature_gates() {
  state_dir="$1"
  for pod_component in $CPM_E2E_FG_POD_COMPONENTS; do
    assert_feature_gates_in_component "$pod_component" "$state_dir/component-gates-${pod_component}.txt"
  done
}

# assert_feature_gates_in_component checks GateName=true appears in pod manifests.
assert_feature_gates_in_component() {
  component_label="$1"
  gates_file="$2"
  if [ -z "$component_label" ] || [ -z "$gates_file" ] || [ ! -f "$gates_file" ]; then
    e2e_log "assert_feature_gates_in_component: component label and gates file are required"
    return 1
  fi
  while IFS= read -r gate || [ -n "$gate" ]; do
    [ -z "$gate" ] && continue
    if ! is_flag_in_component "$component_label" "${gate}=true"; then
      e2e_log "feature gate ${gate}=true not found in $component_label pod manifest"
      return 1
    fi
  done < "$gates_file"
  e2e_log "all feature gates present in $component_label pod manifest"
}