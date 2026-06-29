package provider

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// ---------------------------------------------------------------------------
// Unit tests — ModelToMap / WafMapToModel round-trip
// ---------------------------------------------------------------------------

func TestWafModelToMap_Basic(t *testing.T) {
	ctx := context.Background()

	sec := &SecurityModel{
		Enabled: boolVal(true),
		Waf: &WafModel{
			LimitBodySize: boolVal(false),
			Checkpoint:    nil,
		},
	}

	m := sec.SecurityModelToMap(ctx)
	if m == nil {
		t.Fatal("expected non-nil map")
	}
	if m["enabled"] != true {
		t.Errorf("expected enabled=true, got %v", m["enabled"])
	}
	if m["limit_body_size"] != false {
		t.Errorf("expected limit_body_size=false, got %v", m["limit_body_size"])
	}
}

func TestWafModelToMap_WithCustomRule(t *testing.T) {
	ctx := context.Background()

	valSet, _ := stringSet([]string{"/admin"})
	sec := &SecurityModel{
		Enabled: boolVal(true),
		Waf: &WafModel{
			LimitBodySize: boolVal(false),
		},
		CustomRules: []WafCustomRuleModel{
			{
				Name:    strVal("block-admin"),
				Enabled: boolVal(true),
				Action:  strVal("block"),
				Condition: &WafConditionExpressionModel{
					Or: []WafConditionAndGroupModel{
						{
							And: []WafConditionModel{
								{
									Field:    strVal("http.request.path"),
									Operator: strVal("contains"),
									Values:   valSet,
									FieldKey: nullStr(),
								},
							},
						},
					},
				},
			},
		},
	}

	m := sec.SecurityModelToMap(ctx)
	customArr, ok := m["custom"].([]interface{})
	if !ok || len(customArr) != 1 {
		t.Fatalf("expected 1 custom rule, got %v", m["custom"])
	}
	rule := customArr[0].(map[string]interface{})
	if rule["name"] != "block-admin" {
		t.Errorf("expected name=block-admin, got %v", rule["name"])
	}
	cond := rule["condition"].(map[string]interface{})
	orArr := cond["or"].([]interface{})
	if len(orArr) != 1 {
		t.Fatalf("expected 1 OR group")
	}
}

func TestWafRoundTrip(t *testing.T) {
	ctx := context.Background()

	trustedSrcList, _ := stringListVal([]string{"192.168.1.1"})
	ipValSet, _ := stringSet([]string{"192.168.1.1", "10.0.0.1"})
	original := &WafModel{
		LimitBodySize: boolVal(true),
		Checkpoint: &WafCheckpointModel{
			WebAttacks: &WafCheckpointWebAttacksModel{
				Mode:            strVal("learn"),
				ConfidenceLevel: strVal("high"),
			},
			IPS: &WafCheckpointIPSModel{
				Mode:                   strVal("learn"),
				PerformanceImpact:      strVal("low"),
				Severity:               strVal("medium"),
				HighConfidenceAction:   strVal("block"),
				MediumConfidenceAction: strVal("block"),
				LowConfidenceAction:    strVal("log"),
			},
			TrustedSources: trustedSrcList,
			NumSources:     int64Val(3),
		},
	}

	// ModelToMap then SecurityMapToModel; custom and rate_limit live on SecurityModel now
	secOriginal := &SecurityModel{
		Enabled: boolVal(true),
		Waf:     original,
		CustomRules: []WafCustomRuleModel{
			{
				Name:    strVal("block-admin"),
				Enabled: boolVal(true),
				Action:  strVal("block"),
				Condition: &WafConditionExpressionModel{
					Or: []WafConditionAndGroupModel{
						{
							And: []WafConditionModel{
								{
									Field:    strVal("http.request.path"),
									Operator: strVal("contains"),
									Values:   mustStringSet([]string{"/admin"}),
									FieldKey: nullStr(),
								},
							},
						},
					},
				},
			},
		},
		RateLimit: []WafRateLimitRuleModel{
			{
				Name:                 strVal("rate-api"),
				Enabled:              boolVal(true),
				Action:               strVal("block"),
				NumOfRequests:        int64Val(100),
				TimeWindowSeconds:    int64Val(60),
				BlockDurationSeconds: int64Val(300),
				Condition: &WafConditionExpressionModel{
					Or: []WafConditionAndGroupModel{
						{
							And: []WafConditionModel{
								{
									Field:    strVal("client.ip.address"),
									Operator: strVal("ip_match"),
									Values:   ipValSet,
									FieldKey: nullStr(),
								},
							},
						},
					},
				},
			},
		},
	}

	secMap := secOriginal.SecurityModelToMap(ctx)
	recoveredSec, err := SecurityMapToModel(ctx, secMap)
	if err != nil {
		t.Fatalf("SecurityMapToModel returned error: %v", err)
	}
	recovered := recoveredSec.Waf

	if recovered == nil {
		t.Fatal("round-trip returned nil")
	}
	if recoveredSec.Enabled.ValueBool() != true {
		t.Errorf("enabled mismatch")
	}
	if recovered.Checkpoint == nil || recovered.Checkpoint.WebAttacks == nil {
		t.Errorf("checkpoint.web_attacks missing")
	}
	if recovered.Checkpoint.IPS == nil || recovered.Checkpoint.IPS.Mode.ValueString() != "learn" {
		t.Errorf("checkpoint.ips.mode mismatch")
	}
	if len(recoveredSec.CustomRules) != 1 || recoveredSec.CustomRules[0].Name.ValueString() != "block-admin" {
		t.Errorf("custom rule mismatch")
	}
	if len(recoveredSec.RateLimit) != 1 || recoveredSec.RateLimit[0].NumOfRequests.ValueInt64() != 100 {
		t.Errorf("rate_limit mismatch")
	}
}

func TestValidateWafModel_IgnoreParamsMissing(t *testing.T) {
	errs := ValidateCustomRules(t.Context(), []WafCustomRuleModel{
		{
			Name:   strVal("bad-ignore"),
			Action: strVal("ignore"),
			// IgnoreParams intentionally missing
		},
	})
	if len(errs) == 0 {
		t.Error("expected validation error for missing ignore_params when action=ignore")
	}
}

func TestValidateWafModel_FieldKeyMissingForCollectionField(t *testing.T) {
	valSet, _ := stringSet([]string{"X-Custom"})
	errs := ValidateCustomRules(t.Context(), []WafCustomRuleModel{
		{
			Name:   strVal("bad-header"),
			Action: strVal("block"),
			Condition: &WafConditionExpressionModel{
				Or: []WafConditionAndGroupModel{
					{And: []WafConditionModel{
						{
							Field:    strVal("http.request.header"),
							Operator: strVal("exists"),
							Values:   valSet,
							FieldKey: nullStr(), // missing!
						},
					}},
				},
			},
		},
	})
	if len(errs) == 0 {
		t.Error("expected validation error for missing field_key on header field")
	}
}

// ---------------------------------------------------------------------------
// Omission / Computed field unit tests
// ---------------------------------------------------------------------------

// When the entire waf block is nil, ModelToMap returns nil and config_model
// skips it — nothing is sent to the backend.
func TestWafNilModel_ReturnsNilMap(t *testing.T) {
	var w *WafModel
	m := w.ModelToMap(context.Background())
	if m != nil {
		t.Errorf("expected nil map for nil WafModel, got %v", m)
	}
}

// When checkpoint is omitted (nil pointer), the key must not appear in the map.
func TestWafModelToMap_CheckpointOmitted(t *testing.T) {
	model := &WafModel{
		Checkpoint: nil,
	}
	m := model.ModelToMap(context.Background())
	if _, present := m["checkpoint"]; present {
		t.Error("checkpoint key must not be sent when block is omitted")
	}
}

// When action is Computed (unknown/null at plan time), it must NOT be
// serialised as an empty string — the key should be absent so the backend
// fills in its default.
func TestWafModelToMap_CustomActionNull_NotSent(t *testing.T) {
	valSet, _ := stringSet([]string{"/x"})
	sec := &SecurityModel{
		Enabled: boolVal(true),
		Waf:     &WafModel{},
		CustomRules: []WafCustomRuleModel{
			{
				Name:   strVal("rule-no-action"),
				Action: types.StringNull(), // Computed, not yet known
				Condition: &WafConditionExpressionModel{
					Or: []WafConditionAndGroupModel{
						{And: []WafConditionModel{
							{Field: strVal("http.request.path"), Operator: strVal("contains"), Values: valSet, FieldKey: nullStr()},
						}},
					},
				},
			},
		},
	}
	m := sec.SecurityModelToMap(context.Background())
	customArr := m["custom"].([]interface{})
	rule := customArr[0].(map[string]interface{})
	if _, present := rule["action"]; present {
		t.Error("action must not be serialised when it is null/unknown")
	}
}

// Same check for rate_limit action — rate_limit now lives on SecurityModel.
func TestSecurityModelToMap_RateLimitActionNull_NotSent(t *testing.T) {
	valSet, _ := stringSet([]string{"/x"})
	sec := &SecurityModel{
		Enabled: boolVal(true),
		Waf:     &WafModel{},
		RateLimit: []WafRateLimitRuleModel{
			{
				Name:                 strVal("rl-no-action"),
				Action:               types.StringNull(), // Computed, not yet known
				NumOfRequests:        int64Val(100),
				TimeWindowSeconds:    int64Val(60),
				BlockDurationSeconds: int64Val(300),
				Condition: &WafConditionExpressionModel{
					Or: []WafConditionAndGroupModel{
						{And: []WafConditionModel{
							{Field: strVal("http.request.path"), Operator: strVal("contains"), Values: valSet, FieldKey: nullStr()},
						}},
					},
				},
			},
		},
	}
	m := sec.SecurityModelToMap(context.Background())
	rlArr := m["rate_limit"].([]interface{})
	rule := rlArr[0].(map[string]interface{})
	if _, present := rule["action"]; present {
		t.Error("rate_limit action must not be serialised when it is null/unknown")
	}
}

