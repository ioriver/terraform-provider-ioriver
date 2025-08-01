---
# generated by https://github.com/hashicorp/terraform-plugin-docs
page_title: "ioriver_health_monitor Resource - terraform-provider-ioriver"
subcategory: ""
description: |-
  HealthMonitor resource
---

# ioriver_health_monitor (Resource)

HealthMonitor resource

## Example Usage

```terraform
resource "ioriver_health_monitor" "availability_monitor" {
  service = ioriver_service.service.id
  name    = "availability"
  url     = "https://domain.example.com/ping"
}
```

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `name` (String) Health monitor name
- `service` (String) The id of the service this monitor belongs to
- `url` (String) URL to monitor

### Optional

- `enabled` (Boolean) Health monitor port

### Read-Only

- `id` (String) HealthMonitor identifier

## Import

Import is supported using the following syntax:

The [`terraform import` command](https://developer.hashicorp.com/terraform/cli/commands/import) can be used, for example:

```shell
# Health monitor can be imported by specifying service-id,health-monitor-id
terraform import ioriver_health_monitor.example "32489068-0ad6-4823-8c5d-9f4e4c458f93,813d91ff-c2f1-489e-999b-af7f35d73d03"
```
