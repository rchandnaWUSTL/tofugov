variable "flags" {
  type = map(bool)
  default = {
    new_checkout_flow = true
    dark_mode         = false
  }
}

resource "random_id" "flag_set_version" {
  byte_length = 4
}

resource "terraform_data" "feature_flags" {
  input = var.flags
}
