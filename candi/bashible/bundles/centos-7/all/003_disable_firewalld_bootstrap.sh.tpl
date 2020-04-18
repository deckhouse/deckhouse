if bb-flag? is-bootstrapped; then exit 0; fi

systemctl stop firewalld
systemctl disable firewalld
systemctl mask firewalld
