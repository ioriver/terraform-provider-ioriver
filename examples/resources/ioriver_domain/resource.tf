resource "ioriver_domain" "example_domain" {
  service = ioriver_service.service.id
  domain  = "domain.example.com"
  mappings = [
    {
      target_id = ioriver_origin.origin.id
    }
  ]
}
