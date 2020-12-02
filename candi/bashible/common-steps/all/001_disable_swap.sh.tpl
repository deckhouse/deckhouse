swapoff -a
sed -i '/[[:space:]]swap[[:space:]]/d' /etc/fstab

for swapunit in $(systemctl list-units  --type swap | awk '/\.swap /{print $1}'); do
  systemctl stop "$swapunit"
  systemctl mask "$swapunit"
done
