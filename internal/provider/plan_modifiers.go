package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// ---------------------------------------------------------------------------
// namedListPlanModifier
//
// A generic list plan modifier that uses a named string field as the identity
// key instead of list position. Works for any list of objects that have a
// unique name field (domains, origins, log_destinations, …).
//
// Behaviour:
//   - Pure reorder (same set of names, different HCL order): canonicalise the
//     plan back to state order so Terraform sees no diff.
//   - Additions/deletions: keep plan order, resolve computed unknowns (uuid)
//     from state by identity.
//   - resolveNested: optional hook for domain-specific sub-structure resolution
//     (e.g. mappings inside a domain). Pass nil for simple lists.
// ---------------------------------------------------------------------------

type namedListPlanModifier struct {
	nameField     string
	resolveNested func(ctx context.Context, planAttrs, stateAttrs map[string]attr.Value) (map[string]attr.Value, bool)
}

// NamedListPlanModifier returns a generic identity-aware plan modifier.
// nameField is the attribute used as the identity key (e.g. "name", "domain").
func NamedListPlanModifier(nameField string) planmodifier.List {
	return namedListPlanModifier{nameField: nameField}
}

// DomainListPlanModifier returns the identity-aware plan modifier for domains.
// It uses "domain" as the identity key and additionally resolves nested mappings.
func DomainListPlanModifier() planmodifier.List {
	return namedListPlanModifier{
		nameField:     "domain",
		resolveNested: resolveDomainMappings,
	}
}

// OriginSetListPlanModifier returns the identity-aware plan modifier for
// origin_sets. It uses "name" as the identity key and resolves UUIDs of the
// nested origins list positionally (origins have no name field).
func OriginSetListPlanModifier() planmodifier.List {
	return namedListPlanModifier{
		nameField:     "name",
		resolveNested: resolveOriginSetOrigins,
	}
}

// resolveOriginSetOrigins is the resolveNested hook for the origin_sets list.
// Each origin inside a set has a computed uuid but no name, so we resolve the
// UUIDs positionally — the i-th plan origin gets the uuid from the i-th state
// origin (same host → same position → same uuid).
func resolveOriginSetOrigins(ctx context.Context, planAttrs, stateAttrs map[string]attr.Value) (map[string]attr.Value, bool) {
	planOrigins, ok1 := planAttrs["origins"].(types.List)
	stateOrigins, ok2 := stateAttrs["origins"].(types.List)
	if !ok1 || !ok2 {
		return planAttrs, false
	}

	planElems := planOrigins.Elements()
	stateElems := stateOrigins.Elements()

	newElems := make([]attr.Value, len(planElems))
	anyChanged := false

	for i, elem := range planElems {
		if i >= len(stateElems) {
			newElems[i] = elem
			continue
		}
		planObj, ok := elem.(types.Object)
		if !ok {
			newElems[i] = elem
			continue
		}
		stateObj, ok := stateElems[i].(types.Object)
		if !ok {
			newElems[i] = elem
			continue
		}
		newAttrs := shallowCopyAttrMap(planObj.Attributes())
		if resolveUnknownString(newAttrs, stateObj.Attributes(), "uuid") {
			newObj, diags := types.ObjectValue(planObj.AttributeTypes(ctx), newAttrs)
			if !diags.HasError() {
				newElems[i] = newObj
				anyChanged = true
				continue
			}
		}
		newElems[i] = elem
	}

	if !anyChanged {
		return planAttrs, false
	}
	newList, diags := types.ListValue(planOrigins.ElementType(ctx), newElems)
	if diags.HasError() {
		return planAttrs, false
	}
	out := shallowCopyAttrMap(planAttrs)
	out["origins"] = newList
	return out, true
}

func (m namedListPlanModifier) Description(_ context.Context) string {
	return "Resolves computed fields (uuid, …) from prior state by " + m.nameField + ", not by list position. Also suppresses spurious diffs when items are reordered in HCL."
}

