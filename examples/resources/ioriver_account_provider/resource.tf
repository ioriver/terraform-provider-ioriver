// example 1 - Fastly provider
resource "ioriver_account_provider" "fastly" {
  credentials = {
    fastly = "ulMy_iABCh-6fzo6cvRblzEJ1auAlvu"
  }
}

// example 2 - Akamai provider
resource "ioriver_account_provider" "akamai" {
  credentials = {
    akamai = {
      access_token  = "akab-abc2wesh637tjps6-3xhaopkd42oymrt7"
      base_url      = "https://akab-ab7ytkynbz3po2qg-eyvgfghu76cjm3ty.luna.akamaiapis.net"
      client_token  = "akab-vn4xyz5vui8cj4ew-ljtmzunbgtgjhk57"
      client_secret = "jb10neLvAfXiAFkjygUHbMfWyusNlTyRQ0rL4K8ugtQ="
    }
  }
}