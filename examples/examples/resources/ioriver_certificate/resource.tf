resource "ioriver_certificate" "self_managed_cert" {
  name              = "example-cert"
  type              = "SELF_MANAGED"
  certificate       = file("certificate.crt")
  private_key       = file("private.key")
  certificate_chain = file("ca_bundle.crt")
}

resource "ioriver_certificate" "managed_cert" {
  name = "example-managed-cert"
  type = "MANAGED"
  cn   = "[\"domain.example.com\"]"
}
