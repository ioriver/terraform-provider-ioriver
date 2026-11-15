resource "ioriver_attached_file" "blocked" {
  file_type = "HTML"
  contents  = <<-HTML
    <!doctype html>
    <html><body>Access denied.</body></html>
  HTML

  # Alternatively, read an existing UTF-8 file:
  # contents = file("${path.module}/blocked.html")

  lifecycle {
    create_before_destroy = true
  }
}
