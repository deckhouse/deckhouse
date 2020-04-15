resource "openstack_compute_keypair_v2" "ssh" {
  name = var.prefix
  public_key = var.ssh_public_key
}
