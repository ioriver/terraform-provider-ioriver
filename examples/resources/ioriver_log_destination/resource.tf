// example 1 - AWS S3 log destination using IAM assume-role authentication
resource "ioriver_log_destination" "s3_assume_role" {
  service = ioriver_service.service.id
  name    = "cdn-logs"

  aws_s3 = {
    name   = "example-bucket"
    path   = "/logs"
    region = "us-east-1"

    credentials = {
      assume_role = {
        role_arn    = "arn:aws:iam::123456789012:role/IORiverLogsRole"
        external_id = "ioriver-unique-id"
      }
    }
  }
}

// example 2 - AWS S3 log destination using IAM access key authentication
resource "ioriver_log_destination" "s3_access_key" {
  service = ioriver_service.service.id
  name    = "cdn-logs-key"

  aws_s3 = {
    name   = "example-bucket"
    path   = "/logs"
    region = "us-east-1"

    credentials = {
      access_key = {
        access_key = "AKIXXXXXXXXXXXXXXPLE"
        secret_key = "wJaXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXKEY"
      }
    }
  }
}

// example 3 - S3-compatible bucket log destination (e.g. Cloudflare R2, MinIO, Wasabi)
resource "ioriver_log_destination" "compatible_s3" {
  service = ioriver_service.service.id
  name    = "cdn-logs-r2"

  compatible_s3 = {
    name   = "example-compatible-bucket"
    path   = "/logs"
    region = "auto"
    domain = "https://<ACCOUNT_ID>.r2.cloudflarestorage.com"

    credentials = {
      access_key = {
        access_key = "r2-access-key-id"
        secret_key = "r2-secret-access-key"
      }
    }
  }
}
