resource "ioriver_origin_shield" "example_shield" {
  service = ioriver_service.service.id
  location = {
    country     = "US"
    subdivision = "VA"
  }
  providers = [
    {
      service_provider = ioriver_service_provider.cloudfront.id
    }
  ]
}
