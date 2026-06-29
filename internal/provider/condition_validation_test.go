package provider

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

type specCase struct {
	name string
	spec *ConditionSpec
}

func allConditionSpecs() []specCase {
	return []specCase{
		{name: "behavior", spec: BehaviorConditionSpec},
		{name: "waf", spec: WafConditionSpec},
	}
}

func testSet(t *testing.T, vals []string) types.Set {
	t.Helper()
	setVal, diags := types.SetValueFrom(context.Background(), types.StringType, vals)
	if diags.HasError() {
		t.Fatalf("failed to build set value: %v", diags)
	}
	return setVal
}

func mkCond(t *testing.T, field, operator string, value *string, values []string, fieldKey *string) ConditionModel {
	t.Helper()
	cond := ConditionModel{
		Field:    types.StringValue(field),
		Operator: types.StringValue(operator),
		Value:    types.StringNull(),
		Values:   types.SetNull(types.StringType),
		FieldKey: types.StringNull(),
	}

	if value != nil {
		cond.Value = types.StringValue(*value)
	}
	if values != nil {
		cond.Values = testSet(t, values)
	}
	if fieldKey != nil {
		cond.FieldKey = types.StringValue(*fieldKey)
	}

	return cond
}

func ptr(s string) *string {
	return &s
}

func TestValidateCondition_ExistsRejectsValue(t *testing.T) {
	tests := []struct {
		name        string
		spec        *ConditionSpec
		cond        ConditionModel
		errContains string
	}{
		{
			name:        "behavior",
			spec:        BehaviorConditionSpec,
			cond:        mkCond(t, "http.request.path", "exists", ptr("/x"), nil, nil),
			errContains: "must not be given a value",
		},
		{
			name:        "waf",
			spec:        WafConditionSpec,
			cond:        mkCond(t, "http.request.header", "exists", ptr("x"), nil, ptr("X-Test")),
			errContains: "must not be given a value",
		},
	}

	for _, sc := range tests {
		errs := ValidateCondition(t.Context(), sc.cond, "test/loc/", sc.spec)
		if len(errs) == 0 || !strings.Contains(errs[0], sc.errContains) {
			t.Fatalf("%s: expected exists validation error, got %v", sc.name, errs)
		}
	}
}

func TestValidateCondition_ListOperatorRequiresValues(t *testing.T) {
	cond := mkCond(t, "http.request.method", "in", nil, nil, nil)
	for _, sc := range allConditionSpecs() {
		errs := ValidateCondition(t.Context(), cond, "", sc.spec)
		if len(errs) == 0 || !strings.Contains(errs[0], "requires a value") {
			t.Fatalf("%s: expected list operator values error, got %v", sc.name, errs)
		}
	}
}

func TestValidateCondition_ScalarRejectsMultiValues(t *testing.T) {
	cond := mkCond(t, "http.request.path", "eq", nil, []string{"/a", "/b"}, nil)
	for _, sc := range allConditionSpecs() {
		errs := ValidateCondition(t.Context(), cond, "", sc.spec)
		if len(errs) == 0 || !strings.Contains(errs[0], "accepts a single value") {
			t.Fatalf("%s: expected scalar multi-value error, got %v", sc.name, errs)
		}
	}
}

func TestValidateCondition_RejectsBothValueAndValues(t *testing.T) {
	cond := mkCond(t, "http.request.path", "eq", ptr("/x"), []string{"/x"}, nil)
	for _, sc := range allConditionSpecs() {
		errs := ValidateCondition(t.Context(), cond, "test", sc.spec)
		if len(errs) == 0 || !strings.Contains(errs[0], "cannot set both value and values") {
			t.Fatalf("%s: expected value+values conflict error, got %v", sc.name, errs)
		}
	}
}

