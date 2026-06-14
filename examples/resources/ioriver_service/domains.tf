# Domain and mapping configuration patterns.

# ---------------------------------------------------------------------------
# 1. Single domain mapped to one origin (default path pattern /*)
# ---------------------------------------------------------------------------
resource "ioriver_service" "single_domain" {
  name        = "single-domain-service"
  certificate = ioriver_certificate.cert.id

  config = {
    origins = [
      {
        name          = "my-origin"
        custom_origin = { host = "origin.example.com", protocol = "https" }
      }
    ]
    domains = [
      {
        domain = "www.example.com"
        mappings = [
          {
            target_mapping = "my-origin" # default path_pattern is /*
          }
        ]
      }
    ]
  }
}

# ---------------------------------------------------------------------------
# 2. Multiple domains pointing to the same origin, with domain aliases
#    aliases lets you list alternative hostnames for a domain; the CDN
#    will accept requests for them and route them to the same origin.
# ---------------------------------------------------------------------------
resource "ioriver_service" "multi_domain_aliases" {
  name        = "multi-domain-service"
  certificate = ioriver_certificate.cert.id

  config = {
    origins = [
      {
        name          = "prod-origin"
        custom_origin = { host = "prod.example.com", protocol = "https" }
      }
    ]
    domains = [
      {
        domain   = "www.example.com"
        mappings = [{ target_mapping = "prod-origin" }]
      },
      {
        domain   = "example.com"
        aliases  = ["www.example.com", "m.example.com"] # additional accepted hostnames
        mappings = [{ target_mapping = "prod-origin" }]
      }
    ]
  }
}

# ---------------------------------------------------------------------------
# 3. Path-based routing: different URL paths routed to different origins.
#    Mappings are evaluated in order; the first match wins.
# ---------------------------------------------------------------------------
resource "ioriver_service" "path_routing" {
  name        = "path-routing-service"
  certificate = ioriver_certificate.cert.id

  config = {
    origins = [
      {
        name          = "api-origin"
        custom_origin = { host = "api.example.com", protocol = "https" }
      },
      {
        name          = "static-origin"
        custom_origin = { host = "static.example.com", protocol = "https" }
      }
    ]
    domains = [
      {
        domain = "www.example.com"
        mappings = [
          {
            path_pattern   = "/api/*" # requests under /api/ → api-origin
            target_mapping = "api-origin"
          },
          {
            path_pattern   = "/*" # everything else → static-origin
            target_mapping = "static-origin"
          }
        ]
      }
    ]
  }
}

# ---------------------------------------------------------------------------
# 4. Multiple domains, each with its own dedicated origin
# ---------------------------------------------------------------------------
resource "ioriver_service" "multi_domain_multi_origin" {
  name        = "multi-origin-service"
  certificate = ioriver_certificate.cert.id

  config = {
    origins = [
      {
        name          = "web-origin"
        custom_origin = { host = "web.example.com", protocol = "https" }
      },
      {
        name          = "assets-origin"
        custom_origin = { host = "assets.example.com", protocol = "https" }
      }
    ]
    domains = [
      {
        domain   = "www.example.com"
        mappings = [{ target_mapping = "web-origin" }]
      },
      {
        domain   = "cdn.example.com"
        mappings = [{ target_mapping = "assets-origin" }]
      }
    ]
  }
}
