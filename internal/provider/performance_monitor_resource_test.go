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

var performanceMonitorResourceType string = "ioriver_performance_monitor"

func init() {
	var testedObj TestedPerformanceMonitor
	resource.AddTestSweepers(performanceMonitorResourceType, &resource.Sweeper{
		Name: performanceMonitorResourceType,
		F: func(r string) error {
			return testSweepResources[ioriver.PerformanceMonitor](r, testedObj, []string{})
		},
	})
}

type TestedPerformanceMonitor struct {
	TestedObj[ioriver.PerformanceMonitor]
}

func (TestedPerformanceMonitor) Get(client *ioriver.IORiverClient, id string) (*ioriver.PerformanceMonitor, error) {
	serviceId := os.Getenv("IORIVER_TEST_SERVICE_ID")
	return client.GetPerformanceMonitor(serviceId, id)
}

func (TestedPerformanceMonitor) List(client *ioriver.IORiverClient) ([]ioriver.PerformanceMonitor, error) {
	serviceId := os.Getenv("IORIVER_TEST_SERVICE_ID")
	return client.ListPerformanceMonitors(serviceId)
}

func (TestedPerformanceMonitor) Delete(client *ioriver.IORiverClient, object ioriver.PerformanceMonitor, excludeIds []string) error {
	idx := slices.IndexFunc(excludeIds, func(id string) bool { return id == object.Id })
	if idx < 0 {
		serviceId := os.Getenv("IORIVER_TEST_SERVICE_ID")
		return client.DeletePerformanceMonitor(serviceId, object.Id)
	} else {
		return nil
	}
}

func TestAccIORiverPerformanceMonitor_Basic(t *testing.T) {
	var performanceMonitor ioriver.PerformanceMonitor
	var testedObj TestedPerformanceMonitor

	serviceId := os.Getenv("IORIVER_TEST_SERVICE_ID")
	testDomain := os.Getenv("IORIVER_TEST_DOMAIN")
	rndName := generateRandomResourceName()
	url := "https://tf-test-1." + testDomain + "/" + rndName
	resourceName := performanceMonitorResourceType + "." + rndName

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(s *terraform.State) error {
			return testAccCheckResourceDestroy[ioriver.PerformanceMonitor](s, testedObj, performanceMonitorResourceType)
		},
		Steps: []resource.TestStep{
			{
				Config: testAccCheckPerformanceMonitorConfig(rndName, serviceId, url),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ioriver.PerformanceMonitor](resourceName, &performanceMonitor, testedObj),
					resource.TestCheckResourceAttr(resourceName, "url", url),
				),
			},
			{
				ResourceName:        "ioriver_performance_monitor." + rndName,
				ImportStateIdPrefix: fmt.Sprintf("%s,", serviceId),
				ImportState:         true,
				ImportStateVerify:   true,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ioriver.PerformanceMonitor](resourceName, &performanceMonitor, testedObj),
				),
			},
		},
	})
}

func TestAccIORiverPerformanceMonitor_Update(t *testing.T) {
	var performanceMonitor ioriver.PerformanceMonitor
	var testedObj TestedPerformanceMonitor

	serviceId := os.Getenv("IORIVER_TEST_SERVICE_ID")
	testDomain := os.Getenv("IORIVER_TEST_DOMAIN")
	rndName := generateRandomResourceName()
	url := "https://tf-test-1." + testDomain + "/" + rndName
	updatedUrl := url + "-updated"
	resourceName := performanceMonitorResourceType + "." + rndName

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(s *terraform.State) error {
			return testAccCheckResourceDestroy[ioriver.PerformanceMonitor](s, testedObj, performanceMonitorResourceType)
		},
		Steps: []resource.TestStep{
			{
				Config: testAccCheckPerformanceMonitorConfig(rndName, serviceId, url),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ioriver.PerformanceMonitor](resourceName, &performanceMonitor, testedObj),
				),
			},
			{
				Config: testAccCheckPerformanceMonitorConfig(rndName, serviceId, updatedUrl),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ioriver.PerformanceMonitor](resourceName, &performanceMonitor, testedObj),
					resource.TestCheckResourceAttr(resourceName, "url", updatedUrl),
				),
			},
		},
	})
}

func testAccCheckPerformanceMonitorConfig(rndName string, serviceId string, url string) string {
	return fmt.Sprintf(`
	resource "ioriver_performance_monitor" "%s" {
		service        = "%s"
		name           = "test-perf-monitor"
		url            = "%s"
	}`, rndName, serviceId, url)
}
