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

var spResourceType string = "ioriver_service_provider"

func init() {
	var testedObj TestedServiceProvider
	excludeId := os.Getenv("IORIVER_TEST_SERVICE_PROVIDER_ID")
	resource.AddTestSweepers(spResourceType, &resource.Sweeper{
		Name: spResourceType,
		F: func(r string) error {
			return testSweepResources[ioriver.ServiceProvider](r, testedObj, []string{excludeId})
		},
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

	serviceId := os.Getenv("IORIVER_TEST_SERVICE_ID")
	fastlyToken := os.Getenv("IORIVER_TEST_FASTLY_API_TOKEN")
	rndName := generateRandomResourceName()
	resourceName := spResourceType + "." + rndName

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(s *terraform.State) error {
			return testAccCheckResourceDestroy[ioriver.ServiceProvider](s, testedObj, spResourceType)
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
				ResourceName:            "ioriver_service_provider." + rndName,
				ImportStateIdPrefix:     fmt.Sprintf("%s,", serviceId),
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"is_unmanaged"}, // ignore since this field cannot be read
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
		provider_name = "fastly"
		credentials = {
		  fastly = "%s"
		}
	}

	resource "ioriver_service_provider" "%s" {
		service          = "%s"
		account_provider = ioriver_account_provider.test_account_provider.id
	  }`, accountProviderToken, rndName, serviceId)
}
