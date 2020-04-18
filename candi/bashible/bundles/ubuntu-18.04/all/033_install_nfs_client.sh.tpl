# Install nfs to be able to mount nfs shares in pods

if bb-flag? is-bootstrapped; then exit 0; fi

export DEBIAN_FRONTEND=noninteractive

apt -o Dpkg::Options::="--force-confdef" -o Dpkg::Options::="--force-confold" install -qy nfs-common
