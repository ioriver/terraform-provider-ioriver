# Canonical starting-point example for an IO River service.
# One HTTPS origin, one domain, 60-minute edge cache, compression enabled.

resource "ioriver_service" "example" {
  name        = "my-service"
  description = "My IO River CDN service"
  certificate = ioriver_certificate.cert.id

  config = {
    origins = [
      {
        name = "primary-origin"
        custom_origin = {
          host     = "origin.example.com"
          protocol = "https"
        }
      }
    ]

    domains = [
      {
        domain = "www.example.com"
        mappings = [
          {
            target_mapping = "primary-origin"
          }
        ]
      }
    ]

    behaviors = {
      default = {
        actions = {
          cache_ttl   = 3600 # 60-minute edge cache
          compression = true
        }
      }
    }
  }
}
