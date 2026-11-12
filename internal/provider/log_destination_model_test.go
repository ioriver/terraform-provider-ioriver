package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/defaults"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

const testAccLogDestCredsKey = "AKIAIOSFODNN7EXAMPLE"
const testAccLogDestCredsSecret = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"

func testAccLoggingDestination(kind, name, source string, version int, credentials bool) string {
	assumeRole := strings.HasSuffix(kind, "_assume_role")
	kind = strings.TrimSuffix(kind, "_assume_role")
	settings := fmt.Sprintf("credentials_version = %d", version)
	if kind == "splunk" {
		settings += `
			url = "https://splunk.example.com:8088/services/collector/raw"
			channel_id = "00000000-0000-4000-8000-000000000001"
			use_insecure = false`
		if credentials {
			settings += fmt.Sprintf("\n credentials = { token = %q }", fmt.Sprintf("test-token-%d", version))
		}
	} else {
		settings += `
			name = "test-log-bucket"
			path = "/delivery"
			region = "us-east-1"`
		if kind == "compatible_s3" {
			settings += "\n domain = \"https://s3.example.com\""
		}
		if credentials {
			if assumeRole {
				settings += `
					credentials = { assume_role = {
						role_arn = "arn:aws:iam::123456789012:role/IORiverLogsRole"
						external_id = "ioriver-test-external-id"
					} }`
			} else {
				settings += fmt.Sprintf(`
				credentials = { access_key = { access_key = %q, secret_key = %q } }
			`, testAccLogDestCredsKey, testAccLogDestCredsSecret)
			}
		}
	}
	fields := `["timestamp", "service_id", "service_uid", "status_code", "request_headers", "response_headers"]`
	headers := `request_headers = ["x-request-id"]
		response_headers = ["content-type"]`
	if source == "security" {
		fields = `["timestamp", "service_id", "service_uid", "action", "severity_level", "ja4"]`
		headers = ""
	}
	return fmt.Sprintf(`{
		name = %q
		source = %q
		file_format = "json-line-delimited"
		selected_fields = %s
		%s
		object_name = "/delivery/{random}.gz"
		timestamp_format = "unix"
		max_samples_per_object = 250000
		max_gap_seconds = 600
		%s = { %s }
	}`, name, source, fields, headers, kind, settings)
}

func testAccLoggingConfig(name, certID string, destinations []string, trafficDestination, securityDestination string) string {
	return fmt.Sprintf(`
locals {
	traffic_destination  = %[4]q
	security_destination = %[5]q
}

resource "ioriver_service" %[1]q {
	name = %[1]q
	description = "Logging configuration lifecycle test"
	certificates = [%[2]q]
	config = {
		log_destinations = [%[3]s]

		behaviors = local.traffic_destination != "" ? {
			default = {
				actions = {
					stream_logs = {
						log_destination = local.traffic_destination
						log_sampling_rate = 10
					}
				}
			}
			custom = [{
				name = "logging"
				path_pattern = "/delivery/*"
				actions = {
					stream_logs = {
						log_destination = local.traffic_destination
						log_sampling_rate = 10
					}
				}
			}]
		} : null

		security = {
			enabled = true
			logs_shipping = local.security_destination != "" ? {
				destination = local.security_destination
				enabled = false
				severity_min_threshold = "high"
				allowed_requests_sampling_rate = 25
			} : null
		}
	}
}`, name, certID, strings.Join(destinations, ",\n"), trafficDestination, securityDestination)
}

func testAccCheckLoggingState(address, kind string, names []string, replaced, traffic string, security bool,
	serviceID *string, destinationIDs map[string]string) resource.TestCheckFunc {
	block := strings.TrimSuffix(kind, "_assume_role")
	checks := []resource.TestCheckFunc{
		resource.TestCheckResourceAttr(address, "config.log_destinations.#", fmt.Sprint(len(names))),
		testAccCheckLoggingIdentity(address, names, replaced, serviceID, destinationIDs),
	}
	if traffic != "" {
		checks = append(checks,
			resource.TestCheckResourceAttr(address, "config.behaviors.default.actions.stream_logs.log_destination", traffic),
			resource.TestCheckResourceAttr(address, "config.behaviors.custom.0.actions.stream_logs.log_destination", traffic),
		)
	} else {
		checks = append(checks,
			resource.TestCheckNoResourceAttr(address, "config.behaviors.default.actions.stream_logs.log_destination"),
			resource.TestCheckNoResourceAttr(address, "config.behaviors.custom.0.actions.stream_logs.log_destination"),
		)
	}
	for i, name := range names {
		prefix := fmt.Sprintf("config.log_destinations.%d.", i)
		source := "traffic"
		fields := []string{"timestamp", "service_id", "service_uid", "status_code", "request_headers", "response_headers"}
		if security && name == "alpha" {
			source = "security"
			fields = []string{"timestamp", "service_id", "service_uid", "action", "severity_level", "ja4"}
		}
		checks = append(checks,
			resource.TestCheckResourceAttr(address, prefix+"name", name),
			resource.TestCheckResourceAttrSet(address, prefix+"uuid"),
			resource.TestCheckResourceAttr(address, prefix+"source", source),
			resource.TestCheckResourceAttr(address, prefix+"timestamp_format", "unix"),
			resource.TestCheckResourceAttr(address, prefix+"object_name", "/delivery/{random}.gz"),
			resource.TestCheckResourceAttr(address, prefix+"max_samples_per_object", "250000"),
			resource.TestCheckResourceAttr(address, prefix+"max_gap_seconds", "600"),
			resource.TestCheckResourceAttr(address, prefix+"selected_fields.#", fmt.Sprint(len(fields))),
			resource.TestCheckNoResourceAttr(address, prefix+block+".credentials.token"),
			resource.TestCheckNoResourceAttr(address, prefix+block+".credentials.access_key.access_key"),
			resource.TestCheckNoResourceAttr(address, prefix+block+".credentials.access_key.secret_key"),
			resource.TestCheckNoResourceAttr(address, prefix+block+".credentials.assume_role.role_arn"),
			resource.TestCheckNoResourceAttr(address, prefix+block+".credentials.assume_role.external_id"),
		)
		for j, field := range fields {
			checks = append(checks, resource.TestCheckResourceAttr(address, prefix+"selected_fields."+fmt.Sprint(j), field))
		}
		if source == "traffic" {
			checks = append(checks,
				resource.TestCheckResourceAttr(address, prefix+"request_headers.0", "x-request-id"),
				resource.TestCheckResourceAttr(address, prefix+"response_headers.0", "content-type"),
			)
		}
		if block == "splunk" {
			checks = append(checks,
				resource.TestCheckResourceAttr(address, prefix+"splunk.url", "https://splunk.example.com:8088/services/collector/raw"),
				resource.TestCheckResourceAttr(address, prefix+"splunk.channel_id", "00000000-0000-4000-8000-000000000001"),
				resource.TestCheckResourceAttr(address, prefix+"splunk.use_insecure", "false"),
			)
		} else {
			checks = append(checks,
				resource.TestCheckResourceAttr(address, prefix+block+".name", "test-log-bucket"),
				resource.TestCheckResourceAttr(address, prefix+block+".path", "/delivery"),
				resource.TestCheckResourceAttr(address, prefix+block+".region", "us-east-1"),
			)
			if block == "compatible_s3" {
				checks = append(checks, resource.TestCheckResourceAttr(address, prefix+block+".domain", "https://s3.example.com"))
			}
		}
	}
	if security {
		checks = append(checks,
			resource.TestCheckResourceAttr(address, "config.security.logs_shipping.destination", "alpha"),
			resource.TestCheckResourceAttr(address, "config.security.logs_shipping.enabled", "false"),
			resource.TestCheckResourceAttr(address, "config.security.logs_shipping.severity_min_threshold", "high"),
			resource.TestCheckResourceAttr(address, "config.security.logs_shipping.allowed_requests_sampling_rate", "25"),
		)
	} else {
		checks = append(checks, resource.TestCheckNoResourceAttr(address, "config.security.logs_shipping.destination"))
	}
	return resource.ComposeTestCheckFunc(checks...)
}

func testAccCheckLoggingIdentity(address string, names []string, replaced string,
	serviceID *string, destinationIDs map[string]string) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		rs, ok := state.RootModule().Resources[address]
		if !ok || rs.Primary.ID == "" {
			return fmt.Errorf("service is missing from state")
		}
		if *serviceID != "" && *serviceID != rs.Primary.ID {
			return fmt.Errorf("logging update replaced the service")
		}
		*serviceID = rs.Primary.ID
		for i, name := range names {
			id := rs.Primary.Attributes[fmt.Sprintf("config.log_destinations.%d.uuid", i)]
			if old := destinationIDs[name]; old != "" {
				if name == replaced && old == id {
					return fmt.Errorf("source change did not replace destination %q", name)
				}
				if name != replaced && old != id {
					return fmt.Errorf("unchanged destination %q was replaced", name)
				}
			}
			destinationIDs[name] = id
		}
		return nil
	}
}

