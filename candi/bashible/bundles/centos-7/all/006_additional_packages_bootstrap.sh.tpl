if bb-flag? is-bootstrapped; then exit 0; fi

export DEBIAN_FRONTEND=noninteractive

yum install -y nfs-utils
