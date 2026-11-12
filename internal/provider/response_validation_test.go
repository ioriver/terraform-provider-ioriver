package provider

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestResponseUTF8ByteLimits(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		max  int
		fail bool
	}{
		{"inline boundary", strings.Repeat("a", 1024), 1024, false},
		{"inline overflow", strings.Repeat("a", 1025), 1024, true},
		{"multibyte boundary", strings.Repeat("\u00e9", 512), 1024, false},
		{"multibyte overflow", strings.Repeat("\u00e9", 513), 1024, true},
		{"file boundary", strings.Repeat("a", 2*1024*1024), 2 * 1024 * 1024, false},
		{"file overflow", strings.Repeat("a", 2*1024*1024+1), 2 * 1024 * 1024, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var resp validator.StringResponse
			stringvalidator.LengthAtMost(tc.max).ValidateString(context.Background(), validator.StringRequest{
				Path: path.Root("body"), ConfigValue: types.StringValue(tc.body),
			}, &resp)
			if resp.Diagnostics.HasError() != tc.fail {
				t.Fatalf("expected error=%t, got %v", tc.fail, resp.Diagnostics)
			}
		})
	}
}

func TestCustomResponseRedirectValidation(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		code    int64
		headers map[string]string
		fail    bool
	}{
		{100, nil, false}, {599, nil, false},
		{301, nil, true}, {302, map[string]string{"Location": " "}, true},
		{303, map[string]string{"location": "/login"}, false},
		{307, map[string]string{"LOCATION": "/login"}, false},
		{308, map[string]string{"other": "/login"}, true},
		{304, nil, false},
	} {
		headers, diags := types.MapValueFrom(ctx, types.StringType, tc.headers)
		if diags.HasError() {
			t.Fatal(diags)
		}
		err := validateCustomResponse(ctx, CustomResponseModel{
			Name: types.StringValue("test"), StatusCode: types.Int64Value(tc.code),
			Headers: headers, Text: types.StringNull(),
		})
		if (err != nil) != tc.fail {
			t.Fatalf("code=%d headers=%v: expected error=%t, got %v", tc.code, tc.headers, tc.fail, err)
		}
	}
}
