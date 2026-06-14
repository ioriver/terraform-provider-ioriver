# Advanced behavior matching using the condition block.
#
# condition replaces path_pattern when you need to match on anything other
# than the URL path. A condition is an OR-of-ANDs expression:
#
#   or[N].and = a list of conditions that must ALL be true (AND group)
#
# A request matches when ANY or-group matches.
#
# Exactly one of path_pattern or condition must be set on each behavior.
#
# ---- Available fields ----
#   http.request.domain       — request hostname (Host header)
#   http.request.path         — URL path
#   http.request.method       — HTTP method
#   http.request.header       — a specific request header  (requires field_key)
#   http.response.status_code — HTTP response status code
#   http.response.header      — a specific response header (requires field_key)
#   http.request.query_param  — a specific query parameter (requires field_key)
#   client.geo.country        — client country (ISO 3166-1 alpha-2, e.g. "US")
#   client.device.is_mobile   — "true" or "false"
#   client.ip                 — client IP address
#
# ---- Available operators ----
#   eq, ne, lt, gt, le, ge
#   in, not_in
#   match, not_match
#   matches_one_of, does_not_match_any_of
#   regex, not_regex
#   exists, does_not_exist
#   ip_match, not_ip_match   (for client.ip)
#   contains, not_contains, begins_with, not_begins_with, ends_with, not_ends_with
#   contains_word, not_contains_word

resource "ioriver_service" "condition_examples" {
  name        = "condition-matching-service"
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
      custom = [

        # -----------------------------------------------------------------------
        # path_pattern is shorthand for a single http.request.path / match rule.
        # Use it for simple glob-style URL matching.
        # -----------------------------------------------------------------------
        {
          name         = "simple-path-pattern"
          path_pattern = "/api/*"
          actions      = { cache_behavior = "BYPASS" }
        },

        # -----------------------------------------------------------------------
        # 1. Geo-based — deny access from specific countries
        # -----------------------------------------------------------------------
        {
          name = "geo-block"
          condition = {
            or = [
              {
                and = [
                  {
                    field    = "client.geo.country"
                    operator = "in"
                    value    = ["CN", "RU", "KP"]
                  }
                ]
              }
            ]
          }
          actions = { deny_access = true }
        },

        # -----------------------------------------------------------------------
        # 2. Device type — mobile vs desktop
        # -----------------------------------------------------------------------
        {
          name = "mobile-cache-key"
          condition = {
            or = [
              {
                and = [
                  {
                    field    = "client.device.is_mobile"
                    operator = "eq"
                    value    = ["true"]
                  }
                ]
              }
            ]
          }
          actions = {
            cache_key = {
              headers       = []
              cookies       = []
              query_strings = { type = "none" }
              device_type   = true
            }
          }
        },

        # -----------------------------------------------------------------------
        # 3. Request header match — field_key is the header name
        # -----------------------------------------------------------------------
        {
          name = "internal-requests"
          condition = {
            or = [
              {
                and = [
                  {
                    field     = "http.request.header"
                    field_key = "X-Internal-Request" # the header name to inspect
                    operator  = "eq"
                    value     = ["true"]
                  }
                ]
              }
            ]
          }
          actions = { cache_behavior = "BYPASS" }
        },

        # -----------------------------------------------------------------------
        # 4. Query parameter presence — field_key is the parameter name
        # -----------------------------------------------------------------------
        {
          name = "preview-mode"
          condition = {
            or = [
              {
                and = [
                  {
                    field     = "http.request.query_param"
                    field_key = "preview" # the query param to look for
                    operator  = "exists"
                    value     = []
                  }
                ]
              }
            ]
          }
          actions = { cache_behavior = "BYPASS" }
        },

        # -----------------------------------------------------------------------
        # 5. Client IP allowlist
        # -----------------------------------------------------------------------
        {
          name = "ip-allowlist"
          condition = {
            or = [
              {
                and = [
                  {
                    field    = "client.ip"
                    operator = "ip_match"
                    value    = ["203.0.113.0/24", "198.51.100.10"]
                  }
                ]
              }
            ]
          }
          actions = { cache_behavior = "BYPASS" }
        },

        # -----------------------------------------------------------------------
        # 6. HTTP method filtering
        # -----------------------------------------------------------------------
        {
          name = "bypass-cache-for-writes"
          condition = {
            or = [
              {
                and = [
                  {
                    field    = "http.request.method"
                    operator = "in"
                    value    = ["POST", "PUT", "PATCH", "DELETE"]
                  }
                ]
              }
            ]
          }
          actions = { cache_behavior = "BYPASS" }
        },

        # -----------------------------------------------------------------------
        # 7. Response status code matching
        # -----------------------------------------------------------------------
        {
          name = "short-cache-404"
          condition = {
            or = [
              {
                and = [
                  {
                    field    = "http.response.status_code"
                    operator = "eq"
                    value    = ["404"]
                  }
                ]
              }
            ]
          }
          actions = {
            status_codes_ttl = [
              {
                status_code    = "404"
                cache_behavior = "CACHE"
                cache_ttl      = 60 # cache 404s for 60 seconds only
              }
            ]
          }
        },

        # -----------------------------------------------------------------------
        # 8. Combined AND group — ALL conditions in the group must match
        #    Example: specific subdomain AND specific path prefix
        # -----------------------------------------------------------------------
        {
          name = "api-v2-on-subdomain"
          condition = {
            or = [
              {
                and = [
                  {
                    field    = "http.request.domain"
                    operator = "eq"
                    value    = ["api.example.com"]
                  },
                  {
                    field    = "http.request.path"
                    operator = "match"
                    value    = ["/v2/*"]
                  }
                ]
              }
            ]
          }
          actions = { cache_ttl = 0 }
        },

        # -----------------------------------------------------------------------
        # 9. OR groups — request matches if ANY group matches
        #    Example: mobile clients OR clients from Latin America
        # -----------------------------------------------------------------------
        {
          name = "mobile-or-latam"
          condition = {
            or = [
              # Group A: mobile clients
              {
                and = [
                  {
                    field    = "client.device.is_mobile"
                    operator = "eq"
                    value    = ["true"]
                  }
                ]
              },
              # Group B: Latin American countries
              {
                and = [
                  {
                    field    = "client.geo.country"
                    operator = "in"
                    value    = ["BR", "MX", "AR", "CO", "CL", "PE"]
                  }
                ]
              }
            ]
          }
          actions = { browser_cache_ttl = 300 }
        },

        # -----------------------------------------------------------------------
        # 10. Complex example — specific country AND specific header value
        # -----------------------------------------------------------------------
        {
          name = "eu-premium-users"
          condition = {
            or = [
              {
                and = [
                  {
                    field    = "client.geo.country"
                    operator = "in"
                    value    = ["DE", "FR", "NL", "GB"]
                  },
                  {
                    field     = "http.request.header"
                    field_key = "X-User-Tier"
                    operator  = "eq"
                    value     = ["premium"]
                  }
                ]
              }
            ]
          }
          actions = {
            cache_ttl         = 0
            browser_cache_ttl = 0
          }
        }

      ] # end behaviors.custom
    }   # end behaviors
  }
}
