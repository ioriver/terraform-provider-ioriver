resource "ioriver_service_provider" "fastly" {
  service          = ioriver_service.service.id
  account_provider = ioriver_account_provider.fastly.id
}
