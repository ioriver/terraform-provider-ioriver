resource "ioriver_behavior" "example_behavior" {
  service      = ioriver_service.service.id
  name         = "example-behavior"
  path_pattern = "/static/*"
  actions = [
    {
      type                 = "CACHE_BEHAVIOR"
      cache_behavior_value = "CACHE"
    },
    {
      type    = "CACHE_TTL"
      max_ttl = "86400"
    },
    {
      type    = "BROWSER_CACHE_TTL"
      max_ttl = "604800"
    },
  ]
}
