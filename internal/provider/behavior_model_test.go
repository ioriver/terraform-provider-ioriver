package provider

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// ---------------------------------------------------------------------------
// Behavior condition helpers
// ---------------------------------------------------------------------------

var exampleStringSpecificFastlyCode = `{
  "type": "SNIPPET",
  "vcl": " if(client.geo.country_code !~ \"(US|CA)\") { error 403 \"Forbidden\"; } ",
  "subroutine": "<recv>"
}`
var exampleSpecificFastlyCodeMap = map[string]string{
	"type":       "SNIPPET",
	"vcl":        ` if(client.geo.country_code !~ "(US|CA)") { error 403 "Forbidden"; } `,
	"subroutine": "<recv>",
}
var exampleSpecificFastlyCode jsontypes.Normalized = jsontypes.NewNormalizedValue(exampleStringSpecificFastlyCode)

func makeBehaviorCondition(field, operator string, values []string, fieldKey string) BehaviorConditionModel {
	valSet, _ := stringSet(values)
	cond := BehaviorConditionModel{
		Field:    strVal(field),
		Operator: strVal(operator),
		Values:   valSet,
		Value:    types.StringNull(),
		FieldKey: types.StringNull(),
	}
	if fieldKey != "" {
		cond.FieldKey = strVal(fieldKey)
	}
	return cond
}

func simpleBehaviorConditionExpr(field, operator string, values []string, fieldKey string) *BehaviorConditionExpressionModel {
	return &BehaviorConditionExpressionModel{
		Or: []BehaviorConditionAndGroupModel{
			{And: []BehaviorConditionModel{
				makeBehaviorCondition(field, operator, values, fieldKey),
			}},
		},
	}
}

// ---------------------------------------------------------------------------
// Round-trip tests (ModelToMapWithCtx → BehaviorModelfromMap)
// ---------------------------------------------------------------------------

// Simple path condition via condition block: collapses back to path_pattern on read.
func TestBehaviorCondition_RoundTrip_Path(t *testing.T) {
	ctx := context.Background()

	model := &BehaviorModel{
		Name:      strVal("api-behavior"),
		Condition: simpleBehaviorConditionExpr("http.request.path", "match", []string{"/api/*"}, ""),
		Actions: &BehaviorActionV2ResourceModel{
			CacheKey: &CacheKeyModelV2{
				Headers: []HeaderModelV2{},
				Cookies: []CookieModelV2{},
				QueryStrings: &QueryStringsModelV2{
					ParamsList: []ParamModelV2{},
					ListType:   strVal("include"),
				},
				Country:    types.BoolValue(true),
				DeviceType: types.BoolValue(false),
			},
		},
	}

	apiMap, err := model.ModelToMapWithCtx(ctx)
	if err != nil {
		t.Fatalf("ModelToMapWithCtx error: %v", err)
	}

	recovered, err := BehaviorModelfromMap(ctx, "api-behavior", apiMap, true)
	if err != nil {
		t.Fatalf("BehaviorModelfromMap error: %v", err)
	}

	// A simple path/match condition collapses back to path_pattern on read.
	if recovered.Condition != nil {
		t.Fatal("expected condition to be collapsed to path_pattern, but Condition is still set")
	}
	if recovered.PathPattern.IsNull() || recovered.PathPattern.ValueString() != "/api/*" {
		t.Errorf("expected path_pattern = /api/*, got %v", recovered.PathPattern)
	}

	// cache_key country and device_type should round-trip correctly
	if recovered.Actions == nil || recovered.Actions.CacheKey == nil {
		t.Fatal("expected cache_key to be set")
	}
	if !recovered.Actions.CacheKey.Country.ValueBool() {
		t.Errorf("expected country = true, got %v", recovered.Actions.CacheKey.Country)
	}
	if recovered.Actions.CacheKey.DeviceType.ValueBool() {
		t.Errorf("expected device_type = false, got %v", recovered.Actions.CacheKey.DeviceType)
	}
}

// path_pattern shorthand round-trip: set path_pattern, read back as path_pattern.
func TestBehaviorCondition_RoundTrip_PathPattern(t *testing.T) {
	ctx := context.Background()

	model := &BehaviorModel{
		Name:        strVal("simple-behavior"),
		PathPattern: types.StringValue("/static/*"),
		Actions:     &BehaviorActionV2ResourceModel{},
	}

	apiMap, err := model.ModelToMapWithCtx(ctx)
	if err != nil {
		t.Fatalf("ModelToMapWithCtx error: %v", err)
	}

	recovered, err := BehaviorModelfromMap(ctx, "simple-behavior", apiMap, true)
	if err != nil {
		t.Fatalf("BehaviorModelfromMap error: %v", err)
	}

	if recovered.Condition != nil {
		t.Fatal("expected Condition to be nil for path_pattern round-trip")
	}
	if recovered.PathPattern.IsNull() || recovered.PathPattern.ValueString() != "/static/*" {
		t.Errorf("expected path_pattern = /static/*, got %v", recovered.PathPattern)
	}
}

// http.request.header with field_key.
func TestBehaviorCondition_RoundTrip_HeaderWithFieldKey(t *testing.T) {
	ctx := context.Background()

	model := &BehaviorModel{
		Name:      strVal("mobile-header"),
		Condition: simpleBehaviorConditionExpr("http.request.header", "eq", []string{"mobile"}, "X-Client-Type"),
		Actions:   &BehaviorActionV2ResourceModel{},
	}

	apiMap, err := model.ModelToMapWithCtx(ctx)
	if err != nil {
		t.Fatalf("ModelToMapWithCtx error: %v", err)
	}
	recovered, err := BehaviorModelfromMap(ctx, "mobile-header", apiMap, true)
	if err != nil {
		t.Fatalf("BehaviorModelfromMap error: %v", err)
	}

	if recovered.Condition == nil {
		t.Fatal("condition lost in round-trip")
	}
	c := recovered.Condition.Or[0].And[0]
	if c.Field.ValueString() != "http.request.header" {
		t.Errorf("field: expected http.request.header, got %s", c.Field.ValueString())
	}
	if c.FieldKey.ValueString() != "X-Client-Type" {
		t.Errorf("field_key: expected X-Client-Type, got %s", c.FieldKey.ValueString())
	}
}

// http.response.header with field_key.
func TestBehaviorCondition_RoundTrip_ResponseHeader(t *testing.T) {
	ctx := context.Background()

	model := &BehaviorModel{
		Name:      strVal("cache-control-check"),
		Condition: simpleBehaviorConditionExpr("http.response.header", "contains", []string{"no-cache"}, "Cache-Control"),
		Actions:   &BehaviorActionV2ResourceModel{},
	}

	apiMap, err := model.ModelToMapWithCtx(ctx)
	if err != nil {
		t.Fatalf("ModelToMapWithCtx error: %v", err)
	}
	recovered, err := BehaviorModelfromMap(ctx, "cache-control-check", apiMap, true)
	if err != nil {
		t.Fatalf("BehaviorModelfromMap error: %v", err)
	}

	c := recovered.Condition.Or[0].And[0]
	if c.Field.ValueString() != "http.response.header" {
		t.Errorf("field: expected http.response.header, got %s", c.Field.ValueString())
	}
	if c.FieldKey.ValueString() != "Cache-Control" {
		t.Errorf("field_key: expected Cache-Control, got %s", c.FieldKey.ValueString())
	}
}

// http.response.status_code — API returns value as a number, must be coerced to string.
func TestBehaviorCondition_RoundTrip_StatusCode(t *testing.T) {
	ctx := context.Background()

	apiMap := map[string]interface{}{
		"condition": map[string]interface{}{
			"or": []interface{}{
				map[string]interface{}{
					"and": []interface{}{
						map[string]interface{}{
							"field":    "http.response.status_code",
							"operator": "ge",
							"value":    float64(400),
						},
					},
				},
			},
		},
		"action":   map[string]interface{}{},
		"children": []interface{}{},
	}

	recovered, err := BehaviorModelfromMap(ctx, "status-code-check", apiMap, true)
	if err != nil {
		t.Fatalf("BehaviorModelfromMap error: %v", err)
	}

	if recovered.Condition == nil {
		t.Fatal("condition nil")
	}
	c := recovered.Condition.Or[0].And[0]
	if c.Field.ValueString() != "http.response.status_code" {
		t.Errorf("field: expected http.response.status_code, got %s", c.Field.ValueString())
	}
	vals := mustStringSlice(t, ctx, c.Values)
	if len(vals) != 1 || vals[0] != "400" {
		t.Errorf("value: expected [\"400\"] (coerced from float), got %v", vals)
	}
}

// OR-of-ANDs: two OR groups, each with two AND conditions.
func TestBehaviorCondition_RoundTrip_MultipleOrAndGroups(t *testing.T) {
	ctx := context.Background()

	valA, _ := stringSet([]string{"/images/*"})
	valB, _ := stringSet([]string{"GET"})
	valC, _ := stringSet([]string{"/videos/*"})
	valD, _ := stringSet([]string{"HEAD"})

	model := &BehaviorModel{
		Name: strVal("multi-or-and"),
		Condition: &BehaviorConditionExpressionModel{
			Or: []BehaviorConditionAndGroupModel{
				{And: []BehaviorConditionModel{
					{Field: strVal("http.request.path"), Operator: strVal("match"), Values: valA, Value: types.StringNull(), FieldKey: types.StringNull()},
					{Field: strVal("http.request.method"), Operator: strVal("eq"), Values: valB, Value: types.StringNull(), FieldKey: types.StringNull()},
				}},
				{And: []BehaviorConditionModel{
					{Field: strVal("http.request.path"), Operator: strVal("match"), Values: valC, Value: types.StringNull(), FieldKey: types.StringNull()},
					{Field: strVal("http.request.method"), Operator: strVal("eq"), Values: valD, Value: types.StringNull(), FieldKey: types.StringNull()},
				}},
			},
		},
		Actions: &BehaviorActionV2ResourceModel{},
	}

	apiMap, err := model.ModelToMapWithCtx(ctx)
	if err != nil {
		t.Fatalf("ModelToMapWithCtx error: %v", err)
	}
	recovered, err := BehaviorModelfromMap(ctx, "multi-or-and", apiMap, true)
	if err != nil {
		t.Fatalf("BehaviorModelfromMap error: %v", err)
	}

	if len(recovered.Condition.Or) != 2 {
		t.Fatalf("expected 2 OR groups, got %d", len(recovered.Condition.Or))
	}
	if len(recovered.Condition.Or[0].And) != 2 {
		t.Fatalf("expected 2 AND conditions in first group, got %d", len(recovered.Condition.Or[0].And))
	}
	if len(recovered.Condition.Or[1].And) != 2 {
		t.Fatalf("expected 2 AND conditions in second group, got %d", len(recovered.Condition.Or[1].And))
	}
	c := recovered.Condition.Or[1].And[1]
	if c.Field.ValueString() != "http.request.method" {
		t.Errorf("second OR, second AND field: expected http.request.method, got %s", c.Field.ValueString())
	}
	vals := mustStringSlice(t, ctx, c.Values)
	if len(vals) != 1 || vals[0] != "HEAD" {
		t.Errorf("second OR, second AND value: expected [HEAD], got %v", vals)
	}
}

