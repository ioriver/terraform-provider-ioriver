package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"golang.org/x/exp/slices"
	ioriver "github.com/ioriver/ioriver-go"
)

var computeResourceType string = "ioriver_compute"

func init() {
	var testedObj TestedCompute
	resource.AddTestSweepers(computeResourceType, &resource.Sweeper{
		Name: computeResourceType,
		F: func(r string) error {
			return testSweepResources[ioriver.Compute](r, testedObj, []string{})
		},
	})
}

type TestedCompute struct {
	TestedObj[ioriver.Compute]
}

func (TestedCompute) Get(client *ioriver.IORiverClient, id string) (*ioriver.Compute, error) {
	serviceId := os.Getenv("IORIVER_TEST_SERVICE_ID")
	return client.GetCompute(serviceId, id)
}

func (TestedCompute) List(client *ioriver.IORiverClient) ([]ioriver.Compute, error) {
	serviceId := os.Getenv("IORIVER_TEST_SERVICE_ID")
	return client.ListComputes(serviceId)
}

func (TestedCompute) Delete(client *ioriver.IORiverClient, object ioriver.Compute, excludeIds []string) error {
	idx := slices.IndexFunc(excludeIds, func(id string) bool { return id == object.Id })
	if idx < 0 {
		serviceId := os.Getenv("IORIVER_TEST_SERVICE_ID")
		return client.DeleteCompute(serviceId, object.Id)
	} else {
		return nil
	}
}

func TestAccIORiverCompute_Basic(t *testing.T) {
	var compute ioriver.Compute
	var testedObj TestedCompute

	domain := os.Getenv("IORIVER_TEST_DOMAIN")
	serviceId := os.Getenv("IORIVER_TEST_SERVICE_ID")
	rndName := generateRandomResourceName()
	resourceName := computeResourceType + "." + rndName
	responseCode := "async function onResponse(request, response) { response.headers.append('foo', 'bar'); return response; }"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(s *terraform.State) error {
			return testAccCheckResourceDestroy[ioriver.Compute](s, testedObj, computeResourceType)
		},
		Steps: []resource.TestStep{
			{
				Config: testAccCheckComputeConfigBasic(rndName, serviceId, domain, responseCode),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ioriver.Compute](resourceName, &compute, testedObj),
					resource.TestCheckResourceAttr(resourceName, "name", rndName),
				),
			},
			{
				ResourceName:        "ioriver_compute." + rndName,
				ImportStateIdPrefix: fmt.Sprintf("%s,", serviceId),
				ImportState:         true,
				ImportStateVerify:   true,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ioriver.Compute](resourceName, &compute, testedObj),
				),
			},
		},
	})
}

func TestAccIORiverCompute_Update(t *testing.T) {
	var compute ioriver.Compute
	var testedObj TestedCompute

	domain := os.Getenv("IORIVER_TEST_DOMAIN")
	serviceId := os.Getenv("IORIVER_TEST_SERVICE_ID")
	rndName := generateRandomResourceName()
	resourceName := computeResourceType + "." + rndName
	responseCode := "async function onResponse(request, response) { response.headers.append('foo', 'bar'); return response; }"
	updatedName := rndName + "_updated"
	updatedResponseCode := "async function onResponse(request, response) { response.headers.append('foo1', 'bar2'); return response; }"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(s *terraform.State) error {
			return testAccCheckResourceDestroy[ioriver.Compute](s, testedObj, computeResourceType)
		},
		Steps: []resource.TestStep{
			{
				Config: testAccCheckComputeConfigBasic(rndName, serviceId, domain, responseCode),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ioriver.Compute](resourceName, &compute, testedObj),
				),
			},
			{
				Config: testAccCheckComputeConfigUpdate(rndName, updatedName, serviceId, domain, updatedResponseCode),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ioriver.Compute](resourceName, &compute, testedObj),
					resource.TestCheckResourceAttr(resourceName, "name", updatedName),
				),
			},
		},
	})
}

func testAccCheckComputeConfigBasic(rndName string, serviceId string, domain string, responseCode string) string {
	routeDomain1 := rndName + "1." + domain
	routeDomain2 := rndName + "2." + domain
	return fmt.Sprintf(`
	resource "ioriver_compute" "%[1]s" {
		service       = "%[2]s"
		name          = "%[3]s"
		response_code = "%[4]s"
		routes        = [ 
			{
				domain = "%[5]s"
				path   = "/api/*"
			},
			{
				domain = "%[6]s"
				path   = "/test/*"
			}			
		]
	}`, rndName, serviceId, rndName, responseCode, routeDomain1, routeDomain2)
}

func testAccCheckComputeConfigUpdate(rndName string, computeName string, serviceId string, domain string, responseCode string) string {
	routeDomain := rndName + "1." + domain
	return fmt.Sprintf(`
	resource "ioriver_compute" "%[1]s" {
		service       = "%[2]s"
		name          = "%[3]s"
		response_code = "%[4]s"
		routes        = [ 
			{
				domain = "%[5]s"
				path   = "/api/*"
			}
		]
	  }`, rndName, serviceId, computeName, responseCode, routeDomain)
}
