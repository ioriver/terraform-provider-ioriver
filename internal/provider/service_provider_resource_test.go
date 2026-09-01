package provider

import (
	"fmt"
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	ioriver "github.com/ioriver/ioriver-go"
	"golang.org/x/exp/slices"
)

var spResourceType string = "ioriver_service_provider"

func init() {
	var testedObj TestedServiceProvider
	excludeId := os.Getenv("IORIVER_TEST_SERVICE_PROVIDER_ID")
	resource.AddTestSweepers(spResourceType, &resource.Sweeper{
		Name: spResourceType,
		F: func(r string) error {
			return testSweepResources[ioriver.ServiceProvider](r, testedObj, []string{excludeId})
		},
		Dependencies: []string{"ioriver_traffic_policy"},
	})
}

type TestedServiceProvider struct {
	TestedObj[ioriver.ServiceProvider]
}

func (TestedServiceProvider) Get(client *ioriver.IORiverClient, id string) (*ioriver.ServiceProvider, error) {
	serviceId := os.Getenv("IORIVER_TEST_SERVICE_ID")
	return client.GetServiceProvider(serviceId, id)
}

func (TestedServiceProvider) List(client *ioriver.IORiverClient) ([]ioriver.ServiceProvider, error) {
	serviceId := os.Getenv("IORIVER_TEST_SERVICE_ID")
	return client.ListServiceProviders(serviceId)
}

func (TestedServiceProvider) Delete(client *ioriver.IORiverClient, object ioriver.ServiceProvider, excludeIds []string) error {
	idx := slices.IndexFunc(excludeIds, func(id string) bool { return id == object.Id })
	if idx < 0 {
		serviceId := os.Getenv("IORIVER_TEST_SERVICE_ID")
		return client.DeleteServiceProvider(serviceId, object.Id, "disconnect")
	} else {
		return nil
	}
}

func TestAccIORiverServiceProvider_Basic(t *testing.T) {
	var serviceProvider ioriver.ServiceProvider
	var testedObj TestedServiceProvider
	var testedAP TestedAccountProvider

	serviceId := os.Getenv("IORIVER_TEST_SERVICE_ID")
	fastlyToken := os.Getenv("IORIVER_TEST_FASTLY_API_TOKEN")
	rndName := generateRandomResourceName()
	resourceName := spResourceType + "." + rndName

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(s *terraform.State) error {
			if err := testAccCheckResourceDestroy[ioriver.ServiceProvider](s, testedObj, spResourceType); err != nil {
				return err
			}
			// The config always creates an inline account provider — verify it's gone too.
			return testAccCheckResourceDestroy[ioriver.AccountProvider](s, testedAP, apResourceType)
		},
		Steps: []resource.TestStep{
			{
				Config: testAccCheckServiceProviderConfigBasic(rndName, serviceId, fastlyToken),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ioriver.ServiceProvider](resourceName, &serviceProvider, testedObj),
					resource.TestCheckResourceAttr(resourceName, "service", serviceId),
				),
			},
			{
				ResourceName:        "ioriver_service_provider." + rndName,
				ImportStateIdPrefix: fmt.Sprintf("%s,", serviceId),
				ImportState:         true,
				ImportStateVerify:   false, // should be disabled since cname field is populated in a delay
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ioriver.ServiceProvider](resourceName, &serviceProvider, testedObj),
				),
			},
		},
	})
}

func testAccCheckServiceProviderConfigBasic(rndName string, serviceId string, accountProviderToken string) string {
	return fmt.Sprintf(`
	resource "ioriver_account_provider" "test_account_provider" {
		credentials = {
		  fastly = "%s"
		}
	}

	resource "ioriver_service_provider" "%s" {
		service          = "%s"
		account_provider = ioriver_account_provider.test_account_provider.id
	  }`, accountProviderToken, rndName, serviceId)
}

// ---------------------------------------------------------------------------
// Cert ↔ domain coverage tests
// ---------------------------------------------------------------------------