func TestAccIORiverService_LoggingConfigLifecycle(t *testing.T) {
	t.Parallel()
	kinds := []string{"aws_s3", "aws_s3_assume_role", "compatible_s3", "compatible_s3_assume_role", "splunk"}
	kind := kinds[rand.IntN(len(kinds))]
	t.Run(kind, func(t *testing.T) {
		name := generateRandomResourceName()
		address := serviceResourceType + "." + name
		certID := os.Getenv("IORIVER_TEST_CERT_ID")
		var serviceID string
		destinationIDs := map[string]string{}
		destination := func(item, source string, version int, creds bool) string {
			return testAccLoggingDestination(kind, item, source, version, creds)
		}
		config := func(items []string, traffic, security string) string {
			return testAccLoggingConfig(name, certID, items, traffic, security)
		}
		alpha := destination("alpha", "traffic", 1, true)
		alphaNoCreds := destination("alpha", "traffic", 1, false)
		beta := destination("beta", "traffic", 1, true)
		betaNoCreds := destination("beta", "traffic", 1, false)
		securityAlpha := destination("alpha", "security", 2, true)
		resource.Test(t, resource.TestCase{
			PreCheck: func() {
				testAccPreEnvVariable(t, "IORIVER_API_TOKEN")
				testAccPreEnvVariable(t, "IORIVER_TEST_CERT_ID")
			},
			ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
			CheckDestroy: func(s *terraform.State) error {
				return testAccCheckResourceDestroy[ServiceWithConfig](s, TestedService{}, serviceResourceType)
			},
			Steps: []resource.TestStep{
				{
					Config:      config([]string{alphaNoCreds}, "", ""),
					PlanOnly:    true,
					ExpectError: regexp.MustCompile(`(?s)requires.*credentials`),
				},
				{
					// Create with credentials and verify Terraform's refreshed state.
					Config: config([]string{alpha}, "alpha", ""),
					Check: resource.ComposeTestCheckFunc(
						resource.TestCheckResourceAttr(address, "config.log_destinations.0."+strings.TrimSuffix(kind, "_assume_role")+".credentials_version", "1"),
						testAccCheckLoggingState(address, kind, []string{"alpha"}, "", "alpha", false, &serviceID, destinationIDs),
					),
				},
				{
					Config: config([]string{alphaNoCreds}, "alpha", ""),
					Check:  testAccCheckLoggingState(address, kind, []string{"alpha"}, "", "alpha", false, &serviceID, destinationIDs),
				},
				{
					Config:      config([]string{alphaNoCreds, destination("beta", "traffic", 1, false)}, "alpha", ""),
					PlanOnly:    true,
					ExpectError: regexp.MustCompile(`(?s)requires.*credentials`),
				},
				{
					Config: `resource "terraform_data" "credential" { input = "" }` +
						config([]string{alphaNoCreds, testAccLoggingDestinationWithDeferredCredential(kind, "beta")}, "alpha", ""),
					PlanOnly: true, ExpectNonEmptyPlan: true,
				},
				{
					Config: `resource "terraform_data" "credential" { input = "" }` +
						config([]string{alphaNoCreds, testAccLoggingDestinationWithDeferredCredential(kind, "beta")}, "alpha", ""),
					ExpectError: regexp.MustCompile(`(?s)requires.*credentials`),
				},
				{
					// Add beta without resending alpha's write-only credentials.
					Config: config([]string{alphaNoCreds, beta}, "alpha", ""),
					Check:  testAccCheckLoggingState(address, kind, []string{"alpha", "beta"}, "", "alpha", false, &serviceID, destinationIDs),
				},
				{
					Config:      config([]string{destination("alpha", "traffic", 2, false), betaNoCreds}, "alpha", ""),
					PlanOnly:    true,
					ExpectError: regexp.MustCompile(`(?s)requires.*credentials`),
				},
				{
					// Reorder and rotate alpha in the same apply; both UUIDs must survive.
					Config: config([]string{betaNoCreds, destination("alpha", "traffic", 2, true)}, "alpha", ""),
					Check: resource.ComposeTestCheckFunc(
						resource.TestCheckResourceAttr(address, "config.log_destinations.1."+strings.TrimSuffix(kind, "_assume_role")+".credentials_version", "2"),
						testAccCheckLoggingState(address, kind, []string{"beta", "alpha"}, "", "alpha", false, &serviceID, destinationIDs),
					),
				},
				{
					Config:      config([]string{betaNoCreds}, "alpha", ""),
					PlanOnly:    true,
					ExpectError: regexp.MustCompile(`is not defined`),
				},
				{
					Config:      config([]string{betaNoCreds, destination("alpha", "security", 2, false)}, "", "alpha"),
					PlanOnly:    true,
					ExpectError: regexp.MustCompile(`(?s)requires.*credentials`),
				},
				{
					Config:      config([]string{betaNoCreds, securityAlpha}, "alpha", ""),
					PlanOnly:    true,
					ExpectError: regexp.MustCompile(`must use source "traffic"`),
				},
				{
					Config:      config([]string{betaNoCreds, securityAlpha}, "", "beta"),
					PlanOnly:    true,
					ExpectError: regexp.MustCompile(`must use source "security"`),
				},
				{
					// Replace only alpha's destination identity, not the service.
					Config: config([]string{betaNoCreds, securityAlpha}, "beta", "alpha"),
					ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(address, plancheck.ResourceActionUpdate),
					}},
					Check: testAccCheckLoggingState(address, kind, []string{"beta", "alpha"}, "alpha", "beta", true, &serviceID, destinationIDs),
				},
				{
					Config: config([]string{betaNoCreds, destination("alpha", "security", 2, false)}, "beta", "alpha"),
					Check:  testAccCheckLoggingState(address, kind, []string{"beta", "alpha"}, "", "beta", true, &serviceID, destinationIDs),
				},
				{
					Config:      config([]string{betaNoCreds}, "beta", "alpha"),
					PlanOnly:    true,
					ExpectError: regexp.MustCompile(`is not defined`),
				},
				{
					Config: config([]string{betaNoCreds, destination("alpha", "traffic", 2, true)}, "alpha", ""),
					ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(address, plancheck.ResourceActionUpdate),
					}},
					Check: testAccCheckLoggingState(address, kind, []string{"beta", "alpha"}, "alpha", "alpha", false, &serviceID, destinationIDs),
				},
				{
					Config: config([]string{betaNoCreds}, "beta", ""),
					Check:  testAccCheckLoggingState(address, kind, []string{"beta"}, "", "beta", false, &serviceID, destinationIDs),
				},
				{
					Config: config([]string{betaNoCreds, destination("gamma", "traffic", 1, true)}, "gamma", ""),
					Check:  testAccCheckLoggingState(address, kind, []string{"beta", "gamma"}, "", "gamma", false, &serviceID, destinationIDs),
				},
				{
					Config: config(nil, "", ""),
					Check:  testAccCheckLoggingState(address, kind, nil, "", "", false, &serviceID, destinationIDs),
				},
				{Config: config(nil, "", ""), PlanOnly: true, ExpectNonEmptyPlan: false},
			},
		})
	})
}

func testAccLoggingDestinationWithDeferredCredential(kind, name string) string {
	destination := testAccLoggingDestination(kind, name, "traffic", 1, true)
	credential := testAccLogDestCredsKey
	if kind == "splunk" {
		credential = "test-token-1"
	} else if strings.HasSuffix(kind, "_assume_role") {
		credential = "arn:aws:iam::123456789012:role/IORiverLogsRole"
	}
	return strings.Replace(destination, fmt.Sprintf("%q", credential), `terraform_data.credential.output`, 1)
}

func TestServiceLoggingConfigPlans(t *testing.T) {
	for _, kind := range []string{"aws_s3", "aws_s3_assume_role", "compatible_s3", "compatible_s3_assume_role", "splunk"} {
		t.Run(kind, func(t *testing.T) {
			traffic := testAccLoggingDestination(kind, "traffic", "traffic", 1, true)
			security := testAccLoggingDestination(kind, "security", "security", 1, true)
			resource.UnitTest(t, resource.TestCase{
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Steps: []resource.TestStep{{
					Config: `provider "ioriver" { endpoint = "http://127.0.0.1:1/" }` +
						testAccLoggingConfig("logging", "00000000-0000-4000-8000-000000000001", []string{traffic, security}, "traffic", "security"),
					PlanOnly: true, ExpectNonEmptyPlan: true,
				}},
			})
		})
	}
	t.Run("private S3 addition config", func(t *testing.T) {
		resource.UnitTest(t, resource.TestCase{
			ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
			Steps: []resource.TestStep{{
				Config: `provider "ioriver" { endpoint = "http://127.0.0.1:1/" }` +
					strings.Replace(testAccPrivateS3ConfigAdditionalOrigin("logging", "00000000-0000-4000-8000-000000000001", true),
						"credentials_version = 2", fmt.Sprintf("credentials_version = 2\n s3_aws_key = %q\n s3_aws_secret = %q", testAccPrivateS3TestKey, testAccPrivateS3TestSecret), 1),
				PlanOnly: true, ExpectNonEmptyPlan: true,
			}},
		})
	})
}

func TestServiceLoggingConfigComputedFieldsPlans(t *testing.T) {
	for _, source := range []string{"traffic", "security", ""} {
		for _, format := range []string{"json-list", "json-object", "json-line-delimited", "csv"} {
			t.Run(source+"/"+format, func(t *testing.T) {
				destination := fmt.Sprintf(`{
					name = "logs"
					file_format = %q
					aws_s3 = {
						name = "test-log-bucket"
						region = "us-east-1"
						credentials_version = 1
						credentials = { access_key = { access_key = %q, secret_key = %q } }
					}
				}`, format, testAccLogDestCredsKey, testAccLogDestCredsSecret)
				if source != "" {
					destination = strings.Replace(destination, `name = "logs"`, fmt.Sprintf("name = \"logs\"\nsource = %q", source), 1)
				}
				resource.UnitTest(t, resource.TestCase{
					ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
					Steps: []resource.TestStep{{
						Config: `provider "ioriver" { endpoint = "http://127.0.0.1:1/" }` +
							testAccLoggingConfig("logging", "00000000-0000-4000-8000-000000000001", []string{destination}, "", ""),
						PlanOnly: true, ExpectNonEmptyPlan: true,
						ConfigPlanChecks: resource.ConfigPlanChecks{
							PostApplyPreRefresh: []plancheck.PlanCheck{
								plancheck.ExpectUnknownValue("ioriver_service.logging",
									tfjsonpath.New("config").AtMapKey("log_destinations").AtSliceIndex(0).AtMapKey("selected_fields")),
							},
						},
					}},
				})
			})
		}
	}
}

