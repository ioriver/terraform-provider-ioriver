package provider

import (
	"fmt"
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	ioriver "github.com/ioriver/ioriver-go"
)

func mtlsNamedServiceConfig(name, certID, protocol string) string {
	return fmt.Sprintf(`
resource "ioriver_service" "test" {
  name         = %q
  description  = "mTLS lifecycle test"
  certificates = [%q]
  config = {
    protocol = {%s}
  }
}
`, name, certID, protocol)
}

func TestAccIORiverService_MTLS(t *testing.T) {
	name := mtlsTestNamePrefix + generateRandomResourceName()
	bundle, replacement := mtlsAcceptanceBundle(t), mtlsAcceptanceBundle(t)
	certID := os.Getenv("IORIVER_TEST_CERT_ID")
	const protocol = "enable_viewer_mtls = true\nviewer_mtls_truststore_id = ioriver_client_mtls_trust_store.test.id"
	var serviceID string
	check := func(state *terraform.State) error {
		id := state.RootModule().Resources["ioriver_service.test"].Primary.ID
		if serviceID != "" && serviceID != id {
			return fmt.Errorf("mTLS update replaced the service")
		}
		serviceID = id
		current, err := GetServiceWithConfig(testAccClient, id)
		if err != nil {
			return err
		}
		actual, ok := current.Config["protocol"].(map[string]interface{})
		if !ok {
			return fmt.Errorf("backend response is missing protocol configuration")
		}
		storeID := state.RootModule().Resources["ioriver_client_mtls_trust_store.test"].Primary.ID
		if actual["enable_viewer_mtls"] != true || actual["viewer_mtls_truststore_id"] != storeID {
			return fmt.Errorf("backend did not persist the enabled mTLS trust store")
		}
		return nil
	}
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreEnvVariable(t, "IORIVER_API_TOKEN")
			testAccPreEnvVariable(t, "IORIVER_TEST_CERT_ID")
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(state *terraform.State) error {
			if err := testAccCheckResourceDestroy[ServiceWithConfig](state, TestedService{}, serviceResourceType); err != nil {
				return err
			}
			return testAccCheckResourceDestroy[ioriver.ViewerMTLSTrustStore](state, TestedClientMTLSTrustStore{}, clientMTLSTrustStoreResourceType)
		},
		Steps: []resource.TestStep{
			{
				Config: mtlsTrustStoreResourceConfig(name, bundle) + mtlsNamedServiceConfig(name, certID, protocol),
				Check: resource.ComposeTestCheckFunc(check,
					resource.TestCheckResourceAttr("ioriver_service.test", "config.protocol.enable_viewer_mtls", "true")),
			},
			{ResourceName: "ioriver_service.test", ImportState: true, ImportStateVerify: true},
			{
				Config: mtlsTrustStoreResourceConfig(name, replacement) + mtlsNamedServiceConfig(name, certID, protocol),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("ioriver_service.test", plancheck.ResourceActionUpdate),
					plancheck.ExpectResourceAction("ioriver_client_mtls_trust_store.test", plancheck.ResourceActionCreateBeforeDestroy),
				}},
				Check: check,
			},
			{Config: mtlsTrustStoreResourceConfig(name, replacement) + mtlsNamedServiceConfig(name, certID, protocol), PlanOnly: true},
			{
				Config: mtlsTrustStoreResourceConfig(name, replacement) + mtlsNamedServiceConfig(name, certID,
					"enable_viewer_mtls = false\nviewer_mtls_truststore_id = ioriver_client_mtls_trust_store.test.id"),
				Check: resource.TestCheckResourceAttr("ioriver_service.test", "config.protocol.enable_viewer_mtls", "false"),
			},
			{
				Config: mtlsTrustStoreResourceConfig(name, replacement) + mtlsNamedServiceConfig(name, certID, ""),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckNoResourceAttr("ioriver_service.test", "config.protocol.enable_viewer_mtls"),
					resource.TestCheckNoResourceAttr("ioriver_service.test", "config.protocol.viewer_mtls_truststore_id")),
			},
			{Config: mtlsNamedServiceConfig(name, certID, "")},
		},
	})
}

func TestAccIORiverService_MTLSInvalidTrustStore(t *testing.T) {
	name := mtlsTestNamePrefix + generateRandomResourceName()
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreEnvVariable(t, "IORIVER_API_TOKEN")
			testAccPreEnvVariable(t, "IORIVER_TEST_CERT_ID")
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(state *terraform.State) error {
			return testAccCheckResourceDestroy[ServiceWithConfig](state, TestedService{}, serviceResourceType)
		},
		Steps: []resource.TestStep{{
			Config: mtlsNamedServiceConfig(name, os.Getenv("IORIVER_TEST_CERT_ID"),
				fmt.Sprintf("enable_viewer_mtls = true\nviewer_mtls_truststore_id = %q", GenerateUUID())),
			ExpectError: regexp.MustCompile(`(?i)truststore.*not found`),
		}},
	})
}
