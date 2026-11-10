package provider

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestAccIORiverService_PrivateS3Origin_Lifecycle covers creation, conversion,
// rotation, omitted credentials, additions, removal, and import. Invalid configs
// must fail without changing the existing service.
func TestAccIORiverService_PrivateS3Origin_Lifecycle(t *testing.T) {
	t.Parallel()
	var service ServiceWithConfig
	var testedObj TestedService

	certId := os.Getenv("IORIVER_TEST_CERT_ID")
	rndName := generateRandomResourceName()
	resourceName := serviceResourceType + "." + rndName
	var serviceID string
	checkPrivateOrigins := func(count int) resource.TestCheckFunc {
		return func(state *terraform.State) error {
			rs, ok := state.RootModule().Resources[resourceName]
			if !ok || rs.Primary.ID == "" {
				return fmt.Errorf("private S3 service is missing from state")
			}
			if serviceID != "" && serviceID != rs.Primary.ID {
				return fmt.Errorf("private S3 update replaced the service")
			}
			serviceID = rs.Primary.ID
			current, err := GetServiceWithConfig(testAccClient, serviceID)
			if err != nil {
				return err
			}
			origins, ok := current.Config["origins"].([]interface{})
			if !ok || len(origins) != count {
				return fmt.Errorf("backend origin count does not match %d", count)
			}
			for i, raw := range origins {
				origin, ok := raw.(map[string]interface{})
				if !ok {
					return fmt.Errorf("invalid backend origin shape")
				}
				s3, ok := origin["s3_origin"].(map[string]interface{})
				if !ok || s3["is_private"] != true || s3["s3_aws_region"] != "us-west-2" {
					return fmt.Errorf("backend private S3 settings did not persist")
				}
				if s3["s3_aws_key"] != nil || s3["s3_aws_secret"] != nil {
					return fmt.Errorf("backend returned private S3 credentials")
				}
				prefix := fmt.Sprintf("config.origins.%d.s3_origin.", i)
				if rs.Primary.Attributes[prefix+"s3_aws_key"] != "" || rs.Primary.Attributes[prefix+"s3_aws_secret"] != "" {
					return fmt.Errorf("private S3 credentials leaked into state")
				}
			}
			return nil
		}
	}

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreEnvVariable(t, "IORIVER_API_TOKEN")
			testAccPreEnvVariable(t, "IORIVER_TEST_CERT_ID")
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(s *terraform.State) error {
			return testAccCheckResourceDestroy[ServiceWithConfig](s, testedObj, serviceResourceType)
		},
		Steps: []resource.TestStep{
			// Step 1 — plan-only: is_private=true, no creds → error
			{
				Config:      testAccPrivateS3ConfigPrivateMissingBothCreds(rndName, certId),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`(?is)requires\s+both\s+s3_aws_key`),
			},
			// Step 2 — plan-only: is_private=true, only s3_aws_key → error
			{
				Config:      testAccPrivateS3ConfigPrivateMissingSecret(rndName, certId),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`(?is)(requires\s+s3_aws_secret|s3_aws_secret"\s+must\s+be\s+specified)`),
			},
			// Step 3 — plan-only: is_private=false but s3_aws_key set → error
			{
				Config:      testAccPrivateS3ConfigPublicWithKey(rndName, certId),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`(?is)is_private=false\s+but\s+(s3_aws_key|s3_aws_secret|credentials_version|s3_bucket_name|s3_aws_region)\s+is\s+set`),
			},
			// Create a private origin with credentials before exercising updates.
			{
				Config: testAccPrivateS3ConfigFlipToPrivateWithCreds(rndName, certId, 1),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ServiceWithConfig](resourceName, &service, testedObj),
					resource.TestCheckResourceAttr(resourceName, "config.origins.0.s3_origin.is_private", "true"),
					resource.TestCheckNoResourceAttr(resourceName, "config.origins.0.s3_origin.s3_aws_key"),
					resource.TestCheckNoResourceAttr(resourceName, "config.origins.0.s3_origin.s3_aws_secret"),
					checkPrivateOrigins(1),
				),
			},
			// Step 4 — apply: switch to a public custom origin.
			{
				Config: testAccPrivateS3ConfigCustomOrigin(rndName, certId),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ServiceWithConfig](resourceName, &service, testedObj),
					resource.TestCheckResourceAttr(resourceName, "config.origins.0.custom_origin.host", "example.com"),
				),
			},
			// Step 5 — plan-only: replace the custom origin with a private S3 origin
			// (same name) but without providing creds → update-time enforcement fires.
			// State has a non-S3 origin, so the update validator treats this as
			// "adding a new private S3 origin".
			{
				Config:      testAccPrivateS3ConfigFlipToPrivateNoCreds(rndName, certId),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`(?is)adding\s+a\s+new\s+private\s+S3\s+origin\s+requires`),
			},
			// Step 6 — apply: same flip but with creds provided.
			{
				Config: testAccPrivateS3ConfigFlipToPrivateWithCreds(rndName, certId, 1),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ServiceWithConfig](resourceName, &service, testedObj),
					resource.TestCheckResourceAttr(resourceName, "config.origins.0.s3_origin.host", "priv.example.com"),
					resource.TestCheckResourceAttr(resourceName, "config.origins.0.s3_origin.is_private", "true"),
					resource.TestCheckResourceAttr(resourceName, "config.origins.0.s3_origin.credentials_version", "1"),
					checkPrivateOrigins(1),
				),
			},
			// Step 7 — plan-only: bump credentials_version but strip creds → error.
			{
				Config:      testAccPrivateS3ConfigBumpVersionNoCreds(rndName, certId, 2),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`(?is)bumping\s+credentials_version\s+requires`),
			},
			// Step 8 — apply: bump credentials_version with new creds (rotation).
			{
				Config: testAccPrivateS3ConfigFlipToPrivateWithCreds(rndName, certId, 2),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ServiceWithConfig](resourceName, &service, testedObj),
					resource.TestCheckResourceAttr(resourceName, "config.origins.0.s3_origin.credentials_version", "2"),
					checkPrivateOrigins(1),
				),
			},
			// Omission must preserve private credentials on an unchanged origin.
			{
				Config: testAccPrivateS3ConfigBumpVersionNoCreds(rndName, certId, 2),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ServiceWithConfig](resourceName, &service, testedObj),
					resource.TestCheckResourceAttr(resourceName, "config.origins.0.s3_origin.credentials_version", "2"),
					resource.TestCheckNoResourceAttr(resourceName, "config.origins.0.s3_origin.s3_aws_key"),
					resource.TestCheckNoResourceAttr(resourceName, "config.origins.0.s3_origin.s3_aws_secret"),
					checkPrivateOrigins(1),
				),
			},
			{
				Config:      testAccPrivateS3ConfigAdditionalOrigin(rndName, certId, false),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`(?is)adding\s+a\s+new\s+private\s+S3\s+origin\s+requires`),
			},
			{
				Config: testAccPrivateS3ConfigAdditionalOrigin(rndName, certId, true),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ServiceWithConfig](resourceName, &service, testedObj),
					resource.TestCheckResourceAttr(resourceName, "config.origins.#", "2"),
					resource.TestCheckResourceAttr(resourceName, "config.origins.1.name", "new-private"),
					resource.TestCheckNoResourceAttr(resourceName, "config.origins.1.s3_origin.s3_aws_key"),
					resource.TestCheckNoResourceAttr(resourceName, "config.origins.1.s3_origin.s3_aws_secret"),
					checkPrivateOrigins(2),
				),
			},
			{
				Config: testAccPrivateS3ConfigBumpVersionNoCreds(rndName, certId, 2),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ServiceWithConfig](resourceName, &service, testedObj),
					resource.TestCheckResourceAttr(resourceName, "config.origins.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "config.origins.0.name", "swap-origin"),
					checkPrivateOrigins(1),
				),
			},
			{
				// Import must not restore write-only credentials.
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{
					"certificate",
					"certificates.#",
					"certificates.0",
					"config.origins.0.name",
					"config.origins.0.s3_origin.credentials_version",
					"config.origins.0.s3_origin.s3_aws_key",
					"config.origins.0.s3_origin.s3_aws_secret",
				},
			},
		},
	})
}

