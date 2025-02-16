// example 1 - simple origin
resource "ioriver_origin" "example_origin" {
  service = ioriver_service.service.id
  host    = "origin.example.com"
}


// example 2 - s3 bucket origin
resource "ioriver_origin" "example_origin_s3" {
  service = ioriver_service.service.id
  host    = "example.s3.us-east-1.amazonaws.com"
  is_s3   = true
}

// example 3 - private s3 bucket origin
resource "ioriver_origin" "example_origin_private_s3" {
  service = ioriver_service.service.id
  host    = "example.s3.us-east-1.amazonaws.com"
  is_s3   = true
  private_s3 = {
    bucket_name   = "example"
    bucket_region = "us-east-1"
    credentials = {
      access_key = "your_bucket_access_key"
      secret_key = "your_bucket_secret_key"
    }
  }
}
