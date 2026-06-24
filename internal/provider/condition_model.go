package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// ---------------------------------------------------------------------------
// Shared condition model types
//
// Both WAF conditions (security_model.go) and behavior conditions
// (behavior_model.go) share the same on-wire shape:
//
//   field | operator | value(s) | field_key
//
// The only difference between the two is the set of valid fields/operators,
// which is enforced by per-caller schema validators — not by the struct itself.
// ---------------------------------------------------------------------------

// ConditionModel is the canonical Go struct for a single condition.
// Used by both WAF (WafConditionModel alias) and behavior conditions.
type ConditionModel struct {
	Field    types.String `tfsdk:"field"`
	Operator types.String `tfsdk:"operator"`
	Values   types.Set    `tfsdk:"values"`
	Value    types.String `tfsdk:"value"`
	FieldKey types.String `tfsdk:"field_key"`
}

// ConditionAndGroupModel is one AND group (list of conditions).
type ConditionAndGroupModel struct {
	And []ConditionModel `tfsdk:"and"`
}

// ConditionExpressionModel is the top-level OR-of-ANDs expression.
type ConditionExpressionModel struct {
	Or []ConditionAndGroupModel `tfsdk:"or"`
}

// ---------------------------------------------------------------------------
// Shared schema helpers
// ---------------------------------------------------------------------------

// conditionValuesAttr returns the shared `values` SetAttribute.
// Both WAF and behavior condition schemas use this verbatim.
// Synthesis of values from the single-value shorthand `value` is handled by the
// resource-level ModifyPlan (ServiceResource.ModifyPlan) rather than an
// attribute-level plan modifier, so there are no plan modifiers here.
func conditionValuesAttr() schema.SetAttribute {
	return schema.SetAttribute{
		MarkdownDescription: "List of values to match against.\n" +
			"  - For `ip_match`/`not_ip_match` provide CIDR blocks or individual IPs (e.g. `[\"10.0.0.0/8\", \"1.2.3.4\"]`).\n" +
			"  - For `exists`/`does_not_exist` set an empty list (`[]`).\n" +
			"  - For all other operators provide one or more string values. \n  -" +
			"  - Mutually exclusive with `value`.",
		Optional:    true,
		ElementType: types.StringType,
		Validators: []validator.Set{
			setvalidator.ConflictsWith(path.MatchRelative().AtParent().AtName("value")),
			setvalidator.ExactlyOneOf(path.MatchRelative().AtParent().AtName("value")),
		},
	}
}

// conditionValueAttr returns the shared `value` StringAttribute.
// `value = "x"` is a write-once convenience alias for `values = ["x"]`.
// The provider stores the value in `values` — after Read, `value` will be null
// in state, but Terraform will re-apply the config value on the next plan so
// there is no perpetual drift.
func conditionValueAttr() schema.StringAttribute {
	return schema.StringAttribute{
		MarkdownDescription: "Single-value shorthand for `values = [\"...\"]`. Mutually exclusive with `values`.",
		Optional:            true,
		Validators: []validator.String{
			stringvalidator.ConflictsWith(path.MatchRelative().AtParent().AtName("values")),
			stringvalidator.ExactlyOneOf(path.MatchRelative().AtParent().AtName("values")),
		},
	}
}

// ---------------------------------------------------------------------------
// Shared attr-type maps
// ---------------------------------------------------------------------------

// ConditionAttrTypes returns the attr.Type map for ConditionModel.
// Used when constructing types.Object / types.ObjectNull for conditions.
func ConditionAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"field":     types.StringType,
		"operator":  types.StringType,
		"values":    types.SetType{ElemType: types.StringType},
		"value":     types.StringType,
		"field_key": types.StringType,
	}
}

// ConditionExpressionAttrTypes returns the attr.Type map for ConditionExpressionModel.
func ConditionExpressionAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"or": types.ListType{ElemType: types.ObjectType{AttrTypes: map[string]attr.Type{
			"and": types.ListType{ElemType: types.ObjectType{AttrTypes: ConditionAttrTypes()}},
		}}},
	}
}

// conditionExpressionAttrType returns the attr.Type for ConditionExpressionModel as a
// single attr.Type (used in BehaviorAttrTypes where a nested object type is expected).
func conditionExpressionAttrType() attr.Type {
	return types.ObjectType{AttrTypes: ConditionExpressionAttrTypes()}
}

// ---------------------------------------------------------------------------
// Shared serialisation helpers
// ---------------------------------------------------------------------------

// conditionListItemToString converts one item from a JSON-unmarshalled []interface{}
// to a string. JSON numbers (float64), booleans, and other non-string types are
// converted via fmt.Sprintf so no data is silently lost.
func conditionListItemToString(item interface{}) string {
	if s, ok := item.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", item)
}

