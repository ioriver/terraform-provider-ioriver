# Absolute minimum service: name, certificate, one origin, one domain.
# No behavior configuration — the backend applies sensible defaults.

resource "ioriver_service" "minimal" {
  name        = "minimal-service"
  certificate = ioriver_certificate.cert.id

  config = {
    origins = [
      {
        name = "my-origin"
        custom_origin = {
          host     = "origin.example.com"
          protocol = "https"
        }
      }
    ]

    domains = [
      {
        domain = "cdn.example.com"
        mappings = [
          {
            target_mapping = "my-origin"
          }
        ]
      }
    ]
  }
}
