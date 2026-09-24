resource "local_file" "fluentbit_conf" {
  filename = "${path.module}/out/fluent-bit.conf"
  content  = <<-CONF
    [OUTPUT]
        Name  stdout
        Match *
  CONF
}
