package provider

import (
	"fmt"
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	ioriver "github.com/ioriver/ioriver-go"
	"golang.org/x/exp/slices"
)

var serviceResourceType string = "ioriver_service"

func init() {
	var testedObj TestedService
	excludeId := os.Getenv("IORIVER_TEST_SERVICE_ID")
	resource.AddTestSweepers(serviceResourceType, &resource.Sweeper{
		Name: serviceResourceType,
		F: func(r string) error {
			return testSweepResources[ServiceWithConfig](r, testedObj, []string{excludeId})
		},
	})
}

type TestedService struct {
	TestedObj[ServiceWithConfig]
}

func (TestedService) Get(client *ioriver.IORiverClient, id string) (*ServiceWithConfig, error) {
	return GetServiceWithConfig(client, id)
}

func (TestedService) List(client *ioriver.IORiverClient) ([]ServiceWithConfig, error) {
	return ListServicesWithConfig(client)
}

func (TestedService) Delete(client *ioriver.IORiverClient, object ServiceWithConfig, excludeIds []string) error {
	idx := slices.IndexFunc(excludeIds, func(id string) bool { return id == object.Id })
	if idx < 0 {
		return client.DeleteService(object.Id)
	} else {
		return nil
	}
}

// Protocol test — verifies protocol settings are persisted and that omitting
// the protocol block entirely does not cause a "null value" provider crash.
func TestAccIORiverService_Protocol(t *testing.T) {
	var service ServiceWithConfig
	var testedObj TestedService

	certId := os.Getenv("IORIVER_TEST_CERT_ID")
	rndName := generateRandomResourceName()
	resourceName := serviceResourceType + "." + rndName

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckV2(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(s *terraform.State) error {
			return testAccCheckResourceDestroy[ServiceWithConfig](s, testedObj, serviceResourceType)
		},
		Steps: []resource.TestStep{
			{
				// Step 1: protocol block explicitly set
				Config: testAccCheckServiceConfigWithProtocol(rndName, certId, true, true, true),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ServiceWithConfig](resourceName, &service, testedObj),
					resource.TestCheckResourceAttr(resourceName, "config.protocol.http2_enabled", "true"),
					resource.TestCheckResourceAttr(resourceName, "config.protocol.http3_enabled", "true"),
					resource.TestCheckResourceAttr(resourceName, "config.protocol.ipv6_enabled", "true"),
				),
			},
			{
				// Step 2: omit protocol block entirely — must not crash with null error
				Config: testAccCheckServiceConfigWithoutProtocol(rndName, certId),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ServiceWithConfig](resourceName, &service, testedObj),
					resource.TestCheckResourceAttr(resourceName, "config.protocol.http2_enabled", "true"),
					resource.TestCheckResourceAttr(resourceName, "config.protocol.http3_enabled", "false"),
					resource.TestCheckResourceAttr(resourceName, "config.protocol.ipv6_enabled", "true"),
				),
			},
		},
	})
}

// Basic service test - without nested items
func TestAccIORiverService_Basic(t *testing.T) {
	var service ServiceWithConfig
	var testedObj TestedService

	certId := os.Getenv("IORIVER_TEST_CERT_ID")
	rndName := generateRandomResourceName()
	resourceName := serviceResourceType + "." + rndName

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckV2(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(s *terraform.State) error {
			return testAccCheckResourceDestroy[ServiceWithConfig](s, testedObj, serviceResourceType)
		},
		Steps: []resource.TestStep{
			{
				Config: testAccCheckServiceConfigBasic(rndName, certId),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ServiceWithConfig](resourceName, &service, testedObj),
					resource.TestCheckResourceAttr(resourceName, "name", rndName),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.default.actions.cache_ttl", "86400"),
				),
			},
		},
	})
}

func TestAccIORiverService_WithOrigins(t *testing.T) {
	var service ServiceWithConfig
	var testedObj TestedService

	certId := os.Getenv("IORIVER_TEST_CERT_ID")
	rndName := generateRandomResourceName()
	originHosts := []string{"example.com", "example2.com", "example3.com", "example4.com"}
	// originIDs := []string{"origin-1", "origin-2", "origin-3"}
	resourceName := serviceResourceType + "." + rndName

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckV2(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(s *terraform.State) error {
			return testAccCheckResourceDestroy[ServiceWithConfig](s, testedObj, serviceResourceType)
		},
		Steps: []resource.TestStep{
			{ // Set service with origin #1
				Config: testAccServiceConfigOriginsSteps(0, rndName, rndName, certId),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ServiceWithConfig](resourceName, &service, testedObj),
					resource.TestCheckResourceAttr(resourceName, "config.origins.0.custom_origin.host", originHosts[0]),
				),
			},
			{ // Add origin #2 - s3, origin-1 change host
				Config: testAccServiceConfigOriginsSteps(1, rndName, rndName, certId),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ServiceWithConfig](resourceName, &service, testedObj),
					resource.TestCheckResourceAttr(resourceName, "config.origins.0.custom_origin.host", originHosts[3]),
					resource.TestCheckResourceAttr(resourceName, "config.origins.1.s3_origin.host", originHosts[1]),
				),
			},
			{ // Remove origin #1 - only origin-2 (s3) remains at index 0
				Config: testAccServiceConfigOriginsSteps(2, rndName, rndName, certId),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ServiceWithConfig](resourceName, &service, testedObj),
					resource.TestCheckResourceAttr(resourceName, "config.origins.0.s3_origin.host", originHosts[1]),
					resource.TestCheckResourceAttr(resourceName, "config.origins.#", "1"),
				),
			},
			{ // Add origin #3 - origin-2 (s3) at 0, origin-3 (custom) at 1
				Config: testAccServiceConfigOriginsSteps(3, rndName, rndName, certId),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ServiceWithConfig](resourceName, &service, testedObj),
					resource.TestCheckResourceAttr(resourceName, "config.origins.0.s3_origin.host", originHosts[1]),
					resource.TestCheckResourceAttr(resourceName, "config.origins.1.custom_origin.host", originHosts[2]),
					resource.TestCheckResourceAttr(resourceName, "config.origins.#", "2"),
				),
			},
			{ // Expect empty plan - swap didnt change anything check
				// (pure reorder — isPureReorder=true path: plan modifier suppresses diff).
				Config:             testAccServiceConfigOriginsSteps(4, rndName, rndName, certId),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
			{ // Swap order AND add shield to origin #3 simultaneously
				// (isPureReorder=false path: config order wins — origin-3+shield at [0], origin-2 at [1]).
				// State before: [origin-2, origin-3]. HCL: [origin-3+shield, origin-2].
				// Proves the plan modifier handles reorder+update in a single apply.
				Config: testAccServiceConfigOriginsSteps(5, rndName, rndName, certId),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ServiceWithConfig](resourceName, &service, testedObj),
					resource.TestCheckResourceAttr(resourceName, "config.origins.0.custom_origin.host", originHosts[2]),
					resource.TestCheckResourceAttr(resourceName, "config.origins.1.s3_origin.host", originHosts[1]),
					resource.TestCheckResourceAttr(resourceName, "config.origins.0.shield.location.country", "US"),
					resource.TestCheckResourceAttr(resourceName, "config.origins.0.shield.location.subdivision", "VA"),
					resource.TestCheckResourceAttr(resourceName, "config.origins.#", "2"),
				),
			},
			{ // Add cloudflare to shield providers (not connected, validates list expansion)
				Config: testAccServiceConfigOriginsSteps(6, rndName, rndName, certId),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ServiceWithConfig](resourceName, &service, testedObj),
					resource.TestCheckResourceAttr(resourceName, "config.origins.0.custom_origin.host", originHosts[2]),
					resource.TestCheckResourceAttr(resourceName, "config.origins.1.s3_origin.host", originHosts[1]),
					resource.TestCheckResourceAttr(resourceName, "config.origins.0.shield.location.country", "US"),
					resource.TestCheckResourceAttr(resourceName, "config.origins.0.shield.location.subdivision", "VA"),
				),
			},
		},
	})
}

// TestAccIORiverService_WithDomains is a focused regression test for the
// "duplicate uuid" bug: when replacing a domain at the same list index, the
// new domain must NOT inherit the old domain's uuid via positional state copy.
//
// Steps:
//  1. Create [d0] — single domain baseline.
//  2. Expand to [d0, d1, d2] — three domains.
//  3. [d0,d1,d2] → [d0,d2,d3]: d2 shifts idx=2→1, d3 is NEW at idx=2.
//     Without the fix: state[2]=d2(uuid=C) → d3 gets uuid=C → backend 500.
//  4. Shrink to [d0, d3] — two domains remain.
func TestAccIORiverService_WithDomains(t *testing.T) {
	var service ServiceWithConfig
	var testedObj TestedService

	certId := os.Getenv("IORIVER_TEST_CERT_ID")
	rndName := generateRandomResourceName()
	resourceName := serviceResourceType + "." + rndName
	origins := []string{"example.com", "example2.com"}
	domains := []string{
		rndName + ".hey.com",
		rndName + "2.hey.com",
		rndName + "3.hey.com",
		rndName + "4.hey.com",
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckV2(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(s *terraform.State) error {
			return testAccCheckResourceDestroy[ServiceWithConfig](s, testedObj, serviceResourceType)
		},
		Steps: []resource.TestStep{
			{
				// Step 1: Create with a single domain [d0].
				Config: testAccServiceConfigDomainsSteps(0, rndName, rndName, certId,
					origins[0], origins[1], domains[0]),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ServiceWithConfig](resourceName, &service, testedObj),
					resource.TestCheckResourceAttr(resourceName, "config.domains.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "config.domains.0.domain", domains[0]),
				),
			},
			{
				// Step 2: Expand to three domains [d0, d1, d2].
				Config: testAccServiceConfigDomainsSteps(2, rndName, rndName, certId,
					origins[0], origins[1], domains[0], domains[1], domains[2]),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ServiceWithConfig](resourceName, &service, testedObj),
					resource.TestCheckResourceAttr(resourceName, "config.domains.#", "3"),
					resource.TestCheckResourceAttr(resourceName, "config.domains.0.domain", domains[0]),
					resource.TestCheckResourceAttr(resourceName, "config.domains.1.domain", domains[1]),
					resource.TestCheckResourceAttr(resourceName, "config.domains.2.domain", domains[2]),
				),
			},
			{
				// Step 3: [d0,d1,d2] → [d0,d2,d3].
				// d2 moves from idx=2 to idx=1; d3 is brand-new at idx=2.
				// d3 lands at the same index d2 occupied in state — without the fix,
				// UseStateForUnknown() positionally gives d3 d2's uuid, causing a
				// backend 500 "duplicate object with uuid".
				Config: testAccServiceConfigDomainsSteps(2, rndName, rndName, certId,
					origins[0], origins[1], domains[0], domains[2], domains[3]),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ServiceWithConfig](resourceName, &service, testedObj),
					resource.TestCheckResourceAttr(resourceName, "config.domains.#", "3"),
					resource.TestCheckResourceAttr(resourceName, "config.domains.0.domain", domains[0]),
					resource.TestCheckResourceAttr(resourceName, "config.domains.1.domain", domains[2]),
					resource.TestCheckResourceAttr(resourceName, "config.domains.2.domain", domains[3]),
				),
			},
			{
				// Step 4: Shrink to two domains [d0, d3] — drop d2.
				// Uses step 1 template: d0 at [0] → origin_1, d3 at [1] → origin_1.
				Config: testAccServiceConfigDomainsSteps(1, rndName, rndName, certId,
					origins[0], origins[1], domains[0], domains[3]),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ServiceWithConfig](resourceName, &service, testedObj),
					resource.TestCheckResourceAttr(resourceName, "config.domains.#", "2"),
					resource.TestCheckResourceAttr(resourceName, "config.domains.0.domain", domains[0]),
					resource.TestCheckResourceAttr(resourceName, "config.domains.1.domain", domains[3]),
				),
			},
		},
	})
}