// TestAccIORiverServiceProvider_CertCoverage verifies cert ↔ domain coverage
// scenarios when attaching a service provider.
//
//	Step 1: cert2+domain2 only → attach succeeds.
//	Step 2: 1-for-1 replace cert2 → wildcard (ReplaceServiceCertificate path).
//	Step 3: swap wildcard → cert3+domain3 alone → backend rejects (domain2 uncovered).
//	Step 4: keep wildcard, ADD cert3+domain3 → two-cert happy path (both domains covered).
//
// Requires the same env vars as testAccPreCheckV3 + IORIVER_TEST_ACCOUNT_PROVIDER_ID.
func TestAccIORiverServiceProvider_CertCoverage(t *testing.T) {
	t.Parallel()
	certId2 := os.Getenv("IORIVER_TEST_CERT_ID_2")
	certId3 := os.Getenv("IORIVER_TEST_CERT_ID_3")
	certWildcard := os.Getenv("IORIVER_TEST_CERT_ID_4")

	domain2 := os.Getenv("IORIVER_TEST_DOMAIN_2")
	domain3 := os.Getenv("IORIVER_TEST_DOMAIN_3")

	accountProviderId := os.Getenv("IORIVER_TEST_ACCOUNT_PROVIDER_ID")
	fastlyToken := os.Getenv("IORIVER_TEST_FASTLY_API_TOKEN")

	// Prefer reference an existing account provider,
	// or create one inline using the Fastly token.
	if accountProviderId != "" {
		fastlyToken = ""
	}

	rndName := generateRandomResourceName()
	resourceName := spResourceType + "." + rndName

	var testedService TestedService
	var testedAP TestedAccountProvider

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheckV3(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(s *terraform.State) error {
			// Always verify the ephemeral service is destroyed.
			if err := testAccCheckResourceDestroy[ServiceWithConfig](s, testedService, serviceResourceType); err != nil {
				return err
			}
			// Only verify the account provider when it was created inline
			// (fastlyToken path). When accountProviderId is set the AP is
			// pre-existing and must NOT be deleted.
			if fastlyToken != "" {
				if err := testAccCheckResourceDestroy[ioriver.AccountProvider](s, testedAP, apResourceType); err != nil {
					return err
				}
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				// Step 1: cert2 covers domain2 → attach succeeds.
				Config: testAccServiceProviderWithServiceConfig(rndName, certId2, "", "", accountProviderId, fastlyToken, domain2, "", ""),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttrSet(resourceName, "service"),
					resource.TestCheckResourceAttrSet(resourceName, "account_provider"),
				),
			},
			{
				// Step 2: 1-for-1 replace cert2 → wildcard (wildcard covers domain2).
				// Exercises the ReplaceServiceCertificate API path.
				Config: testAccServiceProviderWithServiceConfig(rndName, certWildcard, "", "", accountProviderId, fastlyToken, domain2, "", ""),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttrSet(resourceName, "account_provider"),
				),
			},
			{
				// Step 3: try to replace wildcard → cert3 alone while domain2 is still present.
				// cert3 covers domain3 only, so domain2 would be uncovered → backend rejects.
				Config:      testAccServiceProviderWithServiceConfig(rndName, certId3, "", "", accountProviderId, fastlyToken, domain3, "", ""),
				ExpectError: regexp.MustCompile(`(?is)(The\s+selected\s+certificate\s+cannot\s+be\s+used\s+because\s+it\s+doesn['’]t\s+cover\s+all\s+service\s+domains\.?|Certificate\s+replacement\s+would\s+result\s+in\s+some\s+domains\s+not\s+being\s+covered,\s+which\s+is\s+not\s+supported\.?)`),
			},
			{
				// Step 4: keep wildcard AND add cert3 + domain3.
				// State after Step 3 (ExpectError) is still wildcard+domain2.
				// We add cert3 alongside wildcard (no removal), and add domain3.
				// wildcard covers domain2, cert3 covers domain3 → both covered → succeeds.
				Config: testAccServiceProviderWithServiceConfig(rndName, certWildcard, certId3, "", accountProviderId, fastlyToken, domain2, domain3, ""),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttrSet(resourceName, "service"),
					resource.TestCheckResourceAttrSet(resourceName, "account_provider"),
				),
			},
		},
	})
}

// testAccServiceProviderWithServiceConfig returns HCL that creates a fresh
// ioriver_service (with the supplied certs and domains) together with an
// ioriver_service_provider attached to it.
//
// Pass certId2/certId3 as "" to omit them. Same for domain2/domain3.
// Exactly one of accountProviderId or fastlyToken must be non-empty:
//   - accountProviderId: references a pre-existing ioriver_account_provider by ID.
//   - fastlyToken: creates an inline ioriver_account_provider resource.
func testAccServiceProviderWithServiceConfig(rndName, certId1, certId2, certId3, accountProviderId, fastlyToken, domain1, domain2, domain3 string) string {
	certsHCL := fmt.Sprintf(`"%s"`, certId1)
	if certId2 != "" {
		certsHCL += fmt.Sprintf(`, "%s"`, certId2)
	}
	if certId3 != "" {
		certsHCL += fmt.Sprintf(`, "%s"`, certId3)
	}

	domainsHCL := fmt.Sprintf(`
			{
				domain = "%s"
				mappings = [{ target_mapping = "origin-1" }]
			}`, domain1)
	if domain2 != "" {
		domainsHCL += fmt.Sprintf(`,
			{
				domain = "%s"
				mappings = [{ target_mapping = "origin-1" }]
			}`, domain2)
	}
	if domain3 != "" {
		domainsHCL += fmt.Sprintf(`,
			{
				domain = "%s"
				mappings = [{ target_mapping = "origin-1" }]
			}`, domain3)
	}

	// Build the account_provider reference: inline resource or existing ID.
	var accountProviderHCL string
	var accountProviderRef string
	if fastlyToken != "" {
		accountProviderHCL = fmt.Sprintf(`
resource "ioriver_account_provider" "%s_ap" {
	credentials = {
		fastly = "%s"
	}
}
`, rndName, fastlyToken)
		accountProviderRef = fmt.Sprintf(`ioriver_account_provider.%s_ap.id`, rndName)
	} else {
		accountProviderHCL = ""
		accountProviderRef = fmt.Sprintf(`"%s"`, accountProviderId)
	}

	return fmt.Sprintf(`
%s
resource "ioriver_service" "%s" {
	name        = "%s"
	description = "cert-coverage test service"

	certificates = [%s]

	config = {
		origins = [
			{
				name = "origin-1"
				custom_origin = {
					host     = "origin.example.com"
					protocol = "https"
				}
			}
		]
		domains = [%s]
	}
}

resource "ioriver_service_provider" "%s" {
	service          = ioriver_service.%s.id
	account_provider = %s
}
`, accountProviderHCL, rndName, rndName, certsHCL, domainsHCL,
		rndName, rndName, accountProviderRef)
}
