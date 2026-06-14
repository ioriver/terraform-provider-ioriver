# Full end-to-end example tying all features together:
#   - Two origins (HTTPS custom + private S3)
#   - Two CDN domains with path-based routing
#   - Protocol configuration
#   - Explicitly configured default behavior
#   - Multiple specific behaviors: caching, headers, CORS, redirects, access control
#   - A log destination with streaming
#   - WAF with a custom block rule and rate limiting

locals {
  api_origin_name    = "api-origin"
  assets_origin_name = "assets-origin"
  log_dest_name      = "s3-logs"
}

resource "ioriver_service" "full_example" {
  name        = "full-example-service"
  description = "Complete IO River CDN service configuration"
  certificate = ioriver_certificate.cert.id

  config = {

    # -------------------------------------------------------------------------
    # Protocol
    # -------------------------------------------------------------------------
    protocol = {
      http2_enabled = true
      http3_enabled = true
      ipv6_enabled  = true
    }

    # -------------------------------------------------------------------------
    # Origins
    # -------------------------------------------------------------------------
    origins = [
      {
        name       = local.api_origin_name
        verify_ssl = true
        custom_origin = {
          host     = "api.example.com"
          protocol = "https"
        }
      },
      {
        name = local.assets_origin_name
        s3_origin = {
          host           = "my-assets-bucket.s3.us-east-1.amazonaws.com"
          is_private     = true
          s3_aws_region  = "us-east-1"
          s3_bucket_name = "my-assets-bucket"
        }
      }
    ]

    # -------------------------------------------------------------------------
    # Domains
    # -------------------------------------------------------------------------
    domains = [
      {
        domain = "www.example.com"
        mappings = [
          {
            path_pattern   = "/api/*"
            target_mapping = local.api_origin_name
          },
          {
            path_pattern   = "/*"
            target_mapping = local.assets_origin_name
          }
        ]
      },
      {
        domain   = "cdn.example.com"
        mappings = [{ target_mapping = local.assets_origin_name }]
      }
    ]

    # -------------------------------------------------------------------------
    # Behaviors (default + specific)
    # -------------------------------------------------------------------------
    behaviors = {
      default = {
        actions = {
          viewer_protocol      = "REDIRECT_HTTP_TO_HTTPS"
          cache_behavior       = "CACHE"
          cache_ttl            = 3600
          browser_cache_ttl    = 300
          stale_ttl            = 120
          origin_cache_control = false
          compression          = true

          cache_key = {
            headers       = [{ header = "Accept-Encoding" }]
            cookies       = []
            query_strings = { type = "all" }
            country       = false
            device_type   = false
          }

          status_codes_ttl = [
            { status_code = "4xx", cache_behavior = "BYPASS", cache_ttl = 0 },
            { status_code = "5xx", cache_behavior = "BYPASS", cache_ttl = 0 }
          ]

          allowed_methods = [
            { method = "GET" },
            { method = "HEAD" },
            { method = "OPTIONS" }
          ]
        }
      } # end behaviors.default

      custom = [

        # API — bypass cache, allow all methods, inject real IP header
        {
          name         = "api-bypass-cache"
          path_pattern = "/api/*"
          actions = {
            cache_behavior = "BYPASS"
            allowed_methods = [
              { method = "GET" }, { method = "HEAD" }, { method = "POST" },
              { method = "PUT" }, { method = "PATCH" }, { method = "DELETE" },
              { method = "OPTIONS" }
            ]
            request_headers = [
              {
                name   = "X-Real-IP"
                values = ["$remote_addr"]
                action = "set"
              }
            ]
            true_client_ip = true
          }
        },

        # Static assets — long TTL, no query strings in cache key
        {
          name         = "static-assets"
          path_pattern = "/static/*"
          actions = {
            cache_ttl         = 2592000
            browser_cache_ttl = 86400
            compression       = true
            cache_key = {
              headers       = [{ header = "Accept-Encoding" }]
              cookies       = []
              query_strings = { type = "none" }
              country       = false
              device_type   = false
            }
          }
        },

        # Large downloads — segment files >50 MB in cache
        {
          name         = "large-files"
          path_pattern = "/downloads/*"
          actions = {
            cache_ttl                = 604800
            large_files_optimization = true
          }
        },

        # CORS for the public API
        {
          name         = "cors-public-api"
          path_pattern = "/api/public/*"
          actions = {
            cors = {
              allow_origin      = { mode = "all", override = true }
              allow_headers     = { mode = "all", override = true }
              allow_methods     = { mode = "all", override = true }
              allow_credentials = true
              max_age = {
                value    = 86400
                override = true
              }
            }
            generate_preflight_response = {
              allowed_methods = [
                { method = "GET" }, { method = "POST" }, { method = "OPTIONS" }
              ]
              max_age = 3600
            }
          }
        },

        # Redirect old blog URLs to new article paths
        {
          name         = "legacy-blog-redirect"
          path_pattern = "/blog/*"
          actions = {
            redirect = {
              source      = "/blog/(.*)"
              destination = "https://www.example.com/articles/$1"
            }
          }
        },

        # Require signed URLs for protected downloads
        {
          name         = "signed-downloads"
          path_pattern = "/protected/*"
          actions = {
            url_signing = true
          }
        },

        # Block internal endpoints from external clients
        {
          name = "block-internal"
          condition = {
            or = [
              {
                and = [
                  {
                    field    = "http.request.path"
                    operator = "match"
                    value    = ["/internal/*"]
                  },
                  {
                    field    = "client.ip"
                    operator = "not_ip_match"
                    value    = ["10.0.0.0/8", "172.16.0.0/12"]
                  }
                ]
              }
            ]
          }
          actions = { deny_access = true }
        },

        # Stream API logs to S3
        {
          name         = "log-api"
          path_pattern = "/api/*"
          actions = {
            stream_logs = {
              log_destination   = local.log_dest_name
              log_sampling_rate = 100
            }
          }
        }

      ] # end behaviors.custom
    }   # end behaviors

    # -------------------------------------------------------------------------
    # Log destinations
    # -------------------------------------------------------------------------
    log_destinations = [
      {
        name        = local.log_dest_name
        file_format = "json-line-delimited"

        aws_s3 = {
          name   = "my-cdn-logs-bucket"
          path   = "/access-logs"
          region = "us-east-1"
          credentials = {
            assume_role = {
              role_arn    = "arn:aws:iam::123456789012:role/IORiverLogsRole"
              external_id = "ioriver-unique-id"
            }
          }
        }
      }
    ]

    # -------------------------------------------------------------------------
    # Security (WAF + rate limiting)
    # -------------------------------------------------------------------------
    security = {
      waf = {
        enabled         = true
        limit_body_size = true
        checkpoint = {
          web_attacks = {
            mode             = "prevent"
            confidence_level = "high"
          }
          ips = {
            mode                     = "prevent"
            performance_impact       = "medium"
            severity                 = "medium"
            high_confidence_action   = "block"
            medium_confidence_action = "block"
            low_confidence_action    = "log"
          }
        }
      }

      custom = [
        {
          name    = "allow-internal-ips"
          enabled = true
          action  = "allow"
          condition = {
            or = [
              {
                and = [
                  {
                    field    = "client.ip.address"
                    operator = "ip_match"
                    value    = ["10.0.0.0/8"]
                  }
                ]
              }
            ]
          }
        }
      ]

      rate_limit = [
        {
          name                   = "protect-login"
          enabled                = true
          action                 = "block"
          num_of_requests        = 10
          time_window_seconds    = 60
          block_duration_seconds = 600
          condition = {
            or = [
              {
                and = [
                  {
                    field    = "http.request.path"
                    operator = "eq"
                    value    = ["/api/auth/login"]
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
