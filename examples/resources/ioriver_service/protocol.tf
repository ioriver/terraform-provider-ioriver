# Protocol configuration: HTTP/2, HTTP/3 (QUIC), IPv6, and client mTLS.
# The mTLS fields remain null when omitted. The trust-store CA bundle is
# separate from the server certificate.

resource "ioriver_client_mtls_trust_store" "clients" {
  name   = "trusted-clients"
  bundle = file("${path.module}/certs/client-ca-bundle.pem")

  lifecycle {
    create_before_destroy = true
  }
}

resource "ioriver_service" "protocol_example" {
  name         = "protocol-service"
  certificates = [ioriver_certificate.cert.id]

  config = {
    protocol = {
      http2_enabled = true # enable HTTP/2 multiplexing (recommended)
      http3_enabled = true # enable HTTP/3 / QUIC
      ipv6_enabled  = true # respond on IPv6 addresses

      enable_viewer_mtls        = true
      viewer_mtls_truststore_id = ioriver_client_mtls_trust_store.clients.id
    }
  }
}
