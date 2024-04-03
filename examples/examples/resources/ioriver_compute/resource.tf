resource "ioriver_compute" "example_compute" {
  service       = ioriver_service.service.id
  name          = "example-function"
  request_code  = file("request_func.js")
  response_code = file("response_func.js")
  routes = [
    {
      domain = "domain.example.com"
      path   = "/api/*"
    }
  ]
}