func TestValidateCondition_BehaviorCases(t *testing.T) {
	testCases := []struct {
		name        string
		cond        ConditionModel
		wantErr     bool
		errContains string
	}{
		{
			name:    "bool parser accepts uppercase true",
			cond:    mkCond(t, "client.device.is_mobile", "eq", ptr("TRUE"), nil, nil),
			wantErr: false,
		},
		{
			name:    "bool parser accepts mixed case false",
			cond:    mkCond(t, "client.device.is_mobile", "eq", ptr("False"), nil, nil),
			wantErr: false,
		},
		{
			name:        "invalid bool",
			cond:        mkCond(t, "client.device.is_mobile", "eq", ptr("not-bool"), nil, nil),
			wantErr:     true,
			errContains: "not a valid bool",
		},
		{
			name:        "invalid int",
			cond:        mkCond(t, "http.response.status_code", "eq", ptr("abc"), nil, nil),
			wantErr:     true,
			errContains: "not a valid int",
		},
		{
			name:        "collection field requires field_key",
			cond:        mkCond(t, "http.request.header", "eq", ptr("x"), nil, nil),
			wantErr:     true,
			errContains: "requires field_key",
		},
		{
			name:        "collection field rejects empty field_key",
			cond:        mkCond(t, "http.request.header", "eq", ptr("x"), nil, ptr("")),
			wantErr:     true,
			errContains: "requires field_key",
		},
		{
			name:    "list operator with values is valid",
			cond:    mkCond(t, "http.request.method", "in", nil, []string{"GET", "POST"}, nil),
			wantErr: false,
		},
		{
			name:    "exists with field_key and empty values is valid",
			cond:    mkCond(t, "http.request.header", "exists", nil, []string{}, ptr("X-Debug")),
			wantErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			errs := ValidateCondition(t.Context(), tc.cond, "behavior.loc", BehaviorConditionSpec)
			if tc.wantErr {
				if len(errs) == 0 {
					t.Fatalf("expected error containing %q, got none", tc.errContains)
				}
				if tc.errContains != "" && !strings.Contains(errs[0], tc.errContains) {
					t.Fatalf("expected error containing %q, got %v", tc.errContains, errs)
				}
				return
			}
			if len(errs) != 0 {
				t.Fatalf("expected no errors, got %v", errs)
			}
		})
	}
}

func TestValidateCondition_WafCases(t *testing.T) {
	testCases := []struct {
		name        string
		cond        ConditionModel
		wantErr     bool
		errContains string
	}{
		{
			name:    "score lower boundary is valid",
			cond:    mkCond(t, "action_token.score", "eq", ptr("0.0"), nil, ptr("web")),
			wantErr: false,
		},
		{
			name:    "action_token score lower boundary is valid",
			cond:    mkCond(t, "action_token.score", "eq", ptr("0.0"), nil, ptr("web")),
			wantErr: false,
		},
		{
			name:    "score upper boundary is valid",
			cond:    mkCond(t, "action_token.score", "eq", ptr("1.0"), nil, ptr("web")),
			wantErr: false,
		},
		{
			name:    "bot validation result bool is valid",
			cond:    mkCond(t, "bot_validation.result", "eq", ptr("passed"), nil, nil),
			wantErr: false,
		},
		{
			name:        "score below range",
			cond:        mkCond(t, "action_token.score", "eq", ptr("-0.01"), nil, ptr("web")),
			wantErr:     true,
			errContains: "out of range",
		},
		{
			name:        "action_token score below range",
			cond:        mkCond(t, "action_token.score", "eq", ptr("-0.01"), nil, ptr("web")),
			wantErr:     true,
			errContains: "out of range",
		},
		{
			name:        "invalid uri_raw for eq",
			cond:        mkCond(t, "http.request.uri_raw", "eq", ptr("/not-full-url"), nil, nil),
			wantErr:     true,
			errContains: "requires a full URL",
		},
		{
			name:        "invalid int parse for asn",
			cond:        mkCond(t, "client.ip.asn", "eq", ptr("abc"), nil, nil),
			wantErr:     true,
			errContains: "not a valid int",
		},
		{
			name:        "score out of range",
			cond:        mkCond(t, "action_token.score", "eq", ptr("1.5"), nil, ptr("web")),
			wantErr:     true,
			errContains: "out of range",
		},
		{
			name:        "score above range",
			cond:        mkCond(t, "action_token.score", "eq", ptr("1.01"), nil, ptr("web")),
			wantErr:     true,
			errContains: "out of range",
		},
		{
			name:        "bot validation result rejects non bool",
			cond:        mkCond(t, "bot_validation.result", "eq", ptr("not-bool"), nil, nil),
			wantErr:     true,
			errContains: "must be \"passed\" or \"failed\"",
		},
		{
			name:        "uri_raw begins_with requires full URL prefix",
			cond:        mkCond(t, "http.request.uri_raw", "begins_with", ptr("/api"), nil, nil),
			wantErr:     true,
			errContains: "requires a valid URL prefix",
		},
		{
			name:        "uri_raw regex rejects invalid pattern",
			cond:        mkCond(t, "http.request.uri_raw", "regex", ptr("[broken"), nil, nil),
			wantErr:     true,
			errContains: "not a valid regex",
		},
		{
			name:        "collection field missing field_key",
			cond:        mkCond(t, "http.request.header", "contains", ptr("abc"), nil, nil),
			wantErr:     true,
			errContains: "requires field_key",
		},
		{
			name:    "ip_match with multiple CIDRs",
			cond:    mkCond(t, "client.ip.address", "ip_match", nil, []string{"10.0.0.0/8", "1.2.3.4"}, nil),
			wantErr: false,
		},
		{
			name:    "body exists is valid",
			cond:    mkCond(t, "http.request.body", "exists", nil, []string{}, nil),
			wantErr: false,
		},
		{
			name:    "body does_not_exist is valid",
			cond:    mkCond(t, "http.request.body", "does_not_exist", nil, []string{}, nil),
			wantErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			errs := ValidateCondition(t.Context(), tc.cond, "waf.loc", WafConditionSpec)
			if tc.wantErr {
				if len(errs) == 0 {
					t.Fatalf("expected error containing %q, got none", tc.errContains)
				}
				if tc.errContains != "" && !strings.Contains(errs[0], tc.errContains) {
					t.Fatalf("expected error containing %q, got %v", tc.errContains, errs)
				}
				return
			}
			if len(errs) != 0 {
				t.Fatalf("expected no errors, got %v", errs)
			}
		})
	}
}