func (m namedListPlanModifier) MarkdownDescription(_ context.Context) string {
	return m.Description(context.Background())
}

func (m namedListPlanModifier) PlanModifyList(ctx context.Context, req planmodifier.ListRequest, resp *planmodifier.ListResponse) {
	// If the whole list is unknown (first apply, import) there is nothing useful
	// in state to copy from — leave the plan as-is.
	if req.PlanValue.IsUnknown() || req.PlanValue.IsNull() {
		return
	}
	if req.StateValue.IsNull() || req.StateValue.IsUnknown() {
		return
	}

	// Build a lookup: identity-name → state attrs, and preserve state order.
	stateByName := make(map[string]map[string]attr.Value)
	stateOrder := make([]string, 0)
	for _, elem := range req.StateValue.Elements() {
		obj, ok := elem.(types.Object)
		if !ok {
			continue
		}
		attrs := obj.Attributes()
		if n, ok := attrs[m.nameField].(types.String); ok && !n.IsNull() && !n.IsUnknown() {
			stateByName[n.ValueString()] = attrs
			stateOrder = append(stateOrder, n.ValueString())
		}
	}

	// Collect plan element names.
	planElems := req.PlanValue.Elements()
	planNames := make(map[string]struct{}, len(planElems))
	for _, elem := range planElems {
		obj, ok := elem.(types.Object)
		if !ok {
			continue
		}
		if n, ok := obj.Attributes()[m.nameField].(types.String); ok && !n.IsNull() && !n.IsUnknown() {
			planNames[n.ValueString()] = struct{}{}
		}
	}

	// Pure reorder: same set of names in a different HCL order — AND no value
	// changes on any item. If values also changed, fall through to the normal
	// path so Terraform sees the diff at the correct config-order positions.
	isPureReorder := len(planNames) == len(stateOrder)
	if isPureReorder {
		for _, name := range stateOrder {
			if _, ok := planNames[name]; !ok {
				isPureReorder = false
				break
			}
		}
	}
	if isPureReorder {
		// Also verify that every named item's plan values are identical to state.
		// If any value changed, it is not a pure reorder.
		for _, elem := range planElems {
			obj, ok := elem.(types.Object)
			if !ok {
				continue
			}
			n, ok := obj.Attributes()[m.nameField].(types.String)
			if !ok || n.IsNull() || n.IsUnknown() {
				continue
			}
			stateAttrs, found := stateByName[n.ValueString()]
			if !found {
				continue
			}
			if !planAttrsMatchState(obj.Attributes(), stateAttrs) {
				isPureReorder = false
				break
			}
		}
	}
	if isPureReorder {
		newElems := make([]attr.Value, 0, len(stateOrder))
		elemType := req.PlanValue.ElementType(ctx)
		for _, name := range stateOrder {
			stateAttrs := stateByName[name]
			// Find the matching plan element for this name.
			var planAttrs map[string]attr.Value
			for _, elem := range planElems {
				obj, ok := elem.(types.Object)
				if !ok {
					continue
				}
				if n, ok := obj.Attributes()[m.nameField].(types.String); ok && n.ValueString() == name {
					planAttrs = obj.Attributes()
					break
				}
			}
			if planAttrs == nil {
				planAttrs = stateAttrs // shouldn't happen, but safe fallback
			}
			merged := shallowCopyAttrMap(planAttrs)
			resolveUnknownString(merged, stateAttrs, "uuid")
			if m.resolveNested != nil {
				if updated, changed := m.resolveNested(ctx, merged, stateAttrs); changed {
					merged = updated
				}
			}
			newObj, diags := types.ObjectValue(elemType.(types.ObjectType).AttrTypes, merged)
			if diags.HasError() {
				return
			}
			newElems = append(newElems, newObj)
		}
		newList, diags := types.ListValue(elemType, newElems)
		if !diags.HasError() {
			resp.PlanValue = newList
		}
		return
	}

	// Non-pure-reorder (additions/deletions): keep plan order but resolve computed
	// unknowns from state by identity.
	newElems := make([]attr.Value, len(planElems))
	anyChanged := false

	for i, elem := range planElems {
		obj, ok := elem.(types.Object)
		if !ok {
			newElems[i] = elem
			continue
		}
		planAttrs := obj.Attributes()

		n, ok := planAttrs[m.nameField].(types.String)
		if !ok || n.IsNull() || n.IsUnknown() {
			newElems[i] = elem
			continue
		}

		stateAttrs, found := stateByName[n.ValueString()]
		if !found {
			// New item — no state entry. If any nested computed field used UseStateForUnknown(),
			// Terraform may have positionally copied a UUID from a *different* item at the same list index.
			// Explicitly reset uuid to unknown so the backend assigns a fresh one.
			newAttrs := shallowCopyAttrMap(planAttrs)
			if _, hasUUID := newAttrs["uuid"]; hasUUID {
				newAttrs["uuid"] = types.StringUnknown()
				newObj, diags := types.ObjectValue(obj.AttributeTypes(ctx), newAttrs)
				if !diags.HasError() {
					newElems[i] = newObj
					anyChanged = true
					continue
				}
			}
			newElems[i] = elem
			continue
		}

		newAttrs := shallowCopyAttrMap(planAttrs)
		changed := resolveUnknownString(newAttrs, stateAttrs, "uuid")

		if m.resolveNested != nil {
			if updated, nestedChanged := m.resolveNested(ctx, newAttrs, stateAttrs); nestedChanged {
				newAttrs = updated
				changed = true
			}
		}

		if changed {
			newObj, diags := types.ObjectValue(obj.AttributeTypes(ctx), newAttrs)
			if diags.HasError() {
				newElems[i] = elem
			} else {
				newElems[i] = newObj
				anyChanged = true
			}
		} else {
			newElems[i] = elem
		}
	}

	if anyChanged {
		newList, diags := types.ListValue(req.PlanValue.ElementType(ctx), newElems)
		if !diags.HasError() {
			resp.PlanValue = newList
		}
	}
}