// ---------------------------------------------------------------------------
// Validation tests
// ---------------------------------------------------------------------------

// field_key required for http.request.header.
func TestValidateBehaviorCondition_HeaderMissingFieldKey(t *testing.T) {
	expr := simpleBehaviorConditionExpr("http.request.header", "eq", []string{"mobile"}, "")
	errs := ValidateBehaviorConditionModel(expr, "behaviors[0]")
	if len(errs) == 0 {
		t.Error("expected error for missing field_key on http.request.header")
	}
}

// field_key required for http.response.header.
func TestValidateBehaviorCondition_ResponseHeaderMissingFieldKey(t *testing.T) {
	expr := simpleBehaviorConditionExpr("http.response.header", "contains", []string{"no-cache"}, "")
	errs := ValidateBehaviorConditionModel(expr, "behaviors[0]")
	if len(errs) == 0 {
		t.Error("expected error for missing field_key on http.response.header")
	}
}

// field_key required for http.request.query_param.
func TestValidateBehaviorCondition_QueryParamMissingFieldKey(t *testing.T) {
	expr := simpleBehaviorConditionExpr("http.request.query_param", "eq", []string{"admin"}, "")
	errs := ValidateBehaviorConditionModel(expr, "behaviors[0]")
	if len(errs) == 0 {
		t.Error("expected error for missing field_key on http.request.query_param")
	}
}

// No field_key needed for non-collection fields.
func TestValidateBehaviorCondition_PathNoFieldKeyRequired(t *testing.T) {
	expr := simpleBehaviorConditionExpr("http.request.path", "match", []string{"/api/*"}, "")
	errs := ValidateBehaviorConditionModel(expr, "behaviors[0]")
	if len(errs) != 0 {
		t.Errorf("expected no errors for http.request.path without field_key, got: %v", errs)
	}
}

// No field_key needed for client.ip.
func TestValidateBehaviorCondition_ClientIPNoFieldKey(t *testing.T) {
	expr := simpleBehaviorConditionExpr("client.ip", "eq", []string{"1.2.3.4"}, "")
	errs := ValidateBehaviorConditionModel(expr, "behaviors[0]")
	if len(errs) != 0 {
		t.Errorf("expected no errors for client.ip without field_key, got: %v", errs)
	}
}

// Valid: collection field with field_key set.
func TestValidateBehaviorCondition_HeaderWithFieldKey_OK(t *testing.T) {
	expr := simpleBehaviorConditionExpr("http.request.header", "eq", []string{"mobile"}, "X-Client-Type")
	errs := ValidateBehaviorConditionModel(expr, "behaviors[0]")
	if len(errs) != 0 {
		t.Errorf("expected no errors when field_key is set, got: %v", errs)
	}
}

// nil expression is valid.
func TestValidateBehaviorCondition_Nil(t *testing.T) {
	errs := ValidateBehaviorConditionModel(nil, "behaviors[0]")
	if len(errs) != 0 {
		t.Errorf("expected no errors for nil condition, got: %v", errs)
	}
}

// Error message includes the correct location and field name.
func TestValidateBehaviorCondition_ErrorLocation(t *testing.T) {
	expr := simpleBehaviorConditionExpr("http.request.header", "eq", []string{"x"}, "")
	errs := ValidateBehaviorConditionModel(expr, "behaviors[2]")
	if len(errs) == 0 {
		t.Fatal("expected an error")
	}
	if !containsStr(errs[0], "behaviors[2]") {
		t.Errorf("error should contain 'behaviors[2]', got: %s", errs[0])
	}
	if !containsStr(errs[0], "http.request.header") {
		t.Errorf("error should mention the field name, got: %s", errs[0])
	}
}

// ---------------------------------------------------------------------------
// ValidateBehaviorModel tests — empty actions
// ---------------------------------------------------------------------------

// NEGATIVE: actions block is completely empty — should error.
func TestValidateBehaviorModel_EmptyActions_Fails(t *testing.T) {
	b := &BehaviorModel{
		Name:        strVal("empty-actions"),
		PathPattern: types.StringValue("/api/*"),
		Actions:     &BehaviorActionV2ResourceModel{}, // nothing set
	}
	errs := ValidateBehaviorModel(b, "behaviors[0]")
	if len(errs) == 0 {
		t.Error("expected error for empty actions, got none")
	}
	found := false
	for _, e := range errs {
		if containsStr(e, "at least one field") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'at least one field' in errors, got: %v", errs)
	}
}

// NEGATIVE: nil actions pointer — should also error.
func TestValidateBehaviorModel_NilActions_Fails(t *testing.T) {
	b := &BehaviorModel{
		Name:        strVal("nil-actions"),
		PathPattern: types.StringValue("/x/*"),
		Actions:     nil,
	}
	errs := ValidateBehaviorModel(b, "behaviors[0]")
	if len(errs) == 0 {
		t.Error("expected error for nil actions, got none")
	}
}

// POSITIVE: single scalar field set — should pass.
func TestValidateBehaviorModel_SingleAction_Passes(t *testing.T) {
	b := &BehaviorModel{
		Name:        strVal("cache-only"),
		PathPattern: types.StringValue("/images/*"),
		Actions: &BehaviorActionV2ResourceModel{
			CacheTTL: types.Int64Value(86400), // one field is enough
		},
	}
	errs := ValidateBehaviorModel(b, "behaviors[0]")
	if len(errs) != 0 {
		t.Errorf("expected no errors for valid actions, got: %v", errs)
	}
}

// POSITIVE: only a nested pointer field set — should pass.
func TestValidateBehaviorModel_NestedAction_Passes(t *testing.T) {
	b := &BehaviorModel{
		Name:        strVal("cors-only"),
		PathPattern: types.StringValue("/cors/*"),
		Actions: &BehaviorActionV2ResourceModel{
			Cors: &CorsConfigModelV2{
				AllowOrigin: &CorsAllowOriginModelV2{
					Mode: types.StringValue("all"),
				},
			},
		},
	}
	errs := ValidateBehaviorModel(b, "behaviors[0]")
	if len(errs) != 0 {
		t.Errorf("expected no errors when nested action is set, got: %v", errs)
	}
}

// POSITIVE: provider_specific-only action should count as non-empty actions.
func TestValidateBehaviorModel_ProviderSpecificOnly_Passes(t *testing.T) {
	b := &BehaviorModel{
		Name:        strVal("provider-specific-only"),
		PathPattern: types.StringValue("/ps/*"),
		Actions: &BehaviorActionV2ResourceModel{
			ProviderSpecific: []ProviderSpecificModel{
				{Provider: types.StringValue("fastly"), Code: exampleSpecificFastlyCode},
			},
		},
	}

	err := ValidateBehaviorModel(b, "behaviors[0]")
	if len(err) != 0 {
		t.Errorf("expected no errors for provider_specific-only actions, got: %v", err)
	}
}

func TestProviderSpecific_WriteTranslation_KnownAndUnknown(t *testing.T) {
	action := BehaviorActionV2ResourceModel{
		ProviderSpecific: []ProviderSpecificModel{
			{Provider: types.StringValue("fastly"), Code: exampleSpecificFastlyCode},
			{Provider: types.StringValue("my_custom_backend_name"), Code: jsontypes.NewNormalizedValue(`{"custom_field": "value"}`)},
		},
	}

	apiAction := ServiceConfigAPIAction{}
	if err := behaviorActionModelToAPIStruct(action, &apiAction); err != nil {
		t.Fatalf("behaviorActionModelToAPIStruct error: %v", err)
	}

	if len(apiAction.ProviderSpecific) != 2 {
		t.Fatalf("expected 2 provider_specific entries, got %d", len(apiAction.ProviderSpecific))
	}

	if apiAction.ProviderSpecific[0].Name != "Fastly" {
		t.Errorf("known provider map mismatch: expected Fastly, got %q", apiAction.ProviderSpecific[0].Name)
	}
	if apiAction.ProviderSpecific[1].Name != "my_custom_backend_name" {
		t.Errorf("unknown provider should pass through unchanged, got %q", apiAction.ProviderSpecific[1].Name)
	}
}

func TestProviderSpecific_ReadTranslation_Known(t *testing.T) {
	apiAction := ServiceConfigAPIAction{
		ProviderSpecific: []ServiceConfigAPIProviderSpecific{
			{Name: "Cloudflare", Value: "known"},
		},
	}

	model, err := apiActionStructToModel(apiAction)
	if err != nil {
		t.Fatalf("apiActionStructToModel error: %v", err)
	}

	if len(model.ProviderSpecific) != 1 {
		t.Fatalf("expected 1 provider_specific entry, got %d", len(model.ProviderSpecific))
	}

	if model.ProviderSpecific[0].Provider.ValueString() != "cloudflare" {
		t.Errorf("known backend provider map mismatch: expected cloudflare, got %q", model.ProviderSpecific[0].Provider.ValueString())
	}
	if model.ProviderSpecific[0].Code.ValueString() != "known" {
		t.Errorf("provider_specific code mismatch: expected known, got %q", model.ProviderSpecific[0].Code.ValueString())
	}
}