func TestValidateCondition_FieldOpValueMatrix(t *testing.T) {
	type expectation struct {
		wantErr     bool
		errContains string
	}

	matrix := []struct {
		name        string
		cond        ConditionModel
		expectation map[string]expectation
	}{
		{
			name: "path eq valid",
			cond: mkCond(t, "http.request.path", "eq", ptr("/login"), nil, nil),
			expectation: map[string]expectation{
				"behavior": {wantErr: false},
				"waf":      {wantErr: false},
			},
		},
		{
			name: "path eq no slash invalid",
			cond: mkCond(t, "http.request.path", "eq", ptr("no-slash"), nil, nil),
			expectation: map[string]expectation{
				"behavior": {wantErr: true, errContains: "must start with '/'"},
				"waf":      {wantErr: true, errContains: "must start with '/'"},
			},
		},
		{
			name: "method lt unsupported for both",
			cond: mkCond(t, "http.request.method", "lt", ptr("GET"), nil, nil),
			expectation: map[string]expectation{
				"behavior": {wantErr: true, errContains: "is not allowed for field"},
				"waf":      {wantErr: true, errContains: "is not allowed for field"},
			},
		},
		{
			name: "unsupported operator for both",
			cond: mkCond(t, "http.request.path", "totally_unknown_op", ptr("/ok"), nil, nil),
			expectation: map[string]expectation{
				"behavior": {wantErr: true, errContains: "is not supported by"},
				"waf":      {wantErr: true, errContains: "is not supported by"},
			},
		},
		{
			name: "header contains requires field_key on both",
			cond: mkCond(t, "http.request.header", "contains", ptr("x"), nil, nil),
			expectation: map[string]expectation{
				"behavior": {wantErr: true, errContains: "is not supported by behavior"},
				"waf":      {wantErr: true, errContains: "requires field_key"},
			},
		},
		{
			name: "header exists with key valid on both",
			cond: mkCond(t, "http.request.header", "exists", nil, []string{}, ptr("X-Debug")),
			expectation: map[string]expectation{
				"behavior": {wantErr: false},
				"waf":      {wantErr: false},
			},
		},
		{
			name: "uri_raw only supported in waf",
			cond: mkCond(t, "http.request.uri_raw", "eq", ptr("https://example.com/a"), nil, nil),
			expectation: map[string]expectation{
				"behavior": {wantErr: true, errContains: "is not supported by behavior"},
				"waf":      {wantErr: false},
			},
		},
		{
			name: "client.ip only supported in behavior",
			cond: mkCond(t, "client.ip", "eq", ptr("10.0.0.1"), nil, nil),
			expectation: map[string]expectation{
				"behavior": {wantErr: false},
				"waf":      {wantErr: true, errContains: "is not supported by waf"},
			},
		},
		{
			name: "regex invalid pattern on path",
			cond: mkCond(t, "http.request.path", "regex", ptr("[broken"), nil, nil),
			expectation: map[string]expectation{
				"behavior": {wantErr: true, errContains: "not a valid regex"},
				"waf":      {wantErr: true, errContains: "not a valid regex"},
			},
		},
		{
			name: "path eq rejects wildcard",
			cond: mkCond(t, "http.request.path", "eq", ptr("/api/*"), nil, nil),
			expectation: map[string]expectation{
				"behavior": {wantErr: true, errContains: "must not contain '*'"},
				"waf":      {wantErr: true, errContains: "must not contain '*'"},
			},
		},
		{
			name: "path eq rejects invalid characters",
			cond: mkCond(t, "http.request.path", "eq", ptr("/bad|path"), nil, nil),
			expectation: map[string]expectation{
				"behavior": {wantErr: true, errContains: "contains invalid characters"},
				"waf":      {wantErr: true, errContains: "contains invalid characters"},
			},
		},
	}

	for _, row := range matrix {
		t.Run(row.name, func(t *testing.T) {
			for _, sc := range allConditionSpecs() {
				exp, ok := row.expectation[sc.name]
				if !ok {
					continue
				}

				errs := ValidateCondition(t.Context(), row.cond, "matrix.loc", sc.spec)
				if exp.wantErr {
					if len(errs) == 0 {
						t.Fatalf("%s: expected error containing %q, got none", sc.name, exp.errContains)
					}
					if exp.errContains != "" && !strings.Contains(errs[0], exp.errContains) {
						t.Fatalf("%s: expected error containing %q, got %v", sc.name, exp.errContains, errs)
					}
					continue
				}

				if len(errs) != 0 {
					t.Fatalf("%s: expected no errors, got %v", sc.name, errs)
				}
			}
		})
	}
}