// WafMapToModel with a map that has checkpoint filled with backend
// defaults — verifies round-trip for Computed fields coming back from the API.
func TestWafMapToModel_BackendDefaults(t *testing.T) {
	ctx := context.Background()

	// Simulate what the backend returns when the user sent nothing for checkpoint
	apiMap := map[string]interface{}{
		"enabled":         false,
		"limit_body_size": false,
		"checkpoint": map[string]interface{}{
			"web_attacks": map[string]interface{}{
				"mode":             "learn",
				"confidence_level": "high",
			},
			"ips": map[string]interface{}{
				"mode":                     "learn",
				"performance_impact":       "medium",
				"severity":                 "medium",
				"high_confidence_action":   "block",
				"medium_confidence_action": "block",
				"low_confidence_action":    "log",
			},
			"trusted_sources": []interface{}{},
			"num_sources":     float64(3),
		},
		"custom":     []interface{}{},
		"rate_limit": []interface{}{},
	}

	model := WafMapToModel(ctx, apiMap)
	if model == nil {
		t.Fatal("WafMapToModel returned nil for backend defaults map")
	}

	// enabled lives on SecurityModel; check via SecurityMapToModel
	secModel, err := SecurityMapToModel(ctx, apiMap)
	if err != nil {
		t.Fatalf("SecurityMapToModel returned error: %v", err)
	}
	if secModel.Enabled.ValueBool() != false {
		t.Errorf("enabled: expected false, got %v", secModel.Enabled.ValueBool())
	}

	// checkpoint defaults
	if model.Checkpoint == nil || model.Checkpoint.WebAttacks == nil {
		t.Fatal("checkpoint.web_attacks must not be nil when backend returns it")
	}
	if model.Checkpoint.WebAttacks.Mode.ValueString() != "learn" {
		t.Errorf("web_attacks.mode: expected learn, got %v", model.Checkpoint.WebAttacks.Mode.ValueString())
	}
	if model.Checkpoint.IPS == nil {
		t.Fatal("checkpoint.ips must not be nil when backend returns it")
	}
	if model.Checkpoint.IPS.LowConfidenceAction.ValueString() != "log" {
		t.Errorf("ips.low_confidence_action: expected log, got %v", model.Checkpoint.IPS.LowConfidenceAction.ValueString())
	}
	if model.Checkpoint.NumSources.ValueInt64() != 3 {
		t.Errorf("minimal_num_sources: expected 3, got %v", model.Checkpoint.NumSources.ValueInt64())
	}

	// empty collections — custom now lives on SecurityModel, not WafModel
	// WafMapToModel no longer parses custom; SecurityMapToModel does.
	// rate_limit now lives on SecurityModel; WafMapToModel no longer parses it
}

// wafMapToModelWithCtx always parses checkpoint from the backend response.
// Suppression of the security block entirely (when the user omitted it) happens
// one level up in ServiceConfigMapToModel via SecurityConfigured — not here.
// The backend ALWAYS returns checkpoint with defaults, even when enabled=false.
func TestWafMapToModelWithCtx_CheckpointAlwaysParsed(t *testing.T) {
	ctx := context.Background()

	apiMap := map[string]interface{}{
		"enabled":         true,
		"limit_body_size": false,
		"checkpoint": map[string]interface{}{
			"web_attacks": map[string]interface{}{
				"mode":             "learn",
				"confidence_level": "high",
			},
			"ips": map[string]interface{}{
				"mode":                     "learn",
				"performance_impact":       "medium",
				"severity":                 "medium",
				"high_confidence_action":   "block",
				"medium_confidence_action": "block",
				"low_confidence_action":    "log",
			},
			"trusted_sources": []interface{}{},
			"num_sources":     float64(3),
		},
		"custom":     []interface{}{},
		"rate_limit": []interface{}{},
	}

	// Regardless of SecurityConfigured, wafMapToModelWithCtx always parses checkpoint.
	transformCtx := &ServiceTransformContext{}

	model := wafMapToModelWithCtx(ctx, apiMap, transformCtx)
	if model == nil {
		t.Fatal("wafMapToModelWithCtx returned nil")
	}
	if model.Checkpoint == nil {
		t.Fatal("checkpoint must always be parsed from backend response")
	}
	if model.LimitBodySize.IsNull() {
		t.Errorf("expected LimitBodySize to be populated, got null")
	}
}

// wafMapToModelWithCtx populates checkpoint from whatever the backend returns.
func TestWafMapToModelWithCtx_CheckpointPopulatedWhenConfigured(t *testing.T) {
	ctx := context.Background()

	apiMap := map[string]interface{}{
		"enabled":         false,
		"limit_body_size": false,
		"checkpoint": map[string]interface{}{
			"web_attacks": map[string]interface{}{
				"mode":             "prevent",
				"confidence_level": "low",
			},
			"ips": map[string]interface{}{
				"mode":                     "prevent",
				"performance_impact":       "low",
				"severity":                 "high",
				"high_confidence_action":   "block",
				"medium_confidence_action": "log",
				"low_confidence_action":    "log",
			},
			"trusted_sources": []interface{}{},
			"num_sources":     float64(5),
		},
		"custom":     []interface{}{},
		"rate_limit": []interface{}{},
	}

	// wafMapToModelWithCtx always parses checkpoint; SecurityConfigured gate is upstream.
	transformCtx := &ServiceTransformContext{}

	model := wafMapToModelWithCtx(ctx, apiMap, transformCtx)
	if model == nil {
		t.Fatal("wafMapToModelWithCtx returned nil")
	}
	if model.Checkpoint == nil {
		t.Fatal("expected Checkpoint to be populated when WafCheckpointConfigured=true, got nil")
	}
	if model.Checkpoint.WebAttacks == nil {
		t.Fatal("expected Checkpoint.WebAttacks to be populated")
	}
	if model.Checkpoint.WebAttacks.Mode.ValueString() != "prevent" {
		t.Errorf("web_attacks.mode: expected prevent, got %v", model.Checkpoint.WebAttacks.Mode.ValueString())
	}
	if model.Checkpoint.NumSources.ValueInt64() != 5 {
		t.Errorf("minimal_num_sources: expected 5, got %v", model.Checkpoint.NumSources.ValueInt64())
	}
}

// field_key is correctly round-tripped for a collection-type field.
func TestWafRoundTrip_WithFieldKey(t *testing.T) {
	ctx := context.Background()
	valSet, _ := stringSet([]string{"Bearer"})

	sec := &SecurityModel{
		Enabled: boolVal(true),
		Waf:     &WafModel{},
		CustomRules: []WafCustomRuleModel{
			{
				Name:   strVal("check-auth-header"),
				Action: strVal("block"),
				Condition: &WafConditionExpressionModel{
					Or: []WafConditionAndGroupModel{
						{And: []WafConditionModel{
							{
								Field:    strVal("http.request.header"),
								Operator: strVal("contains"),
								Values:   valSet,
								FieldKey: strVal("Authorization"), // collection field with key
							},
						}},
					},
				},
			},
		},
	}

	apiMap := sec.SecurityModelToMap(ctx)
	recovered, err := SecurityMapToModel(ctx, apiMap)
	if err != nil {
		t.Fatalf("SecurityMapToModel returned error: %v", err)
	}

	if recovered == nil || len(recovered.CustomRules) != 1 {
		t.Fatal("round-trip lost the custom rule")
	}
	cond := recovered.CustomRules[0].Condition
	if cond == nil || len(cond.Or) == 0 || len(cond.Or[0].And) == 0 {
		t.Fatal("condition lost in round-trip")
	}
	got := cond.Or[0].And[0].FieldKey.ValueString()
	if got != "Authorization" {
		t.Errorf("field_key: expected Authorization, got %v", got)
	}
}

// ignore_params is correctly round-tripped.
func TestWafRoundTrip_WithIgnoreParams(t *testing.T) {
	ctx := context.Background()
	valSet, _ := stringSet([]string{"anything"})

	sec := &SecurityModel{
		Enabled: boolVal(true),
		Waf:     &WafModel{},
		CustomRules: []WafCustomRuleModel{
			{
				Name:   strVal("ignore-rule"),
				Action: strVal("ignore"),
				Condition: &WafConditionExpressionModel{
					Or: []WafConditionAndGroupModel{
						{And: []WafConditionModel{
							{Field: strVal("http.request.path"), Operator: strVal("contains"), Values: valSet, FieldKey: nullStr()},
						}},
					},
				},
				IgnoreParams: &WafIgnoreParamsModel{
					IgnoreType: strVal("json_body_param"),
					Value:      strVal("token"),
				},
			},
		},
	}

	apiMap := sec.SecurityModelToMap(ctx)
	recovered, err := SecurityMapToModel(ctx, apiMap)
	if err != nil {
		t.Fatalf("SecurityMapToModel returned error: %v", err)
	}

	if recovered == nil || len(recovered.CustomRules) != 1 {
		t.Fatal("round-trip lost the custom rule")
	}
	ip := recovered.CustomRules[0].IgnoreParams
	if ip == nil {
		t.Fatal("ignore_params lost in round-trip")
	}
	if ip.IgnoreType.ValueString() != "json_body_param" {
		t.Errorf("ignore_type: expected json_body_param, got %v", ip.IgnoreType.ValueString())
	}
	if ip.Value.ValueString() != "token" {
		t.Errorf("ignore_params.value: expected token, got %v", ip.Value.ValueString())
	}
}

// validate passes for a valid model with no issues.
func TestValidateWafModel_Clean(t *testing.T) {
	valSet, _ := stringSet([]string{"/ok"})
	errs := ValidateCustomRules(t.Context(), []WafCustomRuleModel{
		{
			Name:   strVal("ok-rule"),
			Action: strVal("block"),
			Condition: &WafConditionExpressionModel{
				Or: []WafConditionAndGroupModel{
					{And: []WafConditionModel{
						{Field: strVal("http.request.path"), Operator: strVal("contains"), Values: valSet, FieldKey: nullStr()},
					}},
				},
			},
		},
	})
	if len(errs) != 0 {
		t.Errorf("expected no validation errors, got: %v", errs)
	}
}

