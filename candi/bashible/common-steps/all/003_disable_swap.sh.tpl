swapoff -a
sed -i '/[[:space:]]swap[[:space:]]/d' /etc/fstab
