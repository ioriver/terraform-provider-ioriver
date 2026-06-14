package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listdefault"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// OriginSetOriginModel is used for origins embedded inside an origin_set.
// It has no Name (anonymous) and no Shield (shield is on the set itself).
type OriginSetOriginModel struct {
	Uuid         types.String       `tfsdk:"uuid"`
	Path         types.String       `tfsdk:"path"`
	VerifySSL    types.Bool         `tfsdk:"verify_ssl"`
	TimeoutMs    types.Int64        `tfsdk:"timeout_ms"`
	SNIHostname  types.String       `tfsdk:"sni_hostname"`
	CustomOrigin *CustomOriginModel `tfsdk:"custom_origin"`
	S3Origin     *S3OriginModel     `tfsdk:"s3_origin"`
}

// toOriginModel converts an OriginSetOriginModel to a full OriginModel,
// injecting the set-level shield so ModelToMap() can serialise it correctly.
func (o *OriginSetOriginModel) toOriginModel(shield *OriginShieldModel) OriginModel {
	return OriginModel{
		Uuid:         o.Uuid,
		Name:         types.StringValue(""),
		Path:         o.Path,
		VerifySSL:    o.VerifySSL,
		TimeoutMs:    o.TimeoutMs,
		SNIHostname:  o.SNIHostname,
		Shield:       shield,
		CustomOrigin: o.CustomOrigin,
		S3Origin:     o.S3Origin,
	}
}

// OriginSetModel is the TF model for an origin set.
type OriginSetModel struct {
	Uuid                  types.String           `tfsdk:"uuid"`
	Name                  types.String           `tfsdk:"name"`
	FailoverResponseCodes types.List             `tfsdk:"failover_response_codes"`
	Origins               []OriginSetOriginModel `tfsdk:"origins"`
	Shield                *OriginShieldModel     `tfsdk:"shield"`
}

func (o OriginSetModel) GetName() string {
	return o.Name.ValueString()
}

func OriginSetAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"uuid":                    types.StringType,
		"name":                    types.StringType,
		"failover_response_codes": types.ListType{ElemType: types.Int64Type},
		"origins":                 types.ListType{ElemType: types.ObjectType{AttrTypes: GetOriginSetOriginAttrTypes()}},
		"shield": types.ObjectType{
			AttrTypes: map[string]attr.Type{
				"location": types.ObjectType{
					AttrTypes: map[string]attr.Type{
						"country":     types.StringType,
						"subdivision": types.StringType,
					},
				},
				"providers": types.SetType{ElemType: types.StringType},
			},
		},
	}
}

// defaultFailoverCodes is the backend default when none are specified.
var defaultFailoverCodes = []attr.Value{
	types.Int64Value(500),
	types.Int64Value(502),
	types.Int64Value(503),
	types.Int64Value(504),
}

// OriginSetOriginAttributes returns schema attributes for origins embedded in an
// origin_set — base fields only, no "name" (anonymous) and no "shield" (set-level).
func OriginSetOriginAttributes() map[string]schema.Attribute {
	return originBaseAttributes()
}

func OriginSetAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"uuid": schema.StringAttribute{
			MarkdownDescription: "Origin set UUID (managed by system)",
			Computed:            true,
			// We Do NOT use UseStateForUnknown() here.
			// The OriginSetListPlanModifier resolves uuid from state
		},
		"name": schema.StringAttribute{
			MarkdownDescription: "Origin set name (referenced by domain mappings)",
			Required:            true,
		},
		"failover_response_codes": schema.ListAttribute{
			MarkdownDescription: "HTTP response codes from the primary origin that trigger failover to the secondary.\n" +
				"  - Defaults to [500, 502, 503, 504].",
			ElementType: types.Int64Type,
			Optional:    true,
			Computed:    true,
			Default: listdefault.StaticValue(
				types.ListValueMust(types.Int64Type, defaultFailoverCodes),
			),
		},
		"origins": schema.ListNestedAttribute{
			MarkdownDescription: "Exactly two origins: index 0 = primary, index 1 = failover",
			Required:            true,
			NestedObject: schema.NestedAttributeObject{
				Attributes: OriginSetOriginAttributes(),
			},
		},
		"shield": schema.SingleNestedAttribute{
			MarkdownDescription: "Origin shield shared by all origins in the set.\n" +
				"  - Collapses requests at a chosen PoP before hitting the origin.",
			Optional:   true,
			Attributes: shieldNestedAttributes(),
		},
	}
}