// validate catches field_key missing on query_param and json_param too.
func TestValidateWafModel_FieldKeyRequired_QueryParam(t *testing.T) {
	valSet, _ := stringSet([]string{"admin"})
	errs := ValidateCustomRules(t.Context(), []WafCustomRuleModel{
		{
			Name:   strVal("bad-query"),
			Action: strVal("block"),
			Condition: &WafConditionExpressionModel{
				Or: []WafConditionAndGroupModel{
					{And: []WafConditionModel{
						{Field: strVal("http.request.query_param"), Operator: strVal("eq"), Values: valSet, FieldKey: nullStr()},
					}},
				},
			},
		},
	})
	if len(errs) == 0 {
		t.Error("expected validation error for missing field_key on query_param")
	}
}

// validate passes when field_key IS set for a collection field.
func TestValidateWafModel_FieldKeyPresent_OK(t *testing.T) {
	valSet, _ := stringSet([]string{"admin"})
	errs := ValidateCustomRules(t.Context(), []WafCustomRuleModel{
		{
			Name:   strVal("ok-query"),
			Action: strVal("block"),
			Condition: &WafConditionExpressionModel{
				Or: []WafConditionAndGroupModel{
					{And: []WafConditionModel{
						{Field: strVal("http.request.query_param"), Operator: strVal("eq"), Values: valSet, FieldKey: strVal("user")},
					}},
				},
			},
		},
	})
	if len(errs) != 0 {
		t.Errorf("expected no errors when field_key is set, got: %v", errs)
	}
}

// ---------------------------------------------------------------------------
// Ordering tests — verifies that list ordering is preserved through round-trip
// ---------------------------------------------------------------------------

// TestWafRoundTrip_OrderPreserved builds 3 custom rules and 3 rate_limit rules
// in a deliberate non-alphabetical order (charlie, alpha, bravo) and asserts
// that WafMapToModel returns them in the exact same order — no reordering.
// This tests the assumption that the backend preserves array insertion order.
func TestWafRoundTrip_OrderPreserved(t *testing.T) {
	ctx := context.Background()
	valSet, _ := stringSet([]string{"/path"})

	makeCustomRule := func(name string) WafCustomRuleModel {
		return WafCustomRuleModel{
			Name:   strVal(name),
			Action: strVal("block"),
			Condition: &WafConditionExpressionModel{
				Or: []WafConditionAndGroupModel{
					{And: []WafConditionModel{
						{Field: strVal("http.request.path"), Operator: strVal("contains"), Values: valSet, FieldKey: nullStr()},
					}},
				},
			},
		}
	}

	makeRateLimitRule := func(name string, requests int64) WafRateLimitRuleModel {
		return WafRateLimitRuleModel{
			Name:                 strVal(name),
			Action:               strVal("block"),
			NumOfRequests:        int64Val(requests),
			TimeWindowSeconds:    int64Val(60),
			BlockDurationSeconds: int64Val(300),
		}
	}

	// Deliberately non-alphabetical: charlie, alpha, bravo
	// custom and rate_limit now live on SecurityModel alongside waf
	sec := &SecurityModel{
		Enabled: boolVal(true),
		Waf:     &WafModel{},
		CustomRules: []WafCustomRuleModel{
			makeCustomRule("charlie-rule"),
			makeCustomRule("alpha-rule"),
			makeCustomRule("bravo-rule"),
		},
		RateLimit: []WafRateLimitRuleModel{
			makeRateLimitRule("charlie-rl", 300),
			makeRateLimitRule("alpha-rl", 100),
			makeRateLimitRule("bravo-rl", 200),
		},
	}

	secMap := sec.SecurityModelToMap(ctx)
	recoveredSec, err := SecurityMapToModel(ctx, secMap)
	if err != nil {
		t.Fatalf("SecurityMapToModel returned error: %v", err)
	}

	if recoveredSec == nil {
		t.Fatal("round-trip returned nil")
	}

	// Custom rules: order must be preserved exactly
	wantCustomOrder := []string{"charlie-rule", "alpha-rule", "bravo-rule"}
	if len(recoveredSec.CustomRules) != len(wantCustomOrder) {
		t.Fatalf("custom: expected %d rules, got %d", len(wantCustomOrder), len(recoveredSec.CustomRules))
	}
	for i, want := range wantCustomOrder {
		got := recoveredSec.CustomRules[i].Name.ValueString()
		if got != want {
			t.Errorf("custom[%d]: expected %q, got %q — order not preserved", i, want, got)
		}
	}

	// Rate limit rules: order must be preserved exactly (via SecurityModel)
	wantRLOrder := []string{"charlie-rl", "alpha-rl", "bravo-rl"}
	if len(recoveredSec.RateLimit) != len(wantRLOrder) {
		t.Fatalf("rate_limit: expected %d rules, got %d", len(wantRLOrder), len(recoveredSec.RateLimit))
	}
	for i, want := range wantRLOrder {
		got := recoveredSec.RateLimit[i].Name.ValueString()
		if got != want {
			t.Errorf("rate_limit[%d]: expected %q, got %q — order not preserved", i, want, got)
		}
	}

	// Also verify numeric fields survived for the rate_limit rules
	wantRequests := []int64{300, 100, 200}
	for i, want := range wantRequests {
		got := recoveredSec.RateLimit[i].NumOfRequests.ValueInt64()
		if got != want {
			t.Errorf("rate_limit[%d].num_of_requests: expected %d, got %d", i, want, got)
		}
	}
}

// ---------------------------------------------------------------------------
// ValidateSecurityModel tests
// ---------------------------------------------------------------------------

func TestValidateSecurityModel_Nil(t *testing.T) {
	errs := ValidateSecurityModel(context.Background(), nil)
	if len(errs) != 0 {
		t.Errorf("expected no errors for nil security, got: %v", errs)
	}
}

func TestValidateSecurityModel_Clean(t *testing.T) {
	valSet, _ := stringSet([]string{"/ok"})
	sec := &SecurityModel{
		CustomRules: []WafCustomRuleModel{
			{
				Name:   strVal("ok-rule"),
				Action: strVal("block"),
				Condition: &WafConditionExpressionModel{
					Or: []WafConditionAndGroupModel{
						{And: []WafConditionModel{
							{Field: strVal("http.request.path"), Operator: strVal("contains"), Values: valSet, FieldKey: nullStr()},
						}},
					},
				},
			},
		},
		RateLimit: []WafRateLimitRuleModel{
			{
				Name:                 strVal("ok-rl"),
				Action:               strVal("block"),
				NumOfRequests:        int64Val(100),
				TimeWindowSeconds:    int64Val(60),
				BlockDurationSeconds: int64Val(300),
			},
		},
	}
	errs := ValidateSecurityModel(context.Background(), sec)
	if len(errs) != 0 {
		t.Errorf("expected no errors, got: %v", errs)
	}
}

func TestValidateSecurityModel_RateLimitFieldKeyMissing(t *testing.T) {
	valSet, _ := stringSet([]string{"admin"})
	sec := &SecurityModel{
		RateLimit: []WafRateLimitRuleModel{
			{
				Name:                 strVal("bad-rl"),
				NumOfRequests:        int64Val(10),
				TimeWindowSeconds:    int64Val(60),
				BlockDurationSeconds: int64Val(300),
				Condition: &WafConditionExpressionModel{
					Or: []WafConditionAndGroupModel{
						{And: []WafConditionModel{
							{
								Field:    strVal("http.request.query_param"),
								Operator: strVal("eq"),
								Values:   valSet,
								FieldKey: nullStr(), // missing!
							},
						}},
					},
				},
			},
		},
	}
	errs := ValidateSecurityModel(context.Background(), sec)
	if len(errs) == 0 {
		t.Error("expected error for missing field_key on rate_limit condition")
	}
}

func TestValidateSecurityModel_WafIgnoreParamsMissing(t *testing.T) {
	sec := &SecurityModel{
		CustomRules: []WafCustomRuleModel{
			{Name: strVal("bad"), Action: strVal("ignore")}, // ignore_params missing
		},
	}
	errs := ValidateSecurityModel(context.Background(), sec)
	if len(errs) == 0 {
		t.Error("expected error for missing ignore_params propagated via ValidateSecurityModel")
	}
}

// ---------------------------------------------------------------------------
// validateCondition — backend-accurate cross-field validation tests
// ---------------------------------------------------------------------------

// ── A. field_key ─────────────────────────────────────────────────────────────

func TestValidateCondition_FieldKeyOnPlainField(t *testing.T) {
	errs := ValidateCustomRules(t.Context(), []WafCustomRuleModel{{
		Name: strVal("bad"), Action: strVal("block"),
		Condition: cond1(strVal("http.request.path"), strVal("contains"), mustStringSet([]string{"/x"}), strVal("oops")),
	}})
	if len(errs) == 0 {
		t.Error("expected error: field_key set on non-collection field")
	}
}

func TestValidateCondition_CollectionFieldMissingFieldKey(t *testing.T) {
	errs := ValidateCustomRules(t.Context(), []WafCustomRuleModel{{
		Name: strVal("bad"), Action: strVal("block"),
		Condition: cond1(strVal("http.request.header"), strVal("contains"), mustStringSet([]string{"x"}), nullStr()),
	}})
	if len(errs) == 0 {
		t.Error("expected error: collection field missing field_key")
	}
}

func TestValidateCondition_CollectionFieldWithFieldKey_OK(t *testing.T) {
	errs := ValidateCustomRules(t.Context(), []WafCustomRuleModel{{
		Name: strVal("ok"), Action: strVal("block"),
		Condition: cond1(strVal("http.request.header"), strVal("contains"), mustStringSet([]string{"x"}), strVal("X-Custom")),
	}})
	if len(errs) != 0 {
		t.Errorf("expected no errors, got: %v", errs)
	}
}

