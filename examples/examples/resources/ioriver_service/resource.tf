resource "ioriver_service" "service" {
  name        = "example-myservice"
  description = "This is my service"
  certificate = ioriver_certificate.cert.id
}
