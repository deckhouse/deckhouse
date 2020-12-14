if bb-is-ubuntu-version? 20.04 ; then
  cat <<EOF > /var/lib/bashible/kernel_version_config_by_cloud_provider
desired_version="5.4.0-1029-gcp"
allowed_versions_pattern=""
EOF
elif bb-is-ubuntu-version? 18.04 ; then
  cat <<EOF > /var/lib/bashible/kernel_version_config_by_cloud_provider
desired_version="5.3.0-1018-gcp"
allowed_versions_pattern=""
EOF
fi
