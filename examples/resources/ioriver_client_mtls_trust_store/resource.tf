resource "ioriver_client_mtls_trust_store" "clients" {
  name   = "trusted-clients"
  bundle = file("${path.module}/certs/client-ca-bundle.pem")

  # Create the replacement before switching service references and deleting the old store.
  lifecycle {
    create_before_destroy = true
  }
}

resource "ioriver_service" "mtls" {
  name         = "client-mtls-service"
  certificates = [ioriver_certificate.cert.id]

  config = {
    protocol = {
      enable_viewer_mtls        = true
      viewer_mtls_truststore_id = ioriver_client_mtls_trust_store.clients.id
    }
  }
}
