---
page_title: "ioriver_attached_file Resource - terraform-provider-ioriver"
subcategory: ""
description: |-
  Manages an immutable, account-scoped response attachment.
---

# ioriver_attached_file (Resource)

Uploads HTML text separately from the service that references it. Uploads occur
during apply, not planning.

## Example Usage

```terraform
resource "ioriver_attached_file" "blocked" {
  file_type = "HTML"
  contents = <<-HTML
    <!doctype html>
    <html><body>Access denied.</body></html>
  HTML

  # Alternatively, read an existing UTF-8 file:
  # contents = file("${path.module}/blocked.html")

  lifecycle {
    create_before_destroy = true
  }
}

resource "ioriver_service" "example" {
  name         = "custom-response-service"
  certificates = [ioriver_certificate.cert.id]

  config = {
    custom_responses = [{
      name             = "blocked"
      status_code      = 403
      headers          = { "content-type" = "text/html" }
      attached_file_id = ioriver_attached_file.blocked.id
    }]
  }
}
```

Defining a response does not itself configure WAF to use it.

The attachment example is self-contained. To use `file()` instead, create
`blocked.html` alongside your Terraform configuration before running Terraform.

## Lifecycle

Content changes require replacement. Outer-whitespace-only changes update state
without an API call because the backend trims outer whitespace.

Use `create_before_destroy` so services can switch IDs before deletion is attempted.
**Deletion is rejected while any service configuration version, including historical
versions, references the attachment.** Updating the active config alone may not release
it. The provider surfaces the error; replacement or destroy remains blocked until
the backend references are removed.

Uploads deduplicate within an account. Manage each attachment ID with only one
Terraform resource address and share that resource's ID between services.

## Schema

### Required

- `file_type` (String) Must be `HTML`. Changing it requires replacement.
- `contents` (String) Text contents, limited to 2 MiB of UTF-8 data and 2,000,000
  characters. Must contain non-whitespace characters. Stored in Terraform state.
  Content changes require replacement.

### Read-Only

- `id` (String) Attachment UUID.

## Import

Import retrieves canonical contents with outer whitespace trimmed.

```shell
terraform import ioriver_attached_file.blocked "697dd97d-437e-46f8-915f-1e28bab9df2c"
```