// ---------------------------------------------------------------------------
// HCL builders for the private-S3-origin lifecycle test
// ---------------------------------------------------------------------------

const testAccPrivateS3TestKey = "AKIAIOSFODNN7EXAMPLE"
const testAccPrivateS3TestSecret = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"

func testAccPrivateS3ConfigAdditionalOrigin(name, certID string, credentials bool) string {
	config := testAccPrivateS3ConfigBumpVersionNoCreds(name, certID, 2)
	var credentialFields string
	if credentials {
		credentialFields = fmt.Sprintf("s3_aws_key = %q\n s3_aws_secret = %q", testAccPrivateS3TestKey, testAccPrivateS3TestSecret)
	}
	addition := fmt.Sprintf(`,
			{
				name = "new-private"
				s3_origin = {
					host = "new-private.example.com"
					is_private = true
					s3_aws_region = "us-west-2"
					s3_bucket_name = "new-private-bucket"
					credentials_version = 1
					%s
				}
			}
		]
		domains`, credentialFields)
	return strings.Replace(config, "\n\t\t]\n\t\tdomains", addition, 1)
}

func testAccPrivateS3ConfigPrivateMissingBothCreds(rndName, certId string) string {
	return fmt.Sprintf(`
resource "ioriver_service" "%s" {
	name        = "%s"
	certificate = "%s"
	description = "desc"
	config = {
		origins = [
			{
				name = "s3-priv"
				s3_origin = {
					host                = "priv.example.com"
					is_private          = true
					s3_aws_region       = "us-west-2"
					s3_bucket_name      = "my-bucket"
					credentials_version = 1
				}
			}
		]
		domains = []
	}
}`, rndName, rndName, certId)
}

