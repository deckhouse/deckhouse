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

# Function to pull or push with retry logic
perform_oras_command() {
  command="$1"
  echo "[*] Running oras command: $command"

  retries=3
  attempt=0

  while [ $attempt -lt $retries ]; do
    set +e
    $command
    status=$?
    set -e

    if [ $status -eq 0 ]; then
      echo "[+] Command successful."
      return 0
    else
      echo "[!] $command failed with code $status"
      echo "[!] timeout 5s before retry"
      sleep 5
      attempt=$((attempt + 1))
      echo "[*] Retry attempt $attempt/$retries"
    fi
  done

  echo "[!] Failed after $retries attempts."
  return 1
}


get_vulnerabilities () {
    # List of ALT Linux branches to download
    alt_versions=("c9f2" "c10f1" "p11" "p10" "p9")

    for version in "${alt_versions[@]}"; do
      echo "[*] Processing ALT version: $version"
      dir="cache/vuln-list/alt/$version"
      zip_file="$dir/$version.zip"
      url="https://rdb.altlinux.org/api/errata/export/oval/$version"

      mkdir -p "$dir"

      # Try initial download
      if ! wget -qO "$zip_file" "$url"; then
        echo "[!] Initial download for $version failed, starting retry loop..."

        while ! wget -O "$zip_file" --tries=5 --waitretry=10 --timeout=30 --read-timeout=30 "$url"; do
          echo "[!] Download failed for $version, retrying in 15 seconds..."
          sleep 15
        done
      fi

      # Unzip and clean up
      (
        cd "$dir" && unzip -oq "$version.zip" && rm -f "$version.zip"
      )
    done

    # Download RedOS OVAL file
    redos_dir="cache/vuln-list/redos"
    redos_file="$redos_dir/oval.xml"
    redos_url="https://redos.red-soft.ru/support/secure/redos.xml"

    mkdir -p "$redos_dir"

    # Try initial download
    if ! wget -qO "$redos_file" "$redos_url"; then
      echo "[!] Initial download for RedOS failed, starting retry loop..."

      while ! wget -O "$redos_file" --tries=5 --waitretry=10 --timeout=30 --read-timeout=30 "$redos_url"; do
        echo "[!] Download failed for RedOS, retrying in 15 seconds..."
        sleep 15
      done
    fi

    # sanitize redos oval from incorrect entries of &lt/&gt
    sed -i '/\&lt[^;]/s/\&lt/\&lt;/g' cache/vuln-list/redos/oval.xml
    sed -i '/\&gt[^;]/s/\&gt/\&gt;/g' cache/vuln-list/redos/oval.xml

    mkdir -p ./cache/vuln-list/astra
    mkdir -p ./cache/db
    mkdir -p ./cache/policies

    # Try a single quiet attempt to download the file
    if ! wget -qO cache/vuln-list/astra/scanovalcontent_alse17.deb \
      --ca-certificate=./misc/russian_trusted_sub_ca.cer \
      --no-check-certificate \
      https://bdu.fstec.ru/files/scanovalcontent_alse17.deb; then

      echo "[!] Initial attempt failed, starting retry loop..."
      # Retry loop with verbose output and retry logic
      while ! wget -vdO cache/vuln-list/astra/scanovalcontent_alse17.deb \
        --ca-certificate=./misc/russian_trusted_sub_ca.cer \
        --no-check-certificate \
        --tries=5 \
        --waitretry=10 \
        --timeout=30 \
        --read-timeout=30 \
        https://bdu.fstec.ru/files/scanovalcontent_alse17.deb; do
          echo "[!] Download failed, retrying in 15 seconds..."
          sleep 15
      done

    else
      echo "[+] Successfully downloaded on first attempt."
    fi
    dpkg -x cache/vuln-list/astra/scanovalcontent_alse17.deb cache/vuln-list/astra/; mv cache/vuln-list/astra/var/lib/scanoval/data/AstraSE17VulnsOVAL.xml cache/vuln-list/astra/AstraOVAL.xml
    rm -rf cache/vuln-list/astra/scanovalcontent_alse17.deb cache/vuln-list/astra/var 
}