func TestAccIORiverService_DomainMultiMapping(t *testing.T) {
	var service ServiceWithConfig
	var testedObj TestedService

	certId := os.Getenv("IORIVER_TEST_CERT_ID")
	rndName := generateRandomResourceName()
	resourceName := serviceResourceType + "." + rndName
	domainHost := rndName + ".example.com"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckV2(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(s *terraform.State) error {
			return testAccCheckResourceDestroy[ServiceWithConfig](s, testedObj, serviceResourceType)
		},
		Steps: []resource.TestStep{
			{
				// Step 0: domain has 4 mappings — 2 origin sets + 2 standalone origins.
				Config: testAccServiceConfigMultiMappingSteps(0, rndName, certId, domainHost),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ServiceWithConfig](resourceName, &service, testedObj),

					resource.TestCheckResourceAttr(resourceName, "config.domains.0.domain", domainHost),
					resource.TestCheckResourceAttr(resourceName, "config.domains.0.mappings.#", "4"),

					resource.TestCheckResourceAttr(resourceName, "config.origin_sets.0.name", "set-alpha"),
					resource.TestCheckResourceAttrPair(resourceName, "config.domains.0.mappings.2.target_mapping", resourceName, "config.origins.0.name"),

					resource.TestCheckResourceAttr(resourceName, "config.origin_sets.1.name", "set-beta"),
					resource.TestCheckResourceAttrPair(resourceName, "config.domains.0.mappings.3.target_mapping", resourceName, "config.origins.1.name"),

					resource.TestCheckResourceAttr(resourceName, "config.origins.0.name", "origin-a"),
					resource.TestCheckResourceAttrPair(resourceName, "config.domains.0.mappings.0.target_mapping", resourceName, "config.origin_sets.0.name"),
					resource.TestCheckResourceAttr(resourceName, "config.origins.1.name", "origin-b"),
					resource.TestCheckResourceAttrPair(resourceName, "config.domains.0.mappings.1.target_mapping", resourceName, "config.origin_sets.1.name"),
				),
			},
			{
				// Step 0b: plan-only — expect no diff.
				Config:             testAccServiceConfigMultiMappingSteps(0, rndName, certId, domainHost),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
			{
				// Step 1: trim to 2 mappings (1 origin set + 1 standalone origin).
				Config: testAccServiceConfigMultiMappingSteps(1, rndName, certId, domainHost),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ServiceWithConfig](resourceName, &service, testedObj),
					resource.TestCheckResourceAttr(resourceName, "config.domains.0.domain", domainHost),
					resource.TestCheckResourceAttr(resourceName, "config.domains.0.mappings.#", "2"),

					resource.TestCheckResourceAttr(resourceName, "config.origin_sets.0.name", "set-alpha"),
					resource.TestCheckResourceAttrPair(resourceName, "config.domains.0.mappings.0.target_mapping", resourceName, "config.origin_sets.0.name"),

					resource.TestCheckResourceAttr(resourceName, "config.origins.0.name", "origin-a"),
					resource.TestCheckResourceAttrPair(resourceName, "config.domains.0.mappings.1.target_mapping", resourceName, "config.origins.0.name"),
				),
			},
		},
	})
}

func TestAccIORiverService_WithBehaviors(t *testing.T) {
	var service ServiceWithConfig
	var testedObj TestedService

	certId := os.Getenv("IORIVER_TEST_CERT_ID")
	rndName := generateRandomResourceName()
	resourceName := serviceResourceType + "." + rndName

	rndSubBehaviorName := generateRandomResourceName()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckV2(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(s *terraform.State) error {
			return testAccCheckResourceDestroy[ServiceWithConfig](s, testedObj, serviceResourceType)
		},
		Steps: []resource.TestStep{
			// Step 0: Create service with only a specific behavior (no default_behavior in HCL)
			{
				Config: testAccCheckBehaviorConfigWithBehaviors(rndName, certId, rndSubBehaviorName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ServiceWithConfig](resourceName, &service, testedObj),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.actions.origin_cache_control", "true"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.actions.stale_ttl", "300"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.actions.follow_redirects", "true"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.actions.deny_access", "false"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.actions.true_client_ip", "true"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.actions.deny_access_by_ip.0.ip", "1.2.3.4"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.actions.deny_access_by_time.0.date_time_window.start_date", "2000000000"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.actions.deny_access_by_time.0.date_time_window.end_date", "2100000000"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.actions.url_rewrites.0.source", "/api/test/123"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.actions.url_rewrites.0.destination", "/api/test/1234"),
				),
			},
			// Step 1: all available actions in a single behavior
			{
				Config: testAccAllActionsConfig(rndName, certId, rndSubBehaviorName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ServiceWithConfig](resourceName, &service, testedObj),
					// scalars
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.actions.cache_behavior", "CACHE"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.actions.cache_ttl", "3600"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.actions.browser_cache_ttl", "1800"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.actions.large_files_optimization", "true"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.actions.origin_cache_control", "true"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.actions.stale_ttl", "600"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.actions.follow_redirects", "true"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.actions.compression", "true"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.actions.deny_access", "false"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.actions.true_client_ip", "true"),
					// allowed_methods
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.actions.allowed_methods.#", "3"),
					// deny_access_by_ip
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.actions.deny_access_by_ip.0.ip", "1.2.3.4"),
					// deny_access_by_time
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.actions.deny_access_by_time.0.date_time_window.start_date", "2000000000"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.actions.deny_access_by_time.0.date_time_window.end_date", "2100000000"),
					// url_rewrites
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.actions.url_rewrites.0.source", "/old/path"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.actions.url_rewrites.0.destination", "/new/path"),
					// cache_key
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.actions.cache_key.query_strings.type", "include"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.actions.cache_key.country", "true"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.actions.cache_key.device_type", "false"),
					// host_header
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.actions.host_header.header_value", "origin.example.com"),
					// cors
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.actions.cors.allow_origin.mode", "all"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.actions.cors.allow_credentials", "true"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.actions.cors.max_age.value", "86400"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.actions.cors.max_age.override", "true"),
					// request_headers (set — use set-element check)
					resource.TestCheckTypeSetElemNestedAttrs(resourceName, "config.behaviors.custom.0.actions.request_headers.*", map[string]string{
						"name":   "X-CDN-Origin",
						"action": "set",
					}),
					// response_headers
					resource.TestCheckTypeSetElemNestedAttrs(resourceName, "config.behaviors.custom.0.actions.response_headers.*", map[string]string{
						"name":   "X-Frame-Options",
						"action": "set",
					}),
					// origin_response_headers
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.actions.origin_response_headers.0.name", "X-Internal-Debug"),
					resource.TestCheckTypeSetElemNestedAttrs(resourceName, "config.behaviors.custom.0.actions.origin_response_headers.*", map[string]string{
						"name":   "X-Internal-Debug",
						"action": "delete",
					}),
					// generate_preflight_response
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.actions.generate_preflight_response.max_age", "3600"),
				),
			},
		},
	})
}

// TestAccIORiverService_EmptyActionsIsRejected verifies that an actions block with
// no fields set is rejected at plan time by the provider validator.
func TestAccIORiverService_EmptyActionsIsRejected(t *testing.T) {
	certId := os.Getenv("IORIVER_TEST_CERT_ID")
	rndName := generateRandomResourceName()
	behaviorName := generateRandomResourceName()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckV2(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "ioriver_service" "%s" {
  name        = "%s"
  certificate = "%s"
  config = {
    behaviors = {
      custom = [
        {
          name         = "%s"
          path_pattern = "/lifecycle/test/*"
          actions = {}
        }
      ]
    }
  }
}`, rndName, rndName, certId, behaviorName),
				ExpectError: regexp.MustCompile(`actions must have at least one field set`),
			},
		},
	})
}

// TestAccIORiverService_DomainMappingUnknownOriginIsRejected verifies that
// ValidateConfig catches a domain mapping that references an origin name which
// does not exist in config.origins at plan time — no API call is made.
func TestAccIORiverService_DomainMappingUnknownOriginIsRejected(t *testing.T) {
	certId := os.Getenv("IORIVER_TEST_CERT_ID")
	rndName := generateRandomResourceName()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckV2(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "ioriver_service" "%s" {
  name        = "%s"
  certificate = "%s"
  config = {
    origins = [
      {
        name = "real-origin"
        custom_origin = {
          host     = "example.com"
          protocol = "https"
        }
      }
    ]
    domains = [
      {
        domain = "cdn.example.com"
        mappings = [
          {
            target_mapping = "nonexistent-origin"
          }
        ]
      }
    ]
  }
}`, rndName, rndName, certId),
				ExpectError: regexp.MustCompile(`Unknown origin reference`),
			},
		},
	})
}

