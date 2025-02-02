resource "ioriver_protocol_config" "example_protocol_config" {
  service       = ioriver_service.service.id
  http2_enabled = true
  http3_enabled = true
  ipv6_enabled  = false
}
