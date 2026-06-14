# Protocol configuration — HTTP/2, HTTP/3 (QUIC), and IPv6.
# All three flags are optional and computed. Omitting them lets the backend
# apply its own defaults. Set them explicitly to lock the values in state.

resource "ioriver_service" "protocol_example" {
  name        = "protocol-service"
  certificate = ioriver_certificate.cert.id

  config = {
    protocol = {
      http2_enabled = true # enable HTTP/2 multiplexing (recommended)
      http3_enabled = true # enable HTTP/3 / QUIC
      ipv6_enabled  = true # respond on IPv6 addresses
    }
  }
}
