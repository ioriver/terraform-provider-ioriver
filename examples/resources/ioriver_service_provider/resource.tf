// example 1 - Fastly provider
resource "ioriver_service_provider" "fastly" {
  service          = ioriver_service.service.id
  account_provider = ioriver_account_provider.fastly.id
  service_domain   = ioriver_domain.domain.id
}


// example 2 - Akamai provider with custom data
resource "ioriver_service_provider" "akamai" {
  service              = ioriver_service.service.id
  account_provider     = ioriver_account_provider.akamai.id
  service_domain       = ioriver_domain.domain.id
  provider_custom_data = "{\"group\":\"grp_1234\",\"cp_code\":\"cpc_5678\",\"contract_id\":\"ctr_W-ABCD123\"}"
}