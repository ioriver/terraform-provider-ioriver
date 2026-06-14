package provider

import (
	"fmt"
)

// ─── HCL config generators (called from service_resource_test.go) ─────────────

func testAccCheckServiceConfigWithProtocol(resourceName string, certId string, http2 bool, http3 bool, ipv6 bool) string {
	return fmt.Sprintf(`
resource "%s" "%s" {
	name        = "%s"
	certificate = "%s"
	description = "A generic service"

	config = {
		protocol = {
			http2_enabled = %t
			http3_enabled = %t
			ipv6_enabled  = %t
		}
	}
}`, serviceResourceType, resourceName, resourceName, certId, http2, http3, ipv6)
}

func testAccCheckServiceConfigWithoutProtocol(resourceName string, certId string) string {
	return fmt.Sprintf(`
resource "%s" "%s" {
	name        = "%s"
	certificate = "%s"
	description = "A generic service"

	config = {
	}
}`, serviceResourceType, resourceName, resourceName, certId)
}
