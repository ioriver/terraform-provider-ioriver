# Geo-fencing — restrict access by viewer country.
#
# The block is fully optional. Omit it entirely to apply no restriction.
# When present:
#   - `mode`      is required and must be `allow` (allow-list) or `deny` (deny-list).
#   - `countries` is a set of ISO 3166-1 alpha-2 country codes; defaults to `[]`
#                 when omitted. Backend caps the set at 20 entries.

# Example 1 — deny-list: block traffic from a small set of countries.
resource "ioriver_service" "geo_deny_example" {
  name        = "geo-deny-service"
  certificate = ioriver_certificate.cert.id

  config = {
    geo_fencing = {
      mode      = "deny"
      countries = ["US", "CA", "GB", "DE", "FR"]
    }
  }
}

# Example 2 — allow-list: only serve traffic from a specific set of countries.
resource "ioriver_service" "geo_allow_example" {
  name        = "geo-allow-service"
  certificate = ioriver_certificate.cert.id

  config = {
    geo_fencing = {
      mode      = "allow"
      countries = ["US", "CA", "GB", "DE", "FR"]
    }
  }
}
