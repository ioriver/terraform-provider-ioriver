// example 1 - aws s3 bucket log destination with stream_logs behavior with 10% sampling rate
resource "ioriver_log_destination" "logs_bucket" {
  service = ioriver_service.service.id
  name    = "cdn-logs"
  aws_s3 = {
    name   = "example-bucket"
    path   = "/logs"
    region = "us-east-1"
    credentials = {
      assume_role = {
        role_arn    = "abc"
        external_id = "123"
      }
    }
  }
}

resource "ioriver_behavior" "stream_logs" {
  service      = "%s"
  name         = "stream-logs"
  path_pattern = "/example/*"

  actions = [
    {
      stream_logs = {
        unified_log_destination   = ioriver_log_destination.logs_bucket.id
        unified_log_sampling_rate = "10"
      }
    }
  ]
}


// example 2 - s3 compatible bucket log destination
resource "ioriver_log_destination" "logs_bucket" {
  service = ioriver_service.service.id
  name    = "cdn-logs"
  compatible_s3 = {
    name   = "example-compatible-bucket"
    path   = "/logs"
    region = "eu-central-1"
    domain = "s3.eu-central-1.wasabisys.com"
    credentials = {
      access_key = "abc"
      secret_key = "123"
    }
  }
}
