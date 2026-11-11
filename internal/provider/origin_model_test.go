package provider

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

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
	certificates = ["%s"]
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
	certificates = ["%%s"]
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
	certificates = ["%%s"]
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
	certificates = ["%%s"]
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
	certificates = ["%%s"]
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
	certificates = ["%%s"]
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
	certificates = ["%%s"]
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

func testAccServiceConfigOriginMTLS(name, certID string, enabled bool) string {
	return testAccCheckCertificateConfig("origin_client", name+"-origin-client") + fmt.Sprintf(`
resource "ioriver_service" %[1]q {
	name         = %[1]q
	certificates = [%[2]q]
	description  = "desc"
	config = {
		origins = [{
			name = "origin-1"
			custom_origin = {
				host                = "example.com"
				protocol            = "https"
				enable_mtls         = %[3]t
				mtls_certificate_id = %[3]t ? ioriver_certificate.origin_client.id : null
			}
		}]
		domains = []
	}
}
`, name, certID, enabled)
}

func TestOriginMTLSConversion(t *testing.T) {
	tests := []struct {
		name        string
		enable      types.Bool
		certificate types.String
	}{
		{name: "omitted", enable: types.BoolNull(), certificate: types.StringNull()},
		{name: "disabled with retained certificate", enable: types.BoolValue(false), certificate: types.StringValue("cert-id")},
		{name: "enabled", enable: types.BoolValue(true), certificate: types.StringValue("cert-id")},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			model := originMTLSTestModel(test.enable, test.certificate)
			check := func(origin *CustomOriginModel) {
				t.Helper()
				if test.enable.IsNull() {
					if !origin.EnableMTLS.IsNull() || !origin.MTLSCertificateID.IsNull() {
						t.Fatalf("unset mTLS fields should remain null, got (%v, %v)", origin.EnableMTLS, origin.MTLSCertificateID)
					}
					return
				}
				if origin.EnableMTLS != test.enable || origin.MTLSCertificateID != test.certificate {
					t.Fatalf("mTLS fields = (%v, %v), want (%v, %v)", origin.EnableMTLS, origin.MTLSCertificateID, test.enable, test.certificate)
				}
			}

			// Cover the shared conversion used by direct origins and origin sets.
			outbound, err := model.ModelToMap()
			if err != nil {
				t.Fatal(err)
			}
			decoded, err := originFromMap(context.Background(), outbound)
			if err != nil {
				t.Fatal(err)
			}
			check(decoded.CustomOrigin)

			if test.enable.IsNull() {
				return
			}
			customOrigin := outbound["custom_origin"].(map[string]interface{})
			if customOrigin["enable_mtls"] != test.enable.ValueBool() ||
				customOrigin["mtls_certificate_id"] != test.certificate.ValueString() {
				t.Fatalf("outbound mTLS fields = (%v, %v)", customOrigin["enable_mtls"], customOrigin["mtls_certificate_id"])
			}
		})
	}
}

func originMTLSTestModel(enable types.Bool, certificate types.String) OriginModel {
	return OriginModel{
		Uuid:        types.StringNull(),
		Name:        types.StringValue("origin"),
		Path:        types.StringValue("/"),
		VerifySSL:   types.BoolValue(true),
		TimeoutMs:   types.Int64Null(),
		SNIHostname: types.StringNull(),
		CustomOrigin: &CustomOriginModel{
			Host:              types.StringValue("origin.example.com"),
			Protocol:          types.StringValue("https"),
			CustomHttpPort:    types.Int64Null(),
			CustomHttpsPort:   types.Int64Null(),
			EnableMTLS:        enable,
			MTLSCertificateID: certificate,
		},
	}
}
