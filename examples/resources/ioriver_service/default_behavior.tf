# behaviors.default applies to every request that does not match a specific
# entry in behaviors.custom. It is optional and computed — when omitted the
# backend fills sensible defaults. When set explicitly, only the fields you
# specify are overridden; the rest keep their backend defaults.

# ---------------------------------------------------------------------------
# 1. behaviors.default omitted entirely
#    Terraform will not manage these settings; the CDN uses platform defaults.
# ---------------------------------------------------------------------------
resource "ioriver_service" "default_behavior_omitted" {
  name        = "service-defaults-omitted"
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
        domain   = "www.example.com"
        mappings = [{ target_mapping = "my-origin" }]
      }
    ]

    # No behaviors block — the CDN applies platform defaults.
  }
}

# ---------------------------------------------------------------------------
# 2. behaviors.default explicitly configured
#    Every optional/computed action field is spelled out so the Terraform plan
#    output is fully self-documenting and there are no surprise diffs.
# ---------------------------------------------------------------------------
resource "ioriver_service" "default_behavior_explicit" {
  name        = "service-defaults-explicit"
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
        domain   = "www.example.com"
        mappings = [{ target_mapping = "my-origin" }]
      }
    ]

    behaviors = {
      default = {
        actions = {
          # ---- Caching ----
          cache_behavior       = "CACHE" # CACHE | BYPASS
          cache_ttl            = 86400   # 24-hour edge cache (seconds)
          browser_cache_ttl    = 3600    # 1-hour browser cache (seconds)
          stale_ttl            = 600     # serve stale up to 10 min on origin errors
          origin_cache_control = false   # ignore origin Cache-Control header

          # ---- Cache key ----
          cache_key = {
            headers = [
              { header = "Accept-Encoding" }
            ]
            cookies = []
            query_strings = {
              type = "all" # include | exclude | all | none
            }
            country     = false
            device_type = false
          }

          # ---- Per-status-code TTL overrides ----
          status_codes_ttl = [
            {
              status_code    = "4xx"
              cache_behavior = "BYPASS"
              cache_ttl      = 0
            },
            {
              status_code    = "5xx"
              cache_behavior = "BYPASS"
              cache_ttl      = 0
            }
          ]

          # ---- Protocol ----
          viewer_protocol = "REDIRECT_HTTP_TO_HTTPS" # HTTPS_ONLY | HTTP_AND_HTTPS | REDIRECT_HTTP_TO_HTTPS

          # ---- Allowed request methods ----
          allowed_methods = [
            { method = "GET" },
            { method = "HEAD" },
            { method = "OPTIONS" }
          ]

          # ---- Performance ----
          compression              = true
          large_files_optimization = false
        }
      }
    }
  }
}