// TestAccIORiverService_StreamLogsUnknownLogDestIsRejected verifies that
// ValidateConfig catches a behavior stream_logs block that references a log
// destination name which does not exist in config.log_destinations at plan
// time — no API call is made.
func TestAccIORiverService_StreamLogsUnknownLogDestIsRejected(t *testing.T) {
	certId := os.Getenv("IORIVER_TEST_CERT_ID")
	rndName := generateRandomResourceName()
	behaviorName := generateRandomResourceName()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckV2(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "ioriver_service" "%s" {
  name        = "%s"
  certificate = "%s"
  config = {
    log_destinations = [
      {
        name = "real-dest"
        aws_s3 = {
          name   = "my-bucket"
          path   = "/"
          region = "us-east-1"
        }
      }
    ]
    origins = []
    behaviors = {
      custom = [
        {
          name         = "%s"
          path_pattern = "/logs/*"
          actions = {
            stream_logs = {
              log_destination = "nonexistent-dest"
              log_sampling_rate   = 100
            }
          }
        }
      ]
    }
  }
}`, rndName, rndName, certId, behaviorName),
				ExpectError: regexp.MustCompile(`Unknown log destination reference`),
			},
		},
	})
}

// TestAccIORiverService_DefaultBehaviorLifecycle exercises the default_behavior lifecycle:
//
//	Step 0: Create without default_behavior (omit it — backend fills defaults).
//	        Verifies the 5 backend-always-set fields are Computed in state.
//	Step 1: default block present but actions omitted — DefaultActionsObject should
//	        fill all 5 defaults in the plan without (known after apply).
//	Step 2: Set default_behavior explicitly with some actions.
//	Step 3: Update default_behavior actions (change values).
//	Step 4: Set cache_ttl explicitly to a non-default value.
//	Step 5: Remove cache_ttl — must revert to default (86400), not null/drift.
//	Step 6: Remove default_behavior from HCL (back to omitted) — no drift expected.
//	Step 6b: Plan-only with the same omitted config — must produce no diff.
//	Step 7: Set an optional field (viewer_protocol) that the backend never fills.
//	Step 8: Remove viewer_protocol — must become null, not keep old value.
//	Step 8b: Plan-only with the same config — must produce no diff.
func TestAccIORiverService_DefaultBehaviorLifecycle(t *testing.T) {
	var service ServiceWithConfig
	var testedObj TestedService

	certId := os.Getenv("IORIVER_TEST_CERT_ID")
	rndName := generateRandomResourceName()
	resourceName := serviceResourceType + "." + rndName

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckV2(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(s *terraform.State) error {
			return testAccCheckResourceDestroy[ServiceWithConfig](s, testedObj, serviceResourceType)
		},
		Steps: []resource.TestStep{
			{ // Step 0: omit behaviors entirely — backend fills the 5 defaults
				Config: testAccDefaultBehaviorLifecycleConfig("omit", rndName, certId),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ServiceWithConfig](resourceName, &service, testedObj),
					resource.TestCheckResourceAttr(resourceName, "name", rndName),
					// The 5 backend-always-set fields must be Computed into state
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.default.actions.cache_ttl", "86400"),
					resource.TestCheckResourceAttrSet(resourceName, "config.behaviors.default.actions.cache_key.query_strings.type"),
					resource.TestCheckResourceAttrSet(resourceName, "config.behaviors.default.actions.allowed_methods.#"),
					resource.TestCheckResourceAttrSet(resourceName, "config.behaviors.default.actions.cached_methods.#"),
					resource.TestCheckResourceAttrSet(resourceName, "config.behaviors.default.actions.status_codes_ttl.#"),
				),
			},
			{ // Step 1: default block present but no actions — DefaultActionsObject fires
				Config:             testAccDefaultBehaviorLifecycleConfig("no_actions", rndName, certId),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
			{ // Step 2: set default_behavior explicitly
				Config:             testAccDefaultBehaviorLifecycleConfig("set", rndName, certId),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
			{ // Step 3: update default_behavior (change cache_behavior + compression)
				Config: testAccDefaultBehaviorLifecycleConfig("update", rndName, certId),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ServiceWithConfig](resourceName, &service, testedObj),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.default.actions.cache_behavior", "BYPASS"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.default.actions.compression", "false"),
				),
			},
			{ // Step 4: set cache_ttl + compression explicitly
				Config: testAccDefaultBehaviorLifecycleConfig("set_cache_ttl_and_compression", rndName, certId),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ServiceWithConfig](resourceName, &service, testedObj),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.default.actions.cache_ttl", "3600"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.default.actions.compression", "true"),
				),
			},
			{ // Step 5: remove only cache_ttl, modify compression — cache_ttl must revert to 86400
				Config: testAccDefaultBehaviorLifecycleConfig("remove_cache_ttl_modify_compression", rndName, certId),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ServiceWithConfig](resourceName, &service, testedObj),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.default.actions.cache_ttl", "86400"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.default.actions.compression", "false"),
				),
			},
			{ // Step 6: remove default_behavior from HCL — compression will be reset to true
				Config: testAccDefaultBehaviorLifecycleConfig("omit", rndName, certId),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ServiceWithConfig](resourceName, &service, testedObj),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.default.actions.compression", "true"),
				),
			},
			{ // Step 7: set an optional field (viewer_protocol) — backend never fills this
				Config: testAccDefaultBehaviorLifecycleConfig("set_optional", rndName, certId),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ServiceWithConfig](resourceName, &service, testedObj),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.default.actions.viewer_protocol", "HTTPS_ONLY"),
					// mandatory defaults still present
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.default.actions.cache_ttl", "86400"),
				),
			},
			{ // Step 8: remove viewer_protocol — must be null, not keep old value
				Config: testAccDefaultBehaviorLifecycleConfig("remove_optional", rndName, certId),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ServiceWithConfig](resourceName, &service, testedObj),
					resource.TestCheckNoResourceAttr(resourceName, "config.behaviors.default.actions.viewer_protocol"),
					// mandatory defaults still present after removing the optional field
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.default.actions.cache_ttl", "86400"),
				),
			},
		},
	})
}

// TestAccIORiverService_WithLogDestinations is a full flow test:
//
//	Step 0:   Create service with one log destination (aws_s3), NO credentials.
//	Step 1:   Same log destination, WITH credentials.
//	Step 1b:  Omit credentials — tests optional-on-update semantics: does the API
//	          accept an update without credentials after they've been set?
//	          Pass → API keeps existing; Fail → must always include credentials in HCL.
//	Step 2:   Add a behavior that streams to it.
//	Step 3:   Add a second log destination (aws_s3).
//	Step 4:   Import — works cleanly here: no prior DesiredLogDestOrder so items are
//	          sorted alphabetically (alpha=0, beta=1), which matches the step 3 HCL order.
//	Step 5:   Swap the order of the two log destinations in the HCL (regression
//	          for list-based ordering — should not re-create either dest).
//	Step 5b:  Plan-only with the same swapped config — must produce no diff
//	          (core ordering regression: alignItems must match state to HCL order).
//	Step 6:   Swap order AND update alpha's bucket name simultaneously (proves
//	          reorder+update works in a single apply — the plan modifier fix).
//	Step 7:   Replace log dest 1 with a compatible_s3 type.
//	Step 8:   Remove log dest 1 — only log dest 2 remains.
func TestAccIORiverService_WithLogDestinations(t *testing.T) {
	var service ServiceWithConfig
	var testedObj TestedService

	certId := os.Getenv("IORIVER_TEST_CERT_ID")
	rndName := generateRandomResourceName()
	resourceName := serviceResourceType + "." + rndName
	logDestName1 := "log-dest-alpha"
	logDestName2 := "log-dest-beta"
	behaviorName := generateRandomResourceName()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckV2(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(s *terraform.State) error {
			return testAccCheckResourceDestroy[ServiceWithConfig](s, testedObj, serviceResourceType)
		},
		Steps: []resource.TestStep{
			{ // Step 0: one log destination, no credentials
				Config: testAccServiceConfigLogDestSteps(0, rndName, certId, logDestName1, logDestName2, behaviorName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ServiceWithConfig](resourceName, &service, testedObj),
					resource.TestCheckResourceAttr(resourceName, "config.log_destinations.0.name", logDestName1),
				),
			},
			{ // Step 1: same log destination, with credentials
				Config: testAccServiceConfigLogDestSteps(1, rndName, certId, logDestName1, logDestName2, behaviorName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ServiceWithConfig](resourceName, &service, testedObj),
					resource.TestCheckResourceAttr(resourceName, "config.log_destinations.0.name", logDestName1),
				),
			},
			{ // Step 1b: omit credentials — tests whether the API accepts an update
				// without credentials after they've been set (optional-on-update semantics).
				// Pass → API keeps existing credentials; Fail with "credentials are missing"
				// → API requires them on every update and all HCL steps must include them.
				Config: testAccServiceConfigLogDestSteps(2, rndName, certId, logDestName1, logDestName2, behaviorName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ServiceWithConfig](resourceName, &service, testedObj),
					resource.TestCheckResourceAttr(resourceName, "config.log_destinations.0.name", logDestName1),
				),
			},
			{ // Step 2: add a behavior that streams logs to the destination
				Config: testAccServiceConfigLogDestSteps(3, rndName, certId, logDestName1, logDestName2, behaviorName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ServiceWithConfig](resourceName, &service, testedObj),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.name", behaviorName),
					resource.TestCheckResourceAttrPair(resourceName, "config.log_destinations.0.name",
						resourceName, "config.behaviors.custom.0.actions.stream_logs.log_destination"),
				),
			},
			{ // Step 3: add a second log destination (behavior still streams to first)
				Config: testAccServiceConfigLogDestSteps(4, rndName, certId, logDestName1, logDestName2, behaviorName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ServiceWithConfig](resourceName, &service, testedObj),
					resource.TestCheckResourceAttr(resourceName, "config.log_destinations.1.name", logDestName2),
				),
			},
			{ // Step 4: import — placed here intentionally, before the swap.
				// With no prior DesiredLogDestOrder, alignItems sorts alphabetically:
				// alpha=0, beta=1 — which matches the step 3 HCL order exactly.
				// No name-index ignores needed.
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{
					"config.log_destinations.0.aws_s3.credentials",
					"config.log_destinations.1.aws_s3.credentials",
					// Security config (WAF/checkpoint) is managed via UI and returned by the
					// backend even when not defined in HCL — ignore the entire security block.
					"config.security",
				},
			},
			{ // Step 5 (plan-only): swap order — must produce no diff.
				// Core ordering regression: if the plan modifier is broken, Terraform would
				// see state order (alpha,beta) mismatched with HCL and plan changes.
				Config:             testAccServiceConfigLogDestSteps(5, rndName, certId, logDestName1, logDestName2, behaviorName),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
			{ // Step 6: swap order AND update alpha's bucket name simultaneously.
				// Proves the plan modifier handles reorder+update in one apply step.
				// isPureReorder=false → config order wins: beta at [0], alpha at [1].
				Config: testAccServiceConfigLogDestSteps(6, rndName, certId, logDestName1, logDestName2, behaviorName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ServiceWithConfig](resourceName, &service, testedObj),
					// name assertions anchor the indices so the aws_s3 check is stable.
					resource.TestCheckResourceAttr(resourceName, "config.log_destinations.0.name", logDestName2),
					resource.TestCheckResourceAttr(resourceName, "config.log_destinations.1.name", logDestName1),
					resource.TestCheckResourceAttr(resourceName, "config.log_destinations.1.aws_s3.name", "test-log-bucket-UPDATED"),
				),
			},
			{ // Step 7: replace log dest 1 with compatible_s3 type
				Config: testAccServiceConfigLogDestSteps(7, rndName, certId, logDestName1, logDestName2, behaviorName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ServiceWithConfig](resourceName, &service, testedObj),
					resource.TestCheckResourceAttr(resourceName, "config.log_destinations.1.compatible_s3.name", "compat-bucket"),
				),
			},
			{ // Step 8: remove log dest 1 — only log dest 2 should remain
				Config: testAccServiceConfigLogDestSteps(8, rndName, certId, logDestName1, logDestName2, behaviorName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ServiceWithConfig](resourceName, &service, testedObj),
					resource.TestCheckResourceAttr(resourceName, "config.log_destinations.0.name", logDestName2),
					resource.TestCheckResourceAttr(resourceName, "config.log_destinations.#", "1"),
				),
			},
		},
	})
}

func testAccCheckServiceConfigBasic(resourceName string, certId string) string {
	return fmt.Sprintf(`
locals {
	origin_1_id = "origin-1"
}

resource "%s" "%s" {
	name = "%s"
    certificate = "%s"
	description = "basic service create"

    config = {
    }
}`, serviceResourceType, resourceName, resourceName, certId)
}

// TestAccIORiverService_WithOriginSets is a full lifecycle test for origin sets.
//
//	Step 0:   Create service with one origin set, domain mapped to it (default failover codes).
//	Step 0b:  Plan-only — no diff expected.
//	Step 0c:  Add a second origin set (set1=[0], set2=[1]).
//	Step 0d:  Plan-only pure swap — NamedListPlanModifier isPureReorder=true, diff suppressed.
//	Step 0e:  Swap order AND add failover codes to set-2 simultaneously —
//	          isPureReorder=false, config order wins: set-2=[0], set-1=[1].
//	Step 0f:  Plan-only idempotency after the swap+update apply — must produce no diff.
//	Step 1:   Remove set-2 AND update set-1's failover_response_codes simultaneously —
//	          combined delete+update in one apply, back to one set with codes [503, 504].
//	Step 2:   Swap primary host inside the origin set.
//	Step 3:   Re-point domain mapping to a standalone origin (target_type=origin).
//	Step 4:   Remove the origin set entirely (origin_sets=[]).
func TestAccIORiverService_WithOriginSets(t *testing.T) {
	var service ServiceWithConfig
	var testedObj TestedService

	certId := os.Getenv("IORIVER_TEST_CERT_ID")
	rndName := generateRandomResourceName()
	resourceName := serviceResourceType + "." + rndName

	const (
		setName        = "my-failover-set"
		primaryH       = "primary.example.com"
		failoverH      = "failover.example.com"
		primaryH2      = "primary2.example.com"
		standaloneHost = "standalone.example.com"
	)
	domainHost := rndName + ".example.com"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckV2(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(s *terraform.State) error {
			return testAccCheckResourceDestroy[ServiceWithConfig](s, testedObj, serviceResourceType)
		},
		Steps: []resource.TestStep{
			{
				// Step 0: Create service with one origin set, domain mapped to it.
				Config: testAccServiceConfigOriginSetsSteps(0, rndName, certId, domainHost),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ServiceWithConfig](resourceName, &service, testedObj),
					resource.TestCheckResourceAttr(resourceName, "config.origin_sets.0.name", setName),
					resource.TestCheckResourceAttr(resourceName, "config.origin_sets.0.origins.0.custom_origin.host", primaryH),
					resource.TestCheckResourceAttr(resourceName, "config.origin_sets.0.origins.1.custom_origin.host", failoverH),
					resource.TestCheckResourceAttr(resourceName, "config.origin_sets.0.failover_response_codes.#", "4"),
					resource.TestCheckResourceAttrPair(resourceName, "config.domains.0.mappings.0.target_mapping", resourceName, "config.origin_sets.0.name"),
					resource.TestCheckResourceAttr(resourceName, "config.domains.0.mappings.0.target_type", "origin_set"),
				),
			},
			{
				// Step 0b: plan-only after no changes — must be empty diff.
				Config:             testAccServiceConfigOriginSetsSteps(0, rndName, certId, domainHost),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
			{
				// Step 0c: add a second origin set — set1 at [0], set2 at [1].
				Config: testAccServiceConfigOriginSetsSteps(5, rndName, certId, domainHost),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ServiceWithConfig](resourceName, &service, testedObj),
					resource.TestCheckResourceAttr(resourceName, "config.origin_sets.0.name", "my-failover-set"),
					resource.TestCheckResourceAttr(resourceName, "config.origin_sets.1.name", "my-failover-set-2"),
				),
			},
			{
				// Step 0d (plan-only): swap the two origin sets in HCL — pure reorder, no value
				// changes. NamedListPlanModifier isPureReorder=true path: canonicalises plan back
				// to state order so Terraform sees no diff.
				Config:             testAccServiceConfigOriginSetsSteps(6, rndName, certId, domainHost),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
			{
				// Step 0e: swap order AND add failover_response_codes to set-2 simultaneously.
				// NamedListPlanModifier isPureReorder=false path: config order wins.
				// set-2 (with 2 new codes) lands at [0], set-1 stays at [1].
				Config: testAccServiceConfigOriginSetsSteps(7, rndName, certId, domainHost),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ServiceWithConfig](resourceName, &service, testedObj),
					resource.TestCheckResourceAttr(resourceName, "config.origin_sets.0.name", "my-failover-set-2"),
					resource.TestCheckResourceAttr(resourceName, "config.origin_sets.0.failover_response_codes.#", "2"),
					resource.TestCheckResourceAttr(resourceName, "config.origin_sets.1.name", "my-failover-set"),
				),
			},
			{
				// Step 0f: plan-only idempotency after swap+update — must produce no diff.
				Config:             testAccServiceConfigOriginSetsSteps(7, rndName, certId, domainHost),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
			{
				// Step 1: remove set-2 AND update set-1's failover_response_codes simultaneously.
				// Combined delete+update in one apply: only set-1 remains with codes [503, 504].
				Config: testAccServiceConfigOriginSetsSteps(1, rndName, certId, domainHost),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ServiceWithConfig](resourceName, &service, testedObj),
					resource.TestCheckResourceAttr(resourceName, "config.origin_sets.0.name", setName),
					resource.TestCheckResourceAttr(resourceName, "config.origin_sets.0.failover_response_codes.#", "2"),
					resource.TestCheckResourceAttr(resourceName, "config.origin_sets.0.failover_response_codes.0", "503"),
					resource.TestCheckResourceAttr(resourceName, "config.origin_sets.0.failover_response_codes.1", "504"),
				),
			},
			{
				// Step 2: Swap primary host.
				Config: testAccServiceConfigOriginSetsSteps(2, rndName, certId, domainHost),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ServiceWithConfig](resourceName, &service, testedObj),
					resource.TestCheckResourceAttr(resourceName, "config.origin_sets.0.origins.0.custom_origin.host", primaryH2),
				),
			},
			{
				// Step 3: Re-point domain mapping to a standalone origin (target_type=origin).
				Config: testAccServiceConfigOriginSetsSteps(3, rndName, certId, domainHost),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ServiceWithConfig](resourceName, &service, testedObj),
					resource.TestCheckResourceAttr(resourceName, "config.origins.0.custom_origin.host", standaloneHost),
					resource.TestCheckResourceAttrPair(resourceName, "config.domains.0.mappings.0.target_mapping", resourceName, "config.origins.0.name"),
					resource.TestCheckResourceAttr(resourceName, "config.domains.0.mappings.0.target_type", "origin"),
				),
			},
			{
				// Step 4: Remove the origin set entirely (origin_sets = []).
				Config: testAccServiceConfigOriginSetsSteps(4, rndName, certId, domainHost),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ServiceWithConfig](resourceName, &service, testedObj),
					resource.TestCheckResourceAttr(resourceName, "config.origin_sets.#", "0"),
				),
			},
		},
	})
}

// TestAccIORiverService_OriginSetTooFewOriginsIsRejected verifies that
// ValidateConfig catches an origin set with fewer than 2 origins at plan time.
func TestAccIORiverService_OriginSetTooFewOriginsIsRejected(t *testing.T) {
	certId := os.Getenv("IORIVER_TEST_CERT_ID")
	rndName := generateRandomResourceName()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckV2(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "ioriver_service" "%s" {
  name        = "%s"
  certificate = "%s"
  config = {
    origin_sets = [
      {
        name = "my-set"
        origins = [
          {
            custom_origin = {
              host     = "only-one.example.com"
              protocol = "https"
            }
          }
        ]
      }
    ]
  }
}`, rndName, rndName, certId),
				ExpectError: regexp.MustCompile(`Origin set requires at least 2 origins`),
			},
		},
	})
}

