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

var healthMonitorResourceType string = "ioriver_health_monitor"

func init() {
	var testedObj TestedHealthMonitor
	resource.AddTestSweepers(healthMonitorResourceType, &resource.Sweeper{
		Name: healthMonitorResourceType,
		F: func(r string) error {
			return testSweepResources[ioriver.HealthMonitor](r, testedObj, []string{})
		},
	})
}

type TestedHealthMonitor struct {
	TestedObj[ioriver.HealthMonitor]
}

func (TestedHealthMonitor) Get(client *ioriver.IORiverClient, id string) (*ioriver.HealthMonitor, error) {
	serviceId := os.Getenv("IORIVER_TEST_SERVICE_ID")
	return client.GetHealthMonitor(serviceId, id)
}

func (TestedHealthMonitor) List(client *ioriver.IORiverClient) ([]ioriver.HealthMonitor, error) {
	serviceId := os.Getenv("IORIVER_TEST_SERVICE_ID")
	return client.ListHealthMonitors(serviceId)
}

func (TestedHealthMonitor) Delete(client *ioriver.IORiverClient, object ioriver.HealthMonitor, excludeIds []string) error {
	idx := slices.IndexFunc(excludeIds, func(id string) bool { return id == object.Id })
	if idx < 0 {
		serviceId := os.Getenv("IORIVER_TEST_SERVICE_ID")
		return client.DeleteHealthMonitor(serviceId, object.Id)
	} else {
		return nil
	}
}

func TestAccIORiverHealthMonitor_Basic(t *testing.T) {
	var healthMonitor ioriver.HealthMonitor
	var testedObj TestedHealthMonitor

	serviceId := os.Getenv("IORIVER_TEST_SERVICE_ID")
	testDomain := os.Getenv("IORIVER_TEST_DOMAIN")
	rndName := generateRandomResourceName()
	url := "https://tf-test-1." + testDomain + "/" + rndName
	resourceName := healthMonitorResourceType + "." + rndName

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(s *terraform.State) error {
			return testAccCheckResourceDestroy[ioriver.HealthMonitor](s, testedObj, healthMonitorResourceType)
		},
		Steps: []resource.TestStep{
			{
				Config: testAccCheckHealthMonitorConfig(rndName, serviceId, url),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ioriver.HealthMonitor](resourceName, &healthMonitor, testedObj),
					resource.TestCheckResourceAttr(resourceName, "url", url),
				),
			},
			{
				ResourceName:        "ioriver_health_monitor." + rndName,
				ImportStateIdPrefix: fmt.Sprintf("%s,", serviceId),
				ImportState:         true,
				ImportStateVerify:   true,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ioriver.HealthMonitor](resourceName, &healthMonitor, testedObj),
				),
			},
		},
	})
}

func TestAccIORiverHealthMonitor_Update(t *testing.T) {
	var healthMonitor ioriver.HealthMonitor
	var testedObj TestedHealthMonitor

	serviceId := os.Getenv("IORIVER_TEST_SERVICE_ID")
	testDomain := os.Getenv("IORIVER_TEST_DOMAIN")
	rndName := generateRandomResourceName()
	url := "https://tf-test-1." + testDomain + "/" + rndName
	updatedUrl := url + "-updated"
	resourceName := healthMonitorResourceType + "." + rndName

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(s *terraform.State) error {
			return testAccCheckResourceDestroy[ioriver.HealthMonitor](s, testedObj, healthMonitorResourceType)
		},
		Steps: []resource.TestStep{
			{
				Config: testAccCheckHealthMonitorConfig(rndName, serviceId, url),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ioriver.HealthMonitor](resourceName, &healthMonitor, testedObj),
				),
			},
			{
				Config: testAccCheckHealthMonitorConfig(rndName, serviceId, updatedUrl),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ioriver.HealthMonitor](resourceName, &healthMonitor, testedObj),
					resource.TestCheckResourceAttr(resourceName, "url", updatedUrl),
				),
			},
		},
	})
}

func testAccCheckHealthMonitorConfig(rndName string, serviceId string, url string) string {
	return fmt.Sprintf(`
	resource "ioriver_health_monitor" "%s" {
		service        = "%s"
		name           = "test-health-monitor"
		url            = "%s"
	}`, rndName, serviceId, url)
}
