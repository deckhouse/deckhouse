provider "vsphere" {
  vsphere_server = var.providerClusterConfiguration.provider.server
  user = var.providerClusterConfiguration.provider.username
  password = var.providerClusterConfiguration.provider.password
  allow_unverified_ssl = lookup(var.providerClusterConfiguration.provider, "insecure", false)
  persist_session = true
  vim_session_path = "/tmp/.govmomi/sessions"
  rest_session_path = "/tmp/.govmomi/rest_sessions"
}
