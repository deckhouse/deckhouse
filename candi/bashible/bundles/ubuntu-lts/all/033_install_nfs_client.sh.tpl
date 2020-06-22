if bb-is-ubuntu-version? 18.04 ; then
  bb-apt-install "nfs-common=1:1.3.4-2.1ubuntu5.3"
elif bb-is-ubuntu-version? 16.04 ; then
  bb-apt-install "nfs-common=1:1.2.8-9ubuntu12.2"
else
  bb-log-error "Unsupported Ubuntu version"
  exit 1
fi