func TestServiceLoggingConfigComputedFieldsApply(t *testing.T) {
	captured := make(chan map[string]interface{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/v1/services/create_from_service_config/" {
			http.Error(w, "unexpected request", http.StatusBadRequest)
			return
		}
		var payload map[string]interface{}
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		captured <- payload
		http.Error(w, `{"detail":"computed fields capture complete"}`, http.StatusBadRequest)
	}))
	defer server.Close()
	destination := testAccLoggingDestination("splunk", "security", "security", 1, true)
	destination = strings.Replace(destination,
		`selected_fields = ["timestamp", "service_id", "service_uid", "action", "severity_level", "ja4"]`, "", 1)
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: fmt.Sprintf("provider \"ioriver\" { endpoint = %q }\n", server.URL+"/") +
				testAccLoggingConfig("logging", "00000000-0000-4000-8000-000000000001", []string{destination}, "", ""),
			ExpectError: regexp.MustCompile("computed fields capture complete"),
		}},
	})
	select {
	case payload := <-captured:
		config, ok := payload["service_config"].(map[string]interface{})
		if !ok {
			t.Fatal("request is missing service_config")
		}
		destinations, ok := config["log_destinations"].([]interface{})
		if !ok || len(destinations) != 1 {
			t.Fatal("request must contain one destination")
		}
		destination, ok := destinations[0].(map[string]interface{})
		if !ok {
			t.Fatal("invalid destination")
		}
		if _, exists := destination["selected_fields"]; exists {
			t.Fatal("omitted computed fields must be left to the backend")
		}
	default:
		t.Fatal("Terraform did not send the create request")
	}
}

func TestServiceLoggingConfigConflicts(t *testing.T) {
	traffic := testAccLoggingDestination("aws_s3", "traffic", "traffic", 1, true)
	security := testAccLoggingDestination("splunk", "security", "security", 1, true)
	for _, tc := range []struct {
		name, destination, traffic, security, message string
	}{
		{"default and custom traffic source", security, "security", "", `must use source "traffic"`},
		{"security shipping source", traffic, "", "traffic", `must use source "security"`},
		{"undefined traffic reference", traffic, "missing", "", `is not defined`},
		{"undefined security reference", security, "", "missing", `is not defined`},
		{"traffic fields on security source", strings.Replace(traffic, `source = "traffic"`, `source = "security"`, 1), "", "", `unsupported security field`},
		{"security fields on traffic source", strings.Replace(security, `source = "security"`, `source = "traffic"`, 1), "", "", `unsupported traffic field`},
		{"missing mandatory fields", strings.Replace(security, `"timestamp", `, "", 1), "", "", `selected_fields must include timestamp`},
		{"empty selected fields", strings.Replace(security, `["timestamp", "service_id", "service_uid", "action", "severity_level", "ja4"]`, `[]`, 1), "", "", `must contain at\s+least 1`},
		{"missing selected headers", strings.Replace(traffic, `request_headers = ["x-request-id"]`, `request_headers = []`, 1), "", "", `request_headers must be nonempty`},
		{"invalid header", strings.Replace(traffic, `"x-request-id"`, `"invalid header"`, 1), "", "", `invalid header name`},
		{"combined header limit", strings.Replace(traffic, `["x-request-id"]`, `[`+strings.Repeat(`"x-header",`, 20)+`]`, 1), "", "", `at most 20`},
		{"unselected header", strings.Replace(traffic, `, "request_headers"`, "", 1), "", "", `request_headers must be empty`},
		{"object template", strings.Replace(traffic, `{random}`, `fixed`, 1), "", "", `object_name must`},
		{"too many samples", strings.Replace(traffic, `max_samples_per_object = 250000`, `max_samples_per_object = 250001`, 1), "", "", `(?is)(250000|between)`},
		{"gap too short", strings.Replace(traffic, `max_gap_seconds = 600`, `max_gap_seconds = 59`, 1), "", "", `(?is)(60|between)`},
		{"missing S3 secret", strings.Replace(traffic, fmt.Sprintf("secret_key = %q", testAccLogDestCredsSecret), "", 1), "", "", `(?is)(secret_key|credentials)`},
		{"empty S3 key", strings.Replace(traffic, fmt.Sprintf("%q", testAccLogDestCredsKey), `""`, 1), "", "", `requires credentials`},
		{"empty Splunk token", strings.Replace(security, `"test-token-1"`, `""`, 1), "", "", `requires credentials.token`},
		{"conflicting destination types", strings.TrimSuffix(traffic, "}") + `splunk = { url = "https://splunk.example.com", channel_id = "channel" } }`, "", "", `Invalid Attribute Combination`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resource.UnitTest(t, resource.TestCase{
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Steps: []resource.TestStep{{
					Config: `provider "ioriver" { endpoint = "http://127.0.0.1:1/" }` +
						testAccLoggingConfig("logging", "00000000-0000-4000-8000-000000000001", []string{tc.destination}, tc.traffic, tc.security),
					PlanOnly: true, ExpectError: regexp.MustCompile(tc.message),
				}},
			})
		})
	}
}

func TestServiceLoggingApplyResolvedEmptyCredentials(t *testing.T) {
	for _, kind := range []string{"aws_s3", "aws_s3_assume_role", "compatible_s3", "compatible_s3_assume_role", "splunk", "private_s3"} {
		t.Run(kind, func(t *testing.T) {
			var config string
			message := `(?s)requires.*credentials`
			if kind == "private_s3" {
				config = testAccPrivateS3ConfigFlipToPrivateWithCreds("logging", "00000000-0000-4000-8000-000000000001", 1)
				config = strings.Replace(config, fmt.Sprintf("%q", testAccPrivateS3TestKey), `terraform_data.credential.output`, 1)
				message = `requires s3_aws_key`
			} else {
				destination := testAccLoggingDestinationWithDeferredCredential(kind, "alpha")
				config = testAccLoggingConfig("logging", "00000000-0000-4000-8000-000000000001", []string{destination}, "", "")
			}
			configuration := `provider "ioriver" { endpoint = "http://127.0.0.1:1/" }
resource "terraform_data" "credential" { input = "" }
` + config
			resource.UnitTest(t, resource.TestCase{
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Steps: []resource.TestStep{
					{Config: configuration, PlanOnly: true, ExpectNonEmptyPlan: true},
					{Config: configuration, ExpectError: regexp.MustCompile(message)},
				},
			})
		})
	}
}

// ─── Config generators ────────────────────────────────────────────────────────

