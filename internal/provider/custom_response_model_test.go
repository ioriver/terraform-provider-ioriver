package provider

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
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
			actual, err := customResponsesFromMap(ctx, decoded, &ServiceConfigModel{CustomResponses: list})
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
		prior := &ServiceConfigModel{CustomResponses: types.ListNull(elemType)}
		if explicit {
			prior.CustomResponses = types.ListValueMust(elemType, []attr.Value{})
		}
		actual, err := customResponsesFromMap(ctx, []interface{}{}, prior)
		if err != nil {
			t.Fatal(err)
		}
		if !actual.Equal(prior.CustomResponses) {
			t.Fatalf("explicit=%t: empty/null distinction lost", explicit)
		}
	}
}
