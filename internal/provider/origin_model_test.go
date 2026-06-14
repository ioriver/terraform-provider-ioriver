package provider

import "fmt"

func testAccServiceConfigOriginsSteps(idx int, params ...any) string {
	// Hardcoded test credentials for private S3 origin.
	// The backend stores them in its secret manager; we just need non-empty strings.
	const s3Key = "test-access-key"
	const s3Secret = "test-secret-key"

	// Helper: Array of config strings for stepwise testing
	var testSteps = []string{
		// Step 0: Single origin
		`
locals {
	origin_1_id = "origin-1"
}

resource "ioriver_service" "%s" {
	name        = "%s"
	certificate = "%s"
	description = "desc"
	config = {
		origins = [
			{
				name = local.origin_1_id
				custom_origin = {
					host = "example.com"
					protocol = "https"
				}
			}
		]
		domains = []
	}
}`,
		// Step 1: Add a second origin, change host of origin-1
		fmt.Sprintf(`
locals {
	origin_1_id = "origin-1"
	origin_2_id = "origin-2"
}

resource "ioriver_service" "%%s" {
	name        = "%%s"
	certificate = "%%s"
	description = "desc"
	config = {
		origins = [
			{
				name = local.origin_1_id
				custom_origin = {
					host = "example4.com"
					protocol = "https"
				}
			},
			{
				name = local.origin_2_id
				s3_origin = {
					host           = "example2.com"
					is_private     = true
					s3_aws_region  = "us-west-2"
					s3_bucket_name = "my-bucket"
					credentials_version = 1
					s3_aws_key     = %q
					s3_aws_secret  = %q
				}
			}
		]
		domains = []
	}
}`, s3Key, s3Secret),
		// Step 2: Remove the first origin
		fmt.Sprintf(`
locals {
	origin_2_id = "origin-2"
}
resource "ioriver_service" "%%s" {
	name        = "%%s"
	certificate = "%%s"
	description = "desc"
	config = {
		origins = [
			{
				name = local.origin_2_id
				s3_origin = {
					host           = "example2.com"
					is_private     = true
					s3_aws_region  = "us-west-2"
					s3_bucket_name = "my-bucket"
					credentials_version = 1
					s3_aws_key     = %q
					s3_aws_secret  = %q
				}
			}
		]
		domains = []
	}
}`, s3Key, s3Secret),
		// step 3 - add another origin
		fmt.Sprintf(`
locals {
	origin_2_id = "origin-2"
	origin_3_id = "origin-3"
}
resource "ioriver_service" "%%s" {
	name        = "%%s"
	certificate = "%%s"
	description = "desc"
	config = {
		origins = [
			{
				name = local.origin_2_id
				s3_origin = {
					host           = "example2.com"
					is_private     = true
					s3_aws_region  = "us-west-2"
					s3_bucket_name = "my-bucket"
					credentials_version = 1
					s3_aws_key     = %q
					s3_aws_secret  = %q
				}
			},
			{
				name = local.origin_3_id
				custom_origin = {
					host = "example3.com"
					protocol = "https"
				}
			}
		]
		domains = []
	}
}`, s3Key, s3Secret),
		// step 4 - swap origins in plan (pure reorder — plan modifier suppresses diff)
		fmt.Sprintf(`
locals {
	origin_2_id = "origin-2"
	origin_3_id = "origin-3"
}
resource "ioriver_service" "%%s" {
	name        = "%%s"
	certificate = "%%s"
	description = "desc"
	config = {
		origins = [
			{
				name = local.origin_3_id
				custom_origin = {
					host = "example3.com"
					protocol = "https"
				}
			},
			{
				name = local.origin_2_id
				s3_origin = {
					host           = "example2.com"
					is_private     = true
					s3_aws_region  = "us-west-2"
					s3_bucket_name = "my-bucket"
					credentials_version = 1
					s3_aws_key     = %q
					s3_aws_secret  = %q
				}
			}
		]
		domains = []
	}
}`, s3Key, s3Secret),
		// step 5 - swap order AND add shield to origin_3
		fmt.Sprintf(`
locals {
	origin_2_id = "origin-2"
	origin_3_id = "origin-3"
}
resource "ioriver_service" "%%s" {
	name        = "%%s"
	certificate = "%%s"
	description = "desc"
	config = {
		origins = [
			{
				name = local.origin_3_id
				custom_origin = {
					host = "example3.com"
					protocol = "https"
				}
				shield = {
					location = {
						country     = "US"
						subdivision = "VA"
					}
					providers = ["fastly"]
				}
			},
			{
				name = local.origin_2_id
				s3_origin = {
					host           = "example2.com"
					is_private     = true
					s3_aws_region  = "us-west-2"
					s3_bucket_name = "my-bucket"
					credentials_version = 1
					s3_aws_key     = %q
					s3_aws_secret  = %q
				}
			}
		]
		domains = []
	}
}`, s3Key, s3Secret),
		// step 6 - add cloudflare to shield providers (provider not connected, validates list expansion)
		fmt.Sprintf(`
locals {
	origin_2_id = "origin-2"
	origin_3_id = "origin-3"
}
resource "ioriver_service" "%%s" {
	name        = "%%s"
	certificate = "%%s"
	description = "desc"
	config = {
		origins = [
			{
				name = local.origin_3_id
				custom_origin = {
					host = "example3.com"
					protocol = "https"
				}
				shield = {
					location = {
						country     = "US"
						subdivision = "VA"
					}
					providers = ["fastly", "cloudflare"]
				}
			},
			{
				name = local.origin_2_id
				s3_origin = {
					host           = "example2.com"
					is_private     = true
					s3_aws_region  = "us-west-2"
					s3_bucket_name = "my-bucket"
					credentials_version = 1
					s3_aws_key     = %q
					s3_aws_secret  = %q
				}
			}
		]
		domains = []
	}
}`, s3Key, s3Secret),
	}

	if idx < 0 || idx >= len(testSteps) {
		panic("invalid config step index")
	}
	return fmt.Sprintf(testSteps[idx], params...)
}
