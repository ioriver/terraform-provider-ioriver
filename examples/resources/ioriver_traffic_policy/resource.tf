// example 1 - define a default trafic policy
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
  geos = []

  health_monitors = [
    {
      health_monitor = ioriver_health_monitor.availability_monitor.id
    }
  ]
  performance_monitors = []
}

// example 2 - define a dynamic traffic policy for Europe
resource "ioriver_traffic_policy" "dynamic_eu" {
  service  = ioriver_service.service.id
  type     = "Dynamic"
  failover = false

  providers = [
    {
      service_provider = ioriver_service_provider.fastly.id
    },
    {
      service_provider = ioriver_service_provider.cloudfront.id
    }
  ]
  geos = [
    {
      continent = "EU"
    }
  ]

  health_monitors = []
  performance_monitors = [
    {
      performance_monitor = ioriver_performance_monitor.perf_mon.id
    }
  ]
}

// example 3 - define a cost based traffic policy for North America with performance penalty 10%
resource "ioriver_traffic_policy" "cost_based_na" {
  service             = ioriver_service.service.id
  type                = "Cost"
  failover            = false
  performance_penalty = 10

  providers = [
    {
      service_provider       = ioriver_service_provider.fastly.id
      priority               = 1
      is_commitment_priority = false
    },
    {
      service_provider       = ioriver_service_provider.cloudfront.id
      priority               = 2
      is_commitment_priority = false
    }
  ]
  geos = [
    {
      continent = "NA"
    }
  ]

  health_monitors = []
  performance_monitors = [
    {
      performance_monitor = ioriver_performance_monitor.perf_mon.id
    }
  ]
}
