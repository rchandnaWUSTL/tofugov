variable "keep_legacy_export" {
  type    = bool
  default = false
}

variable "engine_version" {
  type    = string
  default = "postgres-16"
}

resource "local_file" "legacy_customer_export" {
  count    = var.keep_legacy_export ? 1 : 0
  filename = "${path.module}/out/legacy-customer-export.csv"
  content  = "customer_id,email\n1001,alice@example.com\n1002,bob@example.com\n"
}

resource "terraform_data" "primary_database" {
  triggers_replace = var.engine_version
  input = {
    engine         = var.engine_version
    instance_class = "db.r6g.large"
  }
}
