if bb-is-ubuntu-version? 18.04 ; then
  echo "5.3.0-1018-gcp" > /var/lib/bashible/kernel_version_desired_by_cloud_provider
fi