func TestValidateCondition_PrecedenceAndArityBranches(t *testing.T) {
	tests := []struct {
		name          string
		spec          *ConditionSpec
		cond          ConditionModel
		wantErr       string
		notContains   string
		expectNoError bool
	}{
		{
			name:        "unsupported operator is checked before field",
			spec:        BehaviorConditionSpec,
			cond:        mkCond(t, "not.real.field", "totally_unknown_op", ptr("x"), nil, nil),
			wantErr:     `is not supported by behavior conditions`,
			notContains: `field \"not.real.field\" is not supported`,
		},
		{
			name:    "unsupported field is checked after valid operator",
			spec:    BehaviorConditionSpec,
			cond:    mkCond(t, "not.real.field", "eq", ptr("x"), nil, nil),
			wantErr: `field "not.real.field" is not supported by behavior conditions`,
		},
		{
			name:        "operator allowed check happens before field_key requirement",
			spec:        WafConditionSpec,
			cond:        mkCond(t, "http.request.header", "ip_match", nil, []string{"10.0.0.0/8"}, nil),
			wantErr:     `is not allowed for field`,
			notContains: `requires field_key to be set`,
		},
		{
			name:        "field_key check happens before value shape checks",
			spec:        BehaviorConditionSpec,
			cond:        mkCond(t, "http.request.path", "eq", ptr("/a"), []string{"/a"}, ptr("X")),
			wantErr:     `field_key must not be set`,
			notContains: `cannot set both value and values`,
		},
		{
			name:          "arity none with no value passes",
			spec:          BehaviorConditionSpec,
			cond:          mkCond(t, "http.request.path", "exists", nil, []string{}, nil),
			expectNoError: true,
		},
		{
			name:    "arity scalar rejects values with more than one element",
			spec:    BehaviorConditionSpec,
			cond:    mkCond(t, "http.request.path", "eq", nil, []string{"/a", "/b"}, nil),
			wantErr: `accepts a single value`,
		},
		{
			name:          "arity list accepts shorthand value",
			spec:          BehaviorConditionSpec,
			cond:          mkCond(t, "http.request.method", "in", ptr("GET"), nil, nil),
			expectNoError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			errs := ValidateCondition(t.Context(), tc.cond, "branch.loc", tc.spec)
			if tc.expectNoError {
				if len(errs) != 0 {
					t.Fatalf("expected no errors, got %v", errs)
				}
				return
			}

			if len(errs) == 0 {
				t.Fatalf("expected error containing %q, got none", tc.wantErr)
			}

			if tc.wantErr != "" && !strings.Contains(errs[0], tc.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tc.wantErr, errs)
			}

			if tc.notContains != "" && strings.Contains(errs[0], tc.notContains) {
				t.Fatalf("expected first error not to contain %q, got %v", tc.notContains, errs)
			}
		})
	}
}

