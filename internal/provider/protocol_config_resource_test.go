package provider

import (
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"golang.org/x/exp/slices"
	ioriver "github.com/ioriver/ioriver-go"
)

var protocolConfigResourceType string = "ioriver_protocol_config"

func init() {
	var testedObj TestedProtocolConfig
	resource.AddTestSweepers(protocolConfigResourceType, &resource.Sweeper{
		Name: protocolConfigResourceType,
		F: func(r string) error {
			return testSweepResources[ioriver.ProtocolConfig](r, testedObj, []string{})
		},
	})
}

type TestedProtocolConfig struct {
	TestedObj[ioriver.ProtocolConfig]
}

func (TestedProtocolConfig) Get(client *ioriver.IORiverClient, id string) (*ioriver.ProtocolConfig, error) {
	serviceId := os.Getenv("IORIVER_TEST_SERVICE_ID")
	return client.GetProtocolConfig(serviceId, id)
}

func (TestedProtocolConfig) List(client *ioriver.IORiverClient) ([]ioriver.ProtocolConfig, error) {
	serviceId := os.Getenv("IORIVER_TEST_SERVICE_ID")
	return client.ListProtocolConfigs(serviceId)
}

func (TestedProtocolConfig) Delete(client *ioriver.IORiverClient, object ioriver.ProtocolConfig, excludeIds []string) error {
	idx := slices.IndexFunc(excludeIds, func(id string) bool { return id == object.Id })
	if idx < 0 {
		serviceId := os.Getenv("IORIVER_TEST_SERVICE_ID")
		return client.DeleteProtocolConfig(serviceId, object.Id)
	} else {
		return nil
	}
}

func TestAccIORiverProtocolConfig_Basic(t *testing.T) {
	var protocolConfig ioriver.ProtocolConfig
	var testedObj TestedProtocolConfig

	serviceId := os.Getenv("IORIVER_TEST_SERVICE_ID")
	rndName := generateRandomResourceName()
	h2_enabled := rand.Intn(2) == 1
	h3_enabled := rand.Intn(2) == 1
	ipv6_enabled := rand.Intn(2) == 1
	resourceName := protocolConfigResourceType + "." + rndName

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccCheckProtocolConfigConfig(rndName, serviceId, h2_enabled, h3_enabled, ipv6_enabled),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ioriver.ProtocolConfig](resourceName, &protocolConfig, testedObj),
					resource.TestCheckResourceAttr(resourceName, "http2_enabled", strconv.FormatBool(h2_enabled)),
				),
			},
			{
				ResourceName:        "ioriver_protocol_config." + rndName,
				ImportStateIdPrefix: fmt.Sprintf("%s,", serviceId),
				ImportState:         true,
				ImportStateVerify:   true,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ioriver.ProtocolConfig](resourceName, &protocolConfig, testedObj),
				),
			},
		},
	})
}

func TestAccIORiverProtocolConfig_Update(t *testing.T) {
	var protocolConfig ioriver.ProtocolConfig
	var testedObj TestedProtocolConfig

	serviceId := os.Getenv("IORIVER_TEST_SERVICE_ID")
	rndName := generateRandomResourceName()
	h2_enabled := rand.Intn(2) == 1
	h3_enabled := rand.Intn(2) == 1
	ipv6_enabled := rand.Intn(2) == 1
	updatedProtocolConfigHttp2 := !h2_enabled
	resourceName := protocolConfigResourceType + "." + rndName

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccCheckProtocolConfigConfig(rndName, serviceId, h2_enabled, h3_enabled, ipv6_enabled),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ioriver.ProtocolConfig](resourceName, &protocolConfig, testedObj),
				),
			},
			{
				Config: testAccCheckProtocolConfigConfig(rndName, serviceId, updatedProtocolConfigHttp2, h3_enabled, ipv6_enabled),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ioriver.ProtocolConfig](resourceName, &protocolConfig, testedObj),
					resource.TestCheckResourceAttr(resourceName, "http2_enabled", strconv.FormatBool(updatedProtocolConfigHttp2)),
				),
			},
		},
	})
}

func testAccCheckProtocolConfigConfig(rndName string, serviceId string, h2_enabled bool, h3_enabled bool,
	ipv6_enabled bool) string {
	return fmt.Sprintf(`
	resource "ioriver_protocol_config" "%s" {
		service        = "%s"
		http2_enabled  = %t
		http3_enabled  = %t
		ipv6_enabled   = %t
	}`, rndName, serviceId, h2_enabled, h3_enabled, ipv6_enabled)
}