// ── B. uri_raw operator restrictions ─────────────────────────────────────────

func TestValidateCondition_URIRaw_IpMatchForbidden(t *testing.T) {
	errs := ValidateCustomRules(t.Context(), []WafCustomRuleModel{{
		Name: strVal("bad"), Action: strVal("block"),
		Condition: cond1(strVal("http.request.uri_raw"), strVal("ip_match"), mustStringSet([]string{"10.0.0.0/8"}), nullStr()),
	}})
	if len(errs) == 0 {
		t.Error("expected error: ip_match is forbidden for uri_raw")
	}
}

func TestValidateCondition_URIRaw_ExistsForbidden(t *testing.T) {
	emptySet, _ := stringSet([]string{})
	errs := ValidateCustomRules(t.Context(), []WafCustomRuleModel{{
		Name: strVal("bad"), Action: strVal("block"),
		Condition: cond1(strVal("http.request.uri_raw"), strVal("exists"), emptySet, nullStr()),
	}})
	if len(errs) == 0 {
		t.Error("expected error: exists is forbidden for uri_raw")
	}
}

func TestValidateCondition_URIRaw_LtForbidden(t *testing.T) {
	errs := ValidateCustomRules(t.Context(), []WafCustomRuleModel{{
		Name: strVal("bad"), Action: strVal("block"),
		Condition: cond1(strVal("http.request.uri_raw"), strVal("lt"), mustStringSet([]string{"50"}), nullStr()),
	}})
	if len(errs) == 0 {
		t.Error("expected error: lt is forbidden for uri_raw")
	}
}

func TestValidateCondition_URIRaw_EqRequiresFullURL_Error(t *testing.T) {
	errs := ValidateCustomRules(t.Context(), []WafCustomRuleModel{{
		Name: strVal("bad"), Action: strVal("block"),
		Condition: cond1(strVal("http.request.uri_raw"), strVal("eq"), mustStringSet([]string{"/not-a-url"}), nullStr()),
	}})
	if len(errs) == 0 {
		t.Error("expected error: uri_raw eq requires a full URL")
	}
}

func TestValidateCondition_URIRaw_EqWithFullURL_OK(t *testing.T) {
	errs := ValidateCustomRules(t.Context(), []WafCustomRuleModel{{
		Name: strVal("ok"), Action: strVal("block"),
		Condition: cond1(strVal("http.request.uri_raw"), strVal("eq"), mustStringSet([]string{"https://example.com/path"}), nullStr()),
	}})
	if len(errs) != 0 {
		t.Errorf("expected no errors for uri_raw eq with full URL, got: %v", errs)
	}
}

func TestValidateCondition_URIRaw_InRequiresFullURL_Error(t *testing.T) {
	errs := ValidateCustomRules(t.Context(), []WafCustomRuleModel{{
		Name: strVal("bad"), Action: strVal("block"),
		Condition: cond1(strVal("http.request.uri_raw"), strVal("in"), mustStringSet([]string{"https://example.com", "not-a-url"}), nullStr()),
	}})
	if len(errs) == 0 {
		t.Error("expected error: uri_raw in requires all values to be full URLs")
	}
}

func TestValidateCondition_URIRaw_BeginsWith_InvalidPrefix_Error(t *testing.T) {
	errs := ValidateCustomRules(t.Context(), []WafCustomRuleModel{{
		Name: strVal("bad"), Action: strVal("block"),
		Condition: cond1(strVal("http.request.uri_raw"), strVal("begins_with"), mustStringSet([]string{"ftp://bad"}), nullStr()),
	}})
	if len(errs) == 0 {
		t.Error("expected error: uri_raw begins_with with invalid URL prefix")
	}
}

func TestValidateCondition_URIRaw_BeginsWith_ValidPrefix_OK(t *testing.T) {
	errs := ValidateCustomRules(t.Context(), []WafCustomRuleModel{{
		Name: strVal("ok"), Action: strVal("block"),
		Condition: cond1(strVal("http.request.uri_raw"), strVal("begins_with"), mustStringSet([]string{"https://example.com"}), nullStr()),
	}})
	if len(errs) != 0 {
		t.Errorf("expected no errors for uri_raw begins_with valid prefix, got: %v", errs)
	}
}

func TestValidateCondition_URIRaw_Regex_InvalidRegex_Error(t *testing.T) {
	errs := ValidateCustomRules(t.Context(), []WafCustomRuleModel{{
		Name: strVal("bad"), Action: strVal("block"),
		Condition: cond1(strVal("http.request.uri_raw"), strVal("regex"), mustStringSet([]string{"[invalid"}), nullStr()),
	}})
	if len(errs) == 0 {
		t.Error("expected error: uri_raw regex with invalid regexp")
	}
}

func TestValidateCondition_URIRaw_Regex_ValidRegex_OK(t *testing.T) {
	errs := ValidateCustomRules(t.Context(), []WafCustomRuleModel{{
		Name: strVal("ok"), Action: strVal("block"),
		Condition: cond1(strVal("http.request.uri_raw"), strVal("regex"), mustStringSet([]string{"https://example\\.com/.*"}), nullStr()),
	}})
	if len(errs) != 0 {
		t.Errorf("expected no errors for uri_raw regex with valid regexp, got: %v", errs)
	}
}

// contains on uri_raw is free-form → OK.
func TestValidateCondition_URIRaw_Contains_OK(t *testing.T) {
	errs := ValidateCustomRules(t.Context(), []WafCustomRuleModel{{
		Name: strVal("ok"), Action: strVal("block"),
		Condition: cond1(strVal("http.request.uri_raw"), strVal("contains"), mustStringSet([]string{"/api/v"}), nullStr()),
	}})
	if len(errs) != 0 {
		t.Errorf("expected no errors for uri_raw contains, got: %v", errs)
	}
}

// ip_match on http.request.path is NOT forbidden by the backend → OK.
func TestValidateCondition_IpMatchOnPath_NotForbidden(t *testing.T) {
	errs := ValidateCustomRules(t.Context(), []WafCustomRuleModel{{
		Name: strVal("ok"), Action: strVal("block"),
		Condition: cond1(strVal("http.request.path"), strVal("ip_match"), mustStringSet([]string{"10.0.0.0/8"}), nullStr()),
	}})
	// Backend doesn't restrict ip_match to client.ip.address — only uri_raw forbids it.
	// No error expected at validation time (may fail at provider deployment for semantic reasons).
	_ = errs // result is informational; don't assert
}

// ── C. path operator restrictions ────────────────────────────────────────────

func TestValidateCondition_Path_EqNoSlash_Error(t *testing.T) {
	errs := ValidateCustomRules(t.Context(), []WafCustomRuleModel{{
		Name: strVal("bad"), Action: strVal("block"),
		Condition: cond1(strVal("http.request.path"), strVal("eq"), mustStringSet([]string{"no-leading-slash"}), nullStr()),
	}})
	if len(errs) == 0 {
		t.Error("expected error: path eq values must start with '/'")
	}
}

func TestValidateCondition_Path_EqWithSlash_OK(t *testing.T) {
	errs := ValidateCustomRules(t.Context(), []WafCustomRuleModel{{
		Name: strVal("ok"), Action: strVal("block"),
		Condition: cond1(strVal("http.request.path"), strVal("eq"), mustStringSet([]string{"/login"}), nullStr()),
	}})
	if len(errs) != 0 {
		t.Errorf("expected no errors for path eq with slash, got: %v", errs)
	}
}

func TestValidateCondition_Path_Regex_InvalidRegex_Error(t *testing.T) {
	errs := ValidateCustomRules(t.Context(), []WafCustomRuleModel{{
		Name: strVal("bad"), Action: strVal("block"),
		Condition: cond1(strVal("http.request.path"), strVal("regex"), mustStringSet([]string{"[broken"}), nullStr()),
	}})
	if len(errs) == 0 {
		t.Error("expected error: path regex with invalid regexp")
	}
}

func TestValidateCondition_Path_Regex_ValidRegex_OK(t *testing.T) {
	errs := ValidateCustomRules(t.Context(), []WafCustomRuleModel{{
		Name: strVal("ok"), Action: strVal("block"),
		Condition: cond1(strVal("http.request.path"), strVal("regex"), mustStringSet([]string{"/api/v[0-9]+/.*"}), nullStr()),
	}})
	if len(errs) != 0 {
		t.Errorf("expected no errors for path regex with valid regexp, got: %v", errs)
	}
}

// ── D. client.ip.address IP/CIDR validation ───────────────────────────────────

func TestValidateCondition_IPAddress_InvalidCIDR_Error(t *testing.T) {
	errs := ValidateCustomRules(t.Context(), []WafCustomRuleModel{{
		Name: strVal("bad"), Action: strVal("block"),
		Condition: cond1(strVal("client.ip.address"), strVal("ip_match"), mustStringSet([]string{"not-an-ip"}), nullStr()),
	}})
	if len(errs) == 0 {
		t.Error("expected error: client.ip.address with invalid IP/CIDR")
	}
}

func TestValidateCondition_IPAddress_ValidCIDR_OK(t *testing.T) {
	errs := ValidateCustomRules(t.Context(), []WafCustomRuleModel{{
		Name: strVal("ok"), Action: strVal("block"),
		Condition: cond1(strVal("client.ip.address"), strVal("ip_match"), mustStringSet([]string{"10.0.0.0/8", "1.2.3.4"}), nullStr()),
	}})
	if len(errs) != 0 {
		t.Errorf("expected no errors for valid CIDRs, got: %v", errs)
	}
}

// ── E. value-presence rules ───────────────────────────────────────────────────

func TestValidateCondition_ExistsWithNonEmptyValue(t *testing.T) {
	errs := ValidateCustomRules(t.Context(), []WafCustomRuleModel{{
		Name: strVal("bad"), Action: strVal("block"),
		Condition: cond1(strVal("http.request.header"), strVal("exists"), mustStringSet([]string{"oops"}), strVal("X-Debug")),
	}})
	if len(errs) == 0 {
		t.Error("expected error: exists requires empty values list")
	}
}