// OriginSetsToMap converts []OriginSetModel to the backend array format.
// It also returns a name→UUID map so that DomainsToMap can resolve
// target_mapping names to UUIDs in the same request.
func OriginSetsToMap(ctx context.Context, originSets []OriginSetModel, updateTransformCtx *ServiceTransformContext) ([]interface{}, map[string]string, error) {
	namesToUUIDs := make(map[string]string)
	result := make([]interface{}, 0, len(originSets))
	newDesiredOrder := make([]string, 0, len(originSets))

	for i := range originSets {
		os := &originSets[i]

		// Ensure UUID exists (generate if new)
		if os.Uuid.IsNull() || os.Uuid.IsUnknown() || os.Uuid.ValueString() == "" {
			os.Uuid = types.StringValue(GenerateUUID())
		}
		uuid := os.Uuid.ValueString()
		name := os.Name.ValueString()

		// Serialise origins — inject set-level shield via toOriginModel, then use existing ModelToMap.
		originsArray := make([]interface{}, 0, len(os.Origins))
		for j := range os.Origins {
			om := os.Origins[j].toOriginModel(os.Shield)
			originMap, err := om.ModelToMap()
			// Write the generated UUID back so state stays stable across applies.
			os.Origins[j].Uuid = om.Uuid
			if err != nil {
				return nil, nil, fmt.Errorf("origin_set %q origin[%d]: %w", name, j, err)
			}
			originsArray = append(originsArray, originMap)
		}

		// Collect failover_response_codes
		codes := []int64{}
		if !os.FailoverResponseCodes.IsNull() && !os.FailoverResponseCodes.IsUnknown() {
			if diags := os.FailoverResponseCodes.ElementsAs(ctx, &codes, false); diags.HasError() {
				return nil, nil, fmt.Errorf("origin_set %q: failed to read failover_response_codes", name)
			}
		}
		codesInterface := make([]interface{}, len(codes))
		for k, c := range codes {
			codesInterface[k] = c
		}

		osMap := map[string]interface{}{
			"uuid":    uuid,
			"name":    name,
			"type":    "failover", // only supported type; hardcoded
			"origins": originsArray,
			"failover_config": map[string]interface{}{
				"failover_response_codes": codesInterface,
			},
		}
		result = append(result, osMap)
		namesToUUIDs[name] = uuid
		newDesiredOrder = append(newDesiredOrder, name)
	}

	updateTransformCtx.DesiredOriginSetOrder = newDesiredOrder
	return result, namesToUUIDs, nil
}

// OriginSetsFromMap converts the backend array to []OriginSetModel.
// It also returns a UUID→name map for use in DomainsFromMap.
func OriginSetsFromMap(ctx context.Context, raw []interface{}, updateTransformCtx *ServiceTransformContext) ([]OriginSetModel, map[string]string, error) {
	uuidToName := make(map[string]string)
	models := make([]OriginSetModel, 0, len(raw))
	desiredOrder := &updateTransformCtx.DesiredOriginSetOrder

	for _, item := range raw {
		osMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		var m OriginSetModel

		if uuid, ok := osMap["uuid"].(string); ok {
			m.Uuid = types.StringValue(uuid)
		}
		name, ok := osMap["name"].(string)
		if !ok || name == "" {
			// name is stored in the backend payload — missing means API contract violation
			return nil, nil, fmt.Errorf("origin_set missing required field 'name' in API response (uuid=%s)", m.Uuid.ValueString())
		}
		m.Name = types.StringValue(name)

		// Record UUID→name for domain mapping resolution
		if m.Uuid.ValueString() != "" && name != "" {
			uuidToName[m.Uuid.ValueString()] = name
		}

		// Deserialise failover_response_codes
		codes := defaultFailoverCodes
		if fc, ok := osMap["failover_config"].(map[string]interface{}); ok {
			if rawCodes, ok := fc["failover_response_codes"].([]interface{}); ok && len(rawCodes) > 0 {
				codes = make([]attr.Value, 0, len(rawCodes))
				for _, v := range rawCodes {
					switch n := v.(type) {
					case float64:
						codes = append(codes, types.Int64Value(int64(n)))
					case int64:
						codes = append(codes, types.Int64Value(n))
					}
				}
			}
		}
		m.FailoverResponseCodes = types.ListValueMust(types.Int64Type, codes)

		// Deserialise origins (reuse originFromMap).
		// Shield is the same on every origin in the set; lift it to the set level
		// from origin[0] and clear it on each individual origin.
		if originsRaw, ok := osMap["origins"].([]interface{}); ok {
			for i, o := range originsRaw {
				oMap, ok := o.(map[string]interface{})
				if !ok {
					continue
				}
				origin, err := originFromMap(ctx, oMap)
				if err != nil {
					return nil, nil, fmt.Errorf("origin_set %q: %w", name, err)
				}
				// Name is not in the backend payload for origin-set origins.
				// Lift shield from the first origin up to the set model.
				if i == 0 {
					m.Shield = origin.Shield
				}
				// Store as OriginSetOriginModel (no name, no shield).
				m.Origins = append(m.Origins, OriginSetOriginModel{
					Uuid:         origin.Uuid,
					Path:         origin.Path,
					VerifySSL:    origin.VerifySSL,
					TimeoutMs:    origin.TimeoutMs,
					SNIHostname:  origin.SNIHostname,
					CustomOrigin: origin.CustomOrigin,
					S3Origin:     origin.S3Origin,
				})
			}
		}

		models = append(models, m)
	}

	reordered := alignItems(models, *desiredOrder)
	newDesiredOrder := make([]string, 0, len(reordered))
	for _, os := range reordered {
		newDesiredOrder = append(newDesiredOrder, os.Name.ValueString())
	}
	*desiredOrder = newDesiredOrder

	return reordered, uuidToName, nil
}
