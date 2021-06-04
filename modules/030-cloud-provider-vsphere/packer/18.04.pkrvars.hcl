os_version = "18.04"
os_codename = "bionic"
os_family = "ubuntu"
guest_os_type = "ubuntu64Guest"
os_iso_url = "http://cdimage.ubuntu.com/ubuntu/releases/bionic/release/ubuntu-18.04.5-server-amd64.iso"
iso_checksum = "8c5fc24894394035402f66f3824beb7234b757dd2b5531379cb310cedfdf0996"
root_disk_size = 20000
connection_username = "ubuntu"
connection_password = "ubuntu"
boot_command = [
  "<enter><wait><f6><wait><esc><wait>",
  "<bs><bs><bs><bs><bs><bs><bs><bs><bs><bs>",
  "<bs><bs><bs><bs><bs><bs><bs><bs><bs><bs>",
  "<bs><bs><bs><bs><bs><bs><bs><bs><bs><bs>",
  "<bs><bs><bs><bs><bs><bs><bs><bs><bs><bs>",
  "<bs><bs><bs><bs><bs><bs><bs><bs><bs><bs>",
  "<bs><bs><bs><bs><bs><bs><bs><bs><bs><bs>",
  "<bs><bs><bs><bs><bs><bs><bs><bs><bs><bs>",
  "<bs><bs><bs><bs><bs><bs><bs><bs><bs><bs>",
  "<bs><bs><bs>",
  "/install/vmlinuz",
  " initrd=/install/initrd.gz",
  " priority=critical",
  " locale=en_US",
  " url=http://{{ .HTTPIP }}:{{ .HTTPPort }}/preseed.cfg",
  "<enter>"
]