func TestValidateElement_AllKindsAndBranches(t *testing.T) {
	rng := &struct{ Min, Max float64 }{Min: 0.0, Max: 1.0}

	longPath := "/" + strings.Repeat("a", 256)

	tests := []struct {
		name        string
		raw         string
		kind        valueKind
		rng         *struct{ Min, Max float64 }
		op          string
		wantErr     bool
		errContains string
	}{
		// kindString / kindCountry
		{name: "string accepts any text", raw: "abc", kind: kindString, op: "eq"},
		{name: "country accepts two-letter code", raw: "US", kind: kindCountry, op: "eq"},

		// kindInt
		{name: "int valid in range", raw: "1", kind: kindInt, rng: &struct{ Min, Max float64 }{Min: 0, Max: 10}, op: "eq"},
		{name: "int parse fails", raw: "abc", kind: kindInt, op: "eq", wantErr: true, errContains: "not a valid int"},
		{name: "int out of range", raw: "11", kind: kindInt, rng: &struct{ Min, Max float64 }{Min: 0, Max: 10}, op: "eq", wantErr: true, errContains: "out of range"},

		// kindFloat
		{name: "float valid in range", raw: "0.5", kind: kindFloat, rng: rng, op: "eq"},
		{name: "float parse fails", raw: "nope", kind: kindFloat, op: "eq", wantErr: true, errContains: "not a valid float"},
		{name: "float out of range", raw: "1.1", kind: kindFloat, rng: rng, op: "eq", wantErr: true, errContains: "out of range"},

		// kindBool
		{name: "bool valid", raw: "true", kind: kindBool, op: "eq"},
		{name: "bool parse fails", raw: "maybe", kind: kindBool, op: "eq", wantErr: true, errContains: "not a valid bool"},

		// kindIP
		{name: "ip plain valid", raw: "1.2.3.4", kind: kindIP, op: "eq"},
		{name: "ip cidr valid", raw: "10.0.0.0/8", kind: kindIP, op: "eq"},
		{name: "ip invalid", raw: "bad-ip", kind: kindIP, op: "eq", wantErr: true, errContains: "not a valid IP address or CIDR"},

		// kindURL (operator-sensitive)
		{name: "url eq valid full URL", raw: "https://example.com/a", kind: kindURL, op: "eq"},
		{name: "url eq invalid full URL", raw: "/a", kind: kindURL, op: "eq", wantErr: true, errContains: "requires a full URL"},
		{name: "url in invalid full URL", raw: "not-url", kind: kindURL, op: "in", wantErr: true, errContains: "requires a full URL"},
		{name: "url begins_with invalid prefix", raw: "/prefix", kind: kindURL, op: "begins_with", wantErr: true, errContains: "requires a valid URL prefix"},
		{name: "url not_begins_with invalid prefix", raw: "bad", kind: kindURL, op: "not_begins_with", wantErr: true, errContains: "requires a valid URL prefix"},
		{name: "url regex invalid", raw: "[broken", kind: kindURL, op: "regex", wantErr: true, errContains: "not a valid regex"},
		{name: "url not_regex invalid", raw: "[broken", kind: kindURL, op: "not_regex", wantErr: true, errContains: "not a valid regex"},
		{name: "url contains is free form", raw: "not a url", kind: kindURL, op: "contains"},

		// kindURLPrefix
		{name: "url_prefix valid", raw: "https://example.com/pre", kind: kindURLPrefix, op: "eq"},
		{name: "url_prefix invalid", raw: "no-prefix", kind: kindURLPrefix, op: "eq", wantErr: true, errContains: "requires a valid URL prefix"},

		// kindPath
		{name: "path regex valid", raw: "^/api/.*$", kind: kindPath, op: "regex"},
		{name: "path regex invalid", raw: "[broken", kind: kindPath, op: "regex", wantErr: true, errContains: "not a valid regex"},
		{name: "path eq missing slash", raw: "no-slash", kind: kindPath, op: "eq", wantErr: true, errContains: "must start with '/'"},
		{name: "path match missing slash", raw: "no-slash", kind: kindPath, op: "match", wantErr: true, errContains: "must start with '/'"},
		{name: "path eq too long", raw: longPath, kind: kindPath, op: "eq", wantErr: true, errContains: "exceeds 255 chars"},
		{name: "path eq wildcard forbidden", raw: "/api/*", kind: kindPath, op: "eq", wantErr: true, errContains: "must not contain '*'"},
		{name: "path ne wildcard forbidden", raw: "/api/*", kind: kindPath, op: "ne", wantErr: true, errContains: "must not contain '*'"},
		{name: "path invalid chars", raw: "/bad|path", kind: kindPath, op: "eq", wantErr: true, errContains: "contains invalid characters"},
		{name: "path default valid", raw: "/ok-path_1", kind: kindPath, op: "match"},

		// kindRegex
		{name: "regex kind valid", raw: "^ok$", kind: kindRegex, op: "eq"},
		{name: "regex kind invalid", raw: "[broken", kind: kindRegex, op: "eq", wantErr: true, errContains: "not a valid regex"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateElement(tc.raw, tc.kind, tc.rng, "elem.loc", "elem.field", tc.op)
			if tc.wantErr {
				if err == "" {
					t.Fatalf("expected error containing %q, got none", tc.errContains)
				}
				if tc.errContains != "" && !strings.Contains(err, tc.errContains) {
					t.Fatalf("expected error containing %q, got %q", tc.errContains, err)
				}
				return
			}
			if err != "" {
				t.Fatalf("expected no error, got %q", err)
			}
		})
	}
}

