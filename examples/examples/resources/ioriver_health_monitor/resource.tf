resource "ioriver_health_monitor" "availability_monitor" {
  service = ioriver_service.service.id
  name    = "availability"
  url     = "https://domain.example.com/ping"
}
