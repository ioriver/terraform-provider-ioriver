resource "ioriver_origin_shield" "shield" {
  service = ioriver_service.service.id
  origin  = ioriver_origin.origin.id

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
