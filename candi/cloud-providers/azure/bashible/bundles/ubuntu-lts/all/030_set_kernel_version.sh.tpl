if bb-is-ubuntu-version? 18.04 ; then
  echo "5.4.0-1032-azure" > /var/lib/bashible/kernel_version_desired_by_cloud_provider
elif bb-is-ubuntu-version? 20.04 ; then
  echo "5.4.0-1032-azure" > /var/lib/bashible/kernel_version_desired_by_cloud_provider
fi
