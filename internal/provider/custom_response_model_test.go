package provider

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

func TestCustomResponseModelRoundTrip(t *testing.T) {
	ctx := context.Background()
	headers, diags := types.MapValueFrom(ctx, types.StringType, map[string]string{"content-type": "text/html"})
	if diags.HasError() {
		t.Fatal(diags)
	}
	for _, body := range []string{"text", "attachment", "empty"} {
		t.Run(body, func(t *testing.T) {
			model := CustomResponseModel{
				UUID: types.StringValue("697dd97d-437e-46f8-915f-1e28bab9df2c"),
				Name: types.StringValue("blocked"), StatusCode: types.Int64Value(403),
				Headers: headers, Text: types.StringNull(), AttachedFileID: types.StringNull(),
			}
			if body == "text" {
				model.Text = types.StringValue("blocked")
			}
			if body == "attachment" {
				model.AttachedFileID = types.StringValue("fde32731-b531-4489-b7e7-9e93732353eb")
			}
			list, err := ListObjectValueFrom(ctx, CustomResponseAttrTypes(), []CustomResponseModel{model})
			if err != nil {
				t.Fatal(err)
			}
			payload, err := customResponsesToMap(ctx, list)
			if err != nil {
				t.Fatal(err)
			}
			if _, ok := payload[0].(map[string]interface{})["attached_file"]; ok {
				t.Fatal("service config must not contain upload contents")
			}
			encoded, err := json.Marshal(payload)
			if err != nil {
				t.Fatal(err)
			}
			var decoded interface{}
			if err := json.Unmarshal(encoded, &decoded); err != nil {
				t.Fatal(err)
			}
			actual, err := customResponsesFromMap(ctx, decoded, list)
			if err != nil {
				t.Fatal(err)
			}
			if !actual.Equal(list) {
				t.Fatalf("roundtrip changed response: %s", actual)
			}
		})
	}
}

func TestCustomResponseModelRejectsConflictingBody(t *testing.T) {
	ctx := context.Background()
	list, err := ListObjectValueFrom(ctx, CustomResponseAttrTypes(), []CustomResponseModel{{
		Name: types.StringValue("blocked"), StatusCode: types.Int64Value(403),
		Headers: types.MapNull(types.StringType),
		Text:    types.StringValue("blocked"), AttachedFileID: types.StringValue("file-id"),
	}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := customResponsesToMap(ctx, list); err == nil {
		t.Fatal("expected text and attachment conflict")
	}
}

func TestCustomResponseModelEmptyList(t *testing.T) {
	ctx := context.Background()
	elemType := types.ObjectType{AttrTypes: CustomResponseAttrTypes()}
	for _, explicit := range []bool{false, true} {
		prior := types.ListNull(elemType)
		if explicit {
			prior = types.ListValueMust(elemType, []attr.Value{})
		}
		actual, err := customResponsesFromMap(ctx, []interface{}{}, prior)
		if err != nil {
			t.Fatal(err)
		}
		if !actual.Equal(prior) {
			t.Fatalf("explicit=%t: empty/null distinction lost", explicit)
		}
	}
}

func TestCustomResponsesNestedSecurityWireRoundTrip(t *testing.T) {
	ctx := context.Background()
	if _, ok := ConfigAttributes()["custom_responses"]; ok {
		t.Fatal("custom responses must not be a top-level Terraform attribute")
	}
	if _, ok := SecurityAttributes()["custom_responses"]; !ok {
		t.Fatal("security must expose custom responses")
	}
	responses, err := ListObjectValueFrom(ctx, CustomResponseAttrTypes(), []CustomResponseModel{{
		UUID: types.StringValue("response-id"), Name: types.StringValue("blocked"),
		StatusCode: types.Int64Value(403), Headers: types.MapNull(types.StringType),
		Text: types.StringValue("blocked"), AttachedFileID: types.StringNull(),
	}})
	if err != nil {
		t.Fatal(err)
	}
	securityAttrs := defaultSecurityValue.Attributes()
	securityAttrs["custom_responses"] = responses
	security, diags := types.ObjectValue(SecurityAttrTypes(), securityAttrs)
	if diags.HasError() {
		t.Fatal(diags)
	}
	config := &ServiceConfigModel{Security: security, Behaviors: defaultBehaviorsBlockObjectValue}
	transform := &ServiceTransformContext{SecurityConfigured: true}
	wire, err := config.ModelToMap(ctx, transform)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := wire["custom_responses"].([]interface{}); !ok {
		t.Fatalf("expected top-level API custom_responses, got %T", wire["custom_responses"])
	}
	if _, ok := wire["waf"].(map[string]interface{})["custom_responses"]; ok {
		t.Fatal("responses must not be nested in the API waf object")
	}
	encoded, err := json.Marshal(wire)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	result, err := ServiceConfigMapToModel(ctx, decoded, transform, config)
	if err != nil {
		t.Fatal(err)
	}
	var decodedSecurity SecurityModel
	if diags := result.Security.As(ctx, &decodedSecurity, basetypes.ObjectAsOptions{}); diags.HasError() {
		t.Fatal(diags)
	}
	if !decodedSecurity.CustomResponses.Equal(responses) {
		t.Fatalf("response changed on read: %s", decodedSecurity.CustomResponses)
	}
}
