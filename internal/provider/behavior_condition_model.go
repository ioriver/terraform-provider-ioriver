package provider

import (
	"context"
	"strings"

	"github.com/hashicorp/go-set"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

var behaviorConditionFields = []string{
	"http.request.domain",
	"http.request.path",
	"http.request.method",
	"http.request.header",
	"http.response.status_code",
	"http.response.header",
	"client.geo.country",
	"client.device.is_mobile",
	"http.request.query_param",
	"client.ip",
}

var behaviorConditionOperators = []string{
	"eq", "ne",
	"lt", "gt", "le", "ge",
	"in", "not_in",
	"match", "not_match",
	"matches_one_of", "does_not_match_any_of",
	"regex", "not_regex",
	"exists", "does_not_exist",
}

var BehaviorConditionSpec = &ConditionSpec{
	Name: "behavior",
	Operators: map[string]OperatorSpec{
		"eq":                    {Arity: arityScalar},
		"ne":                    {Arity: arityScalar},
		"lt":                    {Arity: arityScalar},
		"le":                    {Arity: arityScalar},
		"gt":                    {Arity: arityScalar},
		"ge":                    {Arity: arityScalar},
		"in":                    {Arity: arityList},
		"not_in":                {Arity: arityList},
		"match":                 {Arity: arityScalar},
		"not_match":             {Arity: arityScalar},
		"matches_one_of":        {Arity: arityList},
		"does_not_match_any_of": {Arity: arityList},
		"regex":                 {Arity: arityScalar},
		"not_regex":             {Arity: arityScalar},
		"exists":                {Arity: arityNone},
		"does_not_exist":        {Arity: arityNone},
	},
	Fields: map[string]FieldSpec{
		"http.request.domain":       {Kind: kindString, Operators: *set.From[string]([]string{"eq", "ne", "in", "not_in", "match", "not_match", "matches_one_of", "does_not_match_any_of", "regex", "not_regex", "exists", "does_not_exist"})},
		"http.request.path":         {Kind: kindPath, Operators: *set.From[string]([]string{"eq", "ne", "in", "not_in", "match", "not_match", "matches_one_of", "does_not_match_any_of", "regex", "not_regex", "exists", "does_not_exist"})},
		"http.request.method":       {Kind: kindString, Operators: *set.From[string]([]string{"eq", "ne", "in", "not_in"})},
		"http.request.header":       {Kind: kindString, RequiresFieldKey: true, Operators: *set.From[string]([]string{"eq", "ne", "in", "not_in", "match", "not_match", "matches_one_of", "does_not_match_any_of", "regex", "not_regex", "exists", "does_not_exist"})},
		"http.response.status_code": {Kind: kindInt, Operators: *set.From[string]([]string{"eq", "ne", "lt", "le", "gt", "ge", "in", "not_in"})},
		"http.response.header":      {Kind: kindString, RequiresFieldKey: true, Operators: *set.From[string]([]string{"eq", "ne", "in", "not_in", "match", "not_match", "matches_one_of", "does_not_match_any_of", "regex", "not_regex", "exists", "does_not_exist"})},
		"client.geo.country":        {Kind: kindCountry, Operators: *set.From[string]([]string{"eq", "ne", "in", "not_in"})},
		"client.device.is_mobile":   {Kind: kindBool, Operators: *set.From[string]([]string{"eq", "ne"})},
		"http.request.query_param":  {Kind: kindString, RequiresFieldKey: true, Operators: *set.From[string]([]string{"eq", "ne", "in", "not_in", "match", "not_match", "matches_one_of", "does_not_match_any_of", "regex", "not_regex", "exists", "does_not_exist"})},
		"client.ip":                 {Kind: kindIP, Operators: *set.From[string]([]string{"eq", "ne", "in", "not_in"})},
	},
}

// ---------------------------------------------------------------------------
// Behavior Condition model types
// ---------------------------------------------------------------------------

// BehaviorConditionModel is an alias for the shared ConditionModel.
// Behavior-specific field/operator validation is applied in the schema only.
type BehaviorConditionModel = ConditionModel

// BehaviorConditionAndGroupModel is an alias for the shared ConditionAndGroupModel.
type BehaviorConditionAndGroupModel = ConditionAndGroupModel

// BehaviorConditionExpressionModel is an alias for the shared ConditionExpressionModel.
type BehaviorConditionExpressionModel = ConditionExpressionModel

// ---------------------------------------------------------------------------
// Behavior Condition schema helpers
// ---------------------------------------------------------------------------

func behaviorConditionAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"field": schema.StringAttribute{
			MarkdownDescription: "Field to match against. Valid values: `" + strings.Join(behaviorConditionFields, "`, `") + "`",
			Required:            true,
			Validators: []validator.String{
				stringvalidator.OneOf(behaviorConditionFields...),
			},
		},
		"operator": schema.StringAttribute{
			MarkdownDescription: "Match operator. Valid values: `" + strings.Join(behaviorConditionOperators, "`, `") + "`",
			Required:            true,
			Validators: []validator.String{
				stringvalidator.OneOf(behaviorConditionOperators...),
			},
		},
		"values": conditionValuesAttr(),
		"value":  conditionValueAttr(),
		"field_key": schema.StringAttribute{
			MarkdownDescription: "Key within the field (required for header and query_param fields)",
			Optional:            true,
		},
	}
}

func behaviorConditionExpressionAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"or": schema.ListNestedAttribute{
			MarkdownDescription: "List of AND groups (OR of ANDs expression)",
			Required:            true,
			NestedObject: schema.NestedAttributeObject{
				Attributes: map[string]schema.Attribute{
					"and": schema.ListNestedAttribute{
						MarkdownDescription: "List of conditions that must ALL match (AND group)",
						Required:            true,
						NestedObject: schema.NestedAttributeObject{
							Attributes: behaviorConditionAttributes(),
						},
					},
				},
			},
			Validators: []validator.List{
				ConditionExpressionValidator(BehaviorConditionSpec),
			},
		},
	}
}

// ---------------------------------------------------------------------------
// Behavior Condition serialisation helpers
// ---------------------------------------------------------------------------

// behaviorConditionExpressionToMap serialises a ConditionExpressionModel → API map.
// All behavior fields send values as a list — no per-field override needed.
func behaviorConditionExpressionToMap(ctx context.Context, expr *BehaviorConditionExpressionModel) map[string]interface{} {
	return ConditionExpressionToMapByCaller(ctx, expr, BehaviorConditionSpec)
}

// behaviorConditionExpressionFromMap deserialises an API condition map → BehaviorConditionExpressionModel.
// prior, when non-nil, is used to decide whether to populate `value` or `values` per condition.
// Pass nil when there is no prior (import) — defaults to `values`.
func behaviorConditionExpressionFromMap(ctx context.Context, raw map[string]interface{}, planFittingItem *BehaviorConditionExpressionModel) (*BehaviorConditionExpressionModel, error) {
	return ConditionExpressionFromMap(ctx, raw, planFittingItem, BehaviorConditionSpec)
}

// ---------------------------------------------------------------------------
// Validation
// ---------------------------------------------------------------------------

// pathPatternToConditionMap converts a simple path pattern string to the API condition map.
// Note: value must be a plain string (not a list) so the backend's path_pattern validators
// can use it as a dict key in _validate_behavior_no_partial_intersects.
func pathPatternToConditionMap(pattern string) map[string]interface{} {
	return map[string]interface{}{
		"or": []interface{}{
			map[string]interface{}{
				"and": []interface{}{
					map[string]interface{}{
						"field":    "http.request.path",
						"operator": "match",
						"value":    pattern,
					},
				},
			},
		},
	}
}

// isSimplePathPattern returns (pattern, true) if the condition map represents a single
// http.request.path / match condition — i.e. the shorthand that path_pattern expands to.
func isSimplePathPattern(condMap map[string]interface{}) (string, bool) {
	orRaw, ok := condMap["or"].([]interface{})
	if !ok || len(orRaw) != 1 {
		return "", false
	}
	orMap, ok := orRaw[0].(map[string]interface{})
	if !ok {
		return "", false
	}
	andRaw, ok := orMap["and"].([]interface{})
	if !ok || len(andRaw) != 1 {
		return "", false
	}
	andMap, ok := andRaw[0].(map[string]interface{})
	if !ok {
		return "", false
	}
	if strOrEmpty(andMap, "field") != "http.request.path" {
		return "", false
	}
	if strOrEmpty(andMap, "operator") != "match" {
		return "", false
	}
	switch v := andMap["value"].(type) {
	case []interface{}:
		if len(v) == 1 {
			if s, ok := v[0].(string); ok {
				return s, true
			}
		}
	case []string:
		if len(v) == 1 {
			return v[0], true
		}
	case string:
		return v, true
	}
	return "", false
}