// testAccServiceConfigLogDestSteps returns step HCL for the log destination
// acceptance test. Parameters: resourceName, certId, logDestName1, logDestName2,
// behaviorName.
//
//	Step 0:  one log dest WITH credentials (create requires them)
//	Step 1:  same log dest, credentials unchanged
//	Step 1b: omit credentials — verifies API keeps existing ones when not sent
//	Step 2:  add a behavior that streams to it
//	Step 3:  add a second log dest, behavior still streams to first
//	Step 4:  swap order of the two log dests in HCL (regression for list ordering)
func testAccServiceConfigLogDestSteps(idx int, params ...any) string {

	steps := []string{
		// ── Step 0: one log destination, WITH credentials ─────────────────────
		`
locals {
	log_dest_name_1 = "%[4]s"
}

resource "ioriver_service" "%[1]s" {
	name        = "%[1]s"
	certificates = ["%[2]s"]
	description = "log dest flow test"

	config = {
		log_destinations = [
			{
				name = local.log_dest_name_1
				aws_s3 = {
					name   = "test-log-bucket"
					path   = "/"
					region = "us-east-1"
					credentials_version = 1
					credentials = {
						access_key = {
							access_key = "AKIAIOSFODNN7EXAMPLE"
							secret_key = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
						}
					}
				}
			}
		]
	}
}`,

		// ── Step 1: same log dest, WITH credentials ───────────────────────────
		`
locals {
	log_dest_name_1 = "%[4]s"
}

resource "ioriver_service" "%[1]s" {
	name        = "%[1]s"
	certificates = ["%[2]s"]
	description = "log dest flow test"

	config = {
		log_destinations = [
			{
				name = local.log_dest_name_1
				aws_s3 = {
					name   = "test-log-bucket"
					path   = "/"
					region = "us-east-1"
					credentials_version = 1
					credentials = {
						access_key = {
							access_key = "AKIAIOSFODNN7EXAMPLE"
							secret_key = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
						}
					}
				}
			}
		]
	}
}`,

		// ── Step 1b: omit credentials — tests optional-on-update semantics ──────
		// If the API accepts this, it keeps existing credentials (only need them
		// on create/rotate).
		`
locals {
	log_dest_name_1 = "%[4]s"
}

resource "ioriver_service" "%[1]s" {
	name        = "%[1]s"
	certificates = ["%[2]s"]
	description = "log dest flow test"

	config = {
		log_destinations = [
			{
				name = local.log_dest_name_1
				aws_s3 = {
					name   = "test-log-bucket"
					path   = "/"
					region = "us-east-1"
				}
			}
		]
	}
}`,

		// ── Step 2: add a behavior that streams to log_dest_name_1 ────────────
		`
locals {
	log_dest_name_1 = "%[4]s"
}

resource "ioriver_service" "%[1]s" {
	name        = "%[1]s"
	certificates = ["%[2]s"]
	description = "log dest flow test"

	config = {
		log_destinations = [
			{
				name = local.log_dest_name_1
				aws_s3 = {
					name   = "test-log-bucket"
					path   = "/"
					region = "us-east-1"
					credentials_version = 1
					credentials = {
						access_key = {
							access_key = "AKIAIOSFODNN7EXAMPLE"
							secret_key = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
						}
					}
				}
			}
		]

		behaviors = {
			custom = [
				{
					name         = "%[6]s"
					path_pattern = "//*"
					actions = {
						cache_behavior = "BYPASS"
						cache_ttl      = 0
						stream_logs = {
							log_destination   = local.log_dest_name_1
							log_sampling_rate = 100
						}
					}
				}
			]
		}
	}
}`,

		// ── Step 3: add second log destination ───────────────────────────────
		`
locals {
	log_dest_name_1 = "%[4]s"
	log_dest_name_2 = "%[5]s"
}

resource "ioriver_service" "%[1]s" {
	name        = "%[1]s"
	certificates = ["%[2]s"]
	description = "log dest flow test"

	config = {



		log_destinations = [
			{
				name = local.log_dest_name_1
				aws_s3 = {
					name   = "test-log-bucket"
					path   = "/"
					region = "us-east-1"
					credentials_version = 1
					credentials = {
						access_key = {
							access_key = "AKIAIOSFODNN7EXAMPLE"
							secret_key = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
						}
					}
				}
			},
			{
				name = local.log_dest_name_2
				aws_s3 = {
					name   = "test-log-bucket-2"
					path   = "/2/"
					region = "eu-west-1"
					credentials_version = 1
					credentials = {
						access_key = {
							access_key = "AKIAIOSFODNN7EXAMPLE"
							secret_key = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
						}
					}
				}
			}
		]

		behaviors = {
			custom = [
				{
					name         = "%[6]s"
					path_pattern = "//*"
					actions = {
						cache_behavior = "BYPASS"
						cache_ttl      = 0
						stream_logs = {
							log_destination   = local.log_dest_name_1
							log_sampling_rate = 100
						}
					}
				}
			]
		}
	}
}`,

		// ── Step 4: swap order of log destinations in HCL ────────────────────
		`
locals {
	log_dest_name_1 = "%[4]s"
	log_dest_name_2 = "%[5]s"
}

resource "ioriver_service" "%[1]s" {
	name        = "%[1]s"
	certificates = ["%[2]s"]
	description = "log dest flow test"

	config = {



		log_destinations = [
			{
				name = local.log_dest_name_2
				aws_s3 = {
					name   = "test-log-bucket-2"
					path   = "/2/"
					region = "eu-west-1"
					credentials_version = 1
					credentials = {
						access_key = {
							access_key = "AKIAIOSFODNN7EXAMPLE"
							secret_key = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
						}
					}
				}
			},
			{
				name = local.log_dest_name_1
				aws_s3 = {
					name   = "test-log-bucket"
					path   = "/"
					region = "us-east-1"
					credentials_version = 1
					credentials = {
						access_key = {
							access_key = "AKIAIOSFODNN7EXAMPLE"
							secret_key = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
						}
					}
				}
			}
		]

		behaviors = {
			custom = [
				{
					name         = "%[6]s"
					path_pattern = "//*"
					actions = {
						cache_behavior = "BYPASS"
						cache_ttl      = 0
						stream_logs = {
							log_destination   = local.log_dest_name_1
							log_sampling_rate = 100
						}
					}
				}
			]
		}
	}
}`,
		// step 5 - swap order AND update alpha's bucket name simultaneously.
		// This proves the plan modifier correctly handles reorder+update in one step.
		`
locals {
	log_dest_name_1 = "%[4]s"
	log_dest_name_2 = "%[5]s"
}

resource "ioriver_service" "%[1]s" {
	name        = "%[1]s"
	certificates = ["%[2]s"]
	description = "log dest flow test"

	config = {
		log_destinations = [
			{
				name = local.log_dest_name_2
				aws_s3 = {
					name   = "test-log-bucket-2"
					path   = "/2/"
					region = "eu-west-1"
					credentials_version = 1
					credentials = {
						access_key = {
							access_key = "AKIAIOSFODNN7EXAMPLE"
							secret_key = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
						}
					}
				}
			},
			{
				name = local.log_dest_name_1
				aws_s3 = {
					name   = "test-log-bucket-UPDATED"
					path   = "/"
					region = "us-east-1"
					credentials_version = 1
					credentials = {
						access_key = {
							access_key = "AKIAIOSFODNN7EXAMPLE"
							secret_key = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
						}
					}
				}
			}
		]

		behaviors = {
			custom = [
				{
					name         = "%[6]s"
					path_pattern = "/*"
					actions = {
						cache_behavior = "BYPASS"
						cache_ttl      = 0
						stream_logs = {
							log_destination   = local.log_dest_name_1
							log_sampling_rate = 100
						}
					}
				}
			]
		}
	}
}`,

		// ── Step 6: replace log dest 1 with compatible_s3 type ───────────────
		`
locals {
	log_dest_name_1 = "%[4]s"
	log_dest_name_2 = "%[5]s"
}

resource "ioriver_service" "%[1]s" {
	name        = "%[1]s"
	certificates = ["%[2]s"]
	description = "log dest flow test"

	config = {
		log_destinations = [
			{
				name = local.log_dest_name_2
				aws_s3 = {
					name   = "test-log-bucket-2"
					path   = "/2/"
					region = "eu-west-1"
					credentials_version = 1
					credentials = {
						access_key = {
							access_key = "AKIAIOSFODNN7EXAMPLE"
							secret_key = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
						}
					}
				}
			},
			{
				name = local.log_dest_name_1
				compatible_s3 = {
					name   = "compat-bucket"
					path   = "/"
					region = "us-east-1"
					domain = "https://s3.example.com"
					credentials_version = 1
					credentials = {
						access_key = {
							access_key = "AKIAIOSFODNN7EXAMPLE"
							secret_key = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
						}
					}
				}
			}
		]

		behaviors = {
			custom = [
				{
					name         = "%[6]s"
					path_pattern = "//*"
					actions = {
						cache_behavior = "BYPASS"
						cache_ttl      = 0
						stream_logs = {
							log_destination   = local.log_dest_name_1
							log_sampling_rate = 100
						}
					}
				}
			]
		}
	}
}`,

		// ── Step 7: remove log dest 1, only log dest 2 remains ───────────────
		`
locals {
	log_dest_name_2 = "%[5]s"
}

resource "ioriver_service" "%[1]s" {
	name        = "%[1]s"
	certificates = ["%[2]s"]
	description = "log dest flow test"

	config = {


		log_destinations = [
			{
				name = local.log_dest_name_2
				aws_s3 = {
					name   = "test-log-bucket-2"
					path   = "/2/"
					region = "eu-west-1"
					credentials_version = 1
					credentials = {
						access_key = {
							access_key = "AKIAIOSFODNN7EXAMPLE"
							secret_key = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
						}
					}
				}
			}
		]
	}
}`,
	}

	if idx < 0 || idx >= len(steps) {
		panic("invalid log dest step index")
	}
	// params order: resourceName, certId, (unused %[3]s placeholder), logDestName1, logDestName2, behaviorName
	// We use named indices (%[1]s…%[6]s), so pass all six positional args.
	// params received: resourceName, certId, logDestName1, logDestName2, behaviorName
	// Map to: %[1]s=resourceName, %[2]s=certId, %[3]s="" (unused), %[4]s=logDestName1, %[5]s=logDestName2, %[6]s=behaviorName
	rn := fmt.Sprintf("%v", params[0])
	ci := fmt.Sprintf("%v", params[1])
	d1 := fmt.Sprintf("%v", params[2])
	d2 := fmt.Sprintf("%v", params[3])
	bn := fmt.Sprintf("%v", params[4])
	return fmt.Sprintf(steps[idx], rn, ci, "", d1, d2, bn)
}

func testAccLogDestCredsConfigAWSNoCreds(resourceName, certID string, version int) string {
	return fmt.Sprintf(`
resource "ioriver_service" "%s" {
	name        = "%s"
	certificate = "%s"
	description = "log-dest-aws-creds"
	config = {
		log_destinations = [
			{
				name = "aws-dest"
				aws_s3 = {
					name                = "my-bucket"
					path                = "/"
					region              = "us-east-1"
					credentials_version = %d
				}
			}
		]
	}
}
`, resourceName, resourceName, certID, version)
}

func testAccLogDestCredsConfigAWSWithCreds(resourceName, certID string, version int) string {
	return fmt.Sprintf(`
resource "ioriver_service" "%s" {
	name        = "%s"
	certificate = "%s"
	description = "log-dest-aws-creds"
	config = {
		log_destinations = [
			{
				name = "aws-dest"
				aws_s3 = {
					name                = "my-bucket"
					path                = "/"
					region              = "us-east-1"
					credentials_version = %d
					credentials = {
						access_key = {
							access_key = %q
							secret_key = %q
						}
					}
				}
			}
		]
	}
}
`, resourceName, resourceName, certID, version, testAccLogDestCredsKey, testAccLogDestCredsSecret)
}

func testAccLogDestCredsConfigCompatibleNoCreds(resourceName, certID string, version int) string {
	return fmt.Sprintf(`
resource "ioriver_service" "%s" {
	name        = "%s"
	certificate = "%s"
	description = "log-dest-compatible-creds"
	config = {
		log_destinations = [
			{
				name = "compat-dest"
				compatible_s3 = {
					name                = "my-bucket"
					path                = "/"
					region              = "us-east-1"
					domain              = "https://s3.example.com"
					credentials_version = %d
				}
			}
		]
	}
}
`, resourceName, resourceName, certID, version)
}

func testAccLogDestCredsConfigCompatibleWithCreds(resourceName, certID string, version int) string {
	return fmt.Sprintf(`
resource "ioriver_service" "%s" {
	name        = "%s"
	certificate = "%s"
	description = "log-dest-compatible-creds"
	config = {
		log_destinations = [
			{
				name = "compat-dest"
				compatible_s3 = {
					name                = "my-bucket"
					path                = "/"
					region              = "us-east-1"
					domain              = "https://s3.example.com"
					credentials_version = %d
					credentials = {
						access_key = {
							access_key = %q
							secret_key = %q
						}
					}
				}
			}
		]
	}
}
`, resourceName, resourceName, certID, version, testAccLogDestCredsKey, testAccLogDestCredsSecret)
}

func testSplunkDestination() LogDestinationModel {
	return LogDestinationModel{
		Name:        types.StringValue("security-splunk"),
		Uuid:        types.StringValue("destination-uuid"),
		Source:      types.StringValue("security"),
		AnonymizeIp: types.BoolValue(false),
		FileFormat:  types.StringValue("json-line-delimited"),
		SelectedFields: types.ListValueMust(types.StringType, []attr.Value{
			types.StringValue("timestamp"), types.StringValue("service_id"), types.StringValue("service_uid"), types.StringValue("action"),
		}),
		RequestHeaders:      types.ListValueMust(types.StringType, []attr.Value{}),
		ResponseHeaders:     types.ListValueMust(types.StringType, []attr.Value{}),
		TimestampFormat:     types.StringValue("unix"),
		ObjectName:          types.StringValue("/security/%Y-%m-%d/{random}.gz"),
		MaxSamplesPerObject: types.Int64Value(250000),
		MaxGapSeconds:       types.Int64Value(600),
		Splunk: &EmbeddedSplunkLogDestinationModel{
			URL:         types.StringValue("https://splunk.example.com:8088/services/collector/raw"),
			ChannelID:   types.StringValue("channel-123"),
			UseInsecure: types.BoolValue(true),
		},
	}
}

