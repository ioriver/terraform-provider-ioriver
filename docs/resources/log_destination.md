---
# generated by https://github.com/hashicorp/terraform-plugin-docs
page_title: "ioriver_log_destination Resource - terraform-provider-ioriver"
subcategory: ""
description: |-
  Log Destination resource
---

# ioriver_log_destination (Resource)

Log Destination resource

## Example Usage

```terraform
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
```

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `name` (String) Log destination name
- `service` (String) The id of the service this log destination belongs to

### Optional

- `aws_s3` (Attributes) Properties of AWS S3 bucket log destination (see [below for nested schema](#nestedatt--aws_s3))
- `compatible_s3` (Attributes) Properties of S3 compatible bucket log destination (see [below for nested schema](#nestedatt--compatible_s3))

### Read-Only

- `id` (String) Log Destination identifier

<a id="nestedatt--aws_s3"></a>
### Nested Schema for `aws_s3`

Required:

- `credentials` (Attributes) Either AWS role or access-key credentials (see [below for nested schema](#nestedatt--aws_s3--credentials))
- `name` (String) Name of the bucket
- `region` (String) Bucket region

Optional:

- `path` (String) The path in the bucket where the logs will be written

<a id="nestedatt--aws_s3--credentials"></a>
### Nested Schema for `aws_s3.credentials`

Optional:

- `access_key` (Attributes) AWS access-key credentials (see [below for nested schema](#nestedatt--aws_s3--credentials--access_key))
- `assume_role` (Attributes) AWS role credentials (see [below for nested schema](#nestedatt--aws_s3--credentials--assume_role))

<a id="nestedatt--aws_s3--credentials--access_key"></a>
### Nested Schema for `aws_s3.credentials.access_key`

Required:

- `access_key` (String, Sensitive) AWS access-key ID
- `secret_key` (String, Sensitive) AWS access-key secret


<a id="nestedatt--aws_s3--credentials--assume_role"></a>
### Nested Schema for `aws_s3.credentials.assume_role`

Required:

- `external_id` (String) AWS role external ID
- `role_arn` (String) AWS role ARN




<a id="nestedatt--compatible_s3"></a>
### Nested Schema for `compatible_s3`

Required:

- `credentials` (Attributes) Access-key credentials (see [below for nested schema](#nestedatt--compatible_s3--credentials))
- `domain` (String) Domain of the bucket
- `name` (String) Name of the bucket
- `region` (String) Bucket region

Optional:

- `path` (String) The path in the bucket where the logs will be written

<a id="nestedatt--compatible_s3--credentials"></a>
### Nested Schema for `compatible_s3.credentials`

Required:

- `access_key` (String, Sensitive) Access-key ID
- `secret_key` (String, Sensitive) Access-key secret

## Import

Import is supported using the following syntax:

```shell
# Log destination can be imported by specifying service-id,log-destination-id
terraform import ioriver_log_destination.example "32489068-0ad6-4823-8c5d-9f4e4c458f93,813d91ff-c2f1-489e-999b-af7f35d73d03"
```