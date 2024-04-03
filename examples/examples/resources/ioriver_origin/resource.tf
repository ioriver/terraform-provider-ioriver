resource "ioriver_origin" "example_origin" {
  service = ioriver_service.service.id
  host    = "origin.example.com"
}
