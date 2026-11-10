# Log destinations route traffic or security logs to external storage.
# Declare destinations in config.log_destinations, then reference them by
# name in behaviors.actions.stream_logs (traffic) or security.logs_shipping.
#
# Supported types: aws_s3 | compatible_s3 | splunk
# Supported file formats: json-list | json-object | json-line-delimited | csv
#
# Credentials are write-only — they are sent to the backend but never
# stored in Terraform state or returned in plan output.
# Required credentials may be unknown during planning, but must resolve to
# non-empty values at apply. This also applies to private S3 origin credentials.
# Source defaults to traffic. Changing source replaces only that destination
# (new UUID), keeps the service, and requires supplying credentials again.
# Changing destination type also requires credentials for the new type,
# independently of credentials_version.
# Omit selected_fields to use backend defaults based on source and file_format.
# Configured lists must be nonempty and include timestamp, service_id and
# service_uid. Header names must still
# be valid tokens of at most 128 characters, with at most 20 headers combined.

resource "ioriver_service" "log_destinations_example" {
  name         = "log-destinations-service"
  certificates = [ioriver_certificate.cert.id]

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

    log_destinations = [

      # -----------------------------------------------------------------------
      # 1. AWS S3 — IAM access key authentication
      # -----------------------------------------------------------------------
      {
        name                   = "s3-access-key"
        source                 = "traffic"
        file_format            = "json-list" # json-list | json-object | json-line-delimited | csv
        anonymize_ip           = false       # set true to remove the last IP octet for privacy
        selected_fields        = ["timestamp", "service_id", "service_uid", "client_ip", "request_headers", "response_headers"]
        request_headers        = ["x-request-id"]
        response_headers       = ["content-type"]
        timestamp_format       = "iso8601"
        object_name            = "/traffic/%Y-%m-%d/{random}.gz"
        max_samples_per_object = 100000
        max_gap_seconds        = 60

        aws_s3 = {
          name   = "my-logs-bucket" # S3 bucket name
          path   = "/cdn-logs"      # key prefix inside the bucket
          region = "us-east-1"

          # Increment credentials_version to push new credentials to the backend.
          # Credentials are only sent when this value changes vs the prior state.
          # Start at 1 on create; bump to 2, 3, … to rotate.
          credentials_version = 1
          credentials = { # write-only — not stored in Terraform state
            access_key = {
              access_key = "AKIXXXXXXXXXXXXXXPLE"
              secret_key = "wJaXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXKEY"
            }
          }
        }
      },

      # -----------------------------------------------------------------------
      # 2. AWS S3 — IAM assume-role authentication
      #    Preferred in cross-account or role-based setups.
      # -----------------------------------------------------------------------
      {
        name        = "s3-assume-role"
        file_format = "json-line-delimited"

        aws_s3 = {
          name   = "my-logs-bucket"
          path   = "/cdn-logs-role"
          region = "eu-west-1"

          credentials_version = 1
          credentials = {
            assume_role = {
              role_arn    = "arn:aws:iam::123456789012:role/IORiverLogsRole"
              external_id = "ioriver-external-id"
            }
          }
        }
      },

      # -----------------------------------------------------------------------
      # 3. S3-compatible endpoint (Cloudflare R2, MinIO, Hydrolix, etc.)
      # -----------------------------------------------------------------------
      {
        name         = "r2-compatible"
        file_format  = "csv"
        anonymize_ip = true # mask last IP octet for GDPR compliance

        compatible_s3 = {
          name   = "my-r2-bucket"
          path   = "/logs"
          region = "auto"
          domain = "https://<ACCOUNT_ID>.r2.cloudflarestorage.com"

          credentials_version = 1
          credentials = {
            access_key = {
              access_key = "r2-access-key-id"
              secret_key = "r2-secret-access-key"
            }
          }
        }
      },
      {
        name            = "security-splunk"
        source          = "security"
        file_format     = "json-line-delimited"
        selected_fields = ["timestamp", "service_id", "service_uid", "client_ip", "action", "severity_level", "ja4"]
        splunk = {
          url          = "https://splunk.example.com:8088/services/collector/raw"
          channel_id   = "00000000-0000-4000-8000-000000000001"
          use_insecure = false
          # Increment credentials_version to rotate the token.
          # New destinations (including type changes to Splunk) always require a token,
          # even when credentials_version is omitted.
          credentials_version = 1
          credentials         = { token = "splunk-hec-token" }
        }
      }

    ]

    security = {
      enabled = true
      logs_shipping = {
        destination                    = "security-splunk"
        enabled                        = true
        severity_min_threshold         = "medium"
        allowed_requests_sampling_rate = 10
      }
    }

    # Wire destinations to request paths with the stream_logs action
    behaviors = {
      custom = [
        {
          name         = "log-api-requests"
          path_pattern = "/api/*"
          actions = {
            stream_logs = {
              log_destination   = "s3-access-key" # must match a log_destinations name
              log_sampling_rate = 100             # stream 100 % of matching requests
            }
          }
        },
        {
          name         = "sample-homepage-logs"
          path_pattern = "/"
          actions = {
            stream_logs = {
              log_destination   = "r2-compatible"
              log_sampling_rate = 10 # sample 10 % of homepage requests
            }
          }
        }
      ] # end behaviors.custom
    }   # end behaviors
  }
}
