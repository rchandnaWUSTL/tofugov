variable "replicas" {
  type    = number
  default = 4
}

resource "random_pet" "release_name" {}

resource "terraform_data" "checkout_service_config" {
  input = {
    replicas  = var.replicas
    log_level = "info"
  }
}
