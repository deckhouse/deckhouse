if bb-is-ubuntu-version? 18.04 ; then
  echo "5.3.0-1017-aws" > /var/lib/bashible/kernel_version_desired_by_cloud_provider
fi