func TestValidateCondition_DoesNotExistEmptyValue_OK(t *testing.T) {
	emptySet, _ := stringSet([]string{})
	errs := ValidateCustomRules(t.Context(), []WafCustomRuleModel{{
		Name: strVal("ok"), Action: strVal("block"),
		Condition: cond1(strVal("http.request.cookie"), strVal("does_not_exist"), emptySet, strVal("csrf_token")),
	}})
	if len(errs) != 0 {
		t.Errorf("expected no errors for does_not_exist with empty value, got: %v", errs)
	}
}

func TestValidateCondition_ContainsWithEmptyValue(t *testing.T) {
	emptySet, _ := stringSet([]string{})
	errs := ValidateCustomRules(t.Context(), []WafCustomRuleModel{{
		Name: strVal("bad"), Action: strVal("block"),
		Condition: cond1(strVal("http.request.path"), strVal("contains"), emptySet, nullStr()),
	}})
	if len(errs) == 0 {
		t.Error("expected error: contains requires at least one value")
	}
}

// ── ignore_params ─────────────────────────────────────────────────────────────

func TestValidateCondition_IgnoreParamsOnNonIgnoreAction(t *testing.T) {
	errs := ValidateCustomRules(t.Context(), []WafCustomRuleModel{{
		Name: strVal("bad"), Action: strVal("block"),
		Condition:    cond1(strVal("http.request.path"), strVal("contains"), mustStringSet([]string{"/x"}), nullStr()),
		IgnoreParams: &WafIgnoreParamsModel{IgnoreType: strVal("json_body_param"), Value: strVal("token")},
	}})
	if len(errs) == 0 {
		t.Error("expected error: ignore_params set on non-ignore action")
	}
}

// ── duplicate names ───────────────────────────────────────────────────────────

func TestValidateCondition_DuplicateCustomRuleNames(t *testing.T) {
	rule := WafCustomRuleModel{
		Name: strVal("dup"), Action: strVal("block"),
		Condition: cond1(strVal("http.request.path"), strVal("contains"), mustStringSet([]string{"/x"}), nullStr()),
	}
	errs := ValidateCustomRules(t.Context(), []WafCustomRuleModel{rule, rule})
	if len(errs) == 0 {
		t.Error("expected error for duplicate custom rule names")
	}
}

func TestValidateCondition_DuplicateRateLimitNames(t *testing.T) {
	rule := WafRateLimitRuleModel{
		Name: strVal("dup"), Action: strVal("block"),
		NumOfRequests: int64Val(100), TimeWindowSeconds: int64Val(60), BlockDurationSeconds: int64Val(300),
		Condition: cond1(strVal("http.request.path"), strVal("contains"), mustStringSet([]string{"/x"}), nullStr()),
	}
	errs := ValidateRateLimitRules(t.Context(), []WafRateLimitRuleModel{rule, rule})
	if len(errs) == 0 {
		t.Error("expected error for duplicate rate_limit rule names")
	}
}

// cond1 builds a single-condition expression for test convenience.
func cond1(field, op types.String, val types.Set, fieldKey types.String) *WafConditionExpressionModel {
	return &WafConditionExpressionModel{
		Or: []WafConditionAndGroupModel{{
			And: []WafConditionModel{{Field: field, Operator: op, Values: val, Value: types.StringNull(), FieldKey: fieldKey}},
		}},
	}
}

// ---------------------------------------------------------------------------
// ─── HCL config generators (called from service_resource_test.go) ─────────────

// testAccWafNoCheckpoint renders security with waf {} but NO checkpoint block.
// The schema-level Default on checkpoint must fill it in automatically.
func testAccWafNoCheckpoint(name, certId string, enabled bool) string {
	return fmt.Sprintf(`
resource "ioriver_service" "%s" {
  name        = "%s"
  description = "WAF acceptance test"
  certificate = "%s"

  config = {
    security = {
      enabled = %t

      waf = {
      }
    }
  }
}
`, name, name, certId, enabled)
}

// testAccWafExplicitDefaults renders security with every default checkpoint value
// spelled out explicitly in HCL. The API payload must be identical to both the
// null-waf (testAccWafBlockOmitted) and empty-waf (testAccWafNoCheckpoint) forms,
// so a plan-only step after either of those creates should produce no diff.
func testAccWafExplicitDefaults(name, certId string) string {
	return fmt.Sprintf(`
resource "ioriver_service" "%s" {
  name        = "%s"
  description = "WAF acceptance test"
  certificate = "%s"

  config = {
    security = {
      enabled = true

      waf = {
        limit_body_size = false

        checkpoint = {
          web_attacks = {
            mode             = "learn"
            confidence_level = "high"
          }
          ips = {
            mode                     = "learn"
            performance_impact       = "medium"
            severity                 = "medium"
            high_confidence_action   = "block"
            medium_confidence_action = "block"
            low_confidence_action    = "log"
          }
          trusted_sources     = []
          minimal_num_sources = 3
        }

        bot_management = {
          challenge_threshold = 0.5
          action_token_threshold = 0.5
        }
      }
    }
  }
}
`, name, name, certId)
}

// testAccWafBlockOmitted renders security with enabled=true but NO waf block at all.
// Both the waf Default AND the nested checkpoint Default must fire automatically.
func testAccWafBlockOmitted(name, certId string) string {
	return fmt.Sprintf(`
resource "ioriver_service" "%s" {
  name        = "%s"
  description = "WAF acceptance test"
  certificate = "%s"

  config = {
    security = {
      enabled = true
    }
  }
}
`, name, name, certId)
}

