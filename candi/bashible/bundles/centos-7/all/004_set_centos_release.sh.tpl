. /etc/os-release
bundleName="${ID}-${VERSION_ID}"
if ! [[ "$bundleName" =~ "centos-" ]]; then
 exit 0
fi

bb-yum-install "centos-release-7-9.2009.0.el7.centos.x86_64"
yum-config-manager --enable C7.8.2003-base C7.8.2003-updates
