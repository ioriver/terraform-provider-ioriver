package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// GeoFencingModel mirrors the backend's geo restriction config section — a
// sub-block of the service config, fully optional, with no resource of its own.
//
// NAMING ASYMMETRY: the user-facing HCL block and Go identifiers in this
// provider are called geo_fencing / GeoFencing, but the backend's wire
// contract still uses geo_restriction. The provider acts as a translation
// layer: tfsdk tag and schema map keys use the new name; the JSON keys
// written/read in config_model.go (ModelToMap / ServiceConfigMapToModel)
// keep the legacy wire name. If/when the backend is renamed, that single
// wire-key change unifies the two.
//
//   - countries: `set[str]`. Backend ENFORCES non-empty when the block is
//     present (HTTP 400 "Geo restriction countries list must not be empty").
//     We mirror that client-side with Required + SizeAtLeast(1) so the
//     failure surfaces at plan time rather than as an apply-time HTTP 400.
//     Backend also caps the set at 20 entries — we don't mirror that,
//     following provider convention for upper-bound caps (see
//     domain.aliases, origins, origin_sets, etc.).
//   - mode:      enum "allow" | "deny"; required when the block is present.
type GeoFencingModel struct {
	Countries types.Set    `tfsdk:"countries"`
	Mode      types.String `tfsdk:"mode"`
}

// geoFencingModes are the values accepted by the backend's mode enum.
var geoFencingModes = []string{"allow", "deny"}

// GeoFencingAttrTypes returns the attr.Type map for GeoFencingModel.
// Required by ConfigAttrTypes() so the geo_fencing block can be embedded
// in the parent config object.
func GeoFencingAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"countries": types.SetType{ElemType: types.StringType},
		"mode":      types.StringType,
	}
}

// GeoFencingAttributes returns the schema for the geo_fencing nested block.
// The block itself is Optional at the parent level (see ConfigAttributes);
// when present, BOTH sub-attributes are required.
func GeoFencingAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"countries": schema.SetAttribute{
			MarkdownDescription: "Set of ISO 3166-1 alpha-2 country codes (e.g. `[\"US\", \"DE\"]`). " +
				"Must contain at least one country",
			ElementType: types.StringType,
			Required:    true,
			Validators: []validator.Set{
				setvalidator.SizeAtLeast(1),
			},
		},
		"mode": schema.StringAttribute{
			MarkdownDescription: "Mode — `allow` (allow-list `countries`) or `deny` (deny-list `countries`).",
			Required:            true,
			Validators: []validator.String{
				stringvalidator.OneOf(geoFencingModes...),
			},
		},
	}
}

// ModelToMap serialises GeoFencingModel to the backend wire format.
// Returns nil when the receiver is nil so the caller can skip the
// "geo_restriction" key entirely (note: legacy wire name — see header) —
// preserving the backend's `optional=True` semantics (omitted block ≠
// empty block).
func (g *GeoFencingModel) ModelToMap(ctx context.Context) map[string]interface{} {
	if g == nil {
		return nil
	}

	out := make(map[string]interface{})

	// mode is Required inside the block — defensive guard for unknown plan values.
	if !g.Mode.IsNull() && !g.Mode.IsUnknown() && g.Mode.ValueString() != "" {
		out["mode"] = g.Mode.ValueString()
	}

	// countries is Required + SizeAtLeast(1) at the schema level, so a non-nil
	// receiver always carries at least one entry. Defensive read anyway.
	countries := []string{}
	if !g.Countries.IsNull() && !g.Countries.IsUnknown() {
		_ = g.Countries.ElementsAs(ctx, &countries, false)
	}
	out["countries"] = countries

	return out
}

// GeoFencingMapToModel deserialises a backend geo_restriction map (legacy wire
// name — see header) into a *GeoFencingModel. Returns nil when the input is
// nil so the caller can leave ServiceConfigModel.GeoFencing as nil — matching
// the user's HCL (no block ⇢ nil ⇢ no diff).
func GeoFencingMapToModel(ctx context.Context, data map[string]interface{}) *GeoFencingModel {
	if data == nil {
		return nil
	}

	g := &GeoFencingModel{
		Countries: types.SetNull(types.StringType),
		Mode:      types.StringNull(),
	}

	if mode, ok := data["mode"].(string); ok && mode != "" {
		g.Mode = types.StringValue(mode)
	}

	if raw, ok := data["countries"].([]interface{}); ok {
		strs := make([]string, 0, len(raw))
		for _, v := range raw {
			if s, ok := v.(string); ok {
				strs = append(strs, s)
			}
		}
		if set, diags := types.SetValueFrom(ctx, types.StringType, strs); !diags.HasError() {
			g.Countries = set
		}
	}

	return g
}
