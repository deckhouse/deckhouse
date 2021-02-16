swapoff -a
sed -i '/[[:space:]]swap[[:space:]]/d' /etc/fstab

for swapunit in $(systemctl list-units --no-legend --plain --no-pager --type swap | cut -f1 -d" "); do
  systemctl stop "$swapunit"
  systemctl mask "$swapunit"
done