// TestAccIORiverService_WafConditions exhaustively tests all supported condition
// field types, operators, and action combinations in a single service apply.
// It also exercises the rule UPDATE path (Step 2): mutates rule 0's operator and
// a rate-limit's num_of_requests, then verifies the changes round-trip correctly.
func TestAccIORiverService_WafConditions(t *testing.T) {
	var service ServiceWithConfig
	var testedObj TestedService

	certId := os.Getenv("IORIVER_TEST_CERT_ID")
	rndName := generateRandomResourceName()
	resourceName := serviceResourceType + "." + rndName

	cfg := func(idx int) string { return testAccWafConditions(rndName, certId, idx) }

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckV2(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(s *terraform.State) error {
			return testAccCheckResourceDestroy[ServiceWithConfig](s, testedObj, serviceResourceType)
		},
		Steps: []resource.TestStep{
			{ // Step 0: apply full conditions matrix.
				Config: cfg(0),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ServiceWithConfig](resourceName, &service, testedObj),
					resource.TestCheckResourceAttr(resourceName, "config.security.enabled", "true"),
					resource.TestCheckResourceAttr(resourceName, "config.security.custom_rules.#", "25"),
					resource.TestCheckResourceAttr(resourceName, "config.security.rate_limit.#", "4"),
					resource.TestCheckResourceAttr(resourceName, "config.security.custom_rules.0.name", "cond-path-begins"),
					resource.TestCheckResourceAttr(resourceName, "config.security.custom_rules.0.condition.or.0.and.0.operator", "begins_with"),
					resource.TestCheckResourceAttr(resourceName, "config.security.custom_rules.4.name", "cond-header-contains"),
					resource.TestCheckResourceAttr(resourceName, "config.security.custom_rules.15.name", "cond-multi-or-and"),
					resource.TestCheckResourceAttr(resourceName, "config.security.custom_rules.24.name", "cond-bot-ge"),
					resource.TestCheckResourceAttr(resourceName, "config.security.rate_limit.0.name", "rl-block"),
					resource.TestCheckResourceAttr(resourceName, "config.security.rate_limit.0.num_of_requests", "100"),
					resource.TestCheckResourceAttr(resourceName, "config.security.rate_limit.3.name", "rl-interactive"),
				),
			},
			{ // Step 1: idempotency — re-apply same config, must produce no diff.
				Config:   cfg(0),
				PlanOnly: true,
			},
			{ // Step 2: mutate two fields — proves the rule UPDATE path works.
				// Rule 0 (cond-path-begins): operator begins_with → contains.
				// rl-block: num_of_requests 100 → 200.
				// All other 24 rules and 3 rate-limits must be unchanged.
				Config: cfg(1),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ServiceWithConfig](resourceName, &service, testedObj),
					resource.TestCheckResourceAttr(resourceName, "config.security.custom_rules.#", "25"),
					resource.TestCheckResourceAttr(resourceName, "config.security.rate_limit.#", "4"),
					// mutated fields
					resource.TestCheckResourceAttr(resourceName, "config.security.custom_rules.0.name", "cond-path-begins"),
					resource.TestCheckResourceAttr(resourceName, "config.security.custom_rules.0.condition.or.0.and.0.operator", "contains"),
					resource.TestCheckResourceAttr(resourceName, "config.security.rate_limit.0.num_of_requests", "200"),
					// unchanged neighbours — spot-check to confirm surgical update
					resource.TestCheckResourceAttr(resourceName, "config.security.custom_rules.1.name", "cond-uri-contains"),
					resource.TestCheckResourceAttr(resourceName, "config.security.rate_limit.1.name", "rl-log"),
					resource.TestCheckResourceAttr(resourceName, "config.security.rate_limit.1.num_of_requests", "1000"),
				),
			},
			{ // Step 2b: idempotency after rule + rate-limit mutation — no drift.
				Config:             cfg(1),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
			{ // Step 3: import — verify full conditions state round-trips correctly.
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
				// On import there is no prior state, so `value` shorthand fields cannot
				// be restored (imported state uses `values` exclusively). We ignore both
				// `value` (present pre-import, absent post-import) and `values.*` (absent
				// pre-import, present post-import) for rule 0's condition.
				ImportStateVerifyIgnore: []string{
					"config.behaviors",
					"config.security.custom_rules.0.condition.or.0.and.0.value",
				},
			},
		},
	})
}

