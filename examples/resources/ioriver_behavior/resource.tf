resource "ioriver_behavior" "example_behavior" {
  service      = ioriver_service.service.id
  name         = "example-behavior"
  path_pattern = "/static/*"
  actions = [
    {
      cache_behavior = "CACHE"
    },
    {
      cached_methods = [
        {
          method = "GET"
        },
        {
          method = "HEAD"
        },
      ]
    },
    {
      cache_ttl = 86400
    },
    {
      cache_key = {
        headers = [
          {
            header = "host"
          },
          {
            header = "origin"
          },
        ],
        cookies = [],
        query_strings = {
          type = "include"
          list = [
            {
              param = "p1"
            },
            {
              param = "p2"
            },
          ]
        },
      },
    },
    {
      browser_cache_ttl = 120
    },
    {
      response_header = {
        header_name  = "foo"
        header_value = "bar"
      }
    },
    {
      cors_header = {
        header_name  = "Access-Control-Allow-Origin"
        header_value = "*"
      }
    },
    {
      status_code_cache = {
        status_code    = "204"
        cache_behavior = "CACHE"
        cache_ttl      = 60
      }
    }
  ]
}
