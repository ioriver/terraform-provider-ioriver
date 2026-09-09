package provider

import (
	"fmt"
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestAccIORiverService_PrivateS3Origin_Lifecycle interleaves plan-time
// ExpectError steps with real apply steps to validate the private-S3-origin
// credential rules end-to-end:
//
//  1. Plan-only: create with is_private=true and no creds → error
//  2. Plan-only: create with is_private=true and only s3_aws_key → error
//  3. Plan-only: create with is_private=false but s3_aws_key set → error
//  4. Apply:     create with a public custom origin (baseline state)
//  5. Plan-only: flip public custom → private S3 without creds → error
//  6. Apply:     flip to private S3 with creds
//  7. Plan-only: bump credentials_version but drop creds → error
//  8. Apply:     bump credentials_version and provide new creds (rotation)
//  9. Import:    round-trip state (WriteOnly creds must round-trip cleanly)
func TestAccIORiverService_PrivateS3Origin_Lifecycle(t *testing.T) {
	t.Parallel()
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
			// Step 4 — apply: create a valid public custom origin as the baseline.
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
				),
			},
			// Step 9 — import: state should round-trip cleanly. WriteOnly creds are
			// not present in state, so the importer must not attempt to reconcile
			// them and the diff after import must be empty.
			{
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