// TestAccIORiverService_WafConfig focuses on the WAF configuration surface:
// checkpoint defaults, enable/disable toggle, and checkpoint field mutations.
// It does NOT test custom rules or rate-limits in depth.
//
//	Step 0:  create with no waf block (null) → backend fills checkpoint defaults.
//	         Verifies defaults round-trip correctly in state.
//	Step 1:  plan-only with waf = {} (empty block) → Default fires; same payload, no diff.
//	Step 2:  plan-only with every default spelled out explicitly in HCL → same payload, no diff.
//	         Steps 1+2 prove all three forms (null/empty/explicit-defaults) are equivalent.
//	Step 3:  toggle enabled=false.
//	Step 4:  re-enable (false→true).
//	Step 5:  mutate checkpoint: learn→prevent, limit_body_size, trusted_sources, minimal_num_sources.
//	Step 6:  import (ImportStateVerify also catches any post-apply drift).
func TestAccIORiverService_WafConfig(t *testing.T) {
	var service ServiceWithConfig
	var testedObj TestedService

	certId := os.Getenv("IORIVER_TEST_CERT_ID")
	rndName := generateRandomResourceName()
	resourceName := serviceResourceType + "." + rndName

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckV2(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(s *terraform.State) error {
			return testAccCheckResourceDestroy[ServiceWithConfig](s, testedObj, serviceResourceType)
		},
		Steps: []resource.TestStep{
			{ // Step 0: no waf block → Default fires; checkpoint defaults must appear.
				Config: testAccWafBlockOmitted(rndName, certId),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ServiceWithConfig](resourceName, &service, testedObj),
					resource.TestCheckResourceAttr(resourceName, "config.security.enabled", "true"),
					resource.TestCheckResourceAttr(resourceName, "config.security.waf.limit_body_size", "false"),
					resource.TestCheckResourceAttr(resourceName, "config.security.waf.checkpoint.web_attacks.mode", "learn"),
					resource.TestCheckResourceAttr(resourceName, "config.security.waf.checkpoint.web_attacks.confidence_level", "high"),
					resource.TestCheckResourceAttr(resourceName, "config.security.waf.checkpoint.ips.mode", "learn"),
					resource.TestCheckResourceAttr(resourceName, "config.security.waf.checkpoint.ips.performance_impact", "medium"),
					resource.TestCheckResourceAttr(resourceName, "config.security.waf.checkpoint.ips.high_confidence_action", "block"),
					resource.TestCheckResourceAttr(resourceName, "config.security.waf.checkpoint.ips.low_confidence_action", "log"),
					resource.TestCheckResourceAttr(resourceName, "config.security.waf.checkpoint.minimal_num_sources", "3"),
				),
			},
			{ // Step 1: waf = {} — checkpoint Default fires; payload must be identical → no diff.
				Config:             testAccWafNoCheckpoint(rndName, certId, true),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
			{ // Step 2: every default spelled out explicitly in HCL — payload still identical → no diff.
				// Proves all three forms (null / empty / explicit-defaults) produce the same API payload.
				Config:             testAccWafExplicitDefaults(rndName, certId),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
			{ // Step 3: toggle enabled=false.
				Config: testAccWafNoCheckpoint(rndName, certId, false),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ServiceWithConfig](resourceName, &service, testedObj),
					resource.TestCheckResourceAttr(resourceName, "config.security.enabled", "false"),
					resource.TestCheckResourceAttr(resourceName, "config.security.waf.checkpoint.web_attacks.mode", "learn"),
				),
			},
			{ // Step 4: re-enable (false→true).
				Config: testAccWafNoCheckpoint(rndName, certId, true),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ServiceWithConfig](resourceName, &service, testedObj),
					resource.TestCheckResourceAttr(resourceName, "config.security.enabled", "true"),
				),
			},
			{ // Step 5: mutate checkpoint (learn→prevent, limit_body_size, trusted_sources, minimal_num_sources).
				Config: testAccServiceConfigWithWaf(rndName, certId, 2),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ServiceWithConfig](resourceName, &service, testedObj),
					resource.TestCheckResourceAttr(resourceName, "config.security.enabled", "true"),
					resource.TestCheckResourceAttr(resourceName, "config.security.waf.limit_body_size", "true"),
					resource.TestCheckResourceAttr(resourceName, "config.security.waf.checkpoint.web_attacks.mode", "prevent"),
					resource.TestCheckResourceAttr(resourceName, "config.security.waf.checkpoint.web_attacks.confidence_level", "medium"),
					resource.TestCheckResourceAttr(resourceName, "config.security.waf.checkpoint.ips.mode", "prevent"),
					resource.TestCheckResourceAttr(resourceName, "config.security.waf.checkpoint.minimal_num_sources", "5"),
				),
			},
			{ // Step 6: import — ImportStateVerify also catches any post-apply drift.
				ResourceName:            resourceName,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"config.behaviors"},
			},
		},
	})
}

