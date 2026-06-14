# Minimal WAF example — enable the managed checkpoint with default settings,
# add one custom block rule, and one rate-limit on the login endpoint.

resource "ioriver_service" "waf_simple" {
  name        = "waf-simple"
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

    security = {
      enabled = true # master WAF on/off switch

      # waf can be omitted entirely — the checkpoint defaults fire automatically.
      # Spell it out here only when you want non-default values.
      waf = {
        limit_body_size = false

        checkpoint = {
          web_attacks = {
            mode             = "learn" # learn | prevent | disabled
            confidence_level = "high"  # high | medium | critical
          }
          ips = {
            mode                   = "learn"
            performance_impact     = "medium" # low | medium | high
            high_confidence_action = "block"  # block | log
            low_confidence_action  = "log"
          }
          minimal_num_sources = 3
        }
      }

      # Block requests to the admin panel from all IPs except trusted ones.
      # Rules are evaluated in order — first match wins.
      custom_rules = [
        {
          name    = "block-admin"
          enabled = true
          action  = "block" # block | log | allow | bypass_managed | challenge | interactive_challenge | ignore
          condition = {
            or = [
              {
                and = [
                  {
                    field    = "http.request.path"
                    operator = "begins_with" # see security_waf_full.tf for all operators
                    value    = ["/admin"]
                  }
                ]
              }
            ]
          }
        }
      ]

      # Block the login endpoint if a client fires more than 5 requests in 10 s;
      # keep them blocked for 5 minutes.
      rate_limit = [
        {
          name                   = "rate-limit-login"
          enabled                = true
          action                 = "block" # block | log | challenge | interactive_challenge
          num_of_requests        = 5
          time_window_seconds    = 10
          block_duration_seconds = 300
          condition = {
            or = [
              {
                and = [
                  {
                    field    = "http.request.path"
                    operator = "eq"
                    value    = ["/login"]
                  }
                ]
              }
            ]
          }
        }
      ]
    }
  }
}
