package provider

import (
	"fmt"
	"math/rand"
	"os"
	"reflect"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	ioriver "github.com/ioriver/ioriver-go"
	"golang.org/x/exp/slices"
)

var behaviorResourceType string = "ioriver_behavior"

func init() {
	var testedObj TestedBehavior
	excludeId := os.Getenv("IORIVER_TEST_DEFAULT_BEHAVIOR_ID")
	resource.AddTestSweepers(behaviorResourceType, &resource.Sweeper{
		Name: behaviorResourceType,
		F: func(r string) error {
			return testSweepResources[ioriver.Behavior](r, testedObj, []string{excludeId})
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

func TestAccIORiverBehavior_Default(t *testing.T) {
	var behavior ioriver.Behavior
	var testedObj TestedBehavior

	serviceId := os.Getenv("IORIVER_TEST_SERVICE_ID")
	defaultBehaviorId := os.Getenv("IORIVER_TEST_DEFAULT_BEHAVIOR_ID")
	rndName := generateRandomResourceName()
	rndTtl := rand.Intn(100100) + 100
	resourceName := behaviorResourceType + "." + rndName

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(s *terraform.State) error {
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: testAccCheckBehaviorConfigDefault(rndName, serviceId, rndTtl),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ioriver.Behavior](resourceName, &behavior, testedObj),
					testAccCheckActionExists(&behavior, "CACHE_TTL", "MaxTTL", strconv.Itoa(rndTtl)),
				),
			},
			{
				Config: testAccCheckBehaviorConfigDefault(rndName, serviceId, rndTtl+100),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ioriver.Behavior](resourceName, &behavior, testedObj),
					testAccCheckActionExists(&behavior, "CACHE_TTL", "MaxTTL", strconv.Itoa(rndTtl+100)),
				),
			},
			{
				Config: " ",
				Check: resource.ComposeTestCheckFunc(
					// verify that default behavior is back with default values
					testAccCheckActionDefaultValue(testedObj, defaultBehaviorId, "CACHE_TTL", "MaxTTL", "86400"),
				),
			},
		},
	})
}

func testAccCheckActionDefaultValue(testedObj TestedBehavior, id string, actionType string, actionField string, actionValue string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		behavior, err := testedObj.Get(testAccClient, id)
		if err != nil {
			return err
		}
		return verifyAction(behavior, actionType, actionField, actionValue)
	}
}

func testAccCheckActionExists(behavior *ioriver.Behavior, actionType string, actionField string, actionValue string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		return verifyAction(behavior, actionType, actionField, actionValue)
	}
}

func verifyAction(behavior *ioriver.Behavior, actionType string, actionField string, actionValue string) error {
	for _, action := range behavior.Actions {
		if action.Type == ioriver.ActionType(actionType) {
			valueOfAction := reflect.ValueOf(action)
			field := valueOfAction.FieldByName(actionField)
			if field.IsValid() {
				fieldStr := fmt.Sprintf("%v", field.Interface())
				if fieldStr == actionValue {
					return nil
				} else {
					return fmt.Errorf("Incorrect value of %s, expected: %s, got: %s", actionField, actionValue, field.String())
				}
			} else {
				return fmt.Errorf("Invalid field %s", actionField)
			}
		}
	}

	return fmt.Errorf("Action of type %s not found", actionType)
}

func testAccCheckBehaviorConfigBasic(rndName string, serviceId string, path_pattern string) string {
	return fmt.Sprintf(`
	resource "ioriver_behavior" "%[1]s" {
		service      = "%[2]s"
		name         = "%[3]s"
		path_pattern = "%[4]s"
		
		actions = [
			{
				cache_key = {
					headers = [
						{
							header = "host"
						},
						{
							header = "origin"
						},
					],
					cookies = [],
					query_strings = {
						type = "include"
						list = [
							{
								param = "p1"
							},
							{
								param = "p2"
							},
						]
					},
				},
			},
			{
				cache_behavior = "CACHE"
			},
			{
				cached_methods = [
					{
						method = "GET"
					},
					{
						method = "HEAD"
					},
				]
			},
			{
				cache_ttl = 86400
			},
			{
				browser_cache_ttl = 120
			},
			{
				response_header = {
					header_name  = "foo"
					header_value = "bar"
				}
			},
			{
				response_header = {
					header_name  = "foo2"
					header_value = "bar2"
				}
			},
			{
				delete_response_header = "del-foo-resp"
			},
			{
				request_header = {
					header_name  = "req-foo"
					header_value = "req-bar"
				}
			},
			{
				delete_request_header = "del-foo-req"
			},
			{
				origin_cache_control = true
			},
			{
				host_header = {
				  header_value = "test.com"
				}
			},
			{
				cors_header = {
					header_name  = "Access-Control-Allow-Origin"
					header_value = "*"
				}
			},
			{
				status_code_cache = {
					status_code    = "204"
					cache_behavior = "CACHE"
					cache_ttl      = 60
				}
			},
			{
				status_code_cache = {
					status_code    = "4xx"
					cache_behavior = "CACHE"
					cache_ttl      = 10
				}
			},
			{
				generate_preflight_response = {
					allowed_methods = [
						{
							method = "OPTIONS"
						},
						{
							method = "GET"
						},
					],
					max_age = 60
				}
			},
			{
				status_code_browser_cache = {
					status_code       = "2xx"
					browser_cache_ttl = 20
				}
			},
			{
				allowed_methods = [
					{
						method = "GET"
					},
					{
						method = "OPTIONS"
					},
					{
						method = "HEAD"
					},
				]
			},
			{
				compression = false
			},
			{
				viewer_protocol = "HTTPS_ONLY"
			},
			{
				generate_response = {
					status_code        = "403"
					response_page_path = "/custom_403"
				}
			},
			{
				large_files_optimization = true
			}
		]
	}`, rndName, serviceId, rndName, path_pattern)
}

func testAccCheckBehaviorConfigDefault(rndName string, serviceId string, ttl int) string {
	return fmt.Sprintf(`
	resource "ioriver_behavior" "%s" {
		service      = "%s"
		name         = "default"
		path_pattern = "*"
		is_default   = true
		
		actions = [
			{
				cache_behavior = "CACHE"
			},		
			{
				cached_methods = [
					{
						method = "OPTIONS"
					},
					{
						method = "GET"
					},
					{
						method = "HEAD"
					},
				]
			},
			{
				cache_ttl = %d
			},
			{
				cache_key = {
					headers = [
						{
							header = "host"
						},
					],
					cookies = [],
					query_strings = {
						type = "all"
						list = []
					},
				},
			},
			{
				status_code_cache = {
					status_code    = "4xx"
					cache_behavior = "CACHE"
					cache_ttl      = 1
				}
			},
			{
				status_code_cache = {
					status_code    = "5xx"
					cache_behavior = "CACHE"
					cache_ttl      = 1
				}
			},
			{
				allowed_methods = [
					{
						method = "GET"
					},
					{
						method = "HEAD"
					},
				]
			},
			{
				compression = false
			},
			{
				generate_response = {
					status_code        = "403"
					response_page_path = "/custom_403"
				}
			},
			{
				host_header = {
					use_origin_host = true
				}
			},
			{
				viewer_protocol = "REDIRECT_HTTP_TO_HTTPS"
			},		
		]
	}`, rndName, serviceId, ttl)
}
