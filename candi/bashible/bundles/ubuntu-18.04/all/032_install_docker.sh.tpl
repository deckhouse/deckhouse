if bb-flag? is-bootstrapped; then exit 0; fi

apt -o Dpkg::Options::="--force-confdef" -o Dpkg::Options::="--force-confold" install -qy "docker.io=18.09.*"
apt-mark hold docker.io