// planAttrsMatchState returns true if every attribute in planAttrs that is
// known (not unknown) in the plan is equal to the corresponding state attribute.
// Unknown plan attributes are skipped — they are computed and will be resolved
// later; they do not constitute a "value change" by the user.
// Nested objects and lists are compared recursively so that unknown computed
// fields buried inside nested structures (e.g. uuid inside origins of an
// origin_set) are also skipped correctly.
func planAttrsMatchState(planAttrs, stateAttrs map[string]attr.Value) bool {
	for k, pv := range planAttrs {
		if pv.IsUnknown() {
			continue // computed fill-in — not a user change
		}
		sv, ok := stateAttrs[k]
		if !ok {
			return false // new field not in state → value changed
		}
		if !attrValueMatchesState(pv, sv) {
			return false
		}
	}
	return true
}

// attrValueMatchesState recursively compares two attr.Values, treating unknown
// plan values as matching (they are computed fields, not user changes).
// For objects and lists it recurses so that deeply nested unknowns (e.g. uuid
// inside the origins of an origin_set) are handled correctly.
func attrValueMatchesState(pv, sv attr.Value) bool {
	if pv.IsUnknown() {
		return true // computed — not a user change
	}
	if pv.Equal(sv) {
		return true
	}
	// Recurse into objects.
	if planObj, ok := pv.(types.Object); ok {
		stateObj, ok := sv.(types.Object)
		if !ok {
			return false
		}
		return planAttrsMatchState(planObj.Attributes(), stateObj.Attributes())
	}
	// Recurse into lists.
	if planList, ok := pv.(types.List); ok {
		stateList, ok := sv.(types.List)
		if !ok {
			return false
		}
		planElems := planList.Elements()
		stateElems := stateList.Elements()
		if len(planElems) != len(stateElems) {
			return false
		}
		for i := range planElems {
			if !attrValueMatchesState(planElems[i], stateElems[i]) {
				return false
			}
		}
		return true
	}
	return false
}

