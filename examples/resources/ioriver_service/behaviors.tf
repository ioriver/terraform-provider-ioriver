# Comprehensive reference for ALL specific behavior actions.
# Each named behavior below demonstrates a distinct group of actions.
# In practice a single behavior can combine multiple actions freely.
#
# Behaviors are matched top-to-bottom; the first matching behavior wins.
# Use path_pattern for simple glob matching, or the condition block for
# advanced matching (see behaviors_conditions.tf).

resource "ioriver_service" "all_actions" {
  name        = "all-actions-reference"
  certificate = ioriver_certificate.cert.id

  config = {
    behaviors = {
      custom = [

        # =======================================================================
        # CACHING
        # =======================================================================

        # Cache TTL, browser cache TTL, stale TTL
        {
          name         = "caching-ttls"
          path_pattern = "/cacheable/*"
          actions = {
            cache_behavior    = "CACHE" # CACHE | BYPASS
            cache_ttl         = 86400   # 24-hour edge cache (seconds)
            browser_cache_ttl = 3600    # 1-hour browser cache (seconds)
            stale_ttl         = 600     # serve stale up to 10 min on origin errors
          }
        },

        # Bypass the cache entirely
        {
          name         = "bypass-cache"
          path_pattern = "/no-cache/*"
          actions = {
            cache_behavior = "BYPASS"
          }
        },

        # Honor the origin's Cache-Control response header
        {
          name         = "origin-cache-control"
          path_pattern = "/honor-origin/*"
          actions = {
            origin_cache_control = true
          }
        },

        # Cache responses for specific HTTP methods (beyond GET/HEAD)
        {
          name         = "cached-methods"
          path_pattern = "/post-cacheable/*"
          actions = {
            cached_methods = [
              { method = "GET" },
              { method = "HEAD" },
              { method = "POST" }
            ]
          }
        },

        # Different cache TTLs per HTTP status code
        {
          name         = "status-code-ttls"
          path_pattern = "/per-status/*"
          actions = {
            status_codes_ttl = [
              {
                status_code    = "200"
                cache_behavior = "CACHE"
                cache_ttl      = 3600
              },
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
          }
        },

        # Browser cache TTL override per status code (list — one entry per code / range)
        {
          name         = "status-code-browser-cache"
          path_pattern = "/browser-cache/*"
          actions = {
            status_code_browser_cache = [
              {
                status_code = "200"
                cache_ttl   = 7200
              }
            ]
          }
        },

        # Custom cache key — vary on specific headers, cookies, and query params
        {
          name         = "custom-cache-key"
          path_pattern = "/vary/*"
          actions = {
            cache_key = {
              headers = [
                { header = "Accept-Language" },
                { header = "X-Country" }
              ]
              cookies = [
                { cookie = "session_id" }
              ]
              query_strings = {
                type = "include" # include | exclude | all | none
                params = [
                  { param = "color" },
                  { param = "size" }
                ]
              }
              country     = true # separate cache entry per client country
              device_type = true # separate cache entry for mobile vs desktop
            }
          }
        },

        # Large-file optimisation — segments files >50 MB inside the cache
        {
          name         = "large-files"
          path_pattern = "/downloads/*"
          actions = {
            large_files_optimization = true
            cache_ttl                = 604800 # 7-day TTL for large files
          }
        },

        # Gzip / Brotli compression
        {
          name         = "compression"
          path_pattern = "/text/*"
          actions = {
            compression = true
          }
        },

        # =======================================================================
        # HEADERS
        # =======================================================================

        # Add / override request headers sent to the origin.
        # action: "set" replaces, "add" appends, "delete" removes the header.
        {
          name         = "request-headers"
          path_pattern = "/with-request-headers/*"
          actions = {
            request_headers = [
              {
                name   = "X-CDN-Origin"
                values = ["io-river"]
                action = "set"
              },
              {
                name   = "Cookie"
                action = "delete" # strip Cookie before forwarding to origin
              }
            ]
          }
        },

        # Add / override response headers sent to the client.
        {
          name         = "response-headers"
          path_pattern = "/with-response-headers/*"
          actions = {
            response_headers = [
              {
                name   = "Strict-Transport-Security"
                values = ["max-age=31536000; includeSubDomains"]
                action = "set"
              },
              {
                name   = "X-Powered-By"
                action = "delete" # remove the header entirely
              }
            ]
          }
        },

        # Modify headers on the origin→CDN response (before caching).
        {
          name         = "origin-response-headers"
          path_pattern = "/strip-internal-headers/*"
          actions = {
            origin_response_headers = [
              {
                name   = "X-Internal-Debug"
                action = "delete"
              }
            ]
          }
        },

        # Override the Host header sent to the origin with a static value
        {
          name         = "host-header-static"
          path_pattern = "/override-host/*"
          actions = {
            host_header = {
              header_value    = "backend.example.com"
              use_origin_host = false
            }
          }
        },

        # Use the origin's own domain as the Host header (useful for S3 / virtual hosting)
        {
          name         = "host-header-use-origin"
          path_pattern = "/use-origin-host/*"
          actions = {
            host_header = {
              use_origin_host = true
            }
          }
        },

        # Inject a True-Client-IP header containing the end-user's real IP
        {
          name         = "true-client-ip"
          path_pattern = "/real-ip/*"
          actions = {
            true_client_ip = true
          }
        },

        # =======================================================================
        # PROTOCOL & VIEWER SECURITY
        # =======================================================================

        # Require HTTPS; respond with 403 to plain HTTP requests
        {
          name         = "https-only"
          path_pattern = "/secure/*"
          actions = {
            viewer_protocol = "HTTPS_ONLY" # HTTPS_ONLY | HTTP_AND_HTTPS | REDIRECT_HTTP_TO_HTTPS
          }
        },

        # Redirect HTTP requests to their HTTPS equivalent
        {
          name         = "redirect-http-to-https"
          path_pattern = "/redirect/*"
          actions = {
            viewer_protocol = "REDIRECT_HTTP_TO_HTTPS"
          }
        },

        # Verify signed-URL tokens before serving content
        {
          name         = "url-signing"
          path_pattern = "/protected/*"
          actions = {
            url_signing = true # requires an ioriver_url_signing_key resource
          }
        },

        # =======================================================================
        # ACCESS CONTROL
        # =======================================================================

        # Block all access to a path (returns 403)
        {
          name         = "deny-all"
          path_pattern = "/internal/*"
          actions = {
            deny_access = true
          }
        },

        # Allow access only from specific IPs / CIDR blocks; deny everyone else
        {
          name         = "allow-from-ip"
          path_pattern = "/admin/*"
          actions = {
            allow_access_only_from_ip = [
              { ip = "203.0.113.10" },
              { ip = "198.51.100.0/24" }
            ]
          }
        },

        # Deny access from specific IPs / CIDR blocks
        {
          name         = "deny-by-ip"
          path_pattern = "/no-vpn/*"
          actions = {
            deny_access_by_ip = [
              { ip = "192.0.2.1" },
              { ip = "10.0.0.0/8" }
            ]
          }
        },

        # Block access during a fixed one-time window (Unix timestamps, UTC)
        {
          name         = "deny-by-time-fixed"
          path_pattern = "/maintenance/*"
          actions = {
            deny_access_by_time = [
              {
                date_time_window = {
                  start_date = 1735689600 # 2025-01-01 00:00 UTC
                  end_date   = 1735776000 # 2025-01-02 00:00 UTC
                }
              }
            ]
          }
        },

        # Block access on a recurring schedule
        {
          name         = "deny-by-time-recurring"
          path_pattern = "/weekend-maintenance/*"
          actions = {
            deny_access_by_time = [
              {
                time_periodic = {
                  start_date          = 1735689600 # first occurrence start (Unix UTC)
                  duration            = 48         # deny window length
                  duration_units      = "h"        # s | m | h | d
                  repeat_period       = 7          # repeat every 7 days
                  repeat_period_units = "d"        # s | m | h | d
                }
              }
            ]
          }
        },

        # =======================================================================
        # REDIRECTS & URL REWRITING
        # =======================================================================

        # Static redirect — send all matched requests to a fixed URL
        {
          name         = "static-redirect"
          path_pattern = "/old-page"
          actions = {
            redirect = {
              destination = "https://www.example.com/new-page"
            }
          }
        },

        # Dynamic redirect — capture group from source pattern → destination
        {
          name         = "dynamic-redirect"
          path_pattern = "/legacy/*"
          actions = {
            redirect = {
              source      = "/legacy/(.*)"
              destination = "https://www.example.com/$1"
            }
          }
        },

        # Static URL rewrite (no client redirect — only changes path to origin)
        {
          name         = "url-rewrite-static"
          path_pattern = "/app"
          actions = {
            url_rewrites = [
              {
                source      = "/app"
                destination = "/index.html"
              }
            ]
          }
        },

        # Regex URL rewrite — strip a path prefix before forwarding to origin
        {
          name         = "url-rewrite-regex"
          path_pattern = "/static/*"
          actions = {
            url_rewrites = [
              {
                source      = "/static/(.*)"
                destination = "/assets/$1"
              }
            ]
          }
        },

        # CDN follows origin 3xx redirects (Location host must be a known origin)
        {
          name         = "follow-redirects"
          path_pattern = "/follow/*"
          actions = {
            follow_redirects = true
          }
        },

        # =======================================================================
        # RESPONSE GENERATION
        # =======================================================================

        # Return a custom response page for specific status code(s)
        # (list — one entry per status code or range, response_url is the page path / URL)
        {
          name         = "generate-response"
          path_pattern = "/not-found/*"
          actions = {
            generate_response = [
              {
                status_code  = "404"
                response_url = "/404.html"
              }
            ]
          }
        },

        # Restrict which HTTP methods the CDN will accept
        {
          name         = "allowed-methods"
          path_pattern = "/api/*"
          actions = {
            allowed_methods = [
              { method = "GET" },
              { method = "HEAD" },
              { method = "POST" },
              { method = "PUT" },
              { method = "PATCH" },
              { method = "DELETE" },
              { method = "OPTIONS" }
            ]
          }
        },

        # =======================================================================
        # CORS
        # =======================================================================

        # CORS — allow all origins, all headers, all methods
        {
          name         = "cors-allow-all"
          path_pattern = "/public-api/*"
          actions = {
            cors = {
              allow_origin = {
                mode     = "all" # all | specific | from_request
                override = true
              }
              allow_headers = {
                mode     = "all"
                override = true
              }
              allow_methods = {
                mode     = "all"
                override = true
              }
              allow_credentials = true
              max_age = {
                value    = 86400
                override = true
              }
            }
          }
        },

        # CORS — allow only specific origins, headers, exposed headers, and methods
        {
          name         = "cors-specific"
          path_pattern = "/restricted-api/*"
          actions = {
            cors = {
              allow_origin = {
                mode     = "specific"
                origins  = ["https://app.example.com", "https://admin.example.com"]
                override = true
              }
              allow_headers = {
                mode     = "specific"
                values   = ["Content-Type", "Authorization", "X-Request-ID"]
                override = true
              }
              expose_headers = {
                mode     = "specific"
                values   = ["X-RateLimit-Limit", "X-RateLimit-Remaining"]
                override = true
              }
              allow_methods = {
                mode     = "specific"
                values   = ["GET", "POST", "OPTIONS"]
                override = true
              }
              allow_credentials = true
              max_age = {
                value    = 3600
                override = true
              }
            }
          }
        },

        # CORS — mirror the request's Origin header back (useful for CDN caching + CORS)
        {
          name         = "cors-from-request"
          path_pattern = "/mirror-origin/*"
          actions = {
            cors = {
              allow_origin = {
                mode     = "from_request"
                override = true
              }
            }
          }
        },

        # Preflight — respond to OPTIONS requests from the CDN edge (no origin round-trip)
        {
          name         = "cors-preflight"
          path_pattern = "/preflight-api/*"
          actions = {
            generate_preflight_response = {
              allowed_methods = [
                { method = "GET" },
                { method = "POST" },
                { method = "OPTIONS" }
              ]
              allowed_headers = ["Content-Type", "Authorization"] # optional
              max_age         = 3600
            }
          }
        },

        # =======================================================================
        # LOG STREAMING
        # =======================================================================

        # Stream CDN access logs for matching requests to a configured destination.
        # The log_destination name must match an entry in config.log_destinations.
        {
          name         = "stream-logs"
          path_pattern = "/logged/*"
          actions = {
            stream_logs = {
              log_destination   = "my-s3-destination" # must match a log_destinations name
              log_sampling_rate = 100                 # 1–100 (percent of requests to stream)
            }
          }
        }

      ] # end behaviors.custom
    }   # end behaviors
  }
}
