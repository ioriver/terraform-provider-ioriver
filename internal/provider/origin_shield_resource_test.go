package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	ioriver "github.com/ioriver/ioriver-go"
	"golang.org/x/exp/slices"
)

var originShieldResourceType string = "ioriver_origin_shield"

func init() {
}

type TestedOriginShield struct {
	TestedObj[ioriver.Origin]
}

func (TestedOriginShield) Get(client *ioriver.IORiverClient, id string) (*ioriver.Origin, error) {
	serviceId := os.Getenv("IORIVER_TEST_SERVICE_ID")
	return client.GetOrigin(serviceId, id)
}

func (TestedOriginShield) List(client *ioriver.IORiverClient) ([]ioriver.Origin, error) {
	serviceId := os.Getenv("IORIVER_TEST_SERVICE_ID")
	return client.ListOrigins(serviceId)
}

func (TestedOriginShield) Delete(client *ioriver.IORiverClient, object ioriver.Origin, excludeIds []string) error {
	idx := slices.IndexFunc(excludeIds, func(id string) bool { return id == object.Id })
	if idx < 0 {
		serviceId := os.Getenv("IORIVER_TEST_SERVICE_ID")
		return client.DeleteOrigin(serviceId, object.Id)
	} else {
		return nil
	}
}

func TestAccIORiverOriginShield_Basic(t *testing.T) {
	var origin ioriver.Origin
	var testedObj TestedOriginShield

	serviceId := os.Getenv("IORIVER_TEST_SERVICE_ID")
	domainId := os.Getenv("IORIVER_TEST_DOMAIN_ID")
	fastlyToken := os.Getenv("IORIVER_TEST_FASTLY_API_TOKEN")
	rndName := generateRandomResourceName()
	shieldSubdivision := "VA"
	resourceName := originShieldResourceType + "." + rndName

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(s *terraform.State) error {
			return testAccCheckResourceDestroy[ioriver.Origin](s, testedObj, originShieldResourceType)
		},
		Steps: []resource.TestStep{
			{
				Config: testAccCheckOriginShieldConfig(rndName, serviceId, domainId, fastlyToken, shieldSubdivision),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ioriver.Origin](resourceName, &origin, testedObj),
					resource.TestCheckResourceAttr(resourceName, "shield_providers.#", "1"),
				),
			},
			{
				ResourceName:        "ioriver_origin_shield." + rndName,
				ImportStateIdPrefix: fmt.Sprintf("%s,", serviceId),
				ImportState:         true,
				ImportStateVerify:   true,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ioriver.Origin](resourceName, &origin, testedObj),
				),
			},
		},
	})
}

func testAccCheckOriginShieldConfig(rndName string, serviceId string, domainId string, fastlyToken string,
	shieldSubdivision string) string {
	return fmt.Sprintf(`
	resource "ioriver_account_provider" "tf_test_account_provider" {
		credentials = {
		  fastly = "%s"
		}
	}

	resource "ioriver_service_provider" "tf_test_service_provider" {
		service          = "%s"
		account_provider = ioriver_account_provider.tf_test_account_provider.id
		service_domain   = "%s"
	}
	
	resource "ioriver_origin" "test_origin" {
		service        = "%s"
		host           = "test-shield.example.com"
		timeout_ms     = 5000
	}

	resource "ioriver_origin_shield" "%s" {
	  service = "%s"
		origin  = ioriver_origin.test_origin.id
		shield_location = {
			country = "US"
			subdivision = "%s"
		}
		shield_providers = [
			{
				service_provider = ioriver_service_provider.tf_test_service_provider.id
			}
		]
	}`, fastlyToken, serviceId, domainId, serviceId, rndName, serviceId, shieldSubdivision)
}
