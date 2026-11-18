---
page_title: "ioriver_tag Resource - terraform-provider-ioriver"
subcategory: ""
description: |-
  Manages an account tag and its service memberships.
---

# ioriver_tag (Resource)

Manages an account tag and its complete set of service memberships visible to the API token. Membership changes are applied individually.

## Example Usage

```terraform
resource "ioriver_tag" "production" {
  name     = "production"
  color    = "red"
  services = [ioriver_service.example.id]
}
```

## Import

Import a tag by its UUID:

```shell
terraform import ioriver_tag.production <tag-id>
```

## Schema

### Required

- `name` (String) Tag name, unique within the account.

### Optional

- `color` (String) Tag color: `green`, `blue`, `purple`, `orange`, `red`, `turquoise`, or `gray`. Defaults to `gray`.
- `services` (Set of String) Authoritative set of service UUIDs for this tag within the API token's visibility. Omitting the attribute or setting an empty set detaches the tag from all visible services.

### Read-Only

- `id` (String) Tag identifier.
