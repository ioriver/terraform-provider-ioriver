resource "ioriver_url_signing_key" "example_key" {
  service        = ioriver_service.service.id
  name           = "test-key"
  public_key     = "abcd"
  encryption_key = "1234"
}
