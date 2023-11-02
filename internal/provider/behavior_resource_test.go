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

var behaviorResourceType string = "ioriver_behavior"

func init() {
	var testedObj TestedBehavior
	resource.AddTestSweepers(behaviorResourceType, &resource.Sweeper{
		Name: behaviorResourceType,
		F: func(r string) error {
			return testSweepResources[ioriver.Behavior](r, testedObj, []string{})
		},
	})
}

type TestedBehavior struct {
	TestedObj[ioriver.Behavior]
}

func (TestedBehavior) Get(client *ioriver.IORiverClient, id string) (*ioriver.Behavior, error) {
	serviceId := os.Getenv("IORIVER_TEST_SERVICE_ID")
	return client.GetBehavior(serviceId, id)
}

func (TestedBehavior) List(client *ioriver.IORiverClient) ([]ioriver.Behavior, error) {
	serviceId := os.Getenv("IORIVER_TEST_SERVICE_ID")
	return client.ListBehaviors(serviceId)
}

func (TestedBehavior) Delete(client *ioriver.IORiverClient, object ioriver.Behavior, excludeIds []string) error {
	idx := slices.IndexFunc(excludeIds, func(id string) bool { return id == object.Id })
	if idx < 0 {
		serviceId := os.Getenv("IORIVER_TEST_SERVICE_ID")
		return client.DeleteBehavior(serviceId, object.Id)
	} else {
		return nil
	}
}

func TestAccIORiverBehavior_Basic(t *testing.T) {
	var behavior ioriver.Behavior
	var testedObj TestedBehavior

	serviceId := os.Getenv("IORIVER_TEST_SERVICE_ID")
	rndName := generateRandomResourceName()
	resourceName := behaviorResourceType + "." + rndName
	pathPattern := "/api/test/*"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(s *terraform.State) error {
			return testAccCheckResourceDestroy[ioriver.Behavior](s, testedObj, behaviorResourceType)
		},
		Steps: []resource.TestStep{
			{
				Config: testAccCheckBehaviorConfigBasic(rndName, serviceId, pathPattern),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ioriver.Behavior](resourceName, &behavior, testedObj),
					resource.TestCheckResourceAttr(resourceName, "name", rndName),
				),
			},
			{
				ResourceName:        "ioriver_behavior." + rndName,
				ImportStateIdPrefix: fmt.Sprintf("%s,", serviceId),
				ImportState:         true,
				ImportStateVerify:   true,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ioriver.Behavior](resourceName, &behavior, testedObj),
				),
			},
		},
	})
}

func testAccCheckBehaviorConfigBasic(rndName string, serviceId string, path_pattern string) string {
	return fmt.Sprintf(`
	resource "ioriver_behavior" "%[1]s" {
		service      = "%[2]s"
		name         = "%[3]s"
		path_pattern = "%[4]s"
		actions = [
				{
          type                  = "SET_RESPONSE_HEADER"
          response_header_name  = "foo"
          response_header_value = "bar"
        },
        {
          type    = "CACHE_TTL"
          max_ttl = 180
        },
        {
          type    = "BROWSER_CACHE_TTL"
          max_ttl = 120
        },
        {
          type = "REDIRECT_HTTP_TO_HTTPS"
        },
        {
          type                         = "ORIGIN_CACHE_CONTROL"
          origin_cache_control_enabled = true
        },
        {
          type   = "BYPASS_CACHE_ON_COOKIE"
          cookie = "abcd"
        },
        {
          type        = "HOST_HEADER_OVERRIDE"
          host_header = "test.com"
        },
        {
          type                  = "SET_CORS_HEADER"
          response_header_name  = "Access-Control-Allow-Origin"
          response_header_value = "*"
        }
#       {
#          type    = "ORIGIN_ERRORS_PASS_THRU"
#          enabled = true
#       }
		]
	}`, rndName, serviceId, rndName, path_pattern)
}
