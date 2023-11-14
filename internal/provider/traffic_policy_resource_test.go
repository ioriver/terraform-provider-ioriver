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

var trafficPolicyResourceType string = "ioriver_traffic_policy"

func init() {
	var testedObj TestedTrafficPolicy
	excludeId := os.Getenv("IORIVER_TEST_DEFAULT_TRAFFIC_POLICY_ID")
	resource.AddTestSweepers(trafficPolicyResourceType, &resource.Sweeper{
		Name: trafficPolicyResourceType,
		F: func(r string) error {
			return testSweepResources[ioriver.TrafficPolicy](r, testedObj, []string{excludeId})
		},
	})
}

type TestedTrafficPolicy struct {
	TestedObj[ioriver.TrafficPolicy]
}

func (TestedTrafficPolicy) Get(client *ioriver.IORiverClient, id string) (*ioriver.TrafficPolicy, error) {
	serviceId := os.Getenv("IORIVER_TEST_SERVICE_ID")
	return client.GetTrafficPolicy(serviceId, id)
}

func (TestedTrafficPolicy) List(client *ioriver.IORiverClient) ([]ioriver.TrafficPolicy, error) {
	serviceId := os.Getenv("IORIVER_TEST_SERVICE_ID")
	return client.ListTrafficPolicies(serviceId)
}

func (TestedTrafficPolicy) Delete(client *ioriver.IORiverClient, object ioriver.TrafficPolicy, excludeIds []string) error {
	idx := slices.IndexFunc(excludeIds, func(id string) bool { return id == object.Id })
	if idx < 0 {
		serviceId := os.Getenv("IORIVER_TEST_SERVICE_ID")
		return client.DeleteTrafficPolicy(serviceId, object.Id)
	} else {
		return nil
	}
}

func TestAccIORiverTrafficPolicy_Basic(t *testing.T) {
	var policy ioriver.TrafficPolicy
	var testedObj TestedTrafficPolicy

	serviceId := os.Getenv("IORIVER_TEST_SERVICE_ID")
	fastlyToken := os.Getenv("IORIVER_TEST_FASTLY_API_TOKEN")
	serviceProviderId := os.Getenv("IORIVER_TEST_SERVICE_PROVIDER_ID")
	rndName := generateRandomResourceName()
	resourceName := trafficPolicyResourceType + "." + rndName
	country := "IL"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(s *terraform.State) error {
			return testAccCheckResourceDestroy[ioriver.TrafficPolicy](s, testedObj, trafficPolicyResourceType)
		},
		Steps: []resource.TestStep{
			{
				Config: testAccCheckTrafficPolicyConfig(rndName, serviceId, fastlyToken, serviceProviderId, country),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ioriver.TrafficPolicy](resourceName, &policy, testedObj),
					resource.TestCheckResourceAttr(resourceName, "service", serviceId),
					resource.TestCheckResourceAttr(resourceName, "geos.0.country", country),
				),
			},
			{
				ResourceName:        "ioriver_traffic_policy." + rndName,
				ImportStateIdPrefix: fmt.Sprintf("%s,", serviceId),
				ImportState:         true,
				ImportStateVerify:   true,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ioriver.TrafficPolicy](resourceName, &policy, testedObj),
				),
			},
		},
	})
}

func TestAccIORiverTrafficPolicy_Update(t *testing.T) {
	var policy ioriver.TrafficPolicy
	var testedObj TestedTrafficPolicy

	serviceId := os.Getenv("IORIVER_TEST_SERVICE_ID")
	fastlyToken := os.Getenv("IORIVER_TEST_FASTLY_API_TOKEN")
	serviceProviderId := os.Getenv("IORIVER_TEST_SERVICE_PROVIDER_ID")
	rndName := generateRandomResourceName()
	resourceName := trafficPolicyResourceType + "." + rndName
	updatedCountry := "US"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(s *terraform.State) error {
			return testAccCheckResourceDestroy[ioriver.TrafficPolicy](s, testedObj, trafficPolicyResourceType)
		},
		Steps: []resource.TestStep{
			{
				Config: testAccCheckTrafficPolicyConfig(rndName, serviceId, fastlyToken, serviceProviderId, "IL"),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ioriver.TrafficPolicy](resourceName, &policy, testedObj),
				),
			},
			{
				Config: testAccCheckTrafficPolicyConfig(rndName, serviceId, fastlyToken, serviceProviderId, updatedCountry),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ioriver.TrafficPolicy](resourceName, &policy, testedObj),
					resource.TestCheckResourceAttr(resourceName, "geos.0.country", updatedCountry),
				),
			},
		},
	})
}

func testAccCheckTrafficPolicyConfig(rndName string, serviceId string, accountProviderToken string, serviceProviderId string, country string) string {
	return fmt.Sprintf(`
	resource "ioriver_account_provider" "traffic_policy_account_provider" {
		provider_name = "fastly"
		credentials = {
		  fastly = "%s"
		}
	}

	resource "ioriver_service_provider" "traffic_policy_service_provider" {
		service          = "%s"
		account_provider = ioriver_account_provider.traffic_policy_account_provider.id

		depends_on = ["ioriver_account_provider.traffic_policy_account_provider"]		
	}

	resource "ioriver_traffic_policy" "%s" {
		service      = "%s"
		type         = "Static"
		failover     = false
		is_default   = false
		providers    = [
			{
				service_provider = ioriver_service_provider.traffic_policy_service_provider.id
				weight           = 100
		  }
		]
		geos = [
			{
				country = "%s"
			},
			{
				continent = "EU"
			}			
		]

		health_monitors = []
		performance_monitors = []

		depends_on = ["ioriver_service_provider.traffic_policy_service_provider"]
	}`, accountProviderToken, serviceId, rndName, serviceId, country)
}