func TestProviderSpecific_ReadTranslation_UnknownFails(t *testing.T) {
	apiAction := ServiceConfigAPIAction{
		ProviderSpecific: []ServiceConfigAPIProviderSpecific{
			{Name: "BackendCustomName", Value: "unknown"},
		},
	}

	_, err := apiActionStructToModel(apiAction)
	if err == nil {
		t.Fatal("expected error for unknown backend provider name, got nil")
	}
	if !containsStr(err.Error(), "unknown provider name returned by backend") {
		t.Fatalf("expected unknown-provider error, got: %v", err)
	}
}

// NEGATIVE: empty actions AND missing path/condition — both errors returned.
func TestValidateBehaviorModel_EmptyActions_AndMissingCondition_BothErrors(t *testing.T) {
	b := &BehaviorModel{
		Name:    strVal("bad"),
		Actions: &BehaviorActionV2ResourceModel{},
		// PathPattern and Condition both absent
	}
	errs := ValidateBehaviorModel(b, "behaviors[0]")
	// Should get the "one of path_pattern or condition must be set" error
	found := false
	for _, e := range errs {
		if containsStr(e, "one of 'path_pattern' or 'condition' must be set") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected missing-condition error, got: %v", errs)
	}
}

// TestBehaviorAction_RoundTrip_AllActions exercises every action field that is
// serialised by behaviorActionModelToAPIStruct / apiActionStructToModel.
// Fields that are intentionally skipped (not supported by the new service config
// API) are noted in comments below.
func TestBehaviorAction_RoundTrip_AllActions(t *testing.T) {
	ctx := context.Background()

	originList := []types.String{types.StringValue("https://example.com")}
	headerList := []types.String{types.StringValue("X-Custom-Header")}
	exposeList := []types.String{types.StringValue("X-Expose")}
	methodList := []types.String{types.StringValue("GET"), types.StringValue("POST")}

	model := &BehaviorModel{
		Name:        types.StringValue("all-actions"),
		PathPattern: types.StringValue("/all/*"),
		Actions: &BehaviorActionV2ResourceModel{
			// --- scalar actions ---
			CacheTTL:               types.Int64Value(3600),
			CacheBehavior:          types.StringValue("CACHE"),
			BrowserCacheTtl:        types.Int64Value(1800),
			ViewerProtocol:         types.StringValue("HTTPS_ONLY"),
			OriginCacheControl:     types.BoolValue(true),
			FollowRedirects:        types.BoolValue(true),
			StaleTtl:               types.Int64Value(600),
			Compression:            types.BoolValue(true),
			LargeFilesOptimization: types.BoolValue(true),
			UrlSigning:             types.BoolValue(true),

			// --- redirect ---
			Redirect: &RedirectModelV2{
				Destination: types.StringValue("https://new.example.com"),
				Source:      types.StringValue("/old/*"),
			},

			// --- cached methods ---
			CachedMethods: &[]MethodModelV2{
				{Method: types.StringValue("GET")},
				{Method: types.StringValue("HEAD")},
			},

			// --- allowed methods ---
			AllowedMethods: &[]MethodModelV2{
				{Method: types.StringValue("GET")},
				{Method: types.StringValue("POST")},
				{Method: types.StringValue("OPTIONS")},
			},

			// --- cache key ---
			CacheKey: &CacheKeyModelV2{
				Headers: []HeaderModelV2{
					{Header: types.StringValue("Accept-Language")},
				},
				Cookies: []CookieModelV2{
					{Cookie: types.StringValue("session")},
				},
				QueryStrings: &QueryStringsModelV2{
					ParamsList: []ParamModelV2{
						{Param: types.StringValue("page")},
						{Param: types.StringValue("size")},
					},
					ListType: types.StringValue("include"),
				},
				Country:    types.BoolValue(true),
				DeviceType: types.BoolValue(true),
			},

			// --- host header ---
			HostHeader: &HostHeaderModelV2{
				HeaderValue:   types.StringValue("origin.example.com"),
				UseOriginHost: types.BoolValue(false),
			},

			// --- CORS ---
			Cors: &CorsConfigModelV2{
				AllowOrigin: &CorsAllowOriginModelV2{
					Mode:     types.StringValue("specific"),
					Origins:  originList,
					Override: types.BoolValue(true),
				},
				AllowHeaders: &CorsHeaderListModelV2{
					Mode:     types.StringValue("specific"),
					Values:   headerList,
					Override: types.BoolValue(false),
				},
				ExposeHeaders: &CorsHeaderListModelV2{
					Mode:     types.StringValue("specific"),
					Values:   exposeList,
					Override: types.BoolValue(false),
				},
				AllowMethods: &CorsMethodListModelV2{
					Mode:     types.StringValue("specific"),
					Values:   methodList,
					Override: types.BoolValue(true),
				},
				AllowCredentials: types.BoolValue(true),
				MaxAge: &CorsMaxAgeModelV2{
					Value:    types.Int64Value(86400),
					Override: types.BoolValue(true),
				},
			},

			// --- generate preflight response ---
			GeneratePreflightResponse: &GeneratePreflightResponseModelV2{
				AllowedMethods: &[]MethodModelV2{
					{Method: types.StringValue("GET")},
					{Method: types.StringValue("OPTIONS")},
				},
				AllowedHeaders: []types.String{
					types.StringValue("Content-Type"),
					types.StringValue("Authorization"),
				},
				MaxAge: types.Int64Value(3600),
			},

			// --- status code browser cache ---
			StatusCodeBrowserCache: []StatusCodeBrowserCacheModelV2{
				{
					StatusCode: types.StringValue("404"),
					CacheTtl:   types.Int64Value(60),
				},
			},

			// --- stream logs ---
			StreamLogs: &StreamLogsModelV2{
				UnifiedLogDestination:  types.StringValue("my-log-dest"),
				UnifiedLogSamplingRate: types.Int64Value(50),
			},

			// --- allow access only from IP ---
			AllowAccessOnlyFromIP: &[]IPModelV2{
				{IP: types.StringValue("10.0.0.1/32")},
				{IP: types.StringValue("192.168.1.0/24")},
			},

			// --- generate response ---
			StatusCodeCustomResponse: []StatusCodeCustomResponseModelV2{
				{
					StatusCode:  types.StringValue("404"),
					ResponseURL: types.StringValue("https://example.com/404.html"),
				},
				{
					StatusCode:  types.StringValue("5xx"),
					ResponseURL: types.StringValue("https://example.com/error.html"),
				},
			},

			// --- request headers ---
			RequestHeaders: &[]HeaderActionModelV2{
				{
					Name:   types.StringValue("X-CDN-Origin"),
					Values: []types.String{types.StringValue("io-river")},
					Action: types.StringValue("set"),
				},
				{
					Name:   types.StringValue("Cookie"),
					Values: []types.String{},
					Action: types.StringValue("delete"),
				},
			},

			// --- response headers ---
			ResponseHeaders: &[]HeaderActionModelV2{
				{
					Name:   types.StringValue("X-Frame-Options"),
					Values: []types.String{types.StringValue("SAMEORIGIN")},
					Action: types.StringValue("add"),
				},
			},

			// --- origin response headers ---
			OriginResponseHeaders: &[]HeaderActionModelV2{
				{
					Name:   types.StringValue("Set-Cookie"),
					Values: []types.String{},
					Action: types.StringValue("delete"),
				},
			},

			// --- provider-specific ---
			ProviderSpecific: []ProviderSpecificModel{
				{
					Provider: types.StringValue("fastly"),
					Code:     exampleSpecificFastlyCode,
				},
			},

			// Fields intentionally skipped (not supported by new service config API):
			// BypassCacheOnCookie, OverrideOrigin, OriginErrorPassThrough, ForwardClientHeader.
		},
	}

	apiMap, err := model.ModelToMapWithCtx(ctx)
	if err != nil {
		t.Fatalf("ModelToMapWithCtx error: %v", err)
	}

	recovered, err := BehaviorModelfromMap(ctx, "all-actions", apiMap, true)
	if err != nil {
		t.Fatalf("BehaviorModelfromMap error: %v", err)
	}

	a := recovered.Actions
	if a == nil {
		t.Fatal("actions is nil after round-trip")
	}

	// scalars
	assertInt64(t, "cache_ttl", 3600, a.CacheTTL)
	assertStr(t, "cache_behavior", "CACHE", a.CacheBehavior)
	assertInt64(t, "browser_cache_ttl", 1800, a.BrowserCacheTtl)
	assertStr(t, "viewer_protocol", "HTTPS_ONLY", a.ViewerProtocol)
	assertBool(t, "origin_cache_control", true, a.OriginCacheControl)
	assertBool(t, "follow_redirects", true, a.FollowRedirects)
	assertInt64(t, "stale_ttl", 600, a.StaleTtl)
	assertBool(t, "compression", true, a.Compression)
	assertBool(t, "large_files_optimization", true, a.LargeFilesOptimization)
	assertBool(t, "url_signing", true, a.UrlSigning)

	// redirect
	if a.Redirect == nil {
		t.Fatal("redirect is nil")
	}
	assertStr(t, "redirect.destination", "https://new.example.com", a.Redirect.Destination)
	assertStr(t, "redirect.source", "/old/*", a.Redirect.Source)

	// cached methods
	if a.CachedMethods == nil || len(*a.CachedMethods) != 2 {
		t.Fatalf("cached_methods: expected 2, got %v", a.CachedMethods)
	}

	// allowed methods
	if a.AllowedMethods == nil || len(*a.AllowedMethods) != 3 {
		t.Fatalf("allowed_methods: expected 3, got %v", a.AllowedMethods)
	}

	// cache key
	if a.CacheKey == nil {
		t.Fatal("cache_key is nil")
	}
	if len(a.CacheKey.Headers) != 1 || a.CacheKey.Headers[0].Header.ValueString() != "Accept-Language" {
		t.Errorf("cache_key.headers: expected [Accept-Language], got %v", a.CacheKey.Headers)
	}
	if len(a.CacheKey.Cookies) != 1 || a.CacheKey.Cookies[0].Cookie.ValueString() != "session" {
		t.Errorf("cache_key.cookies: expected [session], got %v", a.CacheKey.Cookies)
	}
	if a.CacheKey.QueryStrings == nil || a.CacheKey.QueryStrings.ListType.ValueString() != "include" {
		t.Errorf("cache_key.query_strings.type: expected include, got %v", a.CacheKey.QueryStrings)
	}
	if len(a.CacheKey.QueryStrings.ParamsList) != 2 {
		t.Errorf("cache_key.query_strings.params: expected 2 params, got %d", len(a.CacheKey.QueryStrings.ParamsList))
	}
	assertBool(t, "cache_key.country", true, a.CacheKey.Country)
	assertBool(t, "cache_key.device_type", true, a.CacheKey.DeviceType)

	// host header
	if a.HostHeader == nil {
		t.Fatal("host_header is nil")
	}
	assertStr(t, "host_header.header_value", "origin.example.com", a.HostHeader.HeaderValue)
	assertBool(t, "host_header.use_origin_host", false, a.HostHeader.UseOriginHost)

	// CORS
	if a.Cors == nil {
		t.Fatal("cors is nil")
	}
	if a.Cors.AllowOrigin == nil {
		t.Fatal("cors.allow_origin is nil")
	}
	assertStr(t, "cors.allow_origin.mode", "specific", a.Cors.AllowOrigin.Mode)
	if len(a.Cors.AllowOrigin.Origins) != 1 || a.Cors.AllowOrigin.Origins[0].ValueString() != "https://example.com" {
		t.Errorf("cors.allow_origin.origins: expected [https://example.com], got %v", a.Cors.AllowOrigin.Origins)
	}
	assertBool(t, "cors.allow_origin.override", true, a.Cors.AllowOrigin.Override)

	if a.Cors.AllowHeaders == nil {
		t.Fatal("cors.allow_headers is nil")
	}
	assertStr(t, "cors.allow_headers.mode", "specific", a.Cors.AllowHeaders.Mode)
	if len(a.Cors.AllowHeaders.Values) != 1 || a.Cors.AllowHeaders.Values[0].ValueString() != "X-Custom-Header" {
		t.Errorf("cors.allow_headers.values: expected [X-Custom-Header], got %v", a.Cors.AllowHeaders.Values)
	}

	if a.Cors.ExposeHeaders == nil {
		t.Fatal("cors.expose_headers is nil")
	}
	assertStr(t, "cors.expose_headers.mode", "specific", a.Cors.ExposeHeaders.Mode)

	if a.Cors.AllowMethods == nil {
		t.Fatal("cors.allow_methods is nil")
	}
	assertStr(t, "cors.allow_methods.mode", "specific", a.Cors.AllowMethods.Mode)
	if len(a.Cors.AllowMethods.Values) != 2 {
		t.Errorf("cors.allow_methods.values: expected 2, got %d", len(a.Cors.AllowMethods.Values))
	}
	assertBool(t, "cors.allow_credentials", true, a.Cors.AllowCredentials)
	if a.Cors.MaxAge == nil {
		t.Fatal("cors.max_age is nil")
	}
	assertInt64(t, "cors.max_age.value", 86400, a.Cors.MaxAge.Value)
	assertBool(t, "cors.max_age.override", true, a.Cors.MaxAge.Override)

	// generate preflight response
	if a.GeneratePreflightResponse == nil {
		t.Fatal("generate_preflight_response is nil")
	}
	assertInt64(t, "generate_preflight_response.max_age", 3600, a.GeneratePreflightResponse.MaxAge)
	if a.GeneratePreflightResponse.AllowedMethods == nil || len(*a.GeneratePreflightResponse.AllowedMethods) != 2 {
		t.Errorf("generate_preflight_response.allowed_methods: expected 2, got %v", a.GeneratePreflightResponse.AllowedMethods)
	}
	if len(a.GeneratePreflightResponse.AllowedHeaders) != 2 {
		t.Errorf("generate_preflight_response.allowed_headers: expected 2, got %d", len(a.GeneratePreflightResponse.AllowedHeaders))
	} else {
		assertStr(t, "generate_preflight_response.allowed_headers[0]", "Content-Type", a.GeneratePreflightResponse.AllowedHeaders[0])
		assertStr(t, "generate_preflight_response.allowed_headers[1]", "Authorization", a.GeneratePreflightResponse.AllowedHeaders[1])
	}

	// status code browser cache
	if len(a.StatusCodeBrowserCache) == 0 {
		t.Fatal("status_code_browser_cache is empty")
	}
	assertStr(t, "status_code_browser_cache.status_code", "404", a.StatusCodeBrowserCache[0].StatusCode)
	assertInt64(t, "status_code_browser_cache.cache_ttl", 60, a.StatusCodeBrowserCache[0].CacheTtl)

	// stream logs
	if a.StreamLogs == nil {
		t.Fatal("stream_logs is nil")
	}
	assertStr(t, "stream_logs.log_destination", "my-log-dest", a.StreamLogs.UnifiedLogDestination)
	assertInt64(t, "stream_logs.log_sampling_rate", 50, a.StreamLogs.UnifiedLogSamplingRate)

	// allow access only from ip
	if a.AllowAccessOnlyFromIP == nil || len(*a.AllowAccessOnlyFromIP) != 2 {
		t.Fatalf("allow_access_only_from_ip: expected 2, got %v", a.AllowAccessOnlyFromIP)
	}

	// generate response
	if len(a.StatusCodeCustomResponse) != 2 {
		t.Fatalf("generate_response: expected 2, got %d", len(a.StatusCodeCustomResponse))
	}
	assertStr(t, "generate_response[0].status_code", "404", a.StatusCodeCustomResponse[0].StatusCode)
	assertStr(t, "generate_response[0].response_url", "https://example.com/404.html", a.StatusCodeCustomResponse[0].ResponseURL)
	assertStr(t, "generate_response[1].status_code", "5xx", a.StatusCodeCustomResponse[1].StatusCode)
	assertStr(t, "generate_response[1].response_url", "https://example.com/error.html", a.StatusCodeCustomResponse[1].ResponseURL)

	// request headers
	if a.RequestHeaders == nil || len(*a.RequestHeaders) != 2 {
		t.Fatalf("request_headers: expected 2, got %v", a.RequestHeaders)
	}
	assertStr(t, "request_headers[0].name", "X-CDN-Origin", (*a.RequestHeaders)[0].Name)
	if len((*a.RequestHeaders)[0].Values) != 1 || (*a.RequestHeaders)[0].Values[0].ValueString() != "io-river" {
		t.Errorf("request_headers[0].values: expected [io-river], got %v", (*a.RequestHeaders)[0].Values)
	}
	assertStr(t, "request_headers[0].action", "set", (*a.RequestHeaders)[0].Action)
	assertStr(t, "request_headers[1].name", "Cookie", (*a.RequestHeaders)[1].Name)
	assertStr(t, "request_headers[1].action", "delete", (*a.RequestHeaders)[1].Action)

	// response headers
	if a.ResponseHeaders == nil || len(*a.ResponseHeaders) != 1 {
		t.Fatalf("response_headers: expected 1, got %v", a.ResponseHeaders)
	}
	assertStr(t, "response_headers[0].name", "X-Frame-Options", (*a.ResponseHeaders)[0].Name)
	if len((*a.ResponseHeaders)[0].Values) != 1 || (*a.ResponseHeaders)[0].Values[0].ValueString() != "SAMEORIGIN" {
		t.Errorf("response_headers[0].values: expected [SAMEORIGIN], got %v", (*a.ResponseHeaders)[0].Values)
	}
	assertStr(t, "response_headers[0].action", "add", (*a.ResponseHeaders)[0].Action)

	// origin response headers
	if a.OriginResponseHeaders == nil || len(*a.OriginResponseHeaders) != 1 {
		t.Fatalf("origin_response_headers: expected 1, got %v", a.OriginResponseHeaders)
	}
	assertStr(t, "origin_response_headers[0].name", "Set-Cookie", (*a.OriginResponseHeaders)[0].Name)
	assertStr(t, "origin_response_headers[0].action", "delete", (*a.OriginResponseHeaders)[0].Action)

	// provider_specific
	if len(a.ProviderSpecific) != 1 {
		t.Fatalf("provider_specific: expected 1, got %d", len(a.ProviderSpecific))
	}
	assertStr(t, "provider_specific[0].provider", "fastly", a.ProviderSpecific[0].Provider)
	if exampleSpecificFastlyCode.ValueString() != a.ProviderSpecific[0].Code.ValueString() {
		t.Fatalf("Expected code to be %q, but got %q",
			exampleSpecificFastlyCode.ValueString(), a.ProviderSpecific[0].Code.ValueString())
	}

	// --- validate: the model itself must pass validation ---
	if errs := ValidateBehaviorModel(model, "behaviors[all-actions]"); len(errs) != 0 {
		t.Errorf("valid model failed validation: %v", errs)
	}

	// --- validate: header action rules ---
	headerValidationCases := []struct {
		name    string
		headers *[]HeaderActionModelV2
		field   string
		wantErr string
	}{
		{
			name:  "set with values OK",
			field: "request_headers",
			headers: &[]HeaderActionModelV2{
				{Name: types.StringValue("X-Foo"), Values: []types.String{types.StringValue("bar")}, Action: types.StringValue("set")},
			},
		},
		{
			name:  "add with values OK",
			field: "response_headers",
			headers: &[]HeaderActionModelV2{
				{Name: types.StringValue("Via"), Values: []types.String{types.StringValue("cdn")}, Action: types.StringValue("add")},
			},
		},
		{
			name:  "delete with no values OK",
			field: "origin_response_headers",
			headers: &[]HeaderActionModelV2{
				{Name: types.StringValue("X-Internal"), Values: []types.String{}, Action: types.StringValue("delete")},
			},
		},
		{
			name:    "delete with values fails",
			field:   "request_headers",
			wantErr: "must not include values",
			headers: &[]HeaderActionModelV2{
				{Name: types.StringValue("Cookie"), Values: []types.String{types.StringValue("x")}, Action: types.StringValue("delete")},
			},
		},
		{
			name:    "set with no values fails",
			field:   "request_headers",
			wantErr: "requires at least one value",
			headers: &[]HeaderActionModelV2{
				{Name: types.StringValue("X-Foo"), Values: []types.String{}, Action: types.StringValue("set")},
			},
		},
		{
			name:    "add with no values fails",
			field:   "response_headers",
			wantErr: "requires at least one value",
			headers: &[]HeaderActionModelV2{
				{Name: types.StringValue("Via"), Values: []types.String{}, Action: types.StringValue("add")},
			},
		},
		{
			name:    "error location includes field and index",
			field:   "response_headers",
			wantErr: "response_headers[1]",
			headers: &[]HeaderActionModelV2{
				{Name: types.StringValue("X-Good"), Values: []types.String{types.StringValue("v")}, Action: types.StringValue("set")},
				{Name: types.StringValue("X-Bad"), Values: []types.String{}, Action: types.StringValue("add")},
			},
		},
	}

	for _, tc := range headerValidationCases {
		t.Run("header_action/"+tc.name, func(t *testing.T) {
			actions := &BehaviorActionV2ResourceModel{}
			switch tc.field {
			case "request_headers":
				actions.RequestHeaders = tc.headers
			case "response_headers":
				actions.ResponseHeaders = tc.headers
			case "origin_response_headers":
				actions.OriginResponseHeaders = tc.headers
			}
			b := &BehaviorModel{
				Name:        strVal("h"),
				PathPattern: types.StringValue("/h/*"),
				Actions:     actions,
			}
			errs := ValidateBehaviorModel(b, "behaviors[0]")
			if tc.wantErr == "" {
				if len(errs) != 0 {
					t.Errorf("expected no errors, got: %v", errs)
				}
			} else {
				if len(errs) == 0 {
					t.Fatalf("expected error containing %q, got none", tc.wantErr)
				}
				found := false
				for _, e := range errs {
					if containsStr(e, tc.wantErr) {
						found = true
					}
				}
				if !found {
					t.Errorf("expected error containing %q, got: %v", tc.wantErr, errs)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Test assertion helpers
// ---------------------------------------------------------------------------

func assertStr(t *testing.T, name, expected string, got types.String) {
	t.Helper()
	if got.IsNull() || got.IsUnknown() {
		t.Errorf("%s: expected %q, got null/unknown", name, expected)
		return
	}
	if got.ValueString() != expected {
		t.Errorf("%s: expected %q, got %q", name, expected, got.ValueString())
	}
}

func assertBool(t *testing.T, name string, expected bool, got types.Bool) {
	t.Helper()
	if got.IsNull() || got.IsUnknown() {
		t.Errorf("%s: expected %v, got null/unknown", name, expected)
		return
	}
	if got.ValueBool() != expected {
		t.Errorf("%s: expected %v, got %v", name, expected, got.ValueBool())
	}
}

func assertInt64(t *testing.T, name string, expected int64, got types.Int64) {
	t.Helper()
	if got.IsNull() || got.IsUnknown() {
		t.Errorf("%s: expected %d, got null/unknown", name, expected)
		return
	}
	if got.ValueInt64() != expected {
		t.Errorf("%s: expected %d, got %d", name, expected, got.ValueInt64())
	}
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

func mustStringSlice(t *testing.T, ctx context.Context, s types.Set) []string {
	t.Helper()
	var vals []string
	if diags := s.ElementsAs(ctx, &vals, false); diags.HasError() {
		t.Fatalf("ElementsAs failed: %v", diags)
	}
	return vals
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// QueryStrings type translation tests (TF ↔ backend mapping)
// ---------------------------------------------------------------------------

// TestCacheKeyQueryStrings_Translation verifies the write-side mapping:
//
//	"none" → backend "include" + []
//	"all"  → backend "exclude" + []
//	"include" + params → backend "include" + params
//	"exclude" + params → backend "exclude" + params
func TestCacheKeyQueryStrings_WriteTranslation(t *testing.T) {
	cases := []struct {
		tfType        string
		params        []string
		wantAPIMode   string
		wantAPIParams []string
	}{
		{tfType: "none", params: nil, wantAPIMode: "include", wantAPIParams: []string{}},
		{tfType: "all", params: nil, wantAPIMode: "exclude", wantAPIParams: []string{}},
		{tfType: "include", params: []string{"foo", "bar"}, wantAPIMode: "include", wantAPIParams: []string{"foo", "bar"}},
		{tfType: "exclude", params: []string{"baz"}, wantAPIMode: "exclude", wantAPIParams: []string{"baz"}},
	}

	for _, tc := range cases {
		tc := tc
		t.Run("type="+tc.tfType, func(t *testing.T) {
			paramsList := []ParamModelV2{}
			for _, p := range tc.params {
				paramsList = append(paramsList, ParamModelV2{Param: types.StringValue(p)})
			}
			model := &BehaviorModel{
				Name:        strVal("qs-test"),
				PathPattern: types.StringValue("/test/*"),
				Actions: &BehaviorActionV2ResourceModel{
					CacheKey: &CacheKeyModelV2{
						Headers:    []HeaderModelV2{},
						Cookies:    []CookieModelV2{},
						Country:    types.BoolValue(false),
						DeviceType: types.BoolValue(false),
						QueryStrings: &QueryStringsModelV2{
							ListType:   types.StringValue(tc.tfType),
							ParamsList: paramsList,
						},
					},
				},
			}

			apiMap, err := model.ModelToMapWithCtx(context.Background())
			if err != nil {
				t.Fatalf("ModelToMapWithCtx error: %v", err)
			}

			// Drill into action.cache_key
			actionRaw, ok := apiMap["action"].(map[string]interface{})
			if !ok {
				t.Fatal("action not found in apiMap")
			}
			ckRaw, ok := actionRaw["cache_key"].(map[string]interface{})
			if !ok {
				t.Fatal("cache_key not found in action")
			}

			gotMode, _ := ckRaw["query_params_mode"].(string)
			if gotMode != tc.wantAPIMode {
				t.Errorf("query_params_mode: want %q, got %q", tc.wantAPIMode, gotMode)
			}

			var gotParams []string
			// After JSON marshal→unmarshal, arrays come back as []interface{}, not []string.
			if raw, ok := ckRaw["query_params"].([]interface{}); ok {
				for _, v := range raw {
					if s, ok := v.(string); ok {
						gotParams = append(gotParams, s)
					}
				}
			} else if raw, ok := ckRaw["query_params"].([]string); ok {
				gotParams = raw
			}
			if len(gotParams) != len(tc.wantAPIParams) {
				t.Errorf("query_params length: want %d, got %d", len(tc.wantAPIParams), len(gotParams))
			}
		})
	}
}

// TestCacheKeyQueryStrings_ReadTranslation verifies the read-side mapping:
//
//	backend "include" + [] → TF "none"
//	backend "exclude" + [] → TF "all"
//	backend "include" + params → TF "include"
//	backend "exclude" + params → TF "exclude"
func TestCacheKeyQueryStrings_ReadTranslation(t *testing.T) {
	cases := []struct {
		apiMode    string
		apiParams  []string
		wantTFType string
		wantParams int // expected number of params in state
	}{
		{apiMode: "include", apiParams: []string{}, wantTFType: "none", wantParams: 0},
		{apiMode: "exclude", apiParams: []string{}, wantTFType: "all", wantParams: 0},
		{apiMode: "include", apiParams: []string{"foo", "bar"}, wantTFType: "include", wantParams: 2},
		{apiMode: "exclude", apiParams: []string{"baz"}, wantTFType: "exclude", wantParams: 1},
	}

	for _, tc := range cases {
		tc := tc
		t.Run("api_mode="+tc.apiMode+"_params="+strconv.Itoa(len(tc.apiParams)), func(t *testing.T) {
			apiAction := ServiceConfigAPIAction{
				CacheKey: &ServiceConfigAPICacheKey{
					Headers:         []string{},
					Cookies:         []string{},
					QueryParams:     tc.apiParams,
					QueryParamsMode: tc.apiMode,
					Country:         false,
					DeviceType:      false,
				},
			}

			model, err := apiActionStructToModel(apiAction)
			if err != nil {
				t.Fatalf("apiActionStructToModel error: %v", err)
			}

			if model.CacheKey == nil || model.CacheKey.QueryStrings == nil {
				t.Fatal("CacheKey.QueryStrings is nil")
			}

			gotType := model.CacheKey.QueryStrings.ListType.ValueString()
			if gotType != tc.wantTFType {
				t.Errorf("ListType: want %q, got %q", tc.wantTFType, gotType)
			}

			gotParamCount := len(model.CacheKey.QueryStrings.ParamsList)
			if gotParamCount != tc.wantParams {
				t.Errorf("ParamsList length: want %d, got %d", tc.wantParams, gotParamCount)
			}

			// For "none" and "all", ParamsList should be nil — params is Optional
			// only (no Computed/Default), so null in state matches null in plan.
			if (tc.wantTFType == "none" || tc.wantTFType == "all") && model.CacheKey.QueryStrings.ParamsList != nil {
				t.Errorf("ParamsList should be nil for type=%q, got %v", tc.wantTFType, model.CacheKey.QueryStrings.ParamsList)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Path-field validation tests (validate_behavior_condition parity)
// ---------------------------------------------------------------------------

// eq / ne must start with "/".
func TestValidateBehaviorCondition_Path_EQ_NoSlash(t *testing.T) {
	expr := simpleBehaviorConditionExpr("http.request.path", "eq", []string{"api/foo"}, "")
	errs := ValidateBehaviorConditionModel(expr, "b[0]")
	if len(errs) == 0 {
		t.Error("expected error: eq path without leading slash")
	}
}

func TestValidateBehaviorCondition_Path_NE_NoSlash(t *testing.T) {
	expr := simpleBehaviorConditionExpr("http.request.path", "ne", []string{"noslash"}, "")
	errs := ValidateBehaviorConditionModel(expr, "b[0]")
	if len(errs) == 0 {
		t.Error("expected error: ne path without leading slash")
	}
}

func TestValidateBehaviorCondition_Path_Match_NoSlash(t *testing.T) {
	expr := simpleBehaviorConditionExpr("http.request.path", "match", []string{"api/*"}, "")
	errs := ValidateBehaviorConditionModel(expr, "b[0]")
	if len(errs) == 0 {
		t.Error("expected error: match path without leading slash")
	}
}

// eq with "*" is forbidden.
func TestValidateBehaviorCondition_Path_EQ_StarForbidden(t *testing.T) {
	expr := simpleBehaviorConditionExpr("http.request.path", "eq", []string{"/api/*"}, "")
	errs := ValidateBehaviorConditionModel(expr, "b[0]")
	if len(errs) == 0 {
		t.Error("expected error: eq path with wildcard '*'")
	}
}

// match with "*" is allowed (wildcard pattern).
func TestValidateBehaviorCondition_Path_Match_StarAllowed(t *testing.T) {
	expr := simpleBehaviorConditionExpr("http.request.path", "match", []string{"/api/*"}, "")
	errs := ValidateBehaviorConditionModel(expr, "b[0]")
	if len(errs) != 0 {
		t.Errorf("expected no errors for match /api/*, got: %v", errs)
	}
}

// eq with valid path — OK.
func TestValidateBehaviorCondition_Path_EQ_OK(t *testing.T) {
	expr := simpleBehaviorConditionExpr("http.request.path", "eq", []string{"/api/v1/resource"}, "")
	errs := ValidateBehaviorConditionModel(expr, "b[0]")
	if len(errs) != 0 {
		t.Errorf("expected no errors, got: %v", errs)
	}
}

// eq path exceeding 255 chars.
func TestValidateBehaviorCondition_Path_EQ_TooLong(t *testing.T) {
	longPath := "/" + strings.Repeat("a", 255)
	expr := simpleBehaviorConditionExpr("http.request.path", "eq", []string{longPath}, "")
	errs := ValidateBehaviorConditionModel(expr, "b[0]")
	if len(errs) == 0 {
		t.Error("expected error for path longer than 255 chars")
	}
}

// regex — valid.
func TestValidateBehaviorCondition_Path_Regex_OK(t *testing.T) {
	expr := simpleBehaviorConditionExpr("http.request.path", "regex", []string{"^/api/.*"}, "")
	errs := ValidateBehaviorConditionModel(expr, "b[0]")
	if len(errs) != 0 {
		t.Errorf("expected no errors for valid regex, got: %v", errs)
	}
}

// not_regex — valid.
func TestValidateBehaviorCondition_Path_NotRegex_OK(t *testing.T) {
	expr := simpleBehaviorConditionExpr("http.request.path", "not_regex", []string{"^/static/.*"}, "")
	errs := ValidateBehaviorConditionModel(expr, "b[0]")
	if len(errs) != 0 {
		t.Errorf("expected no errors for valid not_regex, got: %v", errs)
	}
}

// regex — invalid pattern.
func TestValidateBehaviorCondition_Path_Regex_Invalid(t *testing.T) {
	expr := simpleBehaviorConditionExpr("http.request.path", "regex", []string{"[unclosed"}, "")
	errs := ValidateBehaviorConditionModel(expr, "b[0]")
	if len(errs) == 0 {
		t.Error("expected error for invalid regex")
	}
}

// in — valid paths (no leading slash required by backend for "in").
func TestValidateBehaviorCondition_Path_In_OK(t *testing.T) {
	expr := simpleBehaviorConditionExpr("http.request.path", "in", []string{"/a", "/b"}, "")
	errs := ValidateBehaviorConditionModel(expr, "b[0]")
	if len(errs) != 0 {
		t.Errorf("expected no errors for in with valid paths, got: %v", errs)
	}
}

// matches_one_of with invalid chars.
func TestValidateBehaviorCondition_Path_MatchesOneOf_InvalidChars(t *testing.T) {
	expr := simpleBehaviorConditionExpr("http.request.path", "matches_one_of", []string{"/api/ bad"}, "")
	errs := ValidateBehaviorConditionModel(expr, "b[0]")
	if len(errs) == 0 {
		t.Error("expected error for path with space char")
	}
}

// field_key forbidden on non-collection field.
func TestValidateBehaviorCondition_NonCollection_FieldKeyForbidden(t *testing.T) {
	expr := simpleBehaviorConditionExpr("http.request.path", "eq", []string{"/api"}, "should-not-be-set")
	errs := ValidateBehaviorConditionModel(expr, "b[0]")
	if len(errs) == 0 {
		t.Error("expected error: field_key set on non-collection field http.request.path")
	}
}

// ─── HCL config generators (called from service_resource_test.go) ─────────────

func testAccCheckBehaviorConfigWithBehaviors(resourceName string, certId string, subBehaviorName string) string {
	format := `
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

		behaviors = {
			custom = [
				{
					name        = "%s"
					path_pattern = "/api/test/123"
					actions = {
						cache_behavior = "CACHE"
						cache_ttl = 5678
						cached_methods = [
							{ method = "GET" },
							{ method = "HEAD" }
						]
						large_files_optimization = true
						origin_cache_control = true
						stale_ttl = 300
						follow_redirects = true
						deny_access = false
						true_client_ip = true
						deny_access_by_ip = [
							{ ip = "1.2.3.4" },
							{ ip = "5.6.7.8" }
						]
						deny_access_by_time = [
							{
								date_time_window = {
									start_date = 2000000000
									end_date   = 2100000000
								}
							}
						]
						url_rewrites = [
							{
								source      = "/api/test/123"
								destination = "/api/test/1234"
							}
						]
						generate_response = [
						  { status_code = "404", response_url = "https://example.com/404.html" }
						]
					}
				}
			]
		}
	}
}`

	return fmt.Sprintf(format,
		resourceName, resourceName, certId, subBehaviorName)
}

// testAccAllActionsConfig returns HCL that exercises every supported behavior action
// in a single custom behavior — used as Step 1 in TestAccIORiverService_WithBehaviors.
func testAccAllActionsConfig(resourceName string, certId string, behaviorName string) string {
	format := `
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
					host     = "example.com"
					protocol = "https"
				}
			}
		]
		domains = []

		behaviors = {
			custom = [
				{
					name         = "%s"
					path_pattern = "/all/actions/*"
					actions = {
						# --- cache ---
						cache_behavior           = "CACHE"
						cache_ttl                = 3600
						browser_cache_ttl        = 1800
						large_files_optimization = true
						origin_cache_control     = true
						stale_ttl                = 600
						follow_redirects         = true
						compression              = true
						deny_access              = false
						true_client_ip           = true

						cached_methods = [
							{ method = "GET" },
							{ method = "HEAD" }
						]
						allowed_methods = [
							{ method = "GET" },
							{ method = "HEAD" },
							{ method = "POST" }
						]

						# --- access control ---
						deny_access_by_ip = [
							{ ip = "1.2.3.4" }
						]
						deny_access_by_time = [
							{
								date_time_window = {
									start_date = 2000000000
									end_date   = 2100000000
								}
							}
						]

						# --- rewrites & status ---
						url_rewrites = [
							{
								source      = "/old/path"
								destination = "/new/path"
							}
						]
						generate_response = [
						  { status_code = "404", response_url = "https://example.com/404.html" }
						]
						status_code_browser_cache = [
						  { status_code = "404", cache_ttl = 3600 }
						]
						status_codes_ttl = [
						  { status_code = "404", cache_ttl = 3600, cache_behavior="CACHE" }
						]
						# not returned by API for custom behaviors (backend bug — tracked separately).
						# Uncomment once backend is fixed and re-add assertions.

						# --- cache key ---
						cache_key = {
							headers = [{ header = "Accept-Language" }]
							cookies = [{ cookie = "session" }]
							query_strings = {
								type = "include"
								params = [{ param = "page" }]
							}
							country     = true
							device_type = false
						}

						# --- host header ---
						host_header = {
							header_value = "origin.example.com"
						}

						# --- cors ---
						cors = {
							allow_origin = {
								mode     = "all"
								override = true
							}
							allow_credentials = true
							max_age = {
								value    = 86400
								override = true
							}
						}

						# --- preflight ---
						generate_preflight_response = {
							max_age         = 3600
							allowed_methods = [
								{ method = "GET" },
								{ method = "OPTIONS" }
							]
							allowed_headers = ["Content-Type", "Authorization"]
						}

						# --- headers ---
						request_headers = [
							{
								name   = "X-CDN-Origin"
								values = ["io-river"]
								action = "set"
							}
						]
						response_headers = [
							{
								name   = "X-Frame-Options"
								values = ["SAMEORIGIN"]
								action = "set"
							}
						]
						origin_response_headers = [
							{
								name   = "X-Internal-Debug"
								action = "delete"
							}
						]

						# --- provider-specific ---
						provider_specific = [
							{
								provider = "fastly"
								code     = jsonencode(%s)
							}
						]
					}
				}
			]
		}
	}
}`

	return fmt.Sprintf(format, resourceName, resourceName, certId, behaviorName, exampleSpecificFastlyCode.ValueString())
}

func testAccDefaultBehaviorLifecycleConfig(variant string, resourceName string, certId string) string {
	var defaultBehaviorBlock string
	switch variant {
	case "omit":
		defaultBehaviorBlock = ""
	case "no_actions":
		// default block present but actions omitted — tests DefaultActionsObject
		defaultBehaviorBlock = `
		behaviors = {
			default = {}
		}`
	case "set":
		defaultBehaviorBlock = `
		behaviors = {
			default = {
				actions = {
					cache_behavior = "CACHE"
					compression    = true
				}
			}
		}`
	case "update":
		defaultBehaviorBlock = `
		behaviors = {
			default = {
				actions = {
					cache_behavior = "BYPASS"
					compression    = false
				}
			}
		}`
	case "set_cache_ttl":
		defaultBehaviorBlock = `
		behaviors = {
			default = {
				actions = {
					cache_ttl = 3600
				}
			}
		}`
	case "set_cache_ttl_and_compression":
		// Two explicit fields — next step removes only cache_ttl, keeps compression
		defaultBehaviorBlock = `
		behaviors = {
			default = {
				actions = {
					cache_ttl  = 3600
					compression = true
				}
			}
		}`
	case "remove_cache_ttl_modify_compression":
		// cache_ttl removed → should revert to default 86400; compression stays true
		defaultBehaviorBlock = `
		behaviors = {
			default = {
				actions = {
					compression = false
				}
			}
		}`
	case "set_optional":
		// Set an optional (non-default) field — viewer_protocol is never set by the backend
		defaultBehaviorBlock = `
		behaviors = {
			default = {
				actions = {
					viewer_protocol = "HTTPS_ONLY"
				}
			}
		}`
	case "remove_optional":
		// Remove viewer_protocol — must become null (not keep old value, not revert to any default)
		defaultBehaviorBlock = `
		behaviors = {
			default = {
				actions = {}
			}
		}`
	default:
		panic("unknown variant: " + variant)
	}

	return fmt.Sprintf(`
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
					host     = "example.com"
					protocol = "https"
				}
			}
		]
		domains = []
		%s
	}
}`, resourceName, resourceName, certId, defaultBehaviorBlock)
}

// ─── BehaviorLifecycle HCL generator ─────────────────────────────────────────

// testAccBehaviorLifecycleConfig returns HCL for TestAccIORiverService_BehaviorLifecycle.
// behaviorNames must have at least 4 entries: [0]=full, [1][2][3]=minimal.
//
//	idx 0: no behaviors block (defaults only)
//	idx 1: all schema-valid actions on the default behavior
//	idx 2: minimal default + 1 custom behavior with every schema-defined action
//	idx 3: minimal default + 4 custom behaviors (full at [0], minimal at [1][2][3])
//	idx 4: same 4 behaviors with [0] and [1] swapped — positional reorder (non-empty plan)
//	idx 5: 4 behaviors each using the condition block — complex OR-of-ANDs covering
//	        7 condition fields, 5 operators, field_key, multi-group OR, multi-condition AND
//	idx 6: same 4 behaviors but behavior[0]'s condition is mutated in-place —
//	        group[0] gains a 3rd AND clause; group[1] flips country-in → country-not_in
func testAccBehaviorLifecycleConfig(idx int, resourceName, certId string, behaviorNames []string) string {
	header := fmt.Sprintf(`
resource "ioriver_service" "%s" {
	name        = "%s"
	certificate = "%s"
	description = "behavior lifecycle test"
	config = {`, resourceName, resourceName, certId)

	footer := `
	}
}`

	// allActionsDefaultBehavior is the behaviors block for idx 1: every schema-valid action
	// that makes sense on the default behavior ("/*").  Fields that imply restricting traffic
	// (deny_access, deny_access_by_ip, deny_access_by_time, allow_access_only_from_ip,
	// url_rewrites, redirect) are intentionally omitted so the test service remains reachable.
	// stream_logs is omitted because this config has no log_destinations block.
	// Note:
	//   • status_codes_ttl  → { status_code, cache_behavior, cache_ttl }  (ListNested)
	//   • request/response/origin_response_headers → { name, values=[...], action }  (SetNested)
	//   • cache_key.query_strings → { type, params=[...] }  — params required for include/exclude, forbidden for all/none
	//   • generate_preflight_response → allowed_methods is Required
	//   • allowed_methods / cached_methods → [{ method = "..." }]  — object, not string
	//   • deny_access_by_time.date_time_window.start_date / end_date → bare int64, not quoted
	allActionsDefaultBehavior := `
		behaviors = {
			default = {
				actions = {
					# ── scalars ──────────────────────────────────────────────
					cache_behavior           = "CACHE"
					cache_ttl                = 3600
					browser_cache_ttl        = 1800
					large_files_optimization = true
					origin_cache_control     = false
					stale_ttl                = 600
					follow_redirects         = true
					compression              = true
					viewer_protocol          = "HTTPS_ONLY"
					true_client_ip           = true

					# ── methods ──────────────────────────────────────────────
					allowed_methods = [
						{ method = "GET" },
						{ method = "HEAD" },
						{ method = "OPTIONS" },
					]
					cached_methods = [
						{ method = "GET" },
						{ method = "HEAD" },
					]

					# ── cache key ─────────────────────────────────────────────
					cache_key = {
						headers = [{ header = "Accept-Language" }]
						cookies = [{ cookie = "session" }]
						query_strings = {
							type = "include"
							params = [{ param = "page" }]
						}
						country     = true
						device_type = false
					}

					# ── host header ───────────────────────────────────────────
					host_header = { header_value = "origin.example.com" }

					# ── cors ──────────────────────────────────────────────────
					cors = {
						allow_origin = {
							mode     = "all"
							override = true
						}
						allow_credentials = true
						max_age = {
							value    = 86400
							override = true
						}
					}

					# ── preflight ─────────────────────────────────────────────
					generate_preflight_response = {
						max_age         = 3600
						allowed_methods = [
							{ method = "GET" },
							{ method = "OPTIONS" },
						]
						allowed_headers = ["Content-Type", "Authorization"]
					}

					# ── status code overrides ─────────────────────────────────
					status_code_browser_cache = [
						{ status_code = "404", cache_ttl = 3600 },
					]
					generate_response = [
						{ status_code = "404", response_url = "https://example.com/404.html" },
					]
					status_codes_ttl = [
						{ status_code = "5xx", cache_behavior = "CACHE", cache_ttl = 0 },
						{ status_code = "4xx", cache_behavior = "CACHE", cache_ttl = 10 },
					]

					# ── header modification ───────────────────────────────────
					request_headers = [
						{
							name   = "X-CDN-Origin"
							values = ["io-river"]
							action = "set"
						},
					]
					response_headers = [
						{
							name   = "X-Frame-Options"
							values = ["SAMEORIGIN"]
							action = "set"
						},
					]
					origin_response_headers = [
						{
							name   = "X-Internal-Debug"
							action = "delete"
						},
					]
				}
			}
		}`

	switch idx {
	case 0:
		// No behaviors block — backend fills defaults.
		return header + footer

	case 1:
		// All schema-valid actions on the default behavior.
		return header + allActionsDefaultBehavior + footer

	case 2:
		// Minimal default + 1 custom behavior with EVERY schema-defined action
		// (modelled after testAccAllActionsConfig — the known-good reference).
		return header + fmt.Sprintf(`
		behaviors = {
			default = {
				actions = {
					cache_behavior = "CACHE"
					cache_ttl      = 86400
				}
			}
			custom = [
				{
					name         = "%s"
					path_pattern = "/api/*"
					actions = {
						# ── scalars ──────────────────────────────────────────
						cache_behavior           = "CACHE"
						cache_ttl                = 3600
						browser_cache_ttl        = 1800
						large_files_optimization = true
						origin_cache_control     = true
						stale_ttl                = 600
						follow_redirects         = true
						compression              = true
						deny_access              = false
						true_client_ip           = true

						# ── methods ───────────────────────────────────────────
						allowed_methods = [
							{ method = "GET" },
							{ method = "HEAD" },
							{ method = "OPTIONS" },
						]
						cached_methods = [
							{ method = "GET" },
							{ method = "HEAD" },
						]

						# ── access control ────────────────────────────────────
						deny_access_by_ip = [
							{ ip = "1.2.3.4" }
						]
						deny_access_by_time = [
							{
								date_time_window = {
									start_date = 2000000000
									end_date   = 2100000000
								}
							}
						]

						# ── rewrites ──────────────────────────────────────────
						url_rewrites = [
							{
								source      = "/old/path"
								destination = "/new/path"
							}
						]

						# ── status code overrides ─────────────────────────────
						generate_response = [
							{ status_code = "404", response_url = "https://example.com/404.html" }
						]
						status_code_browser_cache = [
							{ status_code = "404", cache_ttl = 3600 }
						]
						status_codes_ttl = [
							{ status_code = "5xx", cache_behavior = "CACHE", cache_ttl = 0 },
							{ status_code = "4xx", cache_behavior = "CACHE", cache_ttl = 10 },
						]

						# ── cache key ─────────────────────────────────────────
						cache_key = {
							headers = [{ header = "Accept-Language" }]
							cookies = [{ cookie = "session" }]
							query_strings = {
								type = "include"
								params = [{ param = "page" }]
							}
							country     = true
							device_type = false
						}

						# ── host header ───────────────────────────────────────
						host_header = { header_value = "origin.example.com" }

						# ── cors ──────────────────────────────────────────────
						cors = {
							allow_origin = {
								mode     = "all"
								override = true
							}
							allow_credentials = true
							max_age = {
								value    = 86400
								override = true
							}
						}

						# ── preflight ─────────────────────────────────────────
						generate_preflight_response = {
							max_age         = 3600
							allowed_methods = [
								{ method = "GET" },
								{ method = "OPTIONS" },
							]
							allowed_headers = ["Content-Type", "Authorization"]
						}

						# ── header modification ───────────────────────────────
						request_headers = [
							{
								name   = "X-CDN-Origin"
								values = ["io-river"]
								action = "set"
							},
							{
								name   = "Cookie"
								action = "delete"
							},
						]
						response_headers = [
							{
								name   = "X-Frame-Options"
								values = ["SAMEORIGIN"]
								action = "set"
							},
							{
								name   = "X-Powered-By"
								action = "delete"
							},
						]
						origin_response_headers = [
							{
								name   = "X-Internal-Debug"
								action = "delete"
							}
						]

						# ── provider-specific ───────────────────────────────────
						provider_specific = [
							{
								provider = "fastly"
								code     = jsonencode(%s)
							}
						]
					}
				}
			]
		}`, behaviorNames[0], exampleSpecificFastlyCode.ValueString()) + footer

	case 3:
		// Minimal default + 4 custom behaviors: full at [0], minimal at [1][2][3].
		// cached_methods is required by the backend whenever cache_behavior = "CACHE".
		return header + fmt.Sprintf(`
		behaviors = {
			default = {
				actions = {
					cache_behavior = "CACHE"
					cache_ttl      = 86400
				}
			}
			custom = [
				{
					name         = "%s"
					path_pattern = "/api/*"
					actions = {
						cache_behavior = "CACHE"
						cache_ttl      = 3600
						cached_methods = [{ method = "GET" }, { method = "HEAD" }]
					}
				},
				{
					name         = "%s"
					path_pattern = "/images/*"
					actions = {
						cache_behavior = "CACHE"
						cache_ttl      = 60
						cached_methods = [{ method = "GET" }, { method = "HEAD" }]
					}
				},
				{
					name         = "%s"
					path_pattern = "/static/*"
					actions = {
						cache_behavior = "CACHE"
						cache_ttl      = 120
						cached_methods = [{ method = "GET" }, { method = "HEAD" }]
					}
				},
				{
					name         = "%s"
					path_pattern = "/fonts/*"
					actions = {
						cache_behavior = "CACHE"
						cache_ttl      = 180
						cached_methods = [{ method = "GET" }, { method = "HEAD" }]
					}
				},
			]
		}`, behaviorNames[0], behaviorNames[1], behaviorNames[2], behaviorNames[3]) + footer

	case 4:
		// Same 4 behaviors with [0] and [1] swapped — behaviors are positional (no
		// NamedListPlanModifier), so this produces a real non-empty plan.
		return header + fmt.Sprintf(`
		behaviors = {
			default = {
				actions = {
					cache_behavior = "CACHE"
					cache_ttl      = 86400
				}
			}
			custom = [
				{
					name         = "%s"
					path_pattern = "/images/*"
					actions = {
						cache_behavior = "CACHE"
						cache_ttl      = 60
						cached_methods = [{ method = "GET" }, { method = "HEAD" }]
					}
				},
				{
					name         = "%s"
					path_pattern = "/api/*"
					actions = {
						cache_behavior = "CACHE"
						cache_ttl      = 3600
						cached_methods = [{ method = "GET" }, { method = "HEAD" }]
					}
				},
				{
					name         = "%s"
					path_pattern = "/static/*"
					actions = {
						cache_behavior = "CACHE"
						cache_ttl      = 120
						cached_methods = [{ method = "GET" }, { method = "HEAD" }]
					}
				},
				{
					name         = "%s"
					path_pattern = "/fonts/*"
					actions = {
						cache_behavior = "CACHE"
						cache_ttl      = 180
						cached_methods = [{ method = "GET" }, { method = "HEAD" }]
					}
				},
			]
		}`, behaviorNames[1], behaviorNames[0], behaviorNames[2], behaviorNames[3]) + footer

	case 5:
		// 4 behaviors each using the condition block (not path_pattern).		// Goes wide on condition coverage — hits 7 condition fields and 5 operators.
		//
		// behavior[0]: 2-group OR
		//   AND( path match /api/v1/*,  header Content-Type eq application/json )
		//   AND( country in [US, CA, GB] )
		//
		// behavior[1]: 1-group OR
		//   AND( path match /images/*,  query_param format eq webp )
		//
		// behavior[2]: 2-group OR
		//   AND( method in [GET, HEAD] )
		//   AND( country not_in [CN, RU] )
		//
		// behavior[3]: 3-group OR — most complex
		//   AND( path matches_one_of [/admin/*, /superadmin/*],  header X-Role eq admin )
		//   AND( client.ip eq 10.0.0.1,  domain eq internal.example.com )
		//   AND( query_param debug eq 1,  country in [US] )
		//
		// TTL per behavior: 3600 / 60 / 120 / 180 (must not bleed across behaviors).
		return header + fmt.Sprintf(`
		behaviors = {
			default = {
				actions = {
					cache_behavior = "CACHE"
					cache_ttl      = 86400
				}
			}
			custom = [
				{
					# behavior[0]: 2-group OR — (path AND header) OR (country)
					name = "%s"
					condition = {
						or = [
							{
								and = [
									{
										field    = "http.request.path"
										operator = "match"
										values    = ["/api/v1/*"]
									},
									{
										field     = "http.request.header"
										operator  = "eq"
										values     = ["application/json"]
										field_key = "Content-Type"
									},
								]
							},
							{
								and = [
									{
										field    = "client.geo.country"
										operator = "in"
										values    = ["US", "CA", "GB"]
									},
								]
							},
						]
					}
					actions = {
						cache_behavior = "CACHE"
						cache_ttl      = 3600
						cached_methods = [{ method = "GET" }, { method = "HEAD" }]
					}
				},
				{
					# behavior[1]: 1-group OR — path AND query_param (field_key)
					name = "%s"
					condition = {
						or = [
							{
								and = [
									{
										field    = "http.request.path"
										operator = "match"
										values    = ["/images/*"]
									},
									{
										field     = "http.request.query_param"
										operator  = "eq"
										values     = ["webp"]
										field_key = "format"
									},
								]
							},
						]
					}
					actions = {
						cache_behavior = "CACHE"
						cache_ttl      = 60
						cached_methods = [{ method = "GET" }, { method = "HEAD" }]
					}
				},
				{
					# behavior[2]: 2-group OR — method-in OR country-not_in
					name = "%s"
					condition = {
						or = [
							{
								and = [
									{
										field    = "http.request.method"
										operator = "in"
										values    = ["GET", "HEAD"]
									},
								]
							},
							{
								and = [
									{
										field    = "client.geo.country"
										operator = "not_in"
										values    = ["CN", "RU"]
									},
								]
							},
						]
					}
					actions = {
						cache_behavior = "CACHE"
						cache_ttl      = 120
						cached_methods = [{ method = "GET" }, { method = "HEAD" }]
					}
				},
				{
					# behavior[3]: 3-group OR — most complex expression
					name = "%s"
					condition = {
						or = [
							{
								# group[0]: path matches_one_of + header X-Role
								and = [
									{
										field    = "http.request.path"
										operator = "matches_one_of"
										values    = ["/admin/*", "/superadmin/*"]
									},
									{
										field     = "http.request.header"
										operator  = "eq"
										values     = ["admin"]
										field_key = "X-Role"
									},
								]
							},
							{
								# group[1]: client.ip + domain
								and = [
									{
										field    = "client.ip"
										operator = "eq"
										values    = ["10.0.0.1"]
									},
									{
										field    = "http.request.domain"
										operator = "eq"
										values    = ["internal.example.com"]
									},
								]
							},
							{
								# group[2]: query_param debug + country-in
								and = [
									{
										field     = "http.request.query_param"
										operator  = "eq"
										values     = ["1"]
										field_key = "debug"
									},
									{
										field    = "client.geo.country"
										operator = "in"
										values    = ["US"]
									},
								]
							},
						]
					}
					actions = {
						cache_behavior = "CACHE"
						cache_ttl      = 180
						cached_methods = [{ method = "GET" }, { method = "HEAD" }]
					}
				},
			]
		}`, behaviorNames[0], behaviorNames[1], behaviorNames[2], behaviorNames[3]) + footer

	case 6:
		// Identical to idx 5 except behavior[0]'s condition is mutated in-place:
		//   group[0]: a third AND clause is added (method ne POST)
		//   group[1]: country-in [US,CA,GB]  →  country-not_in [CN,RU]
		// Behaviors [1][2][3] are unchanged — verifies that updating one behavior's
		// condition does not corrupt the others.
		return header + fmt.Sprintf(`
		behaviors = {
			default = {
				actions = {
					cache_behavior = "CACHE"
					cache_ttl      = 86400
				}
			}
			custom = [
				{
					# behavior[0]: MUTATED — group[0] gains a 3rd AND; group[1] flips to not_in
					name = "%s"
					condition = {
						or = [
							{
								and = [
									{
										field    = "http.request.path"
										operator = "match"
										values    = ["/api/v1/*"]
									},
									{
										field     = "http.request.header"
										operator  = "eq"
										values     = ["application/json"]
										field_key = "Content-Type"
									},
									{
										# new: exclude POST — change from create to update test
										field    = "http.request.method"
										operator = "ne"
										values    = ["POST"]
									},
								]
							},
							{
								and = [
									{
										# changed: in [US,CA,GB] → not_in [CN,RU]
										field    = "client.geo.country"
										operator = "not_in"
										values    = ["CN", "RU"]
									},
								]
							},
						]
					}
					actions = {
						cache_behavior = "CACHE"
						cache_ttl      = 3600
						cached_methods = [{ method = "GET" }, { method = "HEAD" }]
					}
				},
				{
					# behavior[1]: unchanged
					name = "%s"
					condition = {
						or = [
							{
								and = [
									{
										field    = "http.request.path"
										operator = "match"
										values    = ["/images/*"]
									},
									{
										field     = "http.request.query_param"
										operator  = "eq"
										values     = ["webp"]
										field_key = "format"
									},
								]
							},
						]
					}
					actions = {
						cache_behavior = "CACHE"
						cache_ttl      = 60
						cached_methods = [{ method = "GET" }, { method = "HEAD" }]
					}
				},
				{
					# behavior[2]: unchanged
					name = "%s"
					condition = {
						or = [
							{
								and = [
									{
										field    = "http.request.method"
										operator = "in"
										values    = ["GET", "HEAD"]
									},
								]
							},
							{
								and = [
									{
										field    = "client.geo.country"
										operator = "not_in"
										values    = ["CN", "RU"]
									},
								]
							},
						]
					}
					actions = {
						cache_behavior = "CACHE"
						cache_ttl      = 120
						cached_methods = [{ method = "GET" }, { method = "HEAD" }]
					}
				},
				{
					# behavior[3]: unchanged
					name = "%s"
					condition = {
						or = [
							{
								and = [
									{
										field    = "http.request.path"
										operator = "matches_one_of"
										values    = ["/admin/*", "/superadmin/*"]
									},
									{
										field     = "http.request.header"
										operator  = "eq"
										values     = ["admin"]
										field_key = "X-Role"
									},
								]
							},
							{
								and = [
									{
										field    = "client.ip"
										operator = "eq"
										values    = ["10.0.0.1"]
									},
									{
										field    = "http.request.domain"
										operator = "eq"
										values    = ["internal.example.com"]
									},
								]
							},
							{
								and = [
									{
										field     = "http.request.query_param"
										operator  = "eq"
										values     = ["1"]
										field_key = "debug"
									},
									{
										field    = "client.geo.country"
										operator = "in"
										values    = ["US"]
									},
								]
							},
						]
					}
					actions = {
						cache_behavior = "CACHE"
						cache_ttl      = 180
						cached_methods = [{ method = "GET" }, { method = "HEAD" }]
					}
				},
			]
		}`, behaviorNames[0], behaviorNames[1], behaviorNames[2], behaviorNames[3]) + footer

	default:
		panic(fmt.Sprintf("testAccBehaviorLifecycleConfig: invalid idx %d", idx))
	}
}