prepare_db () {
    cd ./cache/db
    docker logout ghcr.io
    perform_oras_command "../../oras pull -d -v ghcr.io/aquasecurity/trivy-db:2"
    docker logout ghcr.io
    perform_oras_command "../../oras pull -d -v ghcr.io/aquasecurity/trivy-java-db:1"
    tar zxvf db.tar.gz && rm db.tar.gz
    ../../trivy-db build --cache-dir ../ --output-dir . --update-interval 6h
    tar cvzf db.tar.gz -C ./ trivy.db metadata.json
}

prepare_policies () {
    cd ../policies
    docker logout ghcr.io
    perform_oras_command "../../oras pull -d -v ghcr.io/aquasecurity/trivy-checks:0"
    tar zxvf bundle.tar.gz && rm bundle.tar.gz
    # relax etcd directory ownership check
    sed -i 's/audit: stat -c %U:%G $etcd.datadirs/audit: stat -c %U:%G $etcd.datadirs \| sed '\''s\/root\/etcd\/g'\''/g' commands/kubernetes/etcdDataDirectoryOwnership.yaml
    # relax cni directory permissions check
    sed -i 's/audit: stat -c %a \/\*\/cni\/\*/audit: stat -c %a \/\*\/cni\/net.d\/\*/g' commands/kubernetes/containerNetworkInterfaceFilePermissions.yaml
    tar cvzf ../db/bundle.tar.gz ./
    cd ../db
}


if [ ! -f ./oras ]; then
  echo "[*] oras binary not found, downloading..."

  url="https://github.com/oras-project/oras/releases/download/v1.0.0/oras_1.0.0_linux_amd64.tar.gz"

  # Retry loop for curl download and extract
  while ! curl -sSfL "$url" | tar -xzf -; do
    echo "[!] oras download failed, retrying in 15 seconds..."
    sleep 15
  done

  chmod +x oras
  echo "[+] oras downloaded and ready."
fi

if [ ! -d ./cache ]; then
  echo "Cache not found"
  get_vulnerabilities
  prepare_db
  prepare_policies
else
  if [ "$(( `date +%s` - `stat -L --format %Y ./cache` ))" -gt "300" ]; then
    echo "Remove stale cache"
    rm -rf ./cache
    get_vulnerabilities
    prepare_db
    prepare_policies
  else
    cd ./cache/db
  fi
fi
host=$(echo "$1" | awk -F/ '{print $1}')
case "$host" in
  "$DECKHOUSE_REGISTRY_HOST")
    auth="-u $DECKHOUSE_REGISTRY_USER -p $DECKHOUSE_REGISTRY_PASSWORD"
    ;;
  "$GHCR_HOST")
    auth="-u $GHCR_IO_REGISTRY_USER -p $GHCR_IO_REGISTRY_PASSWORD"
    ;;
  "$DECKHOUSE_CSE_REGISTRY_HOST")
    auth="-u $DECKHOUSE_CSE_REGISTRY_USER -p $DECKHOUSE_CSE_REGISTRY_PASSWORD"
    ;;
  "$DECKHOUSE_DEV_CSE_REGISTRY_HOST")
    auth="-u $DECKHOUSE_CSE_DEV_REGISTRY_USER -p $DECKHOUSE_CSE_DEV_REGISTRY_PASSWORD"
    ;;
  "$DECKHOUSE_DEV_REGISTRY_HOST")
    auth="-u $DECKHOUSE_DEV_REGISTRY_USER -p $DECKHOUSE_DEV_REGISTRY_PASSWORD"
    ;;
  *)
    echo "[!] Unknown registry: $host"
    exit 1
    ;;
esac

perform_oras_command "../../oras push -d -v $auth --artifact-type application/vnd.aquasec.trivy.config.v1+json ${1}/security/trivy-db:2 db.tar.gz:application/vnd.aquasec.trivy.db.layer.v1.tar+gzip"
perform_oras_command "../../oras push -d -v $auth --artifact-type application/octet-stream ${1}/security/trivy-checks:0 bundle.tar.gz:application/vnd.cncf.openpolicyagent.layer.v1.tar+gzip"
perform_oras_command "../../oras push -d -v $auth --artifact-type application/vnd.aquasec.trivy.config.v1+json ${1}/security/trivy-java-db:1 javadb.tar.gz:application/vnd.aquasec.trivy.javadb.layer.v1.tar+gzip"
