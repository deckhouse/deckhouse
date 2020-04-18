if bb-flag? is-bootstrapped; then exit 0; fi

apt -o Dpkg::Options::="--force-confdef" -o Dpkg::Options::="--force-confold" remove -y unattended-upgrades
