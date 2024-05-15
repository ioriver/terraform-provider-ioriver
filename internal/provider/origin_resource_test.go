package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"golang.org/x/exp/slices"
	ioriver "github.com/ioriver/ioriver-go"
)

var originResourceType string = "ioriver_origin"

func init() {
	var testedObj TestedOrigin
	excludeId := os.Getenv("IORIVER_TEST_ORIGIN_ID")
	resource.AddTestSweepers(originResourceType, &resource.Sweeper{
		Name: originResourceType,
		F: func(r string) error {
			return testSweepResources[ioriver.Origin](r, testedObj, []string{excludeId})
		},
	})
}

type TestedOrigin struct {
	TestedObj[ioriver.Origin]
}

func (TestedOrigin) Get(client *ioriver.IORiverClient, id string) (*ioriver.Origin, error) {
	serviceId := os.Getenv("IORIVER_TEST_SERVICE_ID")
	return client.GetOrigin(serviceId, id)
}

func (TestedOrigin) List(client *ioriver.IORiverClient) ([]ioriver.Origin, error) {
	serviceId := os.Getenv("IORIVER_TEST_SERVICE_ID")
	return client.ListOrigins(serviceId)
}

func (TestedOrigin) Delete(client *ioriver.IORiverClient, object ioriver.Origin, excludeIds []string) error {
	idx := slices.IndexFunc(excludeIds, func(id string) bool { return id == object.Id })
	if idx < 0 {
		serviceId := os.Getenv("IORIVER_TEST_SERVICE_ID")
		return client.DeleteOrigin(serviceId, object.Id)
	} else {
		return nil
	}
}

func TestAccIORiverOrigin_Basic(t *testing.T) {
	var origin ioriver.Origin
	var testedObj TestedOrigin

	serviceId := os.Getenv("IORIVER_TEST_SERVICE_ID")
	fastlyToken := os.Getenv("IORIVER_TEST_FASTLY_API_TOKEN")
	rndName := generateRandomResourceName()
	originHost := rndName + ".example.com"
	shieldSubdivision := "VA"
	resourceName := originResourceType + "." + rndName

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(s *terraform.State) error {
			return testAccCheckResourceDestroy[ioriver.Origin](s, testedObj, originResourceType)
		},
		Steps: []resource.TestStep{
			{
				Config: testAccCheckOriginConfig(rndName, serviceId, originHost, fastlyToken, shieldSubdivision),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ioriver.Origin](resourceName, &origin, testedObj),
					resource.TestCheckResourceAttr(resourceName, "host", originHost),
				),
			},
			{
				ResourceName:        "ioriver_origin." + rndName,
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

func TestAccIORiverOrigin_Update(t *testing.T) {
	var origin ioriver.Origin
	var testedObj TestedOrigin

	serviceId := os.Getenv("IORIVER_TEST_SERVICE_ID")
	fastlyToken := os.Getenv("IORIVER_TEST_FASTLY_API_TOKEN")
	rndName := generateRandomResourceName()
	originHost := rndName + ".example.com"
	updatedOriginHost := "updated-" + originHost
	shieldSubdivision := "VA"
	updatedShieldSubdivision := "OR"
	resourceName := originResourceType + "." + rndName

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(s *terraform.State) error {
			return testAccCheckResourceDestroy[ioriver.Origin](s, testedObj, originResourceType)
		},
		Steps: []resource.TestStep{
			{
				Config: testAccCheckOriginConfig(rndName, serviceId, originHost, fastlyToken, shieldSubdivision),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ioriver.Origin](resourceName, &origin, testedObj),
				),
			},
			{
				Config: testAccCheckOriginConfig(rndName, serviceId, updatedOriginHost, fastlyToken, updatedShieldSubdivision),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ioriver.Origin](resourceName, &origin, testedObj),
					resource.TestCheckResourceAttr(resourceName, "host", updatedOriginHost),
				),
			},
		},
	})
}

func testAccCheckOriginConfig(rndName string, serviceId string, host string, fastlyToken string,
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
	}
	
	resource "ioriver_origin" "%s" {
		service        = "%s"
		host           = "%s"
		timeout_ms     = 5000
		shield_location = {
			country = "US"
			subdivision = "%s"
		}
		shield_providers = [
			{
				service_provider = ioriver_service_provider.tf_test_service_provider.id
			}
		]
	}`, fastlyToken, serviceId, rndName, serviceId, host, shieldSubdivision)
}