func TestLogDestinationConfig_SplunkRoundTrip(t *testing.T) {
	original := testSplunkDestination()
	original.Splunk.Credentials = &SplunkCredsModel{Token: types.StringValue("test-token")}
	wire, err := original.ModelToMap()
	if err != nil {
		t.Fatal(err)
	}
	if wire["source"] != "SECURITY" || wire["type"] != "SPLUNK" {
		t.Fatalf("unexpected source/type: %v", wire)
	}
	if wire["credentials"] != `{"token":"test-token"}` {
		t.Fatalf("incorrect credentials payload: %v", wire["credentials"])
	}
	if _, exists := wire["splunk"].(map[string]interface{})["ca_cert"]; exists {
		t.Fatal("ca_cert must not be sent")
	}
	if redacted := redactedLogDestinationsForLog([]interface{}{wire}); strings.Contains(redacted, "test-token") {
		t.Fatal("log output must redact the Splunk token")
	}
	encoded, err := json.Marshal(wire)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	roundTrip, err := MapToModel(decoded)
	if err != nil {
		t.Fatal(err)
	}
	if roundTrip.Source.ValueString() != "security" || roundTrip.Splunk.Credentials != nil {
		t.Fatal("source must be lowercase and credentials must not enter state")
	}
	wireWithoutCredentials, err := roundTrip.ModelToMap()
	if err != nil {
		t.Fatal(err)
	}
	delete(wire, "credentials")
	if !reflect.DeepEqual(wire, wireWithoutCredentials) {
		t.Fatalf("round trip changed config:\nwant %v\ngot %v", wire, wireWithoutCredentials)
	}
	if _, diags := types.ObjectValueFrom(context.Background(), LogDestinationAttrTypes(), roundTrip); diags.HasError() {
		t.Fatalf("model/schema mismatch: %v", diags)
	}
}

func TestLogDestinationConfig_OmittedObjectName(t *testing.T) {
	for _, kind := range []string{"aws_s3", "compatible_s3", "splunk"} {
		t.Run(kind, func(t *testing.T) {
			destination := testSplunkDestination()
			switch kind {
			case "aws_s3":
				destination = firstLogDest(t, makeLogDestPlanData("destination", types.Int64Value(1), true))
			case "compatible_s3":
				destination = firstLogDest(t, makeCompatibleLogDestPlanData("destination", types.Int64Value(1), true))
			}
			destination.ObjectName = types.StringNull()
			wire, err := destination.ModelToMap()
			if err != nil {
				t.Fatal(err)
			}
			encoded, err := json.Marshal(wire)
			if err != nil {
				t.Fatal(err)
			}
			var decoded map[string]interface{}
			if err := json.Unmarshal(encoded, &decoded); err != nil {
				t.Fatal(err)
			}
			if _, exists := decoded["object_name"]; exists {
				t.Fatal("omitted object_name must not be serialized, including as null")
			}
			destination.ObjectName = types.StringValue("/delivery/{random}.gz")
			wire, err = destination.ModelToMap()
			if err != nil {
				t.Fatal(err)
			}
			if wire["object_name"] != "/delivery/{random}.gz" {
				t.Fatal("configured object_name must still be sent")
			}
		})
	}
}

func TestLogDestinationConfig_LegacyDefaults(t *testing.T) {
	destination, err := MapToModel(map[string]interface{}{
		"name": "traffic", "type": "S3", "s3_bucket": "bucket",
		"s3_path": "/", "s3_region": "us-east-1", "log_file_format": "json-list",
	})
	if err != nil {
		t.Fatal(err)
	}
	if destination.Source.ValueString() != "traffic" || destination.TimestampFormat.ValueString() != "iso8601" ||
		destination.MaxSamplesPerObject.ValueInt64() != 100000 || destination.MaxGapSeconds.ValueInt64() != 60 ||
		!destination.ObjectName.IsNull() || len(destination.SelectedFields.Elements()) != 0 {
		t.Fatalf("incorrect legacy defaults: %+v", destination)
	}
}

func TestLogDestinationConfig_SourceReplacementAndReorder(t *testing.T) {
	old := testSplunkDestination()
	old.Source = types.StringValue("traffic")
	other := testSplunkDestination()
	other.Name = types.StringValue("other")
	other.Uuid = types.StringValue("other-uuid")
	changed := testSplunkDestination()
	changed.Uuid = types.StringUnknown()
	plan := logDestListValue([]LogDestinationModel{other, changed})
	state := logDestListValue([]LogDestinationModel{old, other})
	modifier := LogDestinationListPlanModifier()
	response := &planmodifier.ListResponse{PlanValue: plan}
	modifier.PlanModifyList(context.Background(), planmodifier.ListRequest{
		PlanValue: plan, ConfigValue: plan, StateValue: state,
	}, response)
	destinations, err := ListElementsAs[LogDestinationModel](context.Background(), response.PlanValue)
	if err != nil {
		t.Fatal(err)
	}
	if destinations[0].Uuid.ValueString() != "other-uuid" || !destinations[1].Uuid.IsUnknown() {
		t.Fatalf("replacement must affect only changed destination: %+v", destinations)
	}
	transform := &ServiceTransformContext{LogDestNamesToUUIDs: map[string]string{"security-splunk": "destination-uuid", "other": "other-uuid"}}
	maps, err := LogDestinationsToMap(context.Background(), &destinations, transform)
	if err != nil {
		t.Fatal(err)
	}
	if maps[1]["uuid"] == "destination-uuid" || maps[0]["uuid"] != "other-uuid" {
		t.Fatal("source change must create a new destination UUID and preserve other UUIDs")
	}
	if transform.LogDestNamesToUUIDs["security-splunk"] != maps[1]["uuid"] {
		t.Fatal("references must use replacement UUID")
	}
}

func TestLogDestinationConfig_UnchangedSourceKeepsUUID(t *testing.T) {
	destination := testSplunkDestination()
	state := logDestListValue([]LogDestinationModel{destination})
	destination.Uuid = types.StringUnknown()
	plan := logDestListValue([]LogDestinationModel{destination})
	response := &planmodifier.ListResponse{PlanValue: plan}
	LogDestinationListPlanModifier().PlanModifyList(context.Background(), planmodifier.ListRequest{
		PlanValue: plan, ConfigValue: plan, StateValue: state,
	}, response)
	models, err := ListElementsAs[LogDestinationModel](context.Background(), response.PlanValue)
	if err != nil {
		t.Fatal(err)
	}
	if models[0].Uuid.ValueString() != "destination-uuid" {
		t.Fatal("unchanged source must preserve identity")
	}
}

func TestLogDestinationConfig_DynamicSource(t *testing.T) {
	destination := testSplunkDestination()
	state := logDestListValue([]LogDestinationModel{destination})
	destination.Source = types.StringUnknown()
	destination.Uuid = types.StringUnknown()
	plan := logDestListValue([]LogDestinationModel{destination})
	response := &planmodifier.ListResponse{PlanValue: plan}
	LogDestinationListPlanModifier().PlanModifyList(context.Background(), planmodifier.ListRequest{
		PlanValue: plan, ConfigValue: plan, StateValue: state,
	}, response)
	models, err := ListElementsAs[LogDestinationModel](context.Background(), response.PlanValue)
	if err != nil {
		t.Fatal(err)
	}
	if !models[0].Uuid.IsUnknown() {
		t.Fatal("an unknown source must not commit the plan to the old UUID")
	}
	for _, source := range []string{"security", "traffic"} {
		t.Run(source, func(t *testing.T) {
			resolved := models[0]
			resolved.Source = types.StringValue(source)
			data := &ServiceResourceModel{Config: &ServiceConfigModel{LogDestinations: logDestListValue([]LogDestinationModel{resolved})}}
			stateData := &ServiceResourceModel{Config: &ServiceConfigModel{LogDestinations: state}}
			if err := mergeLogDestCredentialsFromConfig(data, data, stateData); err != nil {
				t.Fatal(err)
			}
			final := firstLogDest(t, data)
			if source == "security" && final.Uuid.ValueString() != "destination-uuid" {
				t.Fatal("same resolved source must preserve UUID")
			}
			if source == "traffic" && !final.Uuid.IsUnknown() {
				t.Fatal("changed resolved source must replace UUID")
			}
		})
	}
}

func TestLogDestinationConfig_EmptyReadback(t *testing.T) {
	for _, prior := range []struct {
		name     string
		config   *ServiceConfigModel
		expected types.List
	}{
		{"explicit empty", &ServiceConfigModel{LogDestinations: logDestListValue([]LogDestinationModel{}), Behaviors: defaultBehaviorsBlockObjectValue}, logDestListValue([]LogDestinationModel{})},
		{"omitted", &ServiceConfigModel{LogDestinations: types.ListNull(types.ObjectType{AttrTypes: LogDestinationAttrTypes()}), Behaviors: defaultBehaviorsBlockObjectValue}, types.ListNull(types.ObjectType{AttrTypes: LogDestinationAttrTypes()})},
		{"import", nil, types.ListNull(types.ObjectType{AttrTypes: LogDestinationAttrTypes()})},
	} {
		for _, response := range []struct {
			name string
			data map[string]interface{}
		}{
			{"empty", map[string]interface{}{"log_destinations": []interface{}{}}},
			{"null", map[string]interface{}{"log_destinations": nil}},
			{"omitted", map[string]interface{}{}},
		} {
			t.Run(prior.name+"/"+response.name, func(t *testing.T) {
				transform := &ServiceTransformContext{LogDestNamesToUUIDs: map[string]string{}}
				if prior.config != nil {
					if _, err := prior.config.ModelToMap(context.Background(), transform); err != nil {
						t.Fatal(err)
					}
				}
				model, err := ServiceConfigMapToModel(context.Background(), response.data, transform, prior.config)
				if err != nil {
					t.Fatal(err)
				}
				if !model.LogDestinations.Equal(prior.expected) {
					t.Fatalf("expected %v, got %v", prior.expected, model.LogDestinations)
				}
				encoded, err := json.Marshal(transform)
				if err != nil {
					t.Fatal(err)
				}
				var restored ServiceTransformContext
				if err := json.Unmarshal(encoded, &restored); err != nil {
					t.Fatal(err)
				}
				refreshed, err := ServiceConfigMapToModel(context.Background(), response.data, &restored, model)
				if err != nil {
					t.Fatal(err)
				}
				if !refreshed.LogDestinations.Equal(prior.expected) {
					t.Fatalf("refresh expected %v, got %v", prior.expected, refreshed.LogDestinations)
				}
			})
		}
	}
}