// TestAccIORiverService_WafRules tests custom-rule and rate-limit lifecycle:
//
//	Step 0:  create 7 custom rules (one per action: block/log/allow/bypass_managed/ignore/
//	         challenge/interactive_challenge) + 4 rate-limits (block/log/challenge/interactive_challenge).
//	Step 1:  apply reversed rule order — proves positional ordering is enforced (first-match-wins).
//	Step 2:  restore forward order + mutate r-block operator (begins_with → ends_with) + disable
//	         r-block and r-log (enabled=false). Tests operator mutation and per-rule disable in one shot.
//	Step 2b: idempotency.
//	Step 3:  import.
func TestAccIORiverService_WafRules(t *testing.T) {
	var service ServiceWithConfig
	var testedObj TestedService

	certId := os.Getenv("IORIVER_TEST_CERT_ID")
	rndName := generateRandomResourceName()
	resourceName := serviceResourceType + "." + rndName

	cfg := func(idx int) string { return testAccWafRulesSteps(rndName, certId, idx) }

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckV2(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(s *terraform.State) error {
			return testAccCheckResourceDestroy[ServiceWithConfig](s, testedObj, serviceResourceType)
		},
		Steps: []resource.TestStep{
			{ // Step 0: create all rules and rate-limits.
				Config: cfg(0),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ServiceWithConfig](resourceName, &service, testedObj),
					resource.TestCheckResourceAttr(resourceName, "config.security.enabled", "true"),
					// custom rules — 7 rules, forward order; verify name, action, and full condition for each
					resource.TestCheckResourceAttr(resourceName, "config.security.custom_rules.#", "7"),
					// r-block: http.request.path + begins_with
					resource.TestCheckResourceAttr(resourceName, "config.security.custom_rules.0.name", "r-block"),
					resource.TestCheckResourceAttr(resourceName, "config.security.custom_rules.0.action", "block"),
					resource.TestCheckResourceAttr(resourceName, "config.security.custom_rules.0.condition.or.0.and.0.field", "http.request.path"),
					resource.TestCheckResourceAttr(resourceName, "config.security.custom_rules.0.condition.or.0.and.0.operator", "begins_with"),
					resource.TestCheckTypeSetElemAttr(resourceName, "config.security.custom_rules.0.condition.or.0.and.0.values.*", "/admin"),
					// r-log: http.request.method + in
					resource.TestCheckResourceAttr(resourceName, "config.security.custom_rules.1.name", "r-log"),
					resource.TestCheckResourceAttr(resourceName, "config.security.custom_rules.1.action", "log"),
					resource.TestCheckResourceAttr(resourceName, "config.security.custom_rules.1.condition.or.0.and.0.field", "http.request.method"),
					resource.TestCheckResourceAttr(resourceName, "config.security.custom_rules.1.condition.or.0.and.0.operator", "in"),
					resource.TestCheckTypeSetElemAttr(resourceName, "config.security.custom_rules.1.condition.or.0.and.0.values.*", "DELETE"),
					// r-allow: client.ip.address + ip_match (CIDR)
					resource.TestCheckResourceAttr(resourceName, "config.security.custom_rules.2.name", "r-allow"),
					resource.TestCheckResourceAttr(resourceName, "config.security.custom_rules.2.action", "allow"),
					resource.TestCheckResourceAttr(resourceName, "config.security.custom_rules.2.condition.or.0.and.0.field", "client.ip.address"),
					resource.TestCheckResourceAttr(resourceName, "config.security.custom_rules.2.condition.or.0.and.0.operator", "ip_match"),
					resource.TestCheckTypeSetElemAttr(resourceName, "config.security.custom_rules.2.condition.or.0.and.0.values.*", "10.0.0.0/8"),
					// r-bypass: http.request.uri_raw + contains
					resource.TestCheckResourceAttr(resourceName, "config.security.custom_rules.3.name", "r-bypass"),
					resource.TestCheckResourceAttr(resourceName, "config.security.custom_rules.3.action", "bypass_managed"),
					resource.TestCheckResourceAttr(resourceName, "config.security.custom_rules.3.condition.or.0.and.0.field", "http.request.uri_raw"),
					resource.TestCheckResourceAttr(resourceName, "config.security.custom_rules.3.condition.or.0.and.0.operator", "contains"),
					resource.TestCheckTypeSetElemAttr(resourceName, "config.security.custom_rules.3.condition.or.0.and.0.values.*", "/health"),
					// r-ignore: collection field (query_param + field_key) + exists (empty value list)
					resource.TestCheckResourceAttr(resourceName, "config.security.custom_rules.4.name", "r-ignore"),
					resource.TestCheckResourceAttr(resourceName, "config.security.custom_rules.4.action", "ignore"),
					resource.TestCheckResourceAttr(resourceName, "config.security.custom_rules.4.ignore_params.ignore_type", "query_param"),
					resource.TestCheckResourceAttr(resourceName, "config.security.custom_rules.4.ignore_params.value", "debug"),
					resource.TestCheckResourceAttr(resourceName, "config.security.custom_rules.4.condition.or.0.and.0.field", "http.request.query_param"),
					resource.TestCheckResourceAttr(resourceName, "config.security.custom_rules.4.condition.or.0.and.0.field_key", "debug"),
					resource.TestCheckResourceAttr(resourceName, "config.security.custom_rules.4.condition.or.0.and.0.operator", "exists"),
					resource.TestCheckResourceAttr(resourceName, "config.security.custom_rules.4.condition.or.0.and.0.values.#", "0"),
					// r-challenge: bot.advanced.score + lt (numeric operator)
					resource.TestCheckResourceAttr(resourceName, "config.security.custom_rules.5.name", "r-challenge"),
					resource.TestCheckResourceAttr(resourceName, "config.security.custom_rules.5.action", "challenge"),
					resource.TestCheckResourceAttr(resourceName, "config.security.custom_rules.5.condition.or.0.and.0.field", "bot.advanced.score"),
					resource.TestCheckResourceAttr(resourceName, "config.security.custom_rules.5.condition.or.0.and.0.operator", "lt"),
					resource.TestCheckTypeSetElemAttr(resourceName, "config.security.custom_rules.5.condition.or.0.and.0.values.*", "50"),
					// r-ichallenge: client.geo.country + in (multi-value)
					resource.TestCheckResourceAttr(resourceName, "config.security.custom_rules.6.name", "r-ichallenge"),
					resource.TestCheckResourceAttr(resourceName, "config.security.custom_rules.6.action", "interactive_challenge"),
					resource.TestCheckResourceAttr(resourceName, "config.security.custom_rules.6.condition.or.0.and.0.field", "client.geo.country"),
					resource.TestCheckResourceAttr(resourceName, "config.security.custom_rules.6.condition.or.0.and.0.operator", "in"),
					resource.TestCheckTypeSetElemAttr(resourceName, "config.security.custom_rules.6.condition.or.0.and.0.values.*", "CN"),
					resource.TestCheckTypeSetElemAttr(resourceName, "config.security.custom_rules.6.condition.or.0.and.0.values.*", "KP"),
					resource.TestCheckTypeSetElemAttr(resourceName, "config.security.custom_rules.6.condition.or.0.and.0.values.*", "RU"),
					// rate_limit — 4 rules, all 4 actions; verify numeric fields + condition on each
					resource.TestCheckResourceAttr(resourceName, "config.security.rate_limit.#", "4"),
					resource.TestCheckResourceAttr(resourceName, "config.security.rate_limit.0.name", "rl-block"),
					resource.TestCheckResourceAttr(resourceName, "config.security.rate_limit.0.action", "block"),
					resource.TestCheckResourceAttr(resourceName, "config.security.rate_limit.0.num_of_requests", "100"),
					resource.TestCheckResourceAttr(resourceName, "config.security.rate_limit.0.time_window_seconds", "60"),
					resource.TestCheckResourceAttr(resourceName, "config.security.rate_limit.0.block_duration_seconds", "300"),
					resource.TestCheckResourceAttr(resourceName, "config.security.rate_limit.0.condition.or.0.and.0.field", "http.request.path"),
					resource.TestCheckResourceAttr(resourceName, "config.security.rate_limit.0.condition.or.0.and.0.operator", "begins_with"),
					resource.TestCheckTypeSetElemAttr(resourceName, "config.security.rate_limit.0.condition.or.0.and.0.values.*", "/api/"),
					resource.TestCheckResourceAttr(resourceName, "config.security.rate_limit.1.name", "rl-log"),
					resource.TestCheckResourceAttr(resourceName, "config.security.rate_limit.1.action", "log"),
					resource.TestCheckResourceAttr(resourceName, "config.security.rate_limit.1.num_of_requests", "1000"),
					resource.TestCheckResourceAttr(resourceName, "config.security.rate_limit.1.time_window_seconds", "60"),
					resource.TestCheckResourceAttr(resourceName, "config.security.rate_limit.1.block_duration_seconds", "60"),
					resource.TestCheckResourceAttr(resourceName, "config.security.rate_limit.2.name", "rl-challenge"),
					resource.TestCheckResourceAttr(resourceName, "config.security.rate_limit.2.action", "challenge"),
					resource.TestCheckResourceAttr(resourceName, "config.security.rate_limit.2.num_of_requests", "200"),
					resource.TestCheckResourceAttr(resourceName, "config.security.rate_limit.2.time_window_seconds", "30"),
					resource.TestCheckResourceAttr(resourceName, "config.security.rate_limit.2.block_duration_seconds", "120"),
					resource.TestCheckResourceAttr(resourceName, "config.security.rate_limit.3.name", "rl-ichallenge"),
					resource.TestCheckResourceAttr(resourceName, "config.security.rate_limit.3.action", "interactive_challenge"),
					resource.TestCheckResourceAttr(resourceName, "config.security.rate_limit.3.num_of_requests", "50"),
					resource.TestCheckResourceAttr(resourceName, "config.security.rate_limit.3.time_window_seconds", "60"),
					resource.TestCheckResourceAttr(resourceName, "config.security.rate_limit.3.block_duration_seconds", "600"),
				),
			},
			{ // Step 1: apply reversed order — proves positional ordering is enforced.
				// If order didn't matter, the checks below would fail.
				Config: cfg(1),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ServiceWithConfig](resourceName, &service, testedObj),
					resource.TestCheckResourceAttr(resourceName, "config.security.custom_rules.#", "7"),
					// Verify reversed order: first rule is now r-ichallenge.
					resource.TestCheckResourceAttr(resourceName, "config.security.custom_rules.0.name", "r-ichallenge"),
					resource.TestCheckResourceAttr(resourceName, "config.security.custom_rules.0.action", "interactive_challenge"),
					resource.TestCheckResourceAttr(resourceName, "config.security.custom_rules.6.name", "r-block"),
					resource.TestCheckResourceAttr(resourceName, "config.security.custom_rules.6.action", "block"),
					// rate_limit unchanged
					resource.TestCheckResourceAttr(resourceName, "config.security.rate_limit.0.name", "rl-block"),
				),
			},
			{ // Step 2: restore forward order + mutate r-block operator (begins_with → ends_with)
				// + disable r-block and r-log (enabled=false) + upgrade r-bypass condition to
				// a 2-group OR with a 2-AND first group. Tests reorder, operator mutation,
				// per-rule disable, and multi-condition expression all in a single apply.
				Config: cfg(3),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ServiceWithConfig](resourceName, &service, testedObj),
					resource.TestCheckResourceAttr(resourceName, "config.security.custom_rules.#", "7"),
					// Forward order restored; operator mutated; rules disabled.
					resource.TestCheckResourceAttr(resourceName, "config.security.custom_rules.0.name", "r-block"),
					resource.TestCheckResourceAttr(resourceName, "config.security.custom_rules.0.enabled", "false"),
					resource.TestCheckResourceAttr(resourceName, "config.security.custom_rules.0.condition.or.0.and.0.operator", "ends_with"),
					resource.TestCheckResourceAttr(resourceName, "config.security.custom_rules.1.name", "r-log"),
					resource.TestCheckResourceAttr(resourceName, "config.security.custom_rules.1.enabled", "false"),
					// Unchanged neighbours still enabled.
					resource.TestCheckResourceAttr(resourceName, "config.security.custom_rules.2.name", "r-allow"),
					resource.TestCheckResourceAttr(resourceName, "config.security.custom_rules.2.enabled", "true"),
					// r-bypass: upgraded to 2-group OR with 2-AND first group.
					resource.TestCheckResourceAttr(resourceName, "config.security.custom_rules.3.name", "r-bypass"),
					resource.TestCheckResourceAttr(resourceName, "config.security.custom_rules.3.condition.or.#", "2"),
					// OR group 0: uri_raw AND method
					resource.TestCheckResourceAttr(resourceName, "config.security.custom_rules.3.condition.or.0.and.#", "2"),
					resource.TestCheckResourceAttr(resourceName, "config.security.custom_rules.3.condition.or.0.and.0.field", "http.request.uri_raw"),
					resource.TestCheckResourceAttr(resourceName, "config.security.custom_rules.3.condition.or.0.and.0.operator", "contains"),
					resource.TestCheckTypeSetElemAttr(resourceName, "config.security.custom_rules.3.condition.or.0.and.0.values.*", "/health"),
					resource.TestCheckResourceAttr(resourceName, "config.security.custom_rules.3.condition.or.0.and.1.field", "http.request.method"),
					resource.TestCheckResourceAttr(resourceName, "config.security.custom_rules.3.condition.or.0.and.1.operator", "in"),
					resource.TestCheckTypeSetElemAttr(resourceName, "config.security.custom_rules.3.condition.or.0.and.1.values.*", "GET"),
					// OR group 1: standalone path
					resource.TestCheckResourceAttr(resourceName, "config.security.custom_rules.3.condition.or.1.and.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "config.security.custom_rules.3.condition.or.1.and.0.field", "http.request.path"),
					resource.TestCheckResourceAttr(resourceName, "config.security.custom_rules.3.condition.or.1.and.0.operator", "begins_with"),
					resource.TestCheckTypeSetElemAttr(resourceName, "config.security.custom_rules.3.condition.or.1.and.0.values.*", "/status"),
					resource.TestCheckResourceAttr(resourceName, "config.security.custom_rules.4.name", "r-ignore"),
					resource.TestCheckResourceAttr(resourceName, "config.security.custom_rules.4.enabled", "true"),
					// rate_limit unaffected
					resource.TestCheckResourceAttr(resourceName, "config.security.rate_limit.#", "4"),
					resource.TestCheckResourceAttr(resourceName, "config.security.rate_limit.0.name", "rl-block"),
				),
			},
			{ // Step 2b: idempotency.
				Config:             cfg(3),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
			{ // Step 3: import.
				ResourceName:            resourceName,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"config.behaviors"},
			},
		},
	})
}

