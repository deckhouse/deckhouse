os_version = "22.04"
os_codename = "jammy"
os_family = "ubuntu"
guest_os_type = "ubuntu64Guest"
os_iso_url = "https://releases.ubuntu.com/22.04/ubuntu-22.04.1-live-server-amd64.iso"
iso_checksum = "10f19c5b2b8d6db711582e0e27f5116296c34fe4b313ba45f9b201a5007056cb"
root_disk_size = 20000
connection_username = "ubuntu"
connection_password = "ubuntu"
boot_command = [
        "c<wait>",
        "linux /casper/vmlinuz --- autoinstall ds=\"nocloud-net;seedfrom=http://{{.HTTPIP}}:{{.HTTPPort}}/\"",
        "<enter><wait>",
        "initrd /casper/initrd",
        "<enter><wait>",
        "boot",
        "<enter>"
]
scripts = [
  "scripts/apt.sh",
  "scripts/cleanup.sh",
  "scripts/grub.sh",
  "scripts/clean-ssh-hostkeys.sh",
  "scripts/harden-ubuntu-user.sh"
]

