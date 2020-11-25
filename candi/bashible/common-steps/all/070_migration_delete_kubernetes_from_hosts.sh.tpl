# TODO - remove in future releases
sed -i "/127.0.0.1 kubernetes/d" /etc/hosts

if [ -f "/etc/cloud/templates/hosts.debian.tmpl" ] ; then
  sed -i "/127.0.0.1 kubernetes/d" /etc/cloud/templates/hosts.debian.tmpl
fi