// ConditionExpressionToMap serialises a ConditionExpressionModel → API OR-of-ANDs map.
//
// valueSerializer is an optional hook that controls how the `values` set for a
// single condition is written to the API map.  When nil the default is used:
// the values are sent as a []string list (correct for all behavior fields and
// most WAF fields).  Pass a custom func to override per-field serialisation
// (e.g. WAF's client.ip.address which uses a comma-separated string).
func ConditionExpressionToMap(ctx context.Context, expr *ConditionExpressionModel, valueSerializer func(field string, vals []string) interface{}) map[string]interface{} {
	if expr == nil {
		return nil
	}
	if valueSerializer == nil {
		valueSerializer = func(_ string, vals []string) interface{} { return vals }
	}
	orArr := []interface{}{}
	for _, andGroup := range expr.Or {
		andArr := []interface{}{}
		for _, cond := range andGroup.And {
			condMap := map[string]interface{}{
				"field":    cond.Field.ValueString(),
				"operator": cond.Operator.ValueString(),
			}
			// Coalesce: prefer `values` (set form); fall back to `value` (single-string
			// shorthand) in case the plan modifier hasn't run or both paths are used.
			var vals []string
			if !cond.Values.IsNull() && !cond.Values.IsUnknown() && len(cond.Values.Elements()) > 0 {
				_ = cond.Values.ElementsAs(ctx, &vals, false)
			}
			if len(vals) == 0 && !cond.Value.IsNull() && !cond.Value.IsUnknown() && cond.Value.ValueString() != "" {
				vals = []string{cond.Value.ValueString()}
			}
			condMap["value"] = valueSerializer(cond.Field.ValueString(), vals)
			if !cond.FieldKey.IsNull() && !cond.FieldKey.IsUnknown() && cond.FieldKey.ValueString() != "" {
				condMap["field_key"] = cond.FieldKey.ValueString()
			}
			andArr = append(andArr, condMap)
		}
		orArr = append(orArr, map[string]interface{}{"and": andArr})
	}
	return map[string]interface{}{"or": orArr}
}

// ---------------------------------------------------------------------------
// Shared deserialisation helpers
// ---------------------------------------------------------------------------

// DefaultValueDeserializer converts a raw API value ([]interface{}) → []string.
// This is the standard deserializer for all behavior fields and most WAF fields.
// It also handles scalar values (float64, string, bool) by wrapping them in a
// single-element slice — the API occasionally returns a bare scalar instead of
// a list (e.g. http.response.status_code returns a float64).
func DefaultValueDeserializer(_ string, rawVal interface{}) []string {
	if list, ok := rawVal.([]interface{}); ok {
		out := make([]string, 0, len(list))
		for _, item := range list {
			out = append(out, conditionListItemToString(item))
		}
		return out
	}
	// Scalar value (float64, string, bool, etc.) — wrap in a single-element slice.
	if rawVal != nil {
		return []string{conditionListItemToString(rawVal)}
	}
	return nil
}

// WafValueDeserializer is the WAF-specific deserializer.
// client.ip.address is sent as a comma-separated string; everything else is
// a []interface{} list.
func WafValueDeserializer(field string, rawVal interface{}) []string {
	if field == "client.ip.address" {
		s, _ := rawVal.(string)
		if s == "" {
			return nil
		}
		return strings.Split(s, ",")
	}
	return DefaultValueDeserializer(field, rawVal)
}

// ConditionExpressionFromMap deserialises an API OR-of-ANDs map → ConditionExpressionModel.
//
// prior, when non-nil, is used to decide whether to populate `value`
// (single-string shorthand) or `values` (set form) for each condition.
// Pass nil when there is no prior (e.g. import) — defaults to `values`.
//
// valueDeserializer controls how the raw API value is converted to []string.
// Pass nil to use DefaultValueDeserializer ([]interface{} list).
// Pass WafValueDeserializer for WAF (handles client.ip.address comma-string).
func ConditionExpressionFromMap(
	ctx context.Context,
	raw map[string]interface{},
	prior *ConditionExpressionModel,
	valueDeserializer func(field string, rawVal interface{}) []string,
) *ConditionExpressionModel {
	if raw == nil {
		return nil
	}
	if valueDeserializer == nil {
		valueDeserializer = DefaultValueDeserializer
	}

	orRaw, _ := raw["or"].([]interface{})
	expr := &ConditionExpressionModel{}

	for orIdx, orItem := range orRaw {
		orMap, ok := orItem.(map[string]interface{})
		if !ok {
			continue
		}
		andRaw, _ := orMap["and"].([]interface{})
		andGroup := ConditionAndGroupModel{}

		for andIdx, andItem := range andRaw {
			andMap, ok := andItem.(map[string]interface{})
			if !ok {
				continue
			}
			field, _ := andMap["field"].(string)
			operator, _ := andMap["operator"].(string)
			vals := valueDeserializer(field, andMap["value"])

			// Use prior state to decide which form to restore.
			// We must check IsUnknown() too: on first create, Computed fields the
			// user didn't set are Unknown (not null), which would otherwise trigger
			// a false positive for priorUsedValue.
			priorUsedValue := false
			if prior != nil &&
				orIdx < len(prior.Or) &&
				andIdx < len(prior.Or[orIdx].And) {
				pv := prior.Or[orIdx].And[andIdx].Value
				priorUsedValue = !pv.IsNull() && !pv.IsUnknown() && pv.ValueString() != ""
			}

			cond := ConditionModel{
				Field:    types.StringValue(field),
				Operator: types.StringValue(operator),
				FieldKey: types.StringNull(),
			}

			if priorUsedValue && len(vals) > 0 {
				cond.Value = types.StringValue(vals[0])
				cond.Values = types.SetNull(types.StringType) // Explicitly Null the alternative
			} else {
				if vals == nil {
					vals = []string{}
				}
				valSet, _ := types.SetValueFrom(ctx, types.StringType, vals)
				cond.Values = valSet
				cond.Value = types.StringNull() // Explicitly Null the alternative
			}

			if fk, ok := andMap["field_key"].(string); ok && fk != "" {
				cond.FieldKey = types.StringValue(fk)
			}
			andGroup.And = append(andGroup.And, cond)
		}
		expr.Or = append(expr.Or, andGroup)
	}
	return expr
}
