bb-is-ubuntu-version?() {
  local UBUNTU_VERSION=$1
  if [ "$(source /etc/os-release; echo ${VERSION_ID})" == "${UBUNTU_VERSION}" ] ; then
    return 0
  else
    return 1
  fi
}
