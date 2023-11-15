terraform {
  required_providers {
    ioriver = {
      source = "ioriver/ioriver"
    }
  }
}

provider "ioriver" {
  token = "abcefg1234567"
}
