// example 1 - Fastly provider
resource "ioriver_service_provider" "fastly" {
  service          = ioriver_service.service.id
  account_provider = ioriver_account_provider.fastly.id
}


// example 2 - Akamai provider with custom data
resource "ioriver_service_provider" "akamai" {
  service          = ioriver_service.service.id
  account_provider = ioriver_account_provider.akamai.id
  provider_custom_data = {
    property_group = "grp_1234"
    contract_id    = "ctr_W-ABCD123"
    product        = "prd_9012"
    cp_code        = "cpc_5678"
  }
}