func testAccPrivateS3ConfigPrivateMissingSecret(rndName, certId string) string {
	return fmt.Sprintf(`
resource "ioriver_service" "%s" {
	name        = "%s"
	certificate = "%s"
	description = "desc"
	config = {
		origins = [
			{
				name = "s3-priv"
				s3_origin = {
					host                = "priv.example.com"
					is_private          = true
					s3_aws_region       = "us-west-2"
					s3_bucket_name      = "my-bucket"
					credentials_version = 1
					s3_aws_key          = %q
				}
			}
		]
		domains = []
	}
}`, rndName, rndName, certId, testAccPrivateS3TestKey)
}

func testAccPrivateS3ConfigPublicWithKey(rndName, certId string) string {
	return fmt.Sprintf(`
resource "ioriver_service" "%s" {
	name        = "%s"
	certificate = "%s"
	description = "desc"
	config = {
		origins = [
			{
				name = "s3-pub"
				s3_origin = {
					host                = "pub.example.com"
					is_private          = false
					credentials_version = 1
					s3_aws_key          = %q
					s3_aws_secret       = %q
				}
			}
		]
		domains = []
	}
}`, rndName, rndName, certId, testAccPrivateS3TestKey, testAccPrivateS3TestSecret)
}

func testAccPrivateS3ConfigCustomOrigin(rndName, certId string) string {
	return fmt.Sprintf(`
resource "ioriver_service" "%s" {
	name        = "%s"
	certificate = "%s"
	description = "desc"
	config = {
		origins = [
			{
				name = "swap-origin"
				custom_origin = {
					host     = "example.com"
					protocol = "https"
				}
			}
		]
		domains = []
	}
}`, rndName, rndName, certId)
}

func testAccPrivateS3ConfigFlipToPrivateNoCreds(rndName, certId string) string {
	return fmt.Sprintf(`
resource "ioriver_service" "%s" {
	name        = "%s"
	certificate = "%s"
	description = "desc"
	config = {
		origins = [
			{
				name = "swap-origin"
				s3_origin = {
					host                = "priv.example.com"
					is_private          = true
					s3_aws_region       = "us-west-2"
					s3_bucket_name      = "my-bucket"
					credentials_version = 1
				}
			}
		]
		domains = []
	}
}`, rndName, rndName, certId)
}

func testAccPrivateS3ConfigFlipToPrivateWithCreds(rndName, certId string, credVer int) string {
	return fmt.Sprintf(`
resource "ioriver_service" "%s" {
	name        = "%s"
	certificate = "%s"
	description = "desc"
	config = {
		origins = [
			{
				name = "swap-origin"
				s3_origin = {
					host                = "priv.example.com"
					is_private          = true
					s3_aws_region       = "us-west-2"
					s3_bucket_name      = "my-bucket"
					credentials_version = %d
					s3_aws_key          = %q
					s3_aws_secret       = %q
				}
			}
		]
		domains = []
	}
}`, rndName, rndName, certId, credVer, testAccPrivateS3TestKey, testAccPrivateS3TestSecret)
}

func testAccPrivateS3ConfigBumpVersionNoCreds(rndName, certId string, credVer int) string {
	return fmt.Sprintf(`
resource "ioriver_service" "%s" {
	name        = "%s"
	certificate = "%s"
	description = "desc"
	config = {
		origins = [
			{
				name = "swap-origin"
				s3_origin = {
					host                = "priv.example.com"
					is_private          = true
					s3_aws_region       = "us-west-2"
					s3_bucket_name      = "my-bucket"
					credentials_version = %d
				}
			}
		]
		domains = []
	}
}`, rndName, rndName, certId, credVer)
}
