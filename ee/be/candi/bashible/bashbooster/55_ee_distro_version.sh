# Copyright 2022 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

bb-is-redos-version?() {
  local REDOS_VERSION=$1
  source /etc/os-release
  if [[ "${VERSION_ID}" =~ ^${REDOS_VERSION}.*$ ]] ; then
    return 0
  else
    return 1
  fi
}

bb-is-rosa-version?() {
  local ROSA_VERSION=$1
  source /etc/os-release

  local VERSION_KEY="${VERSION}" # different version key for Cobalt and Chrome releases
  [[ "${VERSION_ID}" == "7.9" ]] && VERSION_KEY="${VERSION_ID}"
  
  if [[ "${VERSION_KEY}" =~ ^${ROSA_VERSION}.*$ ]]; then
    return 0
  else
    return 1
  fi
}

bb-is-astra-version?() {
  local ASTRA_VERSION=$1
  source /etc/os-release
  if [[ "${VERSION_ID}" =~ ^${ASTRA_VERSION}.*$ ]] ; then
    return 0
  else
    return 1
  fi
}

bb-is-altlinux-version?() {
  local ALTLINUX_VERSION=$1
  source /etc/os-release
  if [[ "${VERSION_ID}" =~ ^${ALTLINUX_VERSION}.*$ ]] ; then
    return 0
  else
    return 1
  fi
}
