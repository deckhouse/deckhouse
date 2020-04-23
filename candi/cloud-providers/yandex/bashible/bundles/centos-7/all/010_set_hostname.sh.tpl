# Overriding hostname received from metadata server
hostnamectl set-hostname "$(hostname | cut -d "." -f 1)"
