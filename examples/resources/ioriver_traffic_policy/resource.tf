resource "ioriver_traffic_policy" "default_traffic_policy" {
  service    = ioriver_service.service.id
  type       = "Static"
  failover   = true
  is_default = true
  providers = [
    {
      service_provider = ioriver_service_provider.fastly.id
      weight           = 50
    },
    {
      service_provider = ioriver_service_provider.cloudfront.id
      weight           = 50
    }
  ]
  geos = [
    {
    },
  ]

  health_monitors = [
    ioriver_health_monitor.availability_monitor.id
  ]
  performance_monitors = [
    ioriver_health_monitor.perf_mon.id
  ]
}
