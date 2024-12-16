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

var logDestinationResourceType string = "ioriver_log_destination"

func init() {
	var testedObj TestedLogDestination
	resource.AddTestSweepers(logDestinationResourceType, &resource.Sweeper{
		Name: logDestinationResourceType,
		F: func(r string) error {
			return testSweepResources[ioriver.LogDestination](r, testedObj, []string{})
		},
	})
}

type TestedLogDestination struct {
	TestedObj[ioriver.LogDestination]
}

func (TestedLogDestination) Get(client *ioriver.IORiverClient, id string) (*ioriver.LogDestination, error) {
	serviceId := os.Getenv("IORIVER_TEST_SERVICE_ID")
	return client.GetLogDestination(serviceId, id)
}

func (TestedLogDestination) List(client *ioriver.IORiverClient) ([]ioriver.LogDestination, error) {
	serviceId := os.Getenv("IORIVER_TEST_SERVICE_ID")
	return client.ListLogDestinations(serviceId)
}

func (TestedLogDestination) Delete(client *ioriver.IORiverClient, object ioriver.LogDestination, excludeIds []string) error {
	idx := slices.IndexFunc(excludeIds, func(id string) bool { return id == object.Id })
	if idx < 0 {
		serviceId := os.Getenv("IORIVER_TEST_SERVICE_ID")
		return client.DeleteLogDestination(serviceId, object.Id)
	} else {
		return nil
	}
}

func TestAccIORiverLogDestination_Basic(t *testing.T) {
	var logDestination ioriver.LogDestination
	var testedObj TestedLogDestination

	serviceId := os.Getenv("IORIVER_TEST_SERVICE_ID")
	rndName := generateRandomResourceName()
	resourceName := logDestinationResourceType + "." + rndName
	bucketName := "test-bucket"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(s *terraform.State) error {
			return testAccCheckResourceDestroy[ioriver.LogDestination](s, testedObj, logDestinationResourceType)
		},
		Steps: []resource.TestStep{
			{
				Config: testAccCheckLogDestinationConfigAwsS3(rndName, serviceId, bucketName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ioriver.LogDestination](resourceName, &logDestination, testedObj),
					resource.TestCheckResourceAttr(resourceName, "name", rndName),
				),
			},
			{
				ResourceName:            "ioriver_log_destination." + rndName,
				ImportStateIdPrefix:     fmt.Sprintf("%s,", serviceId),
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"aws_s3.credentials", "compatible_s3.credentials"}, // ignore since this field cannot be read
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ioriver.LogDestination](resourceName, &logDestination, testedObj),
				),
			},
		},
	})
}

func TestAccIORiverLogDestination_Update(t *testing.T) {
	var logDestination ioriver.LogDestination
	var testedObj TestedLogDestination

	serviceId := os.Getenv("IORIVER_TEST_SERVICE_ID")
	rndName := generateRandomResourceName()

	bucketName := "test-bucket"
	updatedBucketName := "updated-" + bucketName
	resourceName := logDestinationResourceType + "." + rndName

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(s *terraform.State) error {
			return testAccCheckResourceDestroy[ioriver.LogDestination](s, testedObj, logDestinationResourceType)
		},
		Steps: []resource.TestStep{
			{
				Config: testAccCheckLogDestinationConfigAwsS3(rndName, serviceId, bucketName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ioriver.LogDestination](resourceName, &logDestination, testedObj),
				),
			},
			{
				Config: testAccCheckLogDestinationConfigAwsS3(rndName, serviceId, updatedBucketName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ioriver.LogDestination](resourceName, &logDestination, testedObj),
					resource.TestCheckResourceAttr(resourceName, "aws_s3.name", updatedBucketName),
				),
			},
		},
	})
}

func TestAccIORiverLogDestinationCompatibleS3_Basic(t *testing.T) {
	var logDestination ioriver.LogDestination
	var testedObj TestedLogDestination

	serviceId := os.Getenv("IORIVER_TEST_SERVICE_ID")
	rndName := generateRandomResourceName()
	resourceName := logDestinationResourceType + "." + rndName
	bucketName := "test-bucket"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(s *terraform.State) error {
			return testAccCheckResourceDestroy[ioriver.LogDestination](s, testedObj, logDestinationResourceType)
		},
		Steps: []resource.TestStep{
			{
				Config: testAccCheckLogDestinationConfigCompatibleS3(rndName, serviceId, bucketName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ioriver.LogDestination](resourceName, &logDestination, testedObj),
					resource.TestCheckResourceAttr(resourceName, "name", rndName),
				),
			},
			{
				ResourceName:            "ioriver_log_destination." + rndName,
				ImportStateIdPrefix:     fmt.Sprintf("%s,", serviceId),
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"aws_s3.credentials", "compatible_s3.credentials"}, // ignore since this field cannot be read
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ioriver.LogDestination](resourceName, &logDestination, testedObj),
				),
			},
		},
	})
}

func testAccCheckLogDestinationConfigAwsS3(rndName string, serviceId string, bucketName string) string {
	return fmt.Sprintf(`
	resource "ioriver_log_destination" "%s" {
		service        = "%s"
		name           = "%s"
		aws_s3         = {
		  name   =   "%s"
			path   =   "/logs"
			region =   "us-east-1"
			credentials = {
			  assume_role = {
				  role_arn     = "abc"
					external_id  = "123"
				}
			}
		}
	}

	resource "ioriver_behavior" "%s" {
		service      = "%s"
		name         = "stream-logs"
		path_pattern = "/test/*"
		
		actions = [
			{
				stream_logs = {
					unified_log_destination = ioriver_log_destination.%s.id
					unified_log_sampling_rate = "10"
        }
      }
		]
  }`, rndName, serviceId, rndName, bucketName, rndName, serviceId, rndName)
}

func testAccCheckLogDestinationConfigCompatibleS3(rndName string, serviceId string, bucketName string) string {
	return fmt.Sprintf(`
	resource "ioriver_log_destination" "%s" {
		service        = "%s"
		name           = "%s"
		compatible_s3  = {
		  name   =   "%s"
			path   =   "/logs"
			region =   "eu-central-1"
			domain =   "s3.eu-central-1.wasabisys.com"
			credentials = {
			  access_key  = "abc"
  			secret_key  = "123"
			}
		}
	}
	resource "ioriver_behavior" "%s" {
		service      = "%s"
		name         = "stream-logs"
		path_pattern = "/test/*"
		
		actions = [
			{
				stream_logs = {
					unified_log_destination = ioriver_log_destination.%s.id
					unified_log_sampling_rate = "10"
        }
      }
		]
  }`, rndName, serviceId, rndName, bucketName, rndName, serviceId, rndName)
}
