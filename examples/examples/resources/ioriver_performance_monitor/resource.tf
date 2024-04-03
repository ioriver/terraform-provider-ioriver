resource "ioriver_performance_monitor" "perf_mon" {
  service = ioriver_service.service.id
  name    = "perf"
  url     = "https://domain.example.com/api/perf"
}
