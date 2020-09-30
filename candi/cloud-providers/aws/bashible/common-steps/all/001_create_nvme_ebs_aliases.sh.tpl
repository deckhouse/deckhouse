if [[ "$FIRST_BASHIBLE_RUN" != "yes" ]]; then
  exit 0
fi

if [ -f /var/lib/bashible/kubernetes-data-device-installed ]; then
  exit 0
fi

volume_names="$(find /dev | grep -i 'nvme[0-21]n1$')"
for volume in ${volume_names}
do
 symlink="$(nvme id-ctrl -v "${volume}" | grep '^0000:' | sed -E 's/.*"(\/dev\/[a-z]+)\.+"$/\1/')"
 if [ ! -z "${symlink}" ] && [ ! -e "${symlink}" ]; then
  ln -s "${volume}" "${symlink}"
 fi
done