func TestLogDestinationConfig_DefaultFieldsRemainStable(t *testing.T) {
	destination := testSplunkDestination()
	destination.SelectedFields = types.ListUnknown(types.StringType)
	config := &ServiceConfigModel{
		LogDestinations: logDestListValue([]LogDestinationModel{destination}),
		Behaviors:       defaultBehaviorsBlockObjectValue,
	}
	transform := &ServiceTransformContext{LogDestNamesToUUIDs: map[string]string{}}
	wire, err := config.ModelToMap(context.Background(), transform)
	if err != nil {
		t.Fatal(err)
	}
	destinations := wire["log_destinations"].([]interface{})
	if _, exists := destinations[0].(map[string]interface{})["selected_fields"]; exists {
		t.Fatal("omitted computed fields must not be sent to the backend")
	}
	destinations[0].(map[string]interface{})["selected_fields"] = []interface{}{"timestamp", "service_id", "service_uid", "action"}
	expected := types.ListValueMust(types.StringType, []attr.Value{
		types.StringValue("timestamp"), types.StringValue("service_id"), types.StringValue("service_uid"), types.StringValue("action"),
	})
	read, err := ServiceConfigMapToModel(context.Background(), wire, transform, config)
	if err != nil {
		t.Fatal(err)
	}
	models, err := ListElementsAs[LogDestinationModel](context.Background(), read.LogDestinations)
	if err != nil {
		t.Fatal(err)
	}
	if !models[0].SelectedFields.Equal(expected) {
		t.Fatal("apply must retain the backend-computed defaults")
	}
	imported, err := ServiceConfigMapToModel(context.Background(), wire, transform, nil)
	if err != nil {
		t.Fatal(err)
	}
	importedModels, err := ListElementsAs[LogDestinationModel](context.Background(), imported.LogDestinations)
	if err != nil {
		t.Fatal(err)
	}
	if !importedModels[0].SelectedFields.Equal(expected) {
		t.Fatal("import must retain actual backend fields without a prior config marker")
	}
	destination.SelectedFields = expected
	config.LogDestinations = logDestListValue([]LogDestinationModel{destination})
	read, err = ServiceConfigMapToModel(context.Background(), wire, transform, config)
	if err != nil {
		t.Fatal(err)
	}
	models, err = ListElementsAs[LogDestinationModel](context.Background(), read.LogDestinations)
	if err != nil {
		t.Fatal(err)
	}
	if !models[0].SelectedFields.Equal(expected) {
		t.Fatal("explicit selected_fields must survive refresh")
	}
	destination.SelectedFields = types.ListValueMust(types.StringType, nil)
	config.LogDestinations = logDestListValue([]LogDestinationModel{destination})
	read, err = ServiceConfigMapToModel(context.Background(), wire, transform, config)
	if err != nil {
		t.Fatal(err)
	}
	models, err = ListElementsAs[LogDestinationModel](context.Background(), read.LogDestinations)
	if err != nil {
		t.Fatal(err)
	}
	if !models[0].SelectedFields.Equal(expected) {
		t.Fatal("refresh must replace a legacy empty state marker with backend fields")
	}
}

func TestLogDestinationConfig_ComputedFieldsPlanStability(t *testing.T) {
	ctx := context.Background()
	first := testSplunkDestination()
	first.Name = types.StringValue("first")
	second := testSplunkDestination()
	second.Name = types.StringValue("second")
	second.Uuid = types.StringValue("second-uuid")
	second.SelectedFields = types.ListValueMust(types.StringType, []attr.Value{
		types.StringValue("timestamp"), types.StringValue("service_id"), types.StringValue("service_uid"),
	})
	state := logDestListValue([]LogDestinationModel{first, second})
	for _, tc := range []struct {
		name    string
		change  func(*LogDestinationModel)
		resolve bool
	}{
		{"unchanged", func(*LogDestinationModel) {}, true},
		{"source changed", func(d *LogDestinationModel) { d.Source = types.StringValue("traffic") }, false},
		{"source unknown", func(d *LogDestinationModel) { d.Source = types.StringUnknown() }, false},
		{"format changed", func(d *LogDestinationModel) { d.FileFormat = types.StringValue("csv") }, false},
		{"format unknown", func(d *LogDestinationModel) { d.FileFormat = types.StringUnknown() }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			plannedFirst, plannedSecond := first, second
			plannedFirst.SelectedFields = types.ListUnknown(types.StringType)
			plannedSecond.SelectedFields = types.ListUnknown(types.StringType)
			tc.change(&plannedFirst)
			plan := logDestListValue([]LogDestinationModel{plannedSecond, plannedFirst})
			response := &planmodifier.ListResponse{PlanValue: plan}
			LogDestinationListPlanModifier().PlanModifyList(ctx, planmodifier.ListRequest{
				PlanValue: plan, ConfigValue: plan, StateValue: state,
			}, response)
			if response.Diagnostics.HasError() {
				t.Fatal(response.Diagnostics)
			}
			destinations, err := ListElementsAs[LogDestinationModel](ctx, response.PlanValue)
			if err != nil {
				t.Fatal(err)
			}
			for _, destination := range destinations {
				if destination.Name.ValueString() == "second" {
					if !destination.SelectedFields.Equal(second.SelectedFields) {
						t.Fatal("computed fields must be restored by name, not list position")
					}
				} else if tc.resolve {
					if !destination.SelectedFields.Equal(first.SelectedFields) {
						t.Fatal("unchanged destination must retain computed backend fields")
					}
				} else if !destination.SelectedFields.IsUnknown() {
					t.Fatal("changed source/format must leave defaults for the backend")
				}
			}
		})
	}
}

func TestLogDestinationConfig_SplunkCredentialsLifecycle(t *testing.T) {
	destination := testSplunkDestination()
	model := func(destination LogDestinationModel) *ServiceResourceModel {
		return &ServiceResourceModel{Config: &ServiceConfigModel{LogDestinations: logDestListValue([]LogDestinationModel{destination})}}
	}
	plan, state := model(destination), model(destination)
	if err := validateCreateLogDestinationCredentials(plan, true); err == nil {
		t.Fatal("creation must require a token")
	}
	if err := validateUpdateLogDestinationCredentials(plan, state, true); err != nil {
		t.Fatalf("ordinary update may omit credentials: %v", err)
	}
	old := destination
	old.Source = types.StringValue("traffic")
	if err := validateUpdateLogDestinationCredentials(plan, model(old), true); err == nil {
		t.Fatal("source replacement requires a token")
	}
	destination.Splunk.Credentials = &SplunkCredsModel{Token: types.StringValue("updated-token")}
	destination.Splunk.CredentialsVersion = types.Int64Value(1)
	config := model(destination)
	planned := testSplunkDestination()
	planned.Splunk.CredentialsVersion = types.Int64Value(1)
	plan = model(planned)
	if err := validateUpdateLogDestinationCredentials(config, model(old), true); err != nil {
		t.Fatal(err)
	}
	if err := mergeLogDestCredentialsFromConfig(plan, config, state); err != nil {
		t.Fatal(err)
	}
	if firstLogDest(t, plan).Splunk.Credentials.Token.ValueString() != "updated-token" {
		t.Fatal("configured token must be sent on update")
	}
}

func TestLogDestinationConfig_SplunkCredentialsVersion(t *testing.T) {
	model := func(version types.Int64, withToken bool) *ServiceResourceModel {
		destination := testSplunkDestination()
		destination.Splunk.CredentialsVersion = version
		if withToken {
			destination.Splunk.Credentials = &SplunkCredsModel{Token: types.StringValue("rotation-token")}
		}
		return &ServiceResourceModel{Config: &ServiceConfigModel{
			LogDestinations: logDestListValue([]LogDestinationModel{destination}),
		}}
	}
	for _, tc := range []struct {
		name         string
		version      types.Int64
		stateVersion types.Int64
		create       bool
		send         bool
	}{
		{"create", types.Int64Value(1), types.Int64Null(), true, true},
		{"same version", types.Int64Value(1), types.Int64Value(1), false, false},
		{"bumped version", types.Int64Value(2), types.Int64Value(1), false, true},
		{"omitted version", types.Int64Null(), types.Int64Value(1), false, false},
		{"unknown version", types.Int64Unknown(), types.Int64Value(1), false, false},
		{"post import", types.Int64Value(1), types.Int64Null(), false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			plan, config := model(tc.version, false), model(tc.version, true)
			state := model(tc.stateVersion, false)
			if tc.create {
				state = nil
			}
			if err := mergeLogDestCredentialsFromConfig(plan, config, state); err != nil {
				t.Fatal(err)
			}
			if (firstLogDest(t, plan).Splunk.Credentials != nil) != tc.send {
				t.Fatalf("expected send=%v", tc.send)
			}
			if !tc.create {
				err := validateUpdateLogDestinationCredentials(model(tc.version, false), state, true)
				if (err != nil) != tc.send {
					t.Fatalf("expected token required=%v, got %v", tc.send, err)
				}
			}
		})
	}
	for _, version := range []types.Int64{types.Int64Null(), types.Int64Unknown(), types.Int64Value(1)} {
		for _, state := range []*ServiceResourceModel{
			{Config: &ServiceConfigModel{LogDestinations: logDestListValue(nil)}},
			makeLogDestStateData("security-splunk", types.Int64Value(1)),
		} {
			for _, allowUnknown := range []bool{true, false} {
				if err := validateUpdateLogDestinationCredentials(model(version, false), state, allowUnknown); err == nil {
					t.Fatalf("new Splunk destination must require token for version %v", version)
				}
				if err := validateUpdateLogDestinationCredentials(model(version, true), state, allowUnknown); err != nil {
					t.Fatalf("supplied token must allow new Splunk destination: %v", err)
				}
				plan := model(version, false)
				if err := mergeLogDestCredentialsFromConfig(plan, model(version, true), state); err != nil {
					t.Fatal(err)
				}
				if firstLogDest(t, plan).Splunk.Credentials == nil {
					t.Fatal("new Splunk destination must send supplied token independently of version")
				}
				merged := firstLogDest(t, plan)
				wire, err := merged.ModelToMap()
				if err != nil {
					t.Fatal(err)
				}
				if wire["credentials"] != `{"token":"rotation-token"}` {
					t.Fatal("new Splunk token must reach the serialized API payload")
				}
			}
		}
	}
}

