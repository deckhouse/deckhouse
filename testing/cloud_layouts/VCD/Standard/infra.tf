terraform {
  backend "s3" {
    bucket                      = "deckhouse-e2e-terraform-state"
    key                         = "state/${var.PREFIX}.tfstate" 
    region                      = "ru-7"
    endpoint                    = "https://s3.ru-7.storage.selcloud.ru"
    skip_region_validation      = true
    skip_credentials_validation = true
  }
  required_version = ">= 0.14.0"
  required_providers {
    vcd = {
      source  = "vmware/vcd"
      version = "= 3.14.1"
    }
  }
}

provider "vcd" {}

variable "PREFIX" {}
variable "VCD_ORG" {}
variable "VCD_VDC" {}



resource "vcd_vapp" "vapp" {
  name = var.PREFIX
}

resource "vcd_vapp_org_network" "vapp_network" {
  org                    = var.VCD_ORG
  vdc                    = var.VCD_VDC
  vapp_name              = vcd_vapp.vapp.name
  org_network_name       = "deckhouse-e2e"
  reboot_vapp_on_removal = true
}