// TestAccIORiverService_BehaviorLifecycle tests the full lifecycle of the
// behaviors block inside a service config:
//
//	Step 1 (idx 0): No behaviors block → backend fills defaults; verify computed fields.
//	Step 2 (idx 1): All schema-valid actions on the default behavior.
//	Step 3 (idx 2): Minimal default + 1 custom behavior with every schema-defined action.
//	Step 4 (idx 3): Minimal default + 4 custom behaviors (full at [0], minimal at [1][2][3]).
//	Step 5 (idx 4, plan-only): Swap custom [0] and [1] → behaviors are positional (no
//	        NamedListPlanModifier), so this produces a non-empty plan (order matters).
//	Step 6 (idx 5): 4 behaviors each using the condition block — complex OR-of-ANDs mixing
//	        7 condition fields, 5+ operators, multi-group OR, multi-condition AND, field_key.
//	        Values asserted at every position. TTL isolation: each behavior's own cache_ttl.
//	Step 6b (plan-only): same config — verifies conditions round-trip with no drift.
//	Step 7 (idx 6): mutate behavior[0]'s condition in-place (add AND clause, flip operator/values)
//	        while leaving behaviors[1][2][3] untouched — exercises the condition UPDATE path.
//	Step 7b (plan-only): idempotency after the condition update — no drift.
func TestAccIORiverService_BehaviorLifecycle(t *testing.T) {
	var service ServiceWithConfig
	var testedObj TestedService

	certId := os.Getenv("IORIVER_TEST_CERT_ID")
	if certId == "" {
		t.Skip("IORIVER_TEST_CERT_ID not set")
	}

	rndName := generateRandomResourceName()
	resourceName := serviceResourceType + "." + rndName

	behaviorNames := []string{
		generateRandomResourceName(),
		generateRandomResourceName(),
		generateRandomResourceName(),
		generateRandomResourceName(),
	}

	cfg := func(idx int) string {
		return testAccBehaviorLifecycleConfig(idx, rndName, certId, behaviorNames)
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckV2(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(s *terraform.State) error {
			return testAccCheckResourceDestroy[ServiceWithConfig](s, testedObj, serviceResourceType)
		},
		Steps: []resource.TestStep{
			{ // Step 1: no behaviors block — verify defaults are populated.
				Config: cfg(0),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ServiceWithConfig](resourceName, &service, testedObj),
					resource.TestCheckResourceAttrSet(resourceName, "config.behaviors.default.actions.cache_ttl"),
					resource.TestCheckResourceAttrSet(resourceName, "config.behaviors.default.actions.cache_behavior"),
					resource.TestCheckResourceAttrSet(resourceName, "config.behaviors.default.actions.cache_key.query_strings.type"),
					resource.TestCheckResourceAttrSet(resourceName, "config.behaviors.default.actions.allowed_methods.#"),
					resource.TestCheckResourceAttrSet(resourceName, "config.behaviors.default.actions.cached_methods.#"),
					resource.TestCheckResourceAttrSet(resourceName, "config.behaviors.default.actions.status_codes_ttl.#"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.#", "0"),
				),
			},
			{ // Step 2: all schema-valid actions on the default behavior.
				Config: cfg(1),
				Check: resource.ComposeTestCheckFunc(
					// scalars
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.default.actions.cache_behavior", "CACHE"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.default.actions.cache_ttl", "3600"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.default.actions.browser_cache_ttl", "1800"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.default.actions.stale_ttl", "600"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.default.actions.follow_redirects", "true"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.default.actions.compression", "true"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.default.actions.large_files_optimization", "true"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.default.actions.origin_cache_control", "false"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.default.actions.true_client_ip", "true"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.default.actions.viewer_protocol", "HTTPS_ONLY"),
					// methods
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.default.actions.allowed_methods.#", "3"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.default.actions.cached_methods.#", "2"),
					// cache key
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.default.actions.cache_key.query_strings.type", "include"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.default.actions.cache_key.country", "true"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.default.actions.cache_key.device_type", "false"),
					// host header
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.default.actions.host_header.header_value", "origin.example.com"),
					// cors
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.default.actions.cors.allow_credentials", "true"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.default.actions.cors.max_age.value", "86400"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.default.actions.cors.max_age.override", "true"),
					// preflight
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.default.actions.generate_preflight_response.max_age", "3600"),
					// status code overrides
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.default.actions.status_code_browser_cache.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.default.actions.generate_response.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.default.actions.status_codes_ttl.#", "2"),
					// header modification
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.default.actions.request_headers.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.default.actions.response_headers.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.default.actions.origin_response_headers.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.#", "0"),
				),
			},
			{ // Step 3: minimal default + 1 custom behavior with every schema-defined action.
				Config: cfg(2),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.default.actions.cache_ttl", "86400"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.name", behaviorNames[0]),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.path_pattern", "/api/*"),
					// scalars
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.actions.cache_behavior", "CACHE"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.actions.cache_ttl", "3600"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.actions.browser_cache_ttl", "1800"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.actions.large_files_optimization", "true"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.actions.origin_cache_control", "true"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.actions.stale_ttl", "600"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.actions.follow_redirects", "true"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.actions.compression", "true"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.actions.true_client_ip", "true"),
					// methods
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.actions.allowed_methods.#", "3"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.actions.cached_methods.#", "2"),
					// access control
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.actions.deny_access_by_ip.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.actions.deny_access_by_time.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.actions.deny_access_by_time.0.date_time_window.start_date", "2000000000"),
					// rewrites
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.actions.url_rewrites.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.actions.url_rewrites.0.source", "/old/path"),
					// status code overrides
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.actions.generate_response.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.actions.status_code_browser_cache.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.actions.status_codes_ttl.#", "2"),
					// cache key
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.actions.cache_key.query_strings.type", "include"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.actions.cache_key.country", "true"),
					// host header
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.actions.host_header.header_value", "origin.example.com"),
					// cors
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.actions.cors.allow_credentials", "true"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.actions.cors.max_age.value", "86400"),
					// preflight
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.actions.generate_preflight_response.max_age", "3600"),
					// header modification
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.actions.request_headers.#", "2"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.actions.response_headers.#", "2"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.actions.origin_response_headers.#", "1"),
				),
			},
			{ // Step 4: default + 4 custom behaviors — TTL isolation: each TTL must stay
				// with its own behavior. path_pattern also asserted to anchor identity.
				Config: cfg(3),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.#", "4"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.name", behaviorNames[0]),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.path_pattern", "/api/*"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.actions.cache_ttl", "3600"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.1.name", behaviorNames[1]),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.1.path_pattern", "/images/*"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.1.actions.cache_ttl", "60"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.2.name", behaviorNames[2]),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.2.path_pattern", "/static/*"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.2.actions.cache_ttl", "120"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.3.name", behaviorNames[3]),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.3.path_pattern", "/fonts/*"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.3.actions.cache_ttl", "180"),
				),
			},
			{ // Step 5 (plan-only): swap custom [0] and [1] — behaviors are positional,
				// NOT managed by NamedListPlanModifier. Order matters, so a positional
				// swap of [0]/[1] (different names, paths, and TTLs) produces a real diff.
				Config:             cfg(4),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
			{ // Step 6: 4 custom behaviors each using the condition block (not path_pattern).
				// Goes wide on condition coverage:
				//   behavior[0]: 2-group OR — path+header(field_key) AND country-in
				//   behavior[1]: 1-group OR — path match + query_param(field_key) eq
				//   behavior[2]: 2-group OR — method-in OR country-not_in
				//   behavior[3]: 3-group OR — path matches_one_of+header, ip+domain, query_param+country
				// Operators hit: match, eq, in, not_in, matches_one_of (5).
				// Fields hit:    http.request.path, http.request.header, client.geo.country,
				//                http.request.method, http.request.query_param, client.ip,
				//                http.request.domain (7 of 10).
				// field_key used on: Content-Type header, X-Role header, format query_param,
				//                    debug query_param.
				// Also verifies TTL isolation: 3600/60/120/180 must not bleed across behaviors.
				Config: cfg(5),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.#", "4"),

					// ── behavior[0]: identity + mutual exclusivity ────────────────────────
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.name", behaviorNames[0]),
					resource.TestCheckNoResourceAttr(resourceName, "config.behaviors.custom.0.path_pattern"),
					// 2-group OR
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.condition.or.#", "2"),
					// group[0]: path AND header
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.condition.or.0.and.#", "2"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.condition.or.0.and.0.field", "http.request.path"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.condition.or.0.and.0.operator", "match"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.condition.or.0.and.0.values.#", "1"),
					resource.TestCheckTypeSetElemAttr(resourceName, "config.behaviors.custom.0.condition.or.0.and.0.values.*", "/api/v1/*"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.condition.or.0.and.1.field", "http.request.header"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.condition.or.0.and.1.operator", "eq"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.condition.or.0.and.1.field_key", "Content-Type"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.condition.or.0.and.1.values.#", "1"),
					resource.TestCheckTypeSetElemAttr(resourceName, "config.behaviors.custom.0.condition.or.0.and.1.values.*", "application/json"),
					// group[1]: country (single condition, 3 values)
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.condition.or.1.and.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.condition.or.1.and.0.field", "client.geo.country"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.condition.or.1.and.0.operator", "in"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.condition.or.1.and.0.values.#", "3"),
					resource.TestCheckTypeSetElemAttr(resourceName, "config.behaviors.custom.0.condition.or.1.and.0.values.*", "US"),
					resource.TestCheckTypeSetElemAttr(resourceName, "config.behaviors.custom.0.condition.or.1.and.0.values.*", "CA"),
					resource.TestCheckTypeSetElemAttr(resourceName, "config.behaviors.custom.0.condition.or.1.and.0.values.*", "GB"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.actions.cache_ttl", "3600"),

					// ── behavior[1]: identity + mutual exclusivity ────────────────────────
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.1.name", behaviorNames[1]),
					resource.TestCheckNoResourceAttr(resourceName, "config.behaviors.custom.1.path_pattern"),
					// 1-group OR
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.1.condition.or.#", "1"),
					// group[0]: path AND query_param (field_key)
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.1.condition.or.0.and.#", "2"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.1.condition.or.0.and.0.field", "http.request.path"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.1.condition.or.0.and.0.operator", "match"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.1.condition.or.0.and.0.values.#", "1"),
					resource.TestCheckTypeSetElemAttr(resourceName, "config.behaviors.custom.1.condition.or.0.and.0.values.*", "/images/*"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.1.condition.or.0.and.1.field", "http.request.query_param"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.1.condition.or.0.and.1.operator", "eq"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.1.condition.or.0.and.1.field_key", "format"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.1.condition.or.0.and.1.values.#", "1"),
					resource.TestCheckTypeSetElemAttr(resourceName, "config.behaviors.custom.1.condition.or.0.and.1.values.*", "webp"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.1.actions.cache_ttl", "60"),

					// ── behavior[2]: identity + mutual exclusivity ────────────────────────
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.2.name", behaviorNames[2]),
					resource.TestCheckNoResourceAttr(resourceName, "config.behaviors.custom.2.path_pattern"),
					// 2-group OR, each with 1 single condition
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.2.condition.or.#", "2"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.2.condition.or.0.and.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.2.condition.or.0.and.0.field", "http.request.method"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.2.condition.or.0.and.0.operator", "in"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.2.condition.or.0.and.0.values.#", "2"),
					resource.TestCheckTypeSetElemAttr(resourceName, "config.behaviors.custom.2.condition.or.0.and.0.values.*", "GET"),
					resource.TestCheckTypeSetElemAttr(resourceName, "config.behaviors.custom.2.condition.or.0.and.0.values.*", "HEAD"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.2.condition.or.1.and.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.2.condition.or.1.and.0.field", "client.geo.country"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.2.condition.or.1.and.0.operator", "not_in"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.2.condition.or.1.and.0.values.#", "2"),
					resource.TestCheckTypeSetElemAttr(resourceName, "config.behaviors.custom.2.condition.or.1.and.0.values.*", "CN"),
					resource.TestCheckTypeSetElemAttr(resourceName, "config.behaviors.custom.2.condition.or.1.and.0.values.*", "RU"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.2.actions.cache_ttl", "120"),

					// ── behavior[3]: identity + mutual exclusivity ────────────────────────
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.3.name", behaviorNames[3]),
					resource.TestCheckNoResourceAttr(resourceName, "config.behaviors.custom.3.path_pattern"),
					// 3-group OR — most complex expression
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.3.condition.or.#", "3"),
					// group[0]: path matches_one_of (multi-value) + header X-Role
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.3.condition.or.0.and.#", "2"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.3.condition.or.0.and.0.field", "http.request.path"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.3.condition.or.0.and.0.operator", "matches_one_of"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.3.condition.or.0.and.0.values.#", "2"),
					resource.TestCheckTypeSetElemAttr(resourceName, "config.behaviors.custom.3.condition.or.0.and.0.values.*", "/admin/*"),
					resource.TestCheckTypeSetElemAttr(resourceName, "config.behaviors.custom.3.condition.or.0.and.0.values.*", "/superadmin/*"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.3.condition.or.0.and.1.field", "http.request.header"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.3.condition.or.0.and.1.operator", "eq"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.3.condition.or.0.and.1.field_key", "X-Role"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.3.condition.or.0.and.1.values.#", "1"),
					resource.TestCheckTypeSetElemAttr(resourceName, "config.behaviors.custom.3.condition.or.0.and.1.values.*", "admin"),
					// group[1]: client.ip + domain
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.3.condition.or.1.and.#", "2"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.3.condition.or.1.and.0.field", "client.ip"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.3.condition.or.1.and.0.operator", "eq"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.3.condition.or.1.and.0.values.#", "1"),
					resource.TestCheckTypeSetElemAttr(resourceName, "config.behaviors.custom.3.condition.or.1.and.0.values.*", "10.0.0.1"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.3.condition.or.1.and.1.field", "http.request.domain"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.3.condition.or.1.and.1.operator", "eq"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.3.condition.or.1.and.1.values.#", "1"),
					resource.TestCheckTypeSetElemAttr(resourceName, "config.behaviors.custom.3.condition.or.1.and.1.values.*", "internal.example.com"),
					// group[2]: query_param debug + country-in
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.3.condition.or.2.and.#", "2"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.3.condition.or.2.and.0.field", "http.request.query_param"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.3.condition.or.2.and.0.operator", "eq"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.3.condition.or.2.and.0.field_key", "debug"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.3.condition.or.2.and.0.values.#", "1"),
					resource.TestCheckTypeSetElemAttr(resourceName, "config.behaviors.custom.3.condition.or.2.and.0.values.*", "1"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.3.condition.or.2.and.1.field", "client.geo.country"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.3.condition.or.2.and.1.operator", "in"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.3.condition.or.2.and.1.values.#", "1"),
					resource.TestCheckTypeSetElemAttr(resourceName, "config.behaviors.custom.3.condition.or.2.and.1.values.*", "US"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.3.actions.cache_ttl", "180"),
				),
			},
			{ // Step 6b (plan-only): same complex conditions — verifies the condition
				// expression round-trips from the backend with no drift.
				Config:             cfg(5),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
			{ // Step 7: update conditions in-place — proves conditions can be mutated on
				// existing behaviors, not just created. Changes behavior[0]'s condition:
				//   - group[0]: add a third AND clause (method ne POST)
				//   - group[1]: country-in [US,CA,GB] → country-not_in [CN,RU]
				// Behavior names stay the same; only the condition expression changes.
				Config: cfg(6),
				Check: resource.ComposeTestCheckFunc(
					// ── behavior[0]: mutated — verify both new and preserved clauses ──────
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.name", behaviorNames[0]),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.condition.or.#", "2"),
					// group[0]: now 3 clauses — clauses [0] and [1] must be unchanged
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.condition.or.0.and.#", "3"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.condition.or.0.and.0.field", "http.request.path"),
					resource.TestCheckTypeSetElemAttr(resourceName, "config.behaviors.custom.0.condition.or.0.and.0.values.*", "/api/v1/*"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.condition.or.0.and.1.field", "http.request.header"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.condition.or.0.and.1.field_key", "Content-Type"),
					resource.TestCheckTypeSetElemAttr(resourceName, "config.behaviors.custom.0.condition.or.0.and.1.values.*", "application/json"),
					// clause [2] is the new one
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.condition.or.0.and.2.field", "http.request.method"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.condition.or.0.and.2.operator", "ne"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.condition.or.0.and.2.values.#", "1"),
					resource.TestCheckTypeSetElemAttr(resourceName, "config.behaviors.custom.0.condition.or.0.and.2.values.*", "POST"),
					// group[1]: same field but operator+values changed
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.condition.or.1.and.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.condition.or.1.and.0.field", "client.geo.country"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.condition.or.1.and.0.operator", "not_in"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.condition.or.1.and.0.values.#", "2"),
					resource.TestCheckTypeSetElemAttr(resourceName, "config.behaviors.custom.0.condition.or.1.and.0.values.*", "CN"),
					resource.TestCheckTypeSetElemAttr(resourceName, "config.behaviors.custom.0.condition.or.1.and.0.values.*", "RU"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.0.actions.cache_ttl", "3600"),
					// ── behaviors [1][2][3]: verify real values, not just one attr each ──
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.1.name", behaviorNames[1]),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.1.condition.or.0.and.1.field_key", "format"),
					resource.TestCheckTypeSetElemAttr(resourceName, "config.behaviors.custom.1.condition.or.0.and.1.values.*", "webp"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.1.actions.cache_ttl", "60"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.2.name", behaviorNames[2]),
					resource.TestCheckTypeSetElemAttr(resourceName, "config.behaviors.custom.2.condition.or.0.and.0.values.*", "GET"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.2.condition.or.1.and.0.operator", "not_in"),
					resource.TestCheckTypeSetElemAttr(resourceName, "config.behaviors.custom.2.condition.or.1.and.0.values.*", "CN"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.2.actions.cache_ttl", "120"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.3.name", behaviorNames[3]),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.3.condition.or.#", "3"),
					resource.TestCheckTypeSetElemAttr(resourceName, "config.behaviors.custom.3.condition.or.0.and.0.values.*", "/admin/*"),
					resource.TestCheckTypeSetElemAttr(resourceName, "config.behaviors.custom.3.condition.or.0.and.0.values.*", "/superadmin/*"),
					resource.TestCheckResourceAttr(resourceName, "config.behaviors.custom.3.actions.cache_ttl", "180"),
				),
			},
			{ // Step 7b (plan-only): idempotency after condition update — no drift.
				Config:             cfg(6),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
		},
	})
}

// GeoFencing test — verifies that the optional geo_fencing block behaves
// correctly across the three lifecycle transitions that exercise the full
// design:
//
//	Step 1: create with mode=deny + 2 countries — populated block round-trips.
//	Step 2: update mode to "allow" and swap the country set — verifies both
//	        mode flip and set-membership update paths in one apply.
//	Step 3: omit the block entirely — the "no default" semantic means the block
//	        must become null in state (no diff loop), unlike the Protocol block
//	        which would default-fill.
func TestAccIORiverService_GeoFencing(t *testing.T) {
	var service ServiceWithConfig
	var testedObj TestedService

	certId := os.Getenv("IORIVER_TEST_CERT_ID")
	rndName := generateRandomResourceName()
	resourceName := serviceResourceType + "." + rndName

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckV2(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(s *terraform.State) error {
			return testAccCheckResourceDestroy[ServiceWithConfig](s, testedObj, serviceResourceType)
		},
		Steps: []resource.TestStep{
			{
				// Step 1: create with deny + {RU, CN}
				Config: testAccCheckServiceConfigWithGeoFencing(rndName, certId, "deny", []string{"RU", "CN"}),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ServiceWithConfig](resourceName, &service, testedObj),
					resource.TestCheckResourceAttr(resourceName, "config.geo_fencing.mode", "deny"),
					resource.TestCheckResourceAttr(resourceName, "config.geo_fencing.countries.#", "2"),
					resource.TestCheckTypeSetElemAttr(resourceName, "config.geo_fencing.countries.*", "RU"),
					resource.TestCheckTypeSetElemAttr(resourceName, "config.geo_fencing.countries.*", "CN"),
				),
			},
			{
				// Step 2: flip mode to allow + swap to {US, DE, FR}
				Config: testAccCheckServiceConfigWithGeoFencing(rndName, certId, "allow", []string{"US", "DE", "FR"}),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ServiceWithConfig](resourceName, &service, testedObj),
					resource.TestCheckResourceAttr(resourceName, "config.geo_fencing.mode", "allow"),
					resource.TestCheckResourceAttr(resourceName, "config.geo_fencing.countries.#", "3"),
					resource.TestCheckTypeSetElemAttr(resourceName, "config.geo_fencing.countries.*", "US"),
					resource.TestCheckTypeSetElemAttr(resourceName, "config.geo_fencing.countries.*", "DE"),
					resource.TestCheckTypeSetElemAttr(resourceName, "config.geo_fencing.countries.*", "FR"),
				),
			},
			{
				// Step 3: omit the block entirely — no default means it must go null.
				// Assert BOTH sub-attributes are absent so a regression that leaks an
				// empty geo_fencing object (e.g. {mode=null, countries=[]}) into
				// state would still fail this step rather than silently passing on
				// the `mode` check alone.
				Config: testAccCheckServiceConfigWithoutGeoFencing(rndName, certId),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ServiceWithConfig](resourceName, &service, testedObj),
					resource.TestCheckNoResourceAttr(resourceName, "config.geo_fencing.mode"),
					resource.TestCheckNoResourceAttr(resourceName, "config.geo_fencing.countries.#"),
				),
			},
		},
	})
}