type conditionValidationRejectionCase struct {
	name         string
	err          string
	securityRule string
	behaviorRule string
}

func conditionValidationRejectionCases() []conditionValidationRejectionCase {
	return []conditionValidationRejectionCase{
		{name: "waf scalar rejects multi-values", err: `accepts a single value`, securityRule: `{ name = "waf-scalar-multi", action = "log", condition = { or = [{ and = [{ field = "http.request.path", operator = "eq", values = ["/a", "/b"] }] }] } }`},
		{name: "waf rejects both value and values", err: `cannot set both value and values`, securityRule: `{ name = "waf-both-value-values", action = "log", condition = { or = [{ and = [{ field = "http.request.path", operator = "eq", value = "/a", values = ["/a"] }] }] } }`},
		{name: "waf list op requires values", err: `requires a value`, securityRule: `{ name = "waf-list-missing", action = "log", condition = { or = [{ and = [{ field = "http.request.method", operator = "in" }] }] } }`},
		{name: "waf list op rejects empty values", err: `requires a value`, securityRule: `{ name = "waf-list-empty", action = "log", condition = { or = [{ and = [{ field = "http.request.method", operator = "in", values = [] }] }] } }`},
		{name: "behavior list op rejects shorthand", err: `requires a value`, behaviorRule: `{ name = "b-list-shorthand", condition = { or = [{ and = [{ field = "http.request.method", operator = "in", values = [] }] }] }, actions = { cache_behavior = "BYPASS" } }`},
		{name: "waf exists rejects shorthand", err: `must not be given a value`, securityRule: `{ name = "waf-exists-shorthand", action = "log", condition = { or = [{ and = [{ field = "http.request.header", field_key = "X-Test", operator = "exists", value = "x" }] }] } }`},
		{name: "waf exists rejects non-empty values", err: `must not be given a value`, securityRule: `{ name = "waf-exists-values", action = "log", condition = { or = [{ and = [{ field = "http.request.header", field_key = "X-Test", operator = "exists", values = ["x"] }] }] } }`},
		{name: "waf operator not allowed for field", err: `is not allowed for field`, securityRule: `{ name = "waf-op-not-allowed", action = "log", condition = { or = [{ and = [{ field = "http.request.method", operator = "contains", value = "GET" }] }] } }`},
		{name: "behavior operator not allowed for field", err: `is not allowed for field`, behaviorRule: `{ name = "b-op-not-allowed", condition = { or = [{ and = [{ field = "http.request.method", operator = "regex", value = "^GET$" }] }] }, actions = { cache_behavior = "BYPASS" } }`},
		{name: "waf uri_raw eq requires full URL", err: `requires a full URL`, securityRule: `{ name = "waf-uri-raw-eq", action = "log", condition = { or = [{ and = [{ field = "http.request.uri_raw", operator = "eq", value = "/x" }] }] } }`},
		{name: "waf uri_raw begins_with requires full URL", err: `requires a valid URL prefix`, securityRule: `{ name = "waf-uri-raw-begins", action = "log", condition = { or = [{ and = [{ field = "http.request.uri_raw", operator = "begins_with", value = "/x" }] }] } }`},
		{name: "waf uri_raw regex rejects invalid pattern", err: `not a valid regex`, securityRule: `{ name = "waf-uri-raw-regex", action = "log", condition = { or = [{ and = [{ field = "http.request.uri_raw", operator = "regex", value = "[broken" }] }] } }`},
		{name: "waf score below range", err: `out of range`, securityRule: `{ name = "waf-score-below", action = "log", condition = { or = [{ and = [{ field = "action_token.score", field_key = "web", operator = "eq", value = "-0.01" }] }] } }`},
		{name: "waf score above range", err: `out of range`, securityRule: `{ name = "waf-score-above", action = "log", condition = { or = [{ and = [{ field = "action_token.score", field_key = "web", operator = "eq", value = "1.01" }] }] } }`},
		{name: "waf action token score below range", err: `out of range`, securityRule: `{ name = "waf-action-token-score-below", action = "log", condition = { or = [{ and = [{ field = "action_token.score", field_key = "web", operator = "eq", value = "-0.01" }] }] } }`},
		{name: "waf bot validation result rejects non bool", err: `must be "passed" or "failed"`, securityRule: `{ name = "waf-bot-validation-bool", action = "log", condition = { or = [{ and = [{ field = "bot_validation.result", operator = "eq", value = "not-bool" }] }] } }`},
		{name: "waf int parser", err: `not a valid int`, securityRule: `{ name = "waf-int-parse", action = "log", condition = { or = [{ and = [{ field = "client.ip.asn", operator = "eq", value = "abc" }] }] } }`},
		{name: "behavior int parser", err: `not a valid int`, behaviorRule: `{ name = "b-int-parse", condition = { or = [{ and = [{ field = "http.response.status_code", operator = "eq", value = "abc" }] }] }, actions = { cache_behavior = "BYPASS" } }`},
		{name: "behavior bool parser", err: `not a valid bool`, behaviorRule: `{ name = "b-bool-parse", condition = { or = [{ and = [{ field = "client.device.is_mobile", operator = "eq", value = "maybe" }] }] }, actions = { cache_behavior = "BYPASS" } }`},
		{name: "waf collection rejects empty field_key", err: `requires field_key to be set`, securityRule: `{ name = "waf-empty-field-key", action = "log", condition = { or = [{ and = [{ field = "http.request.header", field_key = "", operator = "eq", value = "abc" }] }] } }`},
		{name: "behavior collection rejects empty field_key", err: `requires field_key to be set`, behaviorRule: `{ name = "b-empty-field-key", condition = { or = [{ and = [{ field = "http.request.header", field_key = "", operator = "eq", value = "abc" }] }] }, actions = { cache_behavior = "BYPASS" } }`},
		{name: "waf collection needs field_key", err: `requires field_key to be set`, securityRule: `{ name = "waf-missing-field-key", action = "log", condition = { or = [{ and = [{ field = "http.request.header", operator = "eq", value = "abc" }] }] } }`},
		{name: "behavior collection needs field_key", err: `requires field_key to be set`, behaviorRule: `{ name = "b-missing-field-key", condition = { or = [{ and = [{ field = "http.request.header", operator = "eq", value = "abc" }] }] }, actions = { cache_behavior = "BYPASS" } }`},
		{name: "waf non-collection rejects field_key", err: `field_key must not be set`, securityRule: `{ name = "waf-extra-field-key", action = "log", condition = { or = [{ and = [{ field = "http.request.path", operator = "eq", value = "/a", field_key = "X" }] }] } }`},
		{name: "behavior non-collection rejects field_key", err: `field_key must not be set`, behaviorRule: `{ name = "b-extra-field-key", condition = { or = [{ and = [{ field = "http.request.path", operator = "eq", value = "/a", field_key = "X" }] }] }, actions = { cache_behavior = "BYPASS" } }`},
	}
}