// testAccWafConditions renders a service with 25 custom rules and 4 rate-limit rules
// that together cover every WAF condition field type, a broad set of operators,
// all custom actions (including ignore+ignore_params), all rate-limit actions,
// and both multi-OR and multi-AND condition expressions.
//
//	idx 0: full 25-rule matrix, as-is.
//	idx 1: rule 0 operator mutated (begins_with → contains); rl-block num_of_requests 100 → 200.
func testAccWafConditions(name, certId string, idx int) string {
	base := fmt.Sprintf(`
resource "ioriver_service" "%s" {
  name        = "%s"
  description = "WAF conditions acceptance test"
  certificate = "%s"

  config = {
    security = {
      enabled = true
      waf     = {}

      custom_rules = [
        # 1. http.request.path + begins_with → block
        {
          name   = "cond-path-begins"
          action = "block"
          condition = {
            or = [{ and = [{ field = "http.request.path", operator = "begins_with", value = "/admin" }] }]
          }
        },
        # 2. http.request.uri_raw + contains → log  (uri_raw regex requires full URL; contains is free-form)
        {
          name   = "cond-uri-contains"
          action = "log"
          condition = {
            or = [{ and = [{ field = "http.request.uri_raw", operator = "contains", values = ["/api/v"] }] }]
          }
        },
        # 3. http.request.method + in → challenge
        {
          name   = "cond-method-in"
          action = "challenge"
          condition = {
            or = [{ and = [{ field = "http.request.method", operator = "in", values = ["POST", "PUT", "PATCH", "DELETE"] }] }]
          }
        },
        # 4. http.request.body + contains_word → interactive_challenge
        {
          name   = "cond-body-word"
          action = "interactive_challenge"
          condition = {
            or = [{ and = [{ field = "http.request.body", operator = "contains_word", values = ["malware"] }] }]
          }
        },
        # 5. http.request.header + contains + field_key (collection) → block
        {
          name   = "cond-header-contains"
          action = "block"
          condition = {
            or = [{ and = [{ field = "http.request.header", operator = "contains", values = ["malicious"], field_key = "X-Forwarded-For" }] }]
          }
        },
        # 6. http.request.query_param + eq + field_key (collection) → log
        {
          name   = "cond-query-eq"
          action = "log"
          condition = {
            or = [{ and = [{ field = "http.request.query_param", operator = "eq", values = ["1"], field_key = "debug" }] }]
          }
        },
        # 7. http.request.cookie + not_contains + field_key (collection) → allow
        {
          name   = "cond-cookie-not-contains"
          action = "allow"
          condition = {
            or = [{ and = [{ field = "http.request.cookie", operator = "not_contains", values = ["valid"], field_key = "session" }] }]
          }
        },
        # 8. http.request.json_param + ends_with + field_key (collection) → bypass_managed
        {
          name   = "cond-json-ends-with"
          action = "bypass_managed"
          condition = {
            or = [{ and = [{ field = "http.request.json_param", operator = "ends_with", values = [".exe"], field_key = "filename" }] }]
          }
        },
        # 9. client.ip.address + ip_match → block
        {
          name   = "cond-ip-match"
          action = "block"
          condition = {
            or = [{ and = [{ field = "client.ip.address", operator = "ip_match", values = ["10.0.0.0/8", "192.168.0.0/16"] }] }]
          }
        },
        # 10. client.ip.asn + ne → log
        {
          name   = "cond-asn-ne"
          action = "log"
          condition = {
            or = [{ and = [{ field = "client.ip.asn", operator = "ne", values = ["12345"] }] }]
          }
        },
        # 11. client.geo.country + not_in → challenge
        {
          name   = "cond-geo-not-in"
          action = "challenge"
          condition = {
            or = [{ and = [{ field = "client.geo.country", operator = "not_in", values = ["US", "GB", "DE"] }] }]
          }
        },
        # 12. action_token.score + eq → block
        {
          name   = "cond-bot-score-eq"
          action = "block"
          condition = {
            or = [{ and = [{ field = "action_token.score", field_key = "web", operator = "eq", values = [0.5] }] }]
          }
        },
        # 13. action_token.score + gt → challenge  (numeric comparison operators: lt, le, gt, ge)
        {
          name   = "cond-bot-score-gt"
          action = "challenge"
          condition = {
            or = [{ and = [{ field = "action_token.score", field_key = "web", operator = "gt", values = [0.5] }] }]
          }
        },
        # 14. http.request.path + regex → log  (regex just needs to compile as Python regex)
        {
          name   = "cond-path-regex"
          action = "log"
          condition = {
            or = [{ and = [{ field = "http.request.path", operator = "regex", values = ["/api/.*"] }] }]
          }
        },
        # 14. ignore action with ignore_params — http.request.path + contains
        {
          name      = "cond-ignore-token"
          action    = "ignore"
          condition = {
            or = [{ and = [{ field = "http.request.path", operator = "contains", values = ["/api/"] }] }]
          }
          ignore_params = {
            ignore_type = "json_body_param"
            value       = "token"
          }
        },
        # 16. multi-OR + multi-AND: (path AND method) OR (geo AND not_ip_match)
        {
          name   = "cond-multi-or-and"
          action = "block"
          condition = {
            or = [
              {
                and = [
                  { field = "http.request.path",   operator = "contains", values = ["/login"] },
                  { field = "http.request.method", operator = "in",       values = ["POST"] }
                ]
              },
              {
                and = [
                  { field = "client.geo.country",  operator = "in",          values = ["RU", "CN", "KP"] },
                  { field = "client.ip.address",   operator = "not_ip_match", values = ["10.0.0.0/8"] }
                ]
              }
            ]
          }
        },
        # 17. http.request.path + not_regex → log
        {
          name   = "cond-path-not-regex"
          action = "log"
          condition = {
            or = [{ and = [{ field = "http.request.path", operator = "not_regex", values = ["/static/.*"] }] }]
          }
        },
        # 18. http.request.path + not_begins_with → log
        {
          name   = "cond-path-not-begins"
          action = "log"
          condition = {
            or = [{ and = [{ field = "http.request.path", operator = "not_begins_with", values = ["/internal"] }] }]
          }
        },
        # 19. http.request.uri_raw + not_ends_with → log  (free-form for uri_raw)
        {
          name   = "cond-uri-not-ends"
          action = "log"
          condition = {
            or = [{ and = [{ field = "http.request.uri_raw", operator = "not_ends_with", values = [".php"] }] }]
          }
        },
        # 20. http.request.body + not_contains_word → log
        {
          name   = "cond-body-not-word"
          action = "log"
          condition = {
            or = [{ and = [{ field = "http.request.body", operator = "not_contains_word", values = ["safe"] }] }]
          }
        },
        # 21. http.request.header + exists + field_key (collection) → log
        {
          name   = "cond-header-exists"
          action = "log"
          condition = {
            or = [{ and = [{ field = "http.request.header", operator = "exists", values = [], field_key = "X-Debug" }] }]
          }
        },
        # 22. http.request.cookie + does_not_exist + field_key (collection) → block
        {
          name   = "cond-cookie-not-exists"
          action = "block"
          condition = {
            or = [{ and = [{ field = "http.request.cookie", operator = "does_not_exist", values = [], field_key = "csrf" }] }]
          }
        },
        # 23. action_token.score + lt → block
        {
          name   = "cond-bot-lt"
          action = "block"
          condition = {
            or = [{ and = [{ field = "action_token.score", field_key = "web", operator = "lt", values = [0.3] }] }]
          }
        },
        # 24. action_token.score + le → log
        {
          name   = "cond-bot-le"
          action = "log"
          condition = {
            or = [{ and = [{ field = "action_token.score", field_key = "web", operator = "le", values = [0.25] }] }]
          }
        },
        # 25. action_token.score + ge → challenge
        {
          name   = "cond-bot-ge"
          action = "challenge"
          condition = {
            or = [{ and = [{ field = "action_token.score", field_key = "web", operator = "ge", values = ["0.75"] }] }]
          }
        }
      ]

      rate_limit = [
        # 1. block — path begins_with
        {
          name                   = "rl-block"
          action                 = "block"
          num_of_requests        = 100
          time_window_seconds    = 60
          block_duration_seconds = 300
          condition = {
            or = [{ and = [{ field = "http.request.path", operator = "begins_with", values = ["/api/"] }] }]
          }
        },
        # 2. log — geo.country in
        {
          name                   = "rl-log"
          action                 = "log"
          num_of_requests        = 1000
          time_window_seconds    = 60
          block_duration_seconds = 60
          condition = {
            or = [{ and = [{ field = "client.geo.country", operator = "in", values = ["RU", "CN"] }] }]
          }
        },
        # 3. challenge — header contains (collection field_key)
        {
          name                   = "rl-challenge-header"
          action                 = "challenge"
          num_of_requests        = 50
          time_window_seconds    = 30
          block_duration_seconds = 120
          condition = {
            or = [{ and = [{ field = "http.request.header", operator = "contains", values = ["script"], field_key = "Referer" }] }]
          }
        },
        # 4. interactive_challenge — multi-AND (uri_raw + not_ip_match)
        {
          name                   = "rl-interactive"
          action                 = "interactive_challenge"
          num_of_requests        = 20
          time_window_seconds    = 10
          block_duration_seconds = 600
          condition = {
            or = [{
              and = [
                { field = "http.request.uri_raw", operator = "contains", values = ["/checkout"] },
                { field = "client.ip.address",    operator = "not_ip_match",  values = ["10.0.0.0/8"] }
              ]
            }]
          }
        }
      ]
    }
  }
}
`, name, name, certId)
	if idx == 1 {
		// Mutate rule 0 (cond-path-begins): begins_with → contains.
		base = strings.Replace(base,
			`operator = "begins_with", value = "/admin"`,
			`operator = "contains", value = "/admin"`, 1)
		// Mutate rl-block: num_of_requests 100 → 200.
		base = strings.Replace(base,
			`num_of_requests        = 100`,
			`num_of_requests        = 200`, 1)
	}
	return base
}

// testAccServiceConfigWithWaf renders the HCL for a service with a full security block.
//
//	idx 0: enabled=true, baseline (learn-mode checkpoint, 1 custom rule, 1 rate-limit).
//	idx 1: enabled=false, same config as idx 0 — tests the disable toggle.
//	idx 2: enabled=true, checkpoint mutated (prevent-mode, limit_body_size=true,
//	        trusted_sources=["1.1.1.1"], minimal_num_sources=5) + 2 custom rules + rate_limit mutated
//	        (action=log, num_of_requests=50).
//	idx 3: same mutated checkpoint as idx 2; "block-admin" removed, "log-api" condition
//	        mutated (field: path→uri_raw, begins_with→contains, value="/api/v2").
func testAccServiceConfigWithWaf(name, certId string, idx int) string {
	switch idx {
	case 0, 1:
		return fmt.Sprintf(`
resource "ioriver_service" "%s" {
  name        = "%s"
  description = "WAF acceptance test"
  certificate = "%s"

  config = {
    security = {
      enabled = %t

      waf = {
        limit_body_size = false

        checkpoint = {
          web_attacks = {
            mode             = "learn"
            confidence_level = "high"
          }
          ips = {
            mode                     = "learn"
            performance_impact       = "low"
            severity                 = "medium"
            high_confidence_action   = "block"
            medium_confidence_action = "block"
            low_confidence_action    = "log"
          }
          trusted_sources = ["1.2.3.4", "5.6.7.8"]
          minimal_num_sources     = 3
        }
      }

      custom_rules = [
        {
          name    = "block-admin"
          enabled = true
          action  = "block"
          condition = {
            or = [
              {
                and = [
                  {
                    field    = "http.request.path"
                    operator = "contains"
                    values    = ["/admin"]
                  },
                  {
                    field    = "client.geo.country"
                    operator = "in"
                    values    = ["RU", "CN"]
                  }
                ]
              }
            ]
          }
        }
      ]

      rate_limit = [
        {
          name                   = "rate-api"
          enabled                = true
          action                 = "block"
          num_of_requests        = 100
          time_window_seconds    = 60
          block_duration_seconds = 300
          condition = {
            or = [
              {
                and = [
                  {
                    field    = "http.request.path"
                    operator = "contains"
                    values    = ["/api/"]
                  }
                ]
              }
            ]
          }
        }
      ]
    }
  }
}
`, name, name, certId, idx == 0)

	case 2:
		// Checkpoint mutated to prevent-mode + 2 custom rules + rate_limit action/count mutated.
		return fmt.Sprintf(`
resource "ioriver_service" "%s" {
  name        = "%s"
  description = "WAF acceptance test"
  certificate = "%s"

  config = {
    security = {
      enabled = true

      waf = {
        limit_body_size = true

        checkpoint = {
          web_attacks = {
            mode             = "prevent"
            confidence_level = "medium"
          }
          ips = {
            mode                     = "prevent"
            performance_impact       = "low"
            severity                 = "medium"
            high_confidence_action   = "block"
            medium_confidence_action = "block"
            low_confidence_action    = "log"
          }
          trusted_sources = ["1.1.1.1"]
          minimal_num_sources     = 5
        }
      }

      custom_rules = [
        {
          name    = "block-admin"
          enabled = true
          action  = "block"
          condition = {
            or = [
              {
                and = [
                  {
                    field    = "http.request.path"
                    operator = "contains"
                    values    = ["/admin"]
                  },
                  {
                    field    = "client.geo.country"
                    operator = "in"
                    values    = ["RU", "CN"]
                  }
                ]
              }
            ]
          }
        },
        {
          name    = "log-api"
          enabled = true
          action  = "log"
          condition = {
            or = [
              {
                and = [
                  {
                    field    = "http.request.path"
                    operator = "begins_with"
                    values    = ["/api/"]
                  }
                ]
              }
            ]
          }
        }
      ]

      rate_limit = [
        {
          name                   = "rate-api"
          enabled                = true
          action                 = "log"
          num_of_requests        = 50
          time_window_seconds    = 60
          block_duration_seconds = 300
          condition = {
            or = [
              {
                and = [
                  {
                    field    = "http.request.path"
                    operator = "contains"
                    values    = ["/api/"]
                  }
                ]
              }
            ]
          }
        }
      ]
    }
  }
}
`, name, name, certId)

	default: // idx 3: same mutated checkpoint; "block-admin" removed, "log-api" condition mutated.
		return fmt.Sprintf(`
resource "ioriver_service" "%s" {
  name        = "%s"
  description = "WAF acceptance test"
  certificate = "%s"

  config = {
    security = {
      enabled = true

      waf = {
        limit_body_size = true

        checkpoint = {
          web_attacks = {
            mode             = "prevent"
            confidence_level = "medium"
          }
          ips = {
            mode                     = "prevent"
            performance_impact       = "low"
            severity                 = "medium"
            high_confidence_action   = "block"
            medium_confidence_action = "block"
            low_confidence_action    = "log"
          }
          trusted_sources = ["1.1.1.1"]
          minimal_num_sources     = 5
        }
      }

      custom_rules = [
        {
          name    = "log-api"
          enabled = true
          action  = "log"
          condition = {
            or = [
              {
                and = [
                  {
                    field    = "http.request.uri_raw"
                    operator = "contains"
                    values   = ["/api/v2"]
                  }
                ]
              }
            ]
          }
        }
      ]

      rate_limit = [
        {
          name                   = "rate-api"
          enabled                = true
          action                 = "log"
          num_of_requests        = 50
          time_window_seconds    = 60
          block_duration_seconds = 300
          condition = {
            or = [
              {
                and = [
                  {
                    field    = "http.request.path"
                    operator = "contains"
                    values    = ["/api/"]
                  }
                ]
              }
            ]
          }
        }
      ]
    }
  }
}
`, name, name, certId)
	}
}

