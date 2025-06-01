package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	ioriver "github.com/ioriver/ioriver-go"
	"golang.org/x/exp/slices"
)

var urlSigningKeyResourceType string = "ioriver_url_signing_key"

func init() {
	var testedObj TestedUrlSigningKey
	resource.AddTestSweepers(urlSigningKeyResourceType, &resource.Sweeper{
		Name: urlSigningKeyResourceType,
		F: func(r string) error {
			return testSweepResources[ioriver.UrlSigningKey](r, testedObj, []string{})
		},
	})
}

type TestedUrlSigningKey struct {
	TestedObj[ioriver.UrlSigningKey]
}

func (TestedUrlSigningKey) Get(client *ioriver.IORiverClient, id string) (*ioriver.UrlSigningKey, error) {
	serviceId := os.Getenv("IORIVER_TEST_SERVICE_ID")
	return client.GetUrlSigningKey(serviceId, id)
}

func (TestedUrlSigningKey) List(client *ioriver.IORiverClient) ([]ioriver.UrlSigningKey, error) {
	serviceId := os.Getenv("IORIVER_TEST_SERVICE_ID")
	return client.ListUrlSigningKeys(serviceId)
}

func (TestedUrlSigningKey) Delete(client *ioriver.IORiverClient, object ioriver.UrlSigningKey, excludeIds []string) error {
	idx := slices.IndexFunc(excludeIds, func(id string) bool { return id == object.Id })
	if idx < 0 {
		serviceId := os.Getenv("IORIVER_TEST_SERVICE_ID")
		return client.DeleteUrlSigningKey(serviceId, object.Id)
	} else {
		return nil
	}
}

func TestAccIORiverUrlSigningKey_Basic(t *testing.T) {
	var performanceMonitor ioriver.UrlSigningKey
	var testedObj TestedUrlSigningKey

	serviceId := os.Getenv("IORIVER_TEST_SERVICE_ID")
	rndName := generateRandomResourceName()
	resourceName := urlSigningKeyResourceType + "." + rndName
	keyName := "test-su"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(s *terraform.State) error {
			return testAccCheckResourceDestroy[ioriver.UrlSigningKey](s, testedObj, urlSigningKeyResourceType)
		},
		Steps: []resource.TestStep{
			{
				Config: testAccCheckUrlSigningKeyConfig(rndName, serviceId, keyName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ioriver.UrlSigningKey](resourceName, &performanceMonitor, testedObj),
					resource.TestCheckResourceAttr(resourceName, "name", keyName),
				),
			},
			{
				ResourceName:        urlSigningKeyResourceType + "." + rndName,
				ImportStateIdPrefix: fmt.Sprintf("%s,", serviceId),
				ImportState:         true,
				// ignore since these fields cannot be read
				ImportStateVerifyIgnore: []string{"public_key", "encryption_key"},
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ioriver.UrlSigningKey](resourceName, &performanceMonitor, testedObj),
				),
			},
		},
	})
}

func testAccCheckUrlSigningKeyConfig(rndName string, serviceId string, keyName string) string {
	return fmt.Sprintf(`
	
	resource "ioriver_url_signing_key" "%s" {
		service        = "%s"
		name           = "%s"
		public_key     = "abcd"
		encryption_key = "1234"
	}`, rndName, serviceId, keyName)
}
