resource "ioriver_tag" "production" {
  name     = "production"
  color    = "red"
  services = [ioriver_service.example.id]
}