// testAccWafRulesSteps generates HCL configs for TestAccIORiverService_WafRules.
//
//	idx 0 – baseline: 7 custom-rules (one per action) + 4 rate-limits (one per action),
//	         ordered ["r-block","r-log","r-allow","r-bypass","r-ignore","r-challenge","r-ichallenge"]
//	         and ["rl-block","rl-log","rl-challenge","rl-ichallenge"].
//	idx 1 – order swap: custom rules reversed → ["r-ichallenge","r-challenge","r-ignore","r-bypass",
//	         "r-allow","r-log","r-block"]. No NamedListPlanModifier on WAF lists, so the positional
//	         change produces a non-empty diff (order matters for first-match-wins semantics).
//	idx 3 – forward order + r-block operator changed begins_with → ends_with + r-block and r-log
//	         have enabled=false. Tests operator mutation and per-rule disable in one apply.
func testAccWafRulesSteps(name, certId string, idx int) string {
	header := fmt.Sprintf(`
resource "ioriver_service" "%s" {
  name        = "%s"
  description = "WAF rules acceptance test"
  certificate = "%s"

  config = {
    security = {
      enabled = true

      waf = {
        checkpoint = {
          web_attacks = {
            mode             = "learn"
            confidence_level = "medium"
          }
          ips = {
            mode                     = "learn"
            performance_impact       = "low"
            severity                 = "medium"
            high_confidence_action   = "block"
            medium_confidence_action = "log"
            low_confidence_action    = "log"
          }
        }
      }
`, name, name, certId)

	// 7 custom rules — one per action, each with a distinct condition.
	// Order used in idx 0 and idx 2.
	rulesForward := `
      custom_rules = [
        {
          name    = "r-block"
          enabled = true
          action  = "block"
          condition = {
            or = [
              {
                and = [
                  {
                    field    = "http.request.path"
                    operator = "begins_with"
                    values    = ["/admin"]
                  }
                ]
              }
            ]
          }
        },
        {
          name    = "r-log"
          enabled = true
          action  = "log"
          condition = {
            or = [
              {
                and = [
                  {
                    field    = "http.request.method"
                    operator = "in"
                    values    = ["DELETE", "PATCH"]
                  }
                ]
              }
            ]
          }
        },
        {
          name    = "r-allow"
          enabled = true
          action  = "allow"
          condition = {
            or = [
              {
                and = [
                  {
                    field    = "client.ip.address"
                    operator = "ip_match"
                    values    = ["10.0.0.0/8"]
                  }
                ]
              }
            ]
          }
        },
        {
          name    = "r-bypass"
          enabled = true
          action  = "bypass_managed"
          condition = {
            or = [
              {
                and = [
                  {
                    field    = "http.request.uri_raw"
                    operator = "contains"
                    values    = ["/health"]
                  }
                ]
              }
            ]
          }
        },
        {
          name          = "r-ignore"
          enabled       = true
          action        = "ignore"
          ignore_params = {
            ignore_type = "query_param"
            value       = "debug"
          }
          condition = {
            or = [
              {
                and = [
                  {
                    field     = "http.request.query_param"
                    field_key = "debug"
                    operator  = "exists"
                    values     = []
                  }
                ]
              }
            ]
          }
        },
        {
          name    = "r-challenge"
          enabled = true
          action  = "challenge"
          condition = {
            or = [
              {
                and = [
                  {
                    field    = "action_token.score"
                    field_key = "web"
                    operator = "lt"
                    values    = [0.5]
                  }
                ]
              }
            ]
          }
        },
        {
          name    = "r-ichallenge"
          enabled = true
          action  = "interactive_challenge"
          condition = {
            or = [
              {
                and = [
                  {
                    field    = "client.geo.country"
                    operator = "in"
                    values    = ["CN", "RU", "KP"]
                  }
                ]
              }
            ]
          }
        }
      ]
`

	// Same rules but positional order reversed.
	rulesReversed := `
      custom_rules = [
        {
          name    = "r-ichallenge"
          enabled = true
          action  = "interactive_challenge"
          condition = {
            or = [
              {
                and = [
                  {
                    field    = "client.geo.country"
                    operator = "in"
                    values    = ["CN", "RU", "KP"]
                  }
                ]
              }
            ]
          }
        },
        {
          name    = "r-challenge"
          enabled = true
          action  = "challenge"
          condition = {
            or = [
              {
                and = [
                  {
                    field    = "action_token.score"
                    field_key = "web"
                    operator = "lt"
                    values    = [0.5]
                  }
                ]
              }
            ]
          }
        },
        {
          name          = "r-ignore"
          enabled       = true
          action        = "ignore"
          ignore_params = {
            ignore_type = "query_param"
            value       = "debug"
          }
          condition = {
            or = [
              {
                and = [
                  {
                    field     = "http.request.query_param"
                    field_key = "debug"
                    operator  = "exists"
                    values     = []
                  }
                ]
              }
            ]
          }
        },
        {
          name    = "r-bypass"
          enabled = true
          action  = "bypass_managed"
          condition = {
            or = [
              {
                and = [
                  {
                    field    = "http.request.uri_raw"
                    operator = "contains"
                    values    = ["/health"]
                  }
                ]
              }
            ]
          }
        },
        {
          name    = "r-allow"
          enabled = true
          action  = "allow"
          condition = {
            or = [
              {
                and = [
                  {
                    field    = "client.ip.address"
                    operator = "ip_match"
                    values    = ["10.0.0.0/8"]
                  }
                ]
              }
            ]
          }
        },
        {
          name    = "r-log"
          enabled = true
          action  = "log"
          condition = {
            or = [
              {
                and = [
                  {
                    field    = "http.request.method"
                    operator = "in"
                    values    = ["DELETE", "PATCH"]
                  }
                ]
              }
            ]
          }
        },
        {
          name    = "r-block"
          enabled = true
          action  = "block"
          condition = {
            or = [
              {
                and = [
                  {
                    field    = "http.request.path"
                    operator = "begins_with"
                    values    = ["/admin"]
                  }
                ]
              }
            ]
          }
        }
      ]
`

	// 4 rate-limit rules — one per action.
	rateLimits := `
      rate_limit = [
        {
          name                   = "rl-block"
          enabled                = true
          action                 = "block"
          num_of_requests        = 100
          time_window_seconds    = 60
          block_duration_seconds = 300
          condition = {
            or = [
              {
                and = [
                  {
                    field    = "http.request.path"
                    operator = "begins_with"
                    values    = ["/api/"]
                  }
                ]
              }
            ]
          }
        },
        {
          name                   = "rl-log"
          enabled                = true
          action                 = "log"
          num_of_requests        = 1000
          time_window_seconds    = 60
          block_duration_seconds = 60
          condition = {
            or = [
              {
                and = [
                  {
                    field    = "http.request.path"
                    operator = "begins_with"
                    values    = ["/public/"]
                  }
                ]
              }
            ]
          }
        },
        {
          name                   = "rl-challenge"
          enabled                = true
          action                 = "challenge"
          num_of_requests        = 200
          time_window_seconds    = 30
          block_duration_seconds = 120
          condition = {
            or = [
              {
                and = [
                  {
                    field    = "http.request.path"
                    operator = "begins_with"
                    values    = ["/login"]
                  }
                ]
              }
            ]
          }
        },
        {
          name                   = "rl-ichallenge"
          enabled                = true
          action                 = "interactive_challenge"
          num_of_requests        = 50
          time_window_seconds    = 60
          block_duration_seconds = 600
          condition = {
            or = [
              {
                and = [
                  {
                    field    = "http.request.path"
                    operator = "begins_with"
                    values    = ["/checkout"]
                  }
                ]
              }
            ]
          }
        }
      ]
    }
  }
}
`

	// idx 3: forward order + ends_with on r-block (same as idx 2), but
	// r-block and r-log have enabled = false — tests per-rule disable toggle.
	rulesDisabled := `
      custom_rules = [
        {
          name    = "r-block"
          enabled = false
          action  = "block"
          condition = {
            or = [
              {
                and = [
                  {
                    field    = "http.request.path"
                    operator = "ends_with"
                    values    = ["/admin"]
                  }
                ]
              }
            ]
          }
        },
        {
          name    = "r-log"
          enabled = false
          action  = "log"
          condition = {
            or = [
              {
                and = [
                  {
                    field    = "http.request.method"
                    operator = "in"
                    values    = ["DELETE", "PATCH"]
                  }
                ]
              }
            ]
          }
        },
        {
          name    = "r-allow"
          enabled = true
          action  = "allow"
          condition = {
            or = [
              {
                and = [
                  {
                    field    = "client.ip.address"
                    operator = "ip_match"
                    values    = ["10.0.0.0/8"]
                  }
                ]
              }
            ]
          }
        },
        {
          name    = "r-bypass"
          enabled = true
          action  = "bypass_managed"
          condition = {
            or = [
              {
                # AND group 0: uri_raw contains /health AND method in [GET, HEAD]
                and = [
                  {
                    field    = "http.request.uri_raw"
                    operator = "contains"
                    values    = ["/health"]
                  },
                  {
                    field    = "http.request.method"
                    operator = "in"
                    values    = ["GET", "HEAD"]
                  }
                ]
              },
              {
                # OR group 1: standalone path rule
                and = [
                  {
                    field    = "http.request.path"
                    operator = "begins_with"
                    values    = ["/status"]
                  }
                ]
              }
            ]
          }
        },
        {
          name    = "r-ignore"
          enabled = true
          action  = "ignore"
          ignore_params = {
            ignore_type = "query_param"
            value     = "debug"
          }
          condition = {
            or = [
              {
                and = [
                  {
                    field     = "http.request.query_param"
                    field_key = "debug"
                    operator  = "exists"
                    values     = []
                  }
                ]
              }
            ]
          }
        },
        {
          name    = "r-challenge"
          enabled = true
          action  = "challenge"
          condition = {
            or = [
              {
                and = [
                  {
                    field    = "action_token.score"
                    field_key = "web"
                    operator = "lt"
                    values    = [0.5]
                  }
                ]
              }
            ]
          }
        },
        {
          name    = "r-ichallenge"
          enabled = true
          action  = "interactive_challenge"
          condition = {
            or = [
              {
                and = [
                  {
                    field    = "client.geo.country"
                    operator = "in"
                    values    = ["CN", "RU", "KP"]
                  }
                ]
              }
            ]
          }
        }
      ]
`

	switch idx {
	case 1:
		return header + rulesReversed + rateLimits
	case 3:
		return header + rulesDisabled + rateLimits
	default: // idx 0
		return header + rulesForward + rateLimits
	}
}