func testAccConditionValidationRejection(name, certId string, tc conditionValidationRejectionCase) string {
	if tc.securityRule != "" {
		return fmt.Sprintf(`
resource "ioriver_service" "%s" {
	name        = "%s"
	description = "Condition validation rejection test"
	certificate = "%s"

	config = {
		security = {
			enabled = true
			waf     = {}
			custom_rules = [
				%s
			]
		}
	}
}
`, name, name, certId, tc.securityRule)
	}

	return fmt.Sprintf(`
resource "ioriver_service" "%s" {
	name        = "%s"
	description = "Condition validation rejection test"
	certificate = "%s"

	config = {
		security = {
			enabled = false
		}
		behaviors = {
			custom = [
				%s
			]
		}
	}
}
`, name, name, certId, tc.behaviorRule)
}

func testAccBehaviorConditionAllOperators(name, certId string) string {
	behaviors := []string{
		`{ name = "b-eq", condition = { or = [{ and = [{ field = "http.request.domain", operator = "eq", value = "example.com" }] }] }, actions = { cache_behavior = "BYPASS" } }`,
		`{ name = "b-ne", condition = { or = [{ and = [{ field = "http.request.domain", operator = "ne", value = "blocked.example.com" }] }] }, actions = { cache_behavior = "BYPASS" } }`,
		`{ name = "b-lt", condition = { or = [{ and = [{ field = "http.response.status_code", operator = "lt", value = "500" }] }] }, actions = { cache_behavior = "BYPASS" } }`,
		`{ name = "b-gt", condition = { or = [{ and = [{ field = "http.response.status_code", operator = "gt", value = "100" }] }] }, actions = { cache_behavior = "BYPASS" } }`,
		`{ name = "b-le", condition = { or = [{ and = [{ field = "http.response.status_code", operator = "le", value = "404" }] }] }, actions = { cache_behavior = "BYPASS" } }`,
		`{ name = "b-ge", condition = { or = [{ and = [{ field = "http.response.status_code", operator = "ge", value = "200" }] }] }, actions = { cache_behavior = "BYPASS" } }`,
		`{ name = "b-in", condition = { or = [{ and = [{ field = "http.request.method", operator = "in", values = ["GET", "POST"] }] }] }, actions = { cache_behavior = "BYPASS" } }`,
		`{ name = "b-not-in", condition = { or = [{ and = [{ field = "http.request.method", operator = "not_in", values = ["TRACE", "OPTIONS"] }] }] }, actions = { cache_behavior = "BYPASS" } }`,
		`{ name = "b-match", condition = { or = [{ and = [{ field = "http.request.path", operator = "match", value = "/api/*" }] }] }, actions = { cache_behavior = "BYPASS" } }`,
		`{ name = "b-not-match", condition = { or = [{ and = [{ field = "http.request.path", operator = "not_match", value = "/private/*" }] }] }, actions = { cache_behavior = "BYPASS" } }`,
		`{ name = "b-matches-one-of", condition = { or = [{ and = [{ field = "http.request.domain", operator = "matches_one_of", values = ["example.com", "api.example.com"] }] }] }, actions = { cache_behavior = "BYPASS" } }`,
		`{ name = "b-does-not-match-any", condition = { or = [{ and = [{ field = "http.request.domain", operator = "does_not_match_any_of", values = ["blocked.example.com", "deny.example.com"] }] }] }, actions = { cache_behavior = "BYPASS" } }`,
		`{ name = "b-exists", condition = { or = [{ and = [{ field = "http.request.domain", operator = "exists", values = [] }] }] }, actions = { cache_behavior = "BYPASS" } }`,
		`{ name = "b-does-not-exist", condition = { or = [{ and = [{ field = "http.request.domain", operator = "does_not_exist", values = [] }] }] }, actions = { cache_behavior = "BYPASS" } }`,
		`{ name = "b-client-ip-comma", condition = { or = [{ and = [{ field = "client.ip", operator = "in", values = ["10.0.0.1", "10.0.0.2"] }] }] }, actions = { cache_behavior = "BYPASS" } }`,
		`{ name = "b-bool", condition = { or = [{ and = [{ field = "client.device.is_mobile", operator = "eq", value = "true" }] }] }, actions = { cache_behavior = "BYPASS" } }`,
		`{ name = "b-int-list", condition = { or = [{ and = [{ field = "http.response.status_code", operator = "in", values = ["404", "503"] }] }] }, actions = { cache_behavior = "BYPASS" } }`,
		`{ name = "b-req-header", condition = { or = [{ and = [{ field = "http.request.header", field_key = "X-Test", operator = "eq", value = "abc" }] }] }, actions = { cache_behavior = "BYPASS" } }`,
		`{ name = "b-res-header", condition = { or = [{ and = [{ field = "http.response.header", field_key = "X-Up", operator = "eq", value = "ok" }] }] }, actions = { cache_behavior = "BYPASS" } }`,
		`{ name = "b-query", condition = { or = [{ and = [{ field = "http.request.query_param", field_key = "debug", operator = "eq", value = "1" }] }] }, actions = { cache_behavior = "BYPASS" } }`,
		`{ name = "b-regex", condition = { or = [{ and = [{ field = "http.request.path", operator = "regex", value = "^/api/.*$" }] }] }, actions = { cache_behavior = "BYPASS" } }`,
		`{ name = "b-not-regex", condition = { or = [{ and = [{ field = "http.request.path", operator = "not_regex", value = "^/internal/.*$" }] }] }, actions = { cache_behavior = "BYPASS" } }`,
	}

	return fmt.Sprintf(`
resource "ioriver_service" "%s" {
	name        = "%s"
	description = "Behavior all operators acceptance test"
	certificate = "%s"

	config = {
		security = {
			enabled = false
		}
		behaviors = {
			custom = [
				%s
			]
		}
	}
}
`, name, name, certId, strings.Join(behaviors, ",\n\t\t\t\t"))
}
