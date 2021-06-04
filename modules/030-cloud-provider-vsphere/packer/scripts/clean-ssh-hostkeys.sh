#! /bin/bash

# This script forces a new ssh_host_key at first logon

set -e
set -x

sudo rm -fv /etc/ssh/ssh_host_*

#add check for ssh keys on reboot...regenerate if necessary
cat << 'EOL' | sudo tee /etc/rc.local
#!/bin/sh -e
#
# rc.local
#
# This script is executed at the end of each multiuser runlevel.
# Make sure that the script will "" on success or any other
# value on error.
#
# In order to enable or disable this script just change the execution
# bits.a
#
# By default this script does nothing.
test -f /etc/ssh/ssh_host_dsa_key || dpkg-reconfigure openssh-server && systemctl restart sshd
exit 0
EOL

# make sure the script is executable
sudo chmod +x /etc/rc.local
