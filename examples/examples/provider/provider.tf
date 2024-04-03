terraform {
  required_providers {
    ioriver = {
      source = "github.com/ioriver"
    }
  }
}

provider "ioriver" {
  token = "abcefg1234567"
}
