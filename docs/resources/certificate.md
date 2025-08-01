---
# generated by https://github.com/hashicorp/terraform-plugin-docs
page_title: "ioriver_certificate Resource - terraform-provider-ioriver"
subcategory: ""
description: |-
  Certificate resource
---

# ioriver_certificate (Resource)

Certificate resource

## Example Usage

```terraform
resource "ioriver_certificate" "self_managed_cert" {
  name              = "example-cert"
  type              = "SELF_MANAGED"
  certificate       = file("certificate.crt")
  private_key       = file("private.key")
  certificate_chain = file("ca_bundle.crt")
}

resource "ioriver_certificate" "managed_cert" {
  name = "example-managed-cert"
  type = "MANAGED"
  cn   = "[\"domain.example.com\"]"
}
```

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `name` (String) Certificate name
- `type` (String) Certificate type (MANAGED/SELF_MANAGED/EXTERNAL)

### Optional

- `certificate` (String, Sensitive) Certificate content
- `certificate_chain` (String, Sensitive) Certificate chain
- `cn` (String) Certificate CN
- `private_key` (String, Sensitive) Certificate private key
- `providers_certificates` (Attributes Set) Details of the certificate as it is deployed on each provider. This field is required only for EXTERNAL certificates. (see [below for nested schema](#nestedatt--providers_certificates))

### Read-Only

- `challenges` (String) Required DNS challenges
- `id` (String) Certificate identifier
- `not_valid_after` (String) Certificate expiration date
- `status` (String) Certificate status

<a id="nestedatt--providers_certificates"></a>
### Nested Schema for `providers_certificates`

Required:

- `account_provider` (String) The account provider of the provider certificate
- `provider_certificate_id` (String) The id of the certificate within the provider:
							aws - The certificate arn
							fastly - key and certificate ids in the json format: {"private_key_id": "", "certificate_id": ""}

Read-Only:

- `not_valid_after` (String) Certificate expiration date

## Import

Import is supported using the following syntax:

The [`terraform import` command](https://developer.hashicorp.com/terraform/cli/commands/import) can be used, for example:

```shell
# Certificate can be imported by specifying the certificate-id
terraform import ioriver_certificate.example "32489068-0ad6-4823-8c5d-9f4e4c458f93"
```
