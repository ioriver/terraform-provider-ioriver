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

var apResourceType string = "ioriver_account_provider"

func init() {
	var testedObj TestedAccountProvider
	resource.AddTestSweepers(apResourceType, &resource.Sweeper{
		Name: apResourceType,
		F: func(r string) error {
			return testSweepResources[ioriver.AccountProvider](r, testedObj, []string{})
		},
	})
}

type TestedAccountProvider struct {
	TestedObj[ioriver.AccountProvider]
}

func (TestedAccountProvider) Get(client *ioriver.IORiverClient, id string) (*ioriver.AccountProvider, error) {
	return client.GetAccountProvider(id)
}

func (TestedAccountProvider) List(client *ioriver.IORiverClient) ([]ioriver.AccountProvider, error) {
	return client.ListAccountProviders()
}

func (TestedAccountProvider) Delete(client *ioriver.IORiverClient, object ioriver.AccountProvider, excludeIds []string) error {
	idx := slices.IndexFunc(excludeIds, func(id string) bool { return id == object.Id })
	if idx < 0 {
		return client.DeleteAccountProvider(object.Id)
	} else {
		return nil
	}
}

func TestAccIORiverAccountProvider_Basic(t *testing.T) {
	var accountProvider ioriver.AccountProvider
	var testedObj TestedAccountProvider

	fastlyToken := os.Getenv("IORIVER_TEST_FASTLY_API_TOKEN")
	rndName := generateRandomResourceName()
	resourceName := apResourceType + "." + rndName

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(s *terraform.State) error {
			return testAccCheckResourceDestroy[ioriver.AccountProvider](s, testedObj, apResourceType)
		},
		Steps: []resource.TestStep{
			{
				Config: testAccCheckAccountProviderConfigBasic(rndName, fastlyToken),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ioriver.AccountProvider](resourceName, &accountProvider, testedObj),
				),
			},
			{
				ResourceName:            "ioriver_account_provider." + rndName,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"credentials"}, // ignore since this field cannot be read
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ioriver.AccountProvider](resourceName, &accountProvider, testedObj),
				),
			},
		},
	})
}

func testAccCheckAccountProviderConfigBasic(rndName string, fastlyToken string) string {
	return fmt.Sprintf(`
	resource "ioriver_account_provider" "%[1]s" {
		credentials = {
		  fastly = "%[2]s"
		}
	}`, rndName, fastlyToken)
}
