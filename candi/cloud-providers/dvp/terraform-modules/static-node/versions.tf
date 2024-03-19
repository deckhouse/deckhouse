terraform {
  required_version = ">= 0.14.8"
  required_providers {
    kubernetes = {
      source = "hashicorp/kubernetes"
    }
  }
}
