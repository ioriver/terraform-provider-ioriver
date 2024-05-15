resource "ioriver_origin" "example_origin" {
  service = ioriver_service.service.id
  host    = "origin.example.com"
  shield_location = {
    country     = "US"
    subdivision = "VA"
  }
  shield_providers = [
    {
      service_provider = ioriver_service_provider.fastly.id
    },
    {
      service_provider = ioriver_service_provider.cloudfront.id
    }
  ]
}