func TestLogDestinationConfig_SplunkVersionStateRoundTrip(t *testing.T) {
	destination := testSplunkDestination()
	destination.Splunk.CredentialsVersion = types.Int64Value(3)
	config := &ServiceConfigModel{
		LogDestinations: logDestListValue([]LogDestinationModel{destination}),
		Behaviors:       defaultBehaviorsBlockObjectValue,
	}
	transform := &ServiceTransformContext{LogDestNamesToUUIDs: map[string]string{}}
	wire, err := config.ModelToMap(context.Background(), transform)
	if err != nil {
		t.Fatal(err)
	}
	splunk := wire["log_destinations"].([]interface{})[0].(map[string]interface{})["splunk"].(map[string]interface{})
	if _, exists := splunk["credentials_version"]; exists {
		t.Fatal("credentials_version is Terraform-only")
	}
	read, err := ServiceConfigMapToModel(context.Background(), wire, transform, config)
	if err != nil {
		t.Fatal(err)
	}
	models, err := ListElementsAs[LogDestinationModel](context.Background(), read.LogDestinations)
	if err != nil {
		t.Fatal(err)
	}
	if models[0].Splunk.CredentialsVersion.ValueInt64() != 3 {
		t.Fatal("version must be retained in state")
	}
	imported, err := ServiceConfigMapToModel(context.Background(), wire, transform, nil)
	if err != nil {
		t.Fatal(err)
	}
	models, err = ListElementsAs[LogDestinationModel](context.Background(), imported.LogDestinations)
	if err != nil {
		t.Fatal(err)
	}
	if !models[0].Splunk.CredentialsVersion.IsNull() {
		t.Fatal("import must not invent a credentials version")
	}
}

func TestLogDestinationConfig_S3SourceReplacementCredentials(t *testing.T) {
	plan := makeLogDestPlanData("dest", types.Int64Value(1), false)
	config := makeLogDestConfigData("dest", types.Int64Value(1))
	state := makeLogDestStateData("dest", types.Int64Value(1))
	for _, data := range []*ServiceResourceModel{plan, config} {
		destination := firstLogDest(t, data)
		destination.Source = types.StringValue("security")
		data.Config.LogDestinations = logDestListValue([]LogDestinationModel{destination})
	}
	if err := validateUpdateLogDestinationCredentials(plan, state, true); err == nil {
		t.Fatal("source replacement must require credentials even without a version bump")
	}
	if err := mergeLogDestCredentialsFromConfig(plan, config, state); err != nil {
		t.Fatal(err)
	}
	if firstLogDest(t, plan).AwsS3.Credentials == nil {
		t.Fatal("source replacement must resend credentials despite unchanged version")
	}
}

func TestLogDestinationConfig_AddS3WithoutCredentialsVersion(t *testing.T) {
	for _, makePlan := range []func(string, types.Int64, bool) *ServiceResourceModel{
		makeLogDestPlanData, makeCompatibleLogDestPlanData,
	} {
		for _, version := range []types.Int64{types.Int64Null(), types.Int64Unknown(), types.Int64Value(1)} {
			plan := makePlan("new-destination", version, false)
			state := &ServiceResourceModel{Config: &ServiceConfigModel{
				LogDestinations: logDestListValue([]LogDestinationModel{}),
			}}
			requireCredentials := !version.IsNull() && !version.IsUnknown()
			for _, allowUnknown := range []bool{true, false} {
				err := validateUpdateLogDestinationCredentials(plan, state, allowUnknown)
				if (err != nil) != requireCredentials {
					t.Fatalf("version=%v, allowUnknown=%v: expected credentials required=%v, got %v", version, allowUnknown, requireCredentials, err)
				}
			}
		}
	}
}

func TestLogDestinationConfig_HeadersIndependentOfFields(t *testing.T) {
	for _, fieldCase := range []struct {
		name   string
		fields types.List
		source types.String
	}{
		{"omitted", types.ListNull(types.StringType), types.StringValue("traffic")},
		{"empty", types.ListValueMust(types.StringType, nil), types.StringValue("traffic")},
		{"unknown fields", types.ListUnknown(types.StringType), types.StringValue("traffic")},
		{"unknown field element", types.ListValueMust(types.StringType, []attr.Value{types.StringUnknown()}), types.StringValue("traffic")},
		{"unknown source", types.ListNull(types.StringType), types.StringUnknown()},
	} {
		t.Run(fieldCase.name, func(t *testing.T) {
			for _, headerCase := range []struct {
				name    string
				headers []attr.Value
				valid   bool
			}{
				{"valid boundary length", []attr.Value{types.StringValue(strings.Repeat("x", 128))}, true},
				{"invalid syntax", []attr.Value{types.StringValue("invalid header")}, false},
				{"too long", []attr.Value{types.StringValue(strings.Repeat("x", 129))}, false},
				{"null name", []attr.Value{types.StringNull()}, false},
				{"empty name", []attr.Value{types.StringValue("")}, false},
				{"unknown name", []attr.Value{types.StringUnknown()}, true},
			} {
				for _, responseHeaders := range []bool{false, true} {
					destination := testSplunkDestination()
					destination.Source = fieldCase.source
					destination.SelectedFields = fieldCase.fields
					headers := types.ListValueMust(types.StringType, headerCase.headers)
					if responseHeaders {
						destination.ResponseHeaders = headers
					} else {
						destination.RequestHeaders = headers
					}
					errs := ValidateLogDestination(context.Background(), destination)
					if (len(errs) == 0) != headerCase.valid {
						t.Fatalf("%s, response=%v: valid=%v, errors=%v", headerCase.name, responseHeaders, headerCase.valid, errs)
					}
				}
			}
			for _, count := range []int{20, 21} {
				destination := testSplunkDestination()
				destination.Source = fieldCase.source
				destination.SelectedFields = fieldCase.fields
				headers := make([]attr.Value, count-1)
				for i := range headers {
					headers[i] = types.StringValue("x-header")
				}
				destination.RequestHeaders = types.ListValueMust(types.StringType, headers)
				destination.ResponseHeaders = types.ListValueMust(types.StringType, []attr.Value{types.StringValue("content-type")})
				errs := ValidateLogDestination(context.Background(), destination)
				if (len(errs) == 0) != (count == 20) {
					t.Fatalf("combined count %d: errors=%v", count, errs)
				}
			}
		})
	}
}

func TestLogDestinationConfig_TypeChangeCredentials(t *testing.T) {
	model := func(kind string, version types.Int64, withCredentials bool) *ServiceResourceModel {
		switch kind {
		case "aws_s3":
			return makeLogDestPlanData("destination", version, withCredentials)
		case "compatible_s3":
			return makeCompatibleLogDestPlanData("destination", version, withCredentials)
		default:
			destination := testSplunkDestination()
			destination.Name = types.StringValue("destination")
			destination.Source = types.StringValue("traffic")
			destination.Splunk.CredentialsVersion = version
			if withCredentials {
				destination.Splunk.Credentials = &SplunkCredsModel{Token: types.StringValue("new-token")}
			}
			return &ServiceResourceModel{Config: &ServiceConfigModel{
				LogDestinations: logDestListValue([]LogDestinationModel{destination}),
			}}
		}
	}
	kinds := []string{"aws_s3", "compatible_s3", "splunk"}
	for _, from := range kinds {
		for _, to := range kinds {
			if from == to {
				continue
			}
			t.Run(from+" to "+to, func(t *testing.T) {
				for _, version := range []types.Int64{types.Int64Null(), types.Int64Unknown(), types.Int64Value(1)} {
					state := model(from, types.Int64Value(1), false)
					config := model(to, version, true)
					plan := model(to, version, false)
					for _, allowUnknown := range []bool{true, false} {
						if err := validateUpdateLogDestinationCredentials(plan, state, allowUnknown); err == nil {
							t.Fatal("type switch must require credentials independently of version")
						}
						if err := validateUpdateLogDestinationCredentials(config, state, allowUnknown); err != nil {
							t.Fatal(err)
						}
					}
					if err := mergeLogDestCredentialsFromConfig(plan, config, state); err != nil {
						t.Fatal(err)
					}
					merged := firstLogDest(t, plan)
					wire, err := merged.ModelToMap()
					if err != nil {
						t.Fatal(err)
					}
					expected := `{"access_key":"AKIAIOSFODNN7EXAMPLE","secret_key":"wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"}`
					if to == "splunk" {
						expected = `{"token":"new-token"}`
					}
					if wire["credentials"] != expected {
						t.Fatalf("type switch did not forward the new credentials: %v", wire["credentials"])
					}
				}
			})
		}
	}
}

