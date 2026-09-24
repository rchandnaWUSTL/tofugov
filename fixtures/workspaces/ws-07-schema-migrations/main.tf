variable "schema_version" {
  type    = string
  default = "42"
}

resource "null_resource" "run_migrations" {
  triggers = {
    schema_version = var.schema_version
  }

  provisioner "local-exec" {
    command = "echo 'applying schema migration ${var.schema_version} to orders-db'"
  }
}
