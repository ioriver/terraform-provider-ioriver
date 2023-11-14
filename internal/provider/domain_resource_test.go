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

var domainResourceType string = "ioriver_domain"

func init() {
	var testedObj TestedDomain
	excludeId := os.Getenv("IORIVER_TEST_DOMAIN_ID")
	resource.AddTestSweepers(domainResourceType, &resource.Sweeper{
		Name: domainResourceType,
		F: func(r string) error {
			return testSweepResources[ioriver.Domain](r, testedObj, []string{excludeId})
		},
	})
}

type TestedDomain struct {
	TestedObj[ioriver.Domain]
}

func (TestedDomain) Get(client *ioriver.IORiverClient, id string) (*ioriver.Domain, error) {
	serviceId := os.Getenv("IORIVER_TEST_SERVICE_ID")
	return client.GetDomain(serviceId, id)
}

func (TestedDomain) List(client *ioriver.IORiverClient) ([]ioriver.Domain, error) {
	serviceId := os.Getenv("IORIVER_TEST_SERVICE_ID")
	return client.ListDomains(serviceId)
}

func (TestedDomain) Delete(client *ioriver.IORiverClient, object ioriver.Domain, excludeIds []string) error {
	idx := slices.IndexFunc(excludeIds, func(id string) bool { return id == object.Id })
	if idx < 0 {
		serviceId := os.Getenv("IORIVER_TEST_SERVICE_ID")
		return client.DeleteDomain(serviceId, object.Id)
	} else {
		return nil
	}
}

func TestAccIORiverDomain_Basic(t *testing.T) {
	var domain ioriver.Domain
	var testedObj TestedDomain

	testDomain := os.Getenv("IORIVER_TEST_DOMAIN")
	testOriginId := os.Getenv("IORIVER_TEST_ORIGIN_ID")
	serviceId := os.Getenv("IORIVER_TEST_SERVICE_ID")
	subDomain := "tf-test-2." + testDomain
	rndName := generateRandomResourceName()
	resourceName := domainResourceType + "." + rndName

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(s *terraform.State) error {
			return testAccCheckResourceDestroy[ioriver.Domain](s, testedObj, domainResourceType)
		},
		Steps: []resource.TestStep{
			{
				Config: testAccCheckDomainConfig(rndName, serviceId, subDomain, testOriginId),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ioriver.Domain](resourceName, &domain, testedObj),
					resource.TestCheckResourceAttr(resourceName, "domain", subDomain),
				),
			},
			{
				ResourceName:        "ioriver_domain." + rndName,
				ImportStateIdPrefix: fmt.Sprintf("%s,", serviceId),
				ImportState:         true,
				ImportStateVerify:   true,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ioriver.Domain](resourceName, &domain, testedObj),
				),
			},
		},
	})
}

func TestAccIORiverDomain_Update(t *testing.T) {
	var domain ioriver.Domain
	var testedObj TestedDomain

	testDomain := os.Getenv("IORIVER_TEST_DOMAIN")
	testOriginId := os.Getenv("IORIVER_TEST_ORIGIN_ID")
	serviceId := os.Getenv("IORIVER_TEST_SERVICE_ID")
	subDomain := "tf-test-2." + testDomain
	rndName := generateRandomResourceName()
	resourceName := domainResourceType + "." + rndName

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(s *terraform.State) error {
			return testAccCheckResourceDestroy[ioriver.Domain](s, testedObj, domainResourceType)
		},
		Steps: []resource.TestStep{
			{
				Config: testAccCheckDomainConfig(rndName, serviceId, subDomain, testOriginId),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ioriver.Domain](resourceName, &domain, testedObj),
				),
			},
			{
				Config: testAccCheckDomainConfigUpdate(rndName, serviceId, subDomain),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ioriver.Domain](resourceName, &domain, testedObj),
				),
			},
		},
	})
}

func testAccCheckDomainConfig(rndName string, serviceId string, domain string, origin string) string {
	return fmt.Sprintf(`
	resource "ioriver_domain" "%s" {
		service        = "%s"
		domain         = "%s"
		origin         = "%s"
	}`, rndName, serviceId, domain, origin)
}

func testAccCheckDomainConfigUpdate(rndName string, serviceId string, domain string) string {
	return fmt.Sprintf(`
	resource "ioriver_origin" domain_update_origin {
		service        = "%s"
		host           = "example.com"
	}
	resource "ioriver_domain" "%s" {
		service        = "%s"
		domain         = "%s"
		origin         = ioriver_origin.domain_update_origin.id
	}`, serviceId, rndName, serviceId, domain)
}