// resolveDomainMappings is the resolveNested hook for the domains list.
// It resolves computed fields inside the nested mappings list using target_mapping
// as the identity key.
func resolveDomainMappings(ctx context.Context, planAttrs, stateAttrs map[string]attr.Value) (map[string]attr.Value, bool) {
	planMappings, ok1 := planAttrs["mappings"].(types.List)
	stateMappings, ok2 := stateAttrs["mappings"].(types.List)
	if !ok1 || !ok2 {
		return planAttrs, false
	}
	newMappings, changed := resolveMappingsByIdentity(ctx, planMappings, stateMappings)
	if !changed {
		return planAttrs, false
	}
	out := shallowCopyAttrMap(planAttrs)
	out["mappings"] = newMappings
	return out, true
}

// resolveMappingsByIdentity resolves computed fields in a mappings list using
// target_mapping as the identity key. Order is preserved from the plan (mapping
// order is meaningful — it determines path-pattern precedence).
func resolveMappingsByIdentity(ctx context.Context, planList, stateList types.List) (types.List, bool) {
	// Build state lookup: target_mapping → attrs.
	stateByTM := make(map[string]map[string]attr.Value)
	for _, elem := range stateList.Elements() {
		obj, ok := elem.(types.Object)
		if !ok {
			continue
		}
		attrs := obj.Attributes()
		if tmid, ok := attrs["target_mapping"].(types.String); ok && !tmid.IsNull() && !tmid.IsUnknown() {
			stateByTM[tmid.ValueString()] = attrs
		}
	}

	planElems := planList.Elements()
	newElems := make([]attr.Value, len(planElems))
	anyChanged := false

	for i, elem := range planElems {
		obj, ok := elem.(types.Object)
		if !ok {
			newElems[i] = elem
			continue
		}
		planAttrs := obj.Attributes()

		tmid, ok := planAttrs["target_mapping"].(types.String)
		if !ok || tmid.IsNull() || tmid.IsUnknown() {
			newElems[i] = elem
			continue
		}

		stateAttrs, found := stateByTM[tmid.ValueString()]
		if !found {
			newElems[i] = elem
			continue
		}

		newAttrs := shallowCopyAttrMap(planAttrs)
		changed := false

		for _, field := range []string{"uuid", "path_pattern"} {
			if resolveUnknownString(newAttrs, stateAttrs, field) {
				changed = true
			}
		}

		if changed {
			newObj, diags := types.ObjectValue(obj.AttributeTypes(ctx), newAttrs)
			if diags.HasError() {
				newElems[i] = elem
			} else {
				newElems[i] = newObj
				anyChanged = true
			}
		} else {
			newElems[i] = elem
		}
	}

	if !anyChanged {
		return planList, false
	}
	newList, diags := types.ListValue(planList.ElementType(ctx), newElems)
	if diags.HasError() {
		return planList, false
	}
	return newList, true
}

// resolveUnknownString copies field from stateAttrs → planAttrs when the
// planned value is unknown and the state value is known. Returns true if copied.
func resolveUnknownString(planAttrs, stateAttrs map[string]attr.Value, field string) bool {
	pv, ok := planAttrs[field].(types.String)
	if !ok || !pv.IsUnknown() {
		return false
	}
	sv, ok := stateAttrs[field].(types.String)
	if !ok || sv.IsNull() || sv.IsUnknown() {
		return false
	}
	planAttrs[field] = sv
	return true
}

