package provider

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	ioriver "ioriver.io/ioriver/ioriver-go"
)

func generateRandomResourceName() string {
	return acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
}

var testAccClient *ioriver.IORiverClient = ioriver.NewClient(os.Getenv(APITokenEvnVar))

var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"ioriver": providerserver.NewProtocol6WithError(New("test")()),
}

func testAccPreCheck(t *testing.T) {
	testAccPreEnvVariable(t, "IORIVER_API_TOKEN")
	testAccPreEnvVariable(t, "IORIVER_TEST_SERVICE_ID")
	testAccPreEnvVariable(t, "IORIVER_TEST_DOMAIN")
	testAccPreEnvVariable(t, "IORIVER_TEST_CERT_ID")
	testAccPreEnvVariable(t, "IORIVER_TEST_DOMAIN_ID")
	testAccPreEnvVariable(t, "IORIVER_TEST_ORIGIN_ID")
	testAccPreEnvVariable(t, "IORIVER_TEST_SERVICE_PROVIDER_ID")
	testAccPreEnvVariable(t, "IORIVER_TEST_DEFAULT_BEHAVIOR_ID")
	testAccPreEnvVariable(t, "IORIVER_TEST_DEFAULT_TRAFFIC_POLICY_ID")
	testAccPreEnvVariable(t, "IORIVER_TEST_FASTLY_API_TOKEN")
}

type TestedObj[T any] interface {
	Get(client *ioriver.IORiverClient, id string) (*T, error)
	List(client *ioriver.IORiverClient) ([]T, error)
	Delete(client *ioriver.IORiverClient, object T, excludeIds []string) error
}

func testAccCheckObjectExists[T interface{}](n string, newObj *T, testedObj TestedObj[T]) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("Not found: %s", n)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("Id is not set")
		}

		obj, err := testedObj.Get(testAccClient, rs.Primary.ID)
		if err != nil {
			return err
		}

		*newObj = *obj
		return nil
	}
}

func testAccPreEnvVariable(t *testing.T, envVariable string) {
	if v := os.Getenv(envVariable); v == "" {
		msg := fmt.Sprintf("%s must be set for acceptance tests", envVariable)
		t.Fatal(msg)
	}
}

func testAccCheckResourceDestroy[T interface{}](s *terraform.State, testedObj TestedObj[T], resourceType string) error {
	// loop through the resources in state, verifying each object is destroyed
	for _, rs := range s.RootModule().Resources {
		if rs.Type != resourceType {
			continue
		}

		_, err := testedObj.Get(testAccClient, rs.Primary.ID)
		if err == nil {
			return fmt.Errorf("Resource of type (%s), id:(%s) still exists.", resourceType, rs.Primary.ID)
		}

		// If the error is equivalent to 404 not found, the object is destroyed.
		// Otherwise return the error
		if !strings.Contains(err.Error(), "Not found") {
			return err
		}
	}

	return nil
}

func testSweepResources[T interface{}](r string, testedObj TestedObj[T], excludeIds []string) error {

	client, clientErr := sharedClient()
	if clientErr != nil {
		return fmt.Errorf("Failed to create client: %s", clientErr)
	}

	objects, err := testedObj.List(client)
	if err != nil {
		return fmt.Errorf("Error getting list of objects: %w", err)
	}

	for _, obj := range objects {
		err := testedObj.Delete(client, obj, excludeIds)
		if err != nil {
			return fmt.Errorf("Error deleting object: %w", err)
		}
	}

	return nil
}