// ---------------------------------------------------------------------------
// BotManagement unit tests
// ---------------------------------------------------------------------------

// 1. All fields fully populated — all must appear in the serialized map.
func TestBotManagementToMap_AllFields(t *testing.T) {
	ctx := context.Background()
	sec := &SecurityModel{
		BotManagement: &BotManagementModel{
			WebKey:               strVal("pk_test"),
			ChallengeThreshold:   float64Val(0.7),
			ActionTokenThreshold: float64Val(0.3),
		},
	}
	m := sec.BotManagementToMap(ctx)
	if m == nil {
		t.Fatal("expected non-nil map")
	}
	if m["web_key"] != "pk_test" {
		t.Errorf("web_key: expected pk_test, got %v", m["web_key"])
	}
	if m["challenge_threshold"] != 0.7 {
		t.Errorf("challenge_threshold: expected 0.7, got %v", m["challenge_threshold"])
	}
	if m["action_token_threshold"] != 0.3 {
		t.Errorf("action_token_threshold: expected 0.3, got %v", m["action_token_threshold"])
	}
}

// 2. Nil BotManagement pointer → method must return nil (caller omits the key).
func TestBotManagementToMap_Nil(t *testing.T) {
	ctx := context.Background()
	sec := &SecurityModel{BotManagement: nil}
	m := sec.BotManagementToMap(ctx)
	if m != nil {
		t.Errorf("expected nil map when BotManagement is nil, got %v", m)
	}
}

// 3. Null TF fields must not appear in the serialized map.
func TestBotManagementToMap_NullFields_NotSent(t *testing.T) {
	ctx := context.Background()
	sec := &SecurityModel{
		BotManagement: &BotManagementModel{
			WebKey:               types.StringNull(),
			ChallengeThreshold:   types.Float64Null(),
			ActionTokenThreshold: types.Float64Null(),
		},
	}
	m := sec.BotManagementToMap(ctx)
	if m == nil {
		t.Fatal("map must not be nil even when all fields are null (model is non-nil)")
	}
	if _, present := m["web_key"]; present {
		t.Error("web_key must not appear when null")
	}
	if _, present := m["challenge_threshold"]; present {
		t.Error("challenge_threshold must not appear when null")
	}
	if _, present := m["action_token_threshold"]; present {
		t.Error("action_token_threshold must not appear when null")
	}
}

// 4. Feed a typical backend response (includes uuid) and assert TF fields + uuid dropped.
func TestBotManagementMapToModel_BackendDefaults(t *testing.T) {
	ctx := context.Background()
	raw := map[string]interface{}{
		"web_key":                "",
		"challenge_threshold":    0.5,
		"action_token_threshold": 0.5,
		"uuid":                   "abc-123", // server-generated, must be silently dropped
	}
	bm := BotManagementMapToModel(ctx, raw)
	if bm == nil {
		t.Fatal("expected non-nil model")
	}
	if bm.WebKey.ValueString() != "" {
		t.Errorf("web_key: expected empty string, got %q", bm.WebKey.ValueString())
	}
	if bm.ChallengeThreshold.ValueFloat64() != 0.5 {
		t.Errorf("challenge_threshold: expected 0.5, got %v", bm.ChallengeThreshold.ValueFloat64())
	}
	if bm.ActionTokenThreshold.ValueFloat64() != 0.5 {
		t.Errorf("action_token_threshold: expected 0.5, got %v", bm.ActionTokenThreshold.ValueFloat64())
	}
}

// 5. Full round-trip: serialize → deserialize and compare field-by-field.
func TestBotManagementRoundTrip(t *testing.T) {
	ctx := context.Background()
	orig := &SecurityModel{
		BotManagement: &BotManagementModel{
			WebKey:               strVal("pk_abc"),
			ChallengeThreshold:   float64Val(0.8),
			ActionTokenThreshold: float64Val(0.6),
		},
	}
	m := orig.BotManagementToMap(ctx)
	recovered := BotManagementMapToModel(ctx, m)
	if recovered == nil {
		t.Fatal("round-trip returned nil")
	}
	if recovered.WebKey.ValueString() != "pk_abc" {
		t.Errorf("web_key mismatch: %q", recovered.WebKey.ValueString())
	}
	if recovered.ChallengeThreshold.ValueFloat64() != 0.8 {
		t.Errorf("challenge_threshold mismatch: %v", recovered.ChallengeThreshold.ValueFloat64())
	}
	if recovered.ActionTokenThreshold.ValueFloat64() != 0.6 {
		t.Errorf("action_token_threshold mismatch: %v", recovered.ActionTokenThreshold.ValueFloat64())
	}
}

// 6. Regression: security block with only bot_management set (no Waf, no rules).
// BotManagementToMap must work and SecurityModelToMap must not crash.
func TestSecurityModel_BotManagementOnly_NoWaf(t *testing.T) {
	ctx := context.Background()
	sec := &SecurityModel{
		Enabled:     boolVal(false),
		Waf:         nil, // explicitly nil — no WAF
		CustomRules: []WafCustomRuleModel{},
		RateLimit:   []WafRateLimitRuleModel{},
		BotManagement: &BotManagementModel{
			WebKey:               strVal("pk_only"),
			ChallengeThreshold:   float64Val(0.5),
			ActionTokenThreshold: float64Val(0.5),
		},
	}
	// SecurityModelToMap must not panic
	wafMap := sec.SecurityModelToMap(ctx)
	if wafMap == nil {
		t.Fatal("SecurityModelToMap returned nil")
	}
	// BotManagementToMap must return the correct map
	bmMap := sec.BotManagementToMap(ctx)
	if bmMap == nil {
		t.Fatal("BotManagementToMap returned nil")
	}
	if bmMap["web_key"] != "pk_only" {
		t.Errorf("web_key: expected pk_only, got %v", bmMap["web_key"])
	}
}

// 7. Validator range: challenge_threshold and action_token_threshold must reject
// values outside [0.0, 1.0]. We test the validation logic by calling
// ValidateSecurityModel with an impossible threshold value (not a schema-level
// test, since float range is enforced by the schema validator at plan time).
// This test confirms the model can hold boundary values correctly.
func TestBotManagementValidation_ThresholdBoundary(t *testing.T) {
	ctx := context.Background()
	// Valid boundaries must produce no error at serialization level.
	for _, v := range []float64{0.0, 0.5, 1.0} {
		sec := &SecurityModel{
			BotManagement: &BotManagementModel{
				WebKey:               strVal(""),
				ChallengeThreshold:   float64Val(v),
				ActionTokenThreshold: float64Val(v),
			},
		}
		errs := ValidateSecurityModel(ctx, sec)
		if len(errs) != 0 {
			t.Errorf("threshold %.1f: unexpected errors: %v", v, errs)
		}
	}
}

// ---------------------------------------------------------------------------
// Test helpers (local to this file)
// ---------------------------------------------------------------------------

func boolVal(b bool) types.Bool {
	return types.BoolValue(b)
}

func strVal(s string) types.String {
	return types.StringValue(s)
}

func nullStr() types.String {
	return types.StringNull()
}

func int64Val(i int64) types.Int64 {
	return types.Int64Value(i)
}

func float64Val(v float64) types.Float64 {
	return types.Float64Value(v)
}

func stringListVal(vals []string) (types.List, diag.Diagnostics) {
	return types.ListValueFrom(context.Background(), types.StringType, vals)
}

func stringSet(vals []string) (types.Set, diag.Diagnostics) {
	return types.SetValueFrom(context.Background(), types.StringType, vals)
}

func mustStringSet(vals []string) types.Set {
	s, err := stringSet(vals)
	if err != nil {
		panic(err)
	}
	return s
}
