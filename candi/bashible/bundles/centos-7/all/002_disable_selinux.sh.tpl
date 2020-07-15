# Set SELinux in permissive mode (effectively disabling it)
setenforce 0 || true
sed -i 's/^SELINUX=enforcing$/SELINUX=permissive/' /etc/selinux/config
