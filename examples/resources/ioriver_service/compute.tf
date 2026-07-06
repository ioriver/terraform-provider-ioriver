resource "ioriver_service" "compute_example" {
  name        = "compute-service"
  certificate = ioriver_certificate.cert.id

  config = {
    compute = {
      user_compute = [
        {
          name   = "edge-function"
          routes = ["www.example.com/api/*"]

          viewer_request  = file("viewer_request.js")
          origin_request  = file("origin_request.js")
          origin_response = file("origin_response.js")
          viewer_response = file("viewer_response.js")
        }
      ]

      report = {
        sending_reports_threshold = 100
        trigger_send_interval     = 60
      }
    }
  }
}