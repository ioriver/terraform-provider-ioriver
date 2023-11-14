package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"golang.org/x/exp/slices"
	ioriver "ioriver.io/ioriver/ioriver-go"
)

var serviceResourceType string = "ioriver_service"

func init() {
	var testedObj TestedService
	excludeId := os.Getenv("IORIVER_TEST_SERVICE_ID")
	resource.AddTestSweepers(serviceResourceType, &resource.Sweeper{
		Name: serviceResourceType,
		F: func(r string) error {
			return testSweepResources[ioriver.Service](r, testedObj, []string{excludeId})
		},
	})
}

type TestedService struct {
	TestedObj[ioriver.Service]
}

func (TestedService) Get(client *ioriver.IORiverClient, id string) (*ioriver.Service, error) {
	return client.GetService(id)
}

func (TestedService) List(client *ioriver.IORiverClient) ([]ioriver.Service, error) {
	return client.ListServices()
}

func (TestedService) Delete(client *ioriver.IORiverClient, object ioriver.Service, excludeIds []string) error {
	idx := slices.IndexFunc(excludeIds, func(id string) bool { return id == object.Id })
	if idx < 0 {
		return client.DeleteService(object.Id)
	} else {
		return nil
	}
}

func TestAccIORiverService_Basic(t *testing.T) {
	var service ioriver.Service
	var testedObj TestedService

	certId := os.Getenv("IORIVER_TEST_CERT_ID")
	rndName := generateRandomResourceName()
	resourceName := serviceResourceType + "." + rndName

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(s *terraform.State) error {
			return testAccCheckResourceDestroy[ioriver.Service](s, testedObj, serviceResourceType)
		},
		Steps: []resource.TestStep{
			{
				Config: testAccCheckServiceConfig(rndName, rndName, rndName, certId),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ioriver.Service](resourceName, &service, testedObj),
					resource.TestCheckResourceAttr(resourceName, "name", rndName),
				),
			},
			{
				ResourceName:      "ioriver_service." + rndName,
				ImportState:       true,
				ImportStateVerify: true,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ioriver.Service](resourceName, &service, testedObj),
				),
			},
		},
	})
}

func TestAccIORiverService_Update(t *testing.T) {
	var service ioriver.Service
	var testedObj TestedService

	certId := os.Getenv("IORIVER_TEST_CERT_ID")
	rndName := generateRandomResourceName()
	resourceName := serviceResourceType + "." + rndName
	// updatedServiceName := rndName + "_updated"
	// updatedDescription := rndName + "_updated_desc"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(s *terraform.State) error {
			return testAccCheckResourceDestroy[ioriver.Service](s, testedObj, serviceResourceType)
		},
		Steps: []resource.TestStep{
			{
				Config: testAccCheckServiceConfig(rndName, rndName, rndName, certId),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ioriver.Service](resourceName, &service, testedObj),
				),
			},
			// TODO - enable back once service update flow is fixed
			// {
			// 	Config: testAccCheckServiceConfig(subdomain, rndName, updatedServiceName, updatedDescription, certId),
			// 	Check: resource.ComposeTestCheckFunc(
			// 		resource.TestCheckResourceAttr(resourceName, "name", rndName),
			// 	),
			// },
		},
	})
}

func testAccCheckServiceConfig(resourceName string, serviceName string, description string, certId string) string {
	return fmt.Sprintf(`
	resource "ioriver_service" "%s" {
		name          = "%s"
		description   = "%s"
		certificate   = "%s"
	  }`, resourceName, serviceName, description, certId)
}
