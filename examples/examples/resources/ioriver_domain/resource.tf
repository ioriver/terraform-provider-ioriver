resource "ioriver_domain" "example_domain" {
  service = ioriver_service.service.id
  domain  = "domain.example.com"
  origin  = ioriver_origin.example_origin.id
}
