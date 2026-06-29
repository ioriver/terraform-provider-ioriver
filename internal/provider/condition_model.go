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
	"github.com/hashicorp/terraform-plugin-log/tflog"
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

// valuesFromRaw decodes the raw API value for a single condition into []string,
// using the caller's CommaStringField metadata to pick the right shape. This is
// the inverse of the value-encoding branches in ConditionExpressionToMapByCaller
// — the caller is the single source of truth for both directions.
func valuesFromRaw(field, operator string, rawVal interface{}, spec *ConditionSpec) []string {
	if spec != nil && spec.Operators[operator].CommaString {
		s, _ := rawVal.(string)
		if s == "" {
			return nil
		}
		return strings.Split(s, ",")
	}
	// kindPassFail: backend stores bool, surface as "passed"/"failed"
	if spec != nil && spec.Fields[field].Kind == kindPassFail {
		switch v := rawVal.(type) {
		case bool:
			if v {
				return []string{"passed"}
			}
			return []string{"failed"}
		case string:
			if v == "true" {
				return []string{"passed"}
			}
			return []string{"failed"}
		}
	}
	return DefaultValueDeserializer(field, rawVal)
}

// ConditionExpressionFromMap deserialises an API OR-of-ANDs map → ConditionExpressionModel.
//
// prior, when non-nil, is used to decide whether to populate `value`
// (single-string shorthand) or `values` (set form) for each condition.
// Pass nil when there is no prior (e.g. import) — defaults to `values`.
//
// caller carries the per-flavour metadata (WAF / behavior) — specifically the
// CommaStringField needed to decode comma-joined wire values back into a slice.
// Pass nil for plain []interface{} decoding only.
func ConditionExpressionFromMap(
	ctx context.Context,
	raw map[string]interface{},
	plan *ConditionExpressionModel,
	spec *ConditionSpec,
) (*ConditionExpressionModel, error) {
	tflog.Debug(ctx, fmt.Sprintf("[ConditionExpressionFromMap] Start: map condition: %+v", raw))
	if plan != nil {
		tflog.Debug(ctx, fmt.Sprintf("[ConditionExpressionFromMap] Start: plan condition: %+v", *plan))
	}

	if raw == nil {
		return nil, nil
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
			vals := valuesFromRaw(field, operator, andMap["value"], spec)

			// Use plan state to decide which form to restore.
			// We must check IsUnknown() too: on first create, Computed fields the
			// user didn't set are Unknown (not null), which would otherwise trigger
			// a false positive for planUsedValue.
			planUsedValue := false
			if plan != nil &&
				orIdx < len(plan.Or) &&
				andIdx < len(plan.Or[orIdx].And) {
				pv := plan.Or[orIdx].And[andIdx].Value
				planUsedValue = !pv.IsNull() && !pv.IsUnknown() && pv.ValueString() != ""
			}

			cond := ConditionModel{
				Field:    types.StringValue(field),
				Operator: types.StringValue(operator),
				FieldKey: types.StringNull(),
			}

			tflog.Debug(ctx, fmt.Sprintf("[ConditionExpressionFromMap] 🔍 planUsedValue: %v, vals: %+v\n", planUsedValue, vals))

			if planUsedValue && len(vals) > 0 {
				if len(vals) > 1 {
					return nil, fmt.Errorf("single value set in plan but received more than one element: %+v", vals)
				}
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

	tflog.Debug(ctx, fmt.Sprintf("[ConditionExpressionFromMap] 🔍 Deserialized expression: %+v\n", expr))
	return expr, nil
}

func ValidateConditionModel(expr *ConditionExpressionModel, prefix string, spec *ConditionSpec) []string {
	if expr == nil {
		return nil
	}
	var errs []string
	for j, andGroup := range expr.Or {
		for k, cond := range andGroup.And {
			loc := fmt.Sprintf("%s.condition.or[%d].and[%d]", prefix, j, k)
			errs = append(errs, ValidateCondition(context.Background(), cond, loc, spec)...)
		}
	}
	return errs
}

// ---------------------------------------------------------------------------
// Schema-level validator wrapper around ValidateConditionModel
//
// Attached to the `or` ListNestedAttribute on a condition expression schema
// (see wafConditionExpressionAttributes / behaviorConditionExpressionAttributes).
// The validator decodes the list of AND-groups, wraps it into a
// ConditionExpressionModel, and runs ValidateConditionModel with the
// caller-supplied spec (WAF or behavior).
// ---------------------------------------------------------------------------

type conditionExpressionValidator struct {
	spec *ConditionSpec
}

func (v conditionExpressionValidator) Description(_ context.Context) string {
	return fmt.Sprintf("condition must satisfy %s field/operator/value rules", v.spec.Name)
}

func (v conditionExpressionValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v conditionExpressionValidator) ValidateList(ctx context.Context, req validator.ListRequest, resp *validator.ListResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	var or []ConditionAndGroupModel
	resp.Diagnostics.Append(req.ConfigValue.ElementsAs(ctx, &or, false)...)
	if resp.Diagnostics.HasError() {
		return
	}
	expr := &ConditionExpressionModel{Or: or}

	for _, msg := range ValidateConditionModel(expr, v.spec.Name, v.spec) {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			fmt.Sprintf("Invalid %s condition", v.spec.Name),
			msg,
		)
	}
}

// ConditionExpressionValidator returns a schema validator.List bound to the
// supplied ConditionSpec (for example WafConditionSpec or BehaviorConditionSpec).
func ConditionExpressionValidator(spec *ConditionSpec) validator.List {
	return conditionExpressionValidator{spec: spec}
}