// shallowCopyAttrMap returns a shallow copy of an attr.Value map.
func shallowCopyAttrMap(m map[string]attr.Value) map[string]attr.Value {
	out := make(map[string]attr.Value, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// listNullClearsState is a plan modifier for Optional+Computed list attributes
// where omitting the block in HCL should mean "clear the list" rather than
// "keep whatever is in state". If the config value is null, the plan is kept
// null (provider will send an empty list to the API). If the config provides a
// value and the plan is unknown (Computed fill-in), copy the prior state value
// to avoid unnecessary diffs.
type listNullClearsState struct{}

func ListNullClearsStateModifier() planmodifier.List {
	return listNullClearsState{}
}

func (m listNullClearsState) Description(_ context.Context) string {
	return "If the configured value is null, plan null (clear). Otherwise use state for unknown."
}

func (m listNullClearsState) MarkdownDescription(_ context.Context) string {
	return m.Description(context.Background())
}

func (m listNullClearsState) PlanModifyList(ctx context.Context, req planmodifier.ListRequest, resp *planmodifier.ListResponse) {
	// Config is null → user wants to clear the list. Set plan to empty list so
	// it stays stable after Read (which may return [] from the API).
	if req.ConfigValue.IsNull() {
		resp.PlanValue = types.ListValueMust(req.PlanValue.ElementType(ctx), []attr.Value{})
		return
	}
	// Config has a value but plan is unknown (Computed fill-in) → use prior state.
	if req.PlanValue.IsUnknown() {
		if req.StateValue.IsNull() || req.StateValue.IsUnknown() {
			resp.PlanValue = types.ListValueMust(req.PlanValue.ElementType(ctx), []attr.Value{})
			return
		}
		resp.PlanValue = req.StateValue
	}
}

// ---------------------------------------------------------------------------
// conditionValueSynthesizer
//
// Generic plan modifier for the `values` attribute on WAF and behavior conditions.
// When the user writes the single-value shorthand `value = "x"` instead of
// `values = [...]`, this modifier reads the sibling `value` string from config
// and synthesizes `values = ["x"]` in the plan so the provider always sees a
// populated collection and state stays consistent.
// The same struct implements both planmodifier.Set (WAF conditions) and
// planmodifier.List (behavior conditions) — one shared implementation.
// ---------------------------------------------------------------------------

type conditionValueSynthesizer struct{}

// ConditionValuesSynthesizer returns the modifier typed as planmodifier.Set (WAF and behavior conditions).
func ConditionValuesSynthesizer() planmodifier.Set {
	return conditionValueSynthesizer{}
}

func (m conditionValueSynthesizer) Description(_ context.Context) string {
	return "Synthesizes values = [value] when the single-value shorthand is used."
}
func (m conditionValueSynthesizer) MarkdownDescription(_ context.Context) string {
	return m.Description(context.Background())
}

func (m conditionValueSynthesizer) PlanModifySet(ctx context.Context, req planmodifier.SetRequest, resp *planmodifier.SetResponse) {
	siblingPath := req.Path.ParentPath().AtName("value")
	var singleValue types.String
	if diags := req.Config.GetAttribute(ctx, siblingPath, &singleValue); diags.HasError() {
		return
	}
	if singleValue.IsNull() || singleValue.IsUnknown() {
		return // user used `values` directly — nothing to synthesize
	}
	setVal, diags := types.SetValueFrom(ctx, types.StringType, []string{singleValue.ValueString()})
	if !diags.HasError() {
		resp.PlanValue = setVal
	}
}

// ---------------------------------------------------------------------------
// conditionValueAlwaysNull
//
// Plan modifier for the `value` (string shorthand) attribute on WAF and behavior conditions.
// The provider always writes null for this field in state (the data is stored in
// `values` instead). This modifier forces the planned value to null so that
// state null == plan null, avoiding "inconsistent result after apply".
// ---------------------------------------------------------------------------

type conditionValueAlwaysNull struct{}

func ConditionValueAlwaysNull() planmodifier.String {
	return conditionValueAlwaysNull{}
}

func (m conditionValueAlwaysNull) Description(_ context.Context) string {
	return "Always null in plan/state — value is a write-once convenience alias stored in `values`."
}
func (m conditionValueAlwaysNull) MarkdownDescription(_ context.Context) string {
	return m.Description(context.Background())
}

func (m conditionValueAlwaysNull) PlanModifyString(_ context.Context, _ planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	resp.PlanValue = types.StringUnknown()
}
