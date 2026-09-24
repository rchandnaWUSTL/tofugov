variable "rotation_id" {
  type    = string
  default = "2026-q3"
}

resource "random_password" "db_master" {
  length  = 32
  special = true
  keepers = {
    rotation = var.rotation_id
  }
}

resource "local_sensitive_file" "pgpass" {
  filename = "${path.module}/out/pgpass"
  content  = "orders-db.internal:5432:*:app:${random_password.db_master.result}\n"
}
