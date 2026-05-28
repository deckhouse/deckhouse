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

signature_path="/opt/deckhouse/signature"
pki_path="/etc/kubernetes/pki"
extra_files_path="/etc/kubernetes/deckhouse/extra-files"

public_key_file="signature-public.jwks"
private_key_file="signature-private.jwk"
config_file="encryption-config.yaml"

if [ ! -d "$signature_path" ]; then
  bb-log-info "$signature_path is not exist. Skip prepare signatures"
  exit 0 
fi

# need to create in order
expected_files_in_order=("$config_file" "$public_key_file" "$private_key_file")
declare -A signatures_files=(
  ["$public_key_file"]="$pki_path"
  ["$private_key_file"]="$pki_path"
  ["$config_file"]="$extra_files_path"
)

# internal check arrays
if [[ "${#expected_files_in_order[@]}" != "${#signatures_files[@]}" ]]; then
  bb-log-error "Internal error: expected_files_in_order len != signatures_files len"
  exit 1
fi

for check_sig_file in "${expected_files_in_order[@]}"; do
  if [[ ${signatures_files["$check_sig_file"]+exists} ]]; then
    continue
  fi

  bb-log-error "Internal error: sig file $check_sig_file not exists in signatures_files map"
  exit 1
done

# check restart operation when restart bootstrap
# in dhctl we alway generate key for prevent corruption files
# if all dest files exists we do not need to prepare
# because if we add new files we can corrupt cluster

have_dest_files=()
for sig_file in "${!signatures_files[@]}"; do
  if [ -f "${signatures_files[$sig_file]}/${sig_file}" ]; then
    have_dest_files+=("${signatures_files[$sig_file]}/${sig_file}")
  fi
done

if [[ "${#have_dest_files[@]}" == "${#signatures_files[@]}" ]]; then
  rm -rf "$signature_path"
  bb-log-info "Signatures already prepared!"
  exit 0
fi

not_have_files=()

for exp_file in "${expected_files_in_order[@]}"; do
  if [ ! -f "${signature_path}/${exp_file}" ]; then
    not_have_files+=("${signature_path}/${exp_file}")
  fi
done

if [[ "${#not_have_files[@]}" != "0" ]]; then
  bb-log-error "Signatures directory exists but not have files: ${not_have_files[*]}"
  exit 1
fi

function move_and_fix_permissions() {
  local file="$1"
  local dest_dir="$2"

  local dest_path="${dest_dir}/${file}"
  local src="${signature_path}/${file}"

  if ! mv "$src" "$dest_path"; then
    bb-log-error "Cannot move $src to $dest_path"
    exit 1
  fi

  if ! chown root:root "$dest_path"; then
    bb-log-error "Cannot chown to root $dest_path"
    exit 1
  fi
  
  if ! chmod 0600 "$dest_path"; then
    bb-log-error "Cannot chmod to 0600 $dest_path"
    exit 1
  fi
}

if ! mkdir -p "$extra_files_path"; then
  bb-log-error "Cannot create dir $extra_files_path"
  exit 1
fi

for mv_sig_file in "${expected_files_in_order[@]}"; do
  move_and_fix_permissions "$mv_sig_file" "${signatures_files[$mv_sig_file]}/"
done

rm -rf "$signature_path"
