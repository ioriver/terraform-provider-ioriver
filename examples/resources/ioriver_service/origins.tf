# All origin types and their configuration options.

# ---------------------------------------------------------------------------
# 1. Simple HTTPS custom origin
# ---------------------------------------------------------------------------
resource "ioriver_service" "custom_origin_simple" {
  name        = "custom-origin-simple"
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
        domain   = "cdn.example.com"
        mappings = [{ target_mapping = "my-origin" }]
      }
    ]
  }
}

# ---------------------------------------------------------------------------
# 2. Custom origin with all optional fields
#    - path:              prefix appended to every origin request
#    - verify_ssl:        verify the origin TLS certificate
#    - timeout_ms:        connection timeout in milliseconds
#    - sni_hostname:      TLS SNI hostname (when different from host)
#    - custom_http_port:  non-standard HTTP port
#    - custom_https_port: non-standard HTTPS port
# ---------------------------------------------------------------------------
resource "ioriver_service" "custom_origin_full" {
  name        = "custom-origin-full"
  certificate = ioriver_certificate.cert.id

  config = {
    origins = [
      {
        name         = "full-origin"
        path         = "/api"                # prepend /api to all origin requests
        verify_ssl   = true                  # validate origin TLS certificate
        timeout_ms   = 30000                 # 30 s connection timeout
        sni_hostname = "backend.example.com" # SNI hostname for TLS handshake

        custom_origin = {
          host              = "backend.example.com"
          protocol          = "http_and_https" # http | https | http_and_https
          custom_http_port  = 8080
          custom_https_port = 8443
        }
      }
    ]
    domains = [
      {
        domain   = "cdn.example.com"
        mappings = [{ target_mapping = "full-origin" }]
      }
    ]
  }
}

# ---------------------------------------------------------------------------
# 3. Public S3 bucket origin
# ---------------------------------------------------------------------------
resource "ioriver_service" "s3_origin_public" {
  name        = "s3-origin-public"
  certificate = ioriver_certificate.cert.id

  config = {
    origins = [
      {
        name = "s3-public"
        s3_origin = {
          host              = "my-bucket.s3.us-east-1.amazonaws.com"
          is_static_website = false
          is_private        = false
        }
      }
    ]
    domains = [
      {
        domain   = "assets.example.com"
        mappings = [{ target_mapping = "s3-public" }]
      }
    ]
  }
}

# ---------------------------------------------------------------------------
# 4. Private S3 bucket origin (IAM-authenticated)
#    s3_aws_region and s3_bucket_name are required when is_private = true.
# ---------------------------------------------------------------------------
resource "ioriver_service" "s3_origin_private" {
  name        = "s3-origin-private"
  certificate = ioriver_certificate.cert.id

  config = {
    origins = [
      {
        name = "s3-private"
        s3_origin = {
          host              = "my-private-bucket.s3.us-east-1.amazonaws.com"
          is_static_website = false
          is_private        = true
          s3_aws_region     = "us-east-1"
          s3_bucket_name    = "my-private-bucket"

          # Increment credentials_version to push new credentials to the backend.
          # Credentials are only sent when this value changes vs the prior state.
          # Start at 1 on create; bump to 2, 3, … to rotate.
          credentials_version = 1
          s3_aws_key          = "AKIAIOSFODNN7EXAMPLE"
          s3_aws_secret       = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
        }
      }
    ]
    domains = [
      {
        domain   = "secure-assets.example.com"
        mappings = [{ target_mapping = "s3-private" }]
      }
    ]
  }
}

# ---------------------------------------------------------------------------
# 5. Origin with shield
#    Shield collapses requests at a chosen PoP before they reach the origin,
#    reducing origin load. Specify which CDN providers should use the shield.
# ---------------------------------------------------------------------------
resource "ioriver_service" "origin_with_shield" {
  name        = "origin-with-shield"
  certificate = ioriver_certificate.cert.id

  config = {
    origins = [
      {
        name = "shielded-origin"
        custom_origin = {
          host     = "origin.example.com"
          protocol = "https"
        }
        shield = {
          location = {
            country     = "US"
            subdivision = "VA"
          }
          providers = ["fastly", "cloudflare"]
        }
      }
    ]
    domains = [
      {
        domain   = "cdn.example.com"
        mappings = [{ target_mapping = "shielded-origin" }]
      }
    ]
  }
}