func TestLogDestinationConfig_FieldValidation(t *testing.T) {
	for _, tc := range []struct {
		name   string
		source string
		field  string
		valid  bool
	}{
		{"security action", "security", "action", true},
		{"traffic action rejected", "traffic", "action", false},
		{"traffic status", "traffic", "status_code", true},
		{"security status rejected", "security", "status_code", false},
		{"unknown field", "security", "not-a-field", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			destination := testSplunkDestination()
			destination.Source = types.StringValue(tc.source)
			destination.SelectedFields = types.ListValueMust(types.StringType, []attr.Value{
				types.StringValue("timestamp"), types.StringValue("service_id"), types.StringValue("service_uid"), types.StringValue(tc.field),
			})
			errs := ValidateLogDestination(context.Background(), destination)
			if (len(errs) == 0) != tc.valid {
				t.Fatalf("valid=%v, errors=%v", tc.valid, errs)
			}
		})
	}
	destination := testSplunkDestination()
	destination.SelectedFields = types.ListValueMust(types.StringType, []attr.Value{types.StringValue("timestamp")})
	if errs := ValidateLogDestination(context.Background(), destination); len(errs) != 2 {
		t.Fatalf("missing service IDs must be rejected: %v", errs)
	}
	destination = testSplunkDestination()
	destination.Source = types.StringValue("traffic")
	destination.SelectedFields = types.ListValueMust(types.StringType, []attr.Value{
		types.StringValue("timestamp"), types.StringValue("service_id"), types.StringValue("service_uid"), types.StringValue("request_headers"),
	})
	if errs := ValidateLogDestination(context.Background(), destination); len(errs) == 0 {
		t.Fatal("selected headers must not be empty")
	}
	destination.RequestHeaders = types.ListValueMust(types.StringType, []attr.Value{types.StringValue("x-request-id")})
	if errs := ValidateLogDestination(context.Background(), destination); len(errs) != 0 {
		t.Fatal(errs)
	}
	headers := make([]attr.Value, 21)
	for i := range headers {
		headers[i] = types.StringValue("x-request-id")
	}
	destination.RequestHeaders = types.ListValueMust(types.StringType, headers)
	if errs := ValidateLogDestination(context.Background(), destination); len(errs) == 0 {
		t.Fatal("21 headers must be rejected")
	}
}

func TestSecurityLogsShipping_RoundTripAndClear(t *testing.T) {
	if _, exists := WafAttributes()["logs_shipping"]; exists {
		t.Fatal("logs_shipping belongs beside waf, not inside it")
	}
	if _, exists := SecurityAttributes()["logs_shipping"]; !exists {
		t.Fatal("security must expose logs_shipping")
	}
	shipping := &SecurityLogsShippingModel{
		Destination:               types.StringValue("security-splunk"),
		Enabled:                   types.BoolValue(true),
		SeverityMinThreshold:      types.StringValue("high"),
		AllowedSamplingPercentage: types.Float64Value(25),
	}
	security := &SecurityModel{Enabled: types.BoolValue(true), LogsShipping: shipping}
	securityObject, diags := types.ObjectValueFrom(context.Background(), SecurityAttrTypes(), security)
	if diags.HasError() {
		t.Fatal(diags)
	}
	destination := testSplunkDestination()
	config := &ServiceConfigModel{Security: securityObject, LogDestinations: logDestListValue([]LogDestinationModel{destination})}
	transform := &ServiceTransformContext{LogDestNamesToUUIDs: map[string]string{}}
	wire, err := config.ModelToMap(context.Background(), transform)
	if err != nil {
		t.Fatal(err)
	}
	waf := wire["waf"].(map[string]interface{})
	sent := waf["logs_shipping"].(map[string]interface{})
	if sent["destination"] != "destination-uuid" || sent["severity"] != "high" || sent["allowed_sampling_percentage"] != float64(25) {
		t.Fatalf("unexpected shipping payload: %v", sent)
	}
	if _, exists := sent["severity_min_threshsold"]; exists {
		t.Fatal("shipping payload must use severity, not the legacy spelling")
	}
	read, err := securityMapToModelWithCtx(context.Background(), waf, transform, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(read.LogsShipping, shipping) {
		t.Fatalf("shipping round trip mismatch: %+v", read.LogsShipping)
	}
	legacy := make(map[string]interface{}, len(sent))
	for key, value := range sent {
		legacy[key] = value
	}
	delete(legacy, "severity")
	legacy["severity_min_threshsold"] = "high"
	legacyShipping, err := securityLogsShippingFromMap(legacy, transform)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(legacyShipping, shipping) {
		t.Fatalf("legacy shipping read mismatch: %+v", legacyShipping)
	}
	if _, diags := types.ObjectValueFrom(context.Background(), SecurityAttrTypes(), read); diags.HasError() {
		t.Fatalf("readback schema mismatch: %v", diags)
	}
	if _, err := security.SecurityModelToMap(context.Background(), nil); err == nil {
		t.Fatal("shipping must not silently send a destination name without lookup context")
	}
	if _, err := security.SecurityModelToMap(context.Background(), &ServiceTransformContext{
		LogDestNamesToUUIDs: map[string]string{},
	}); err == nil {
		t.Fatal("unknown shipping destination must fail inside security conversion")
	}
	security.LogsShipping = nil
	cleared, err := security.SecurityModelToMap(context.Background(), transform)
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := cleared["logs_shipping"]; exists {
		t.Fatal("unconfigured logs_shipping must be omitted from the backend payload")
	}
}

func TestSecurityLogsShipping_DefaultObjectRoundTrip(t *testing.T) {
	defaults := map[string]interface{}{
		"destination":                 nil,
		"enabled":                     false,
		"severity":                    "medium",
		"allowed_sampling_percentage": float64(0),
	}
	read, err := securityLogsShippingFromMap(defaults, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !read.Destination.IsNull() || read.Enabled.ValueBool() ||
		read.SeverityMinThreshold.ValueString() != "medium" ||
		read.AllowedSamplingPercentage.ValueFloat64() != 0 {
		t.Fatalf("unexpected default logs_shipping model: %+v", read)
	}
	if _, diags := types.ObjectValueFrom(context.Background(), SecurityLogsShippingAttrTypes(), read); diags.HasError() {
		t.Fatalf("default logs_shipping does not match schema: %v", diags)
	}
	waf, err := (&SecurityModel{LogsShipping: read}).SecurityModelToMap(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(waf["logs_shipping"], defaults) {
		t.Fatalf("default logs_shipping payload mismatch: got %v, want %v", waf["logs_shipping"], defaults)
	}

	shipping := SecurityAttributes()["logs_shipping"].(schema.SingleNestedAttribute)
	if !shipping.Optional || !shipping.Computed || shipping.Default == nil {
		t.Fatal("logs_shipping must be optional and computed with an object default")
	}
	destination := shipping.Attributes["destination"].(schema.StringAttribute)
	if !destination.Optional || destination.Required {
		t.Fatal("logs_shipping destination must allow the default null value")
	}
}

func TestSecurityLogsShipping_References(t *testing.T) {
	destinations := map[string]LogDestinationModel{"security-splunk": testSplunkDestination()}
	for _, tc := range []struct {
		name, source string
		valid        bool
	}{
		{"security-splunk", "security", true},
		{"security-splunk", "traffic", false},
		{"missing", "security", false},
	} {
		message := validateLogDestinationReference(types.StringValue(tc.name), tc.source, destinations, false)
		if (message == "") != tc.valid {
			t.Fatalf("name=%s source=%s: %s", tc.name, tc.source, message)
		}
	}
	_, err := securityLogsShippingFromMap(map[string]interface{}{
		"destination": "missing-uuid", "severity_min_threshsold": "high",
	}, &ServiceTransformContext{LogDestNamesToUUIDs: map[string]string{}})
	if err == nil || !strings.Contains(err.Error(), "unknown destination") {
		t.Fatalf("unresolved UUID must fail explicitly: %v", err)
	}
}

func TestLogDestinationConfig_Limits(t *testing.T) {
	for _, tc := range []struct {
		field string
		value int64
		valid bool
	}{
		{"max_samples_per_object", 9999, false},
		{"max_samples_per_object", 10000, true},
		{"max_samples_per_object", 250000, true},
		{"max_samples_per_object", 250001, false},
		{"max_gap_seconds", 59, false},
		{"max_gap_seconds", 60, true},
		{"max_gap_seconds", 600, true},
		{"max_gap_seconds", 601, false},
	} {
		attribute := LogDestinationAttributes()[tc.field].(schema.Int64Attribute)
		response := &validator.Int64Response{}
		for _, check := range attribute.Validators {
			check.ValidateInt64(context.Background(), validator.Int64Request{ConfigValue: types.Int64Value(tc.value)}, response)
		}
		if response.Diagnostics.HasError() == tc.valid {
			t.Fatalf("%s=%d valid=%v: %v", tc.field, tc.value, tc.valid, response.Diagnostics)
		}
	}
	shipping := SecurityAttributes()["logs_shipping"].(schema.SingleNestedAttribute)
	severity := shipping.Attributes["severity_min_threshold"].(schema.StringAttribute)
	if !severity.Optional || !severity.Computed || severity.Required || severity.Default == nil {
		t.Fatal("shipping severity must be optional and computed with a default")
	}
	defaultResponse := &defaults.StringResponse{}
	severity.Default.DefaultString(context.Background(), defaults.StringRequest{}, defaultResponse)
	if defaultResponse.Diagnostics.HasError() || !defaultResponse.PlanValue.Equal(types.StringValue("medium")) {
		t.Fatalf("shipping severity must default to medium: %+v", defaultResponse)
	}
	for _, value := range []string{"info", "low", "medium", "high", "critical", "", "invalid"} {
		response := &validator.StringResponse{}
		for _, check := range severity.Validators {
			check.ValidateString(context.Background(), validator.StringRequest{ConfigValue: types.StringValue(value)}, response)
		}
		invalid := value == "" || value == "invalid"
		if response.Diagnostics.HasError() != invalid {
			t.Fatalf("incorrect shipping severity validation for %q: %v", value, response.Diagnostics)
		}
	}
	sampling := shipping.Attributes["allowed_requests_sampling_rate"].(schema.Float64Attribute)
	for _, value := range []float64{-0.1, 0, 100, 100.1} {
		response := &validator.Float64Response{}
		for _, check := range sampling.Validators {
			check.ValidateFloat64(context.Background(), validator.Float64Request{ConfigValue: types.Float64Value(value)}, response)
		}
		if response.Diagnostics.HasError() != (value < 0 || value > 100) {
			t.Fatalf("incorrect sampling limit for %v: %v", value, response.Diagnostics)
		}
	}
}

func TestLogDestinationConfig_ProviderSchema(t *testing.T) {
	server := providerserver.NewProtocol6(New("test")())()
	response, err := server.GetProviderSchema(context.Background(), &tfprotov6.GetProviderSchemaRequest{})
	if err != nil {
		t.Fatal(err)
	}
	for _, diagnostic := range response.Diagnostics {
		if diagnostic.Severity == tfprotov6.DiagnosticSeverityError {
			t.Fatalf("invalid provider schema: %s: %s", diagnostic.Summary, diagnostic.Detail)
		}
	}
}
