package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

type ServiceConfigModel struct {
	UUId            types.String           `tfsdk:"uuid" json:"uuid"`
	Name            types.String           `tfsdk:"creation_name" json:"name"`
	ServiceUid      types.String           `tfsdk:"service_uid" json:"service_uid"`
	Protocol        *ProtocolConfigModel   `tfsdk:"protocol" json:"protocol,omitempty"`
	GeoFencing      *GeoFencingModel       `tfsdk:"geo_fencing" json:"geo_fencing,omitempty"`
	Domains         *[]DomainModel         `tfsdk:"domains" json:"domains,omitempty"`
	Origins         types.List             `tfsdk:"origins"`
	OriginSets      []OriginSetModel       `tfsdk:"origin_sets" json:"origin_sets,omitempty"`
	Behaviors       types.Object           `tfsdk:"behaviors"`
	LogDestinations *[]LogDestinationModel `tfsdk:"log_destinations" json:"log_destinations,omitempty"`
	Compute         *ComputeModel          `tfsdk:"compute" json:"compute,omitempty"`
	Security        types.Object           `tfsdk:"security" json:"security,omitempty"`
}

func ConfigAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"uuid":             types.StringType,
		"creation_name":    types.StringType,
		"service_uid":      types.StringType,
		"protocol":         types.ObjectType{AttrTypes: ProtocolAttrTypes()},
		"geo_fencing":      types.ObjectType{AttrTypes: GeoFencingAttrTypes()},
		"domains":          types.ListType{ElemType: types.ObjectType{AttrTypes: DomainAttrTypes()}},
		"origins":          types.ListType{ElemType: types.ObjectType{AttrTypes: GetOriginAttrTypes()}},
		"origin_sets":      types.ListType{ElemType: types.ObjectType{AttrTypes: OriginSetAttrTypes()}},
		"behaviors":        types.ObjectType{AttrTypes: BehaviorsBlockAttrTypes()},
		"log_destinations": types.ListType{ElemType: types.ObjectType{AttrTypes: LogDestinationAttrTypes()}},
		"security":         types.ObjectType{AttrTypes: SecurityAttrTypes()},
		"compute":          types.ObjectType{AttrTypes: ComputeAttrTypes()},
	}
}

// defaultBehaviorsBlockObjectValue is the zero-value behaviors block used when
// the user omits the behaviors block entirely.
var defaultBehaviorsBlockObjectValue = types.ObjectValueMust(
	BehaviorsBlockAttrTypes(),
	map[string]attr.Value{
		"default": defaultBehaviorObjectValue,
		"custom": types.ListValueMust(
			types.ObjectType{AttrTypes: BehaviorAttrTypes()},
			[]attr.Value{},
		),
	},
)

func ConfigAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"uuid": schema.StringAttribute{
			MarkdownDescription: "Unique identifier for the configuration",
			Computed:            true,
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.UseStateForUnknown(),
			},
		},
		"creation_name": schema.StringAttribute{
			MarkdownDescription: "Name of the service when it was created, const once set by backend",
			Computed:            true,
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.UseStateForUnknown(),
			},
		},
		"service_uid": schema.StringAttribute{
			MarkdownDescription: "Unique identifier for the service",
			Computed:            true,
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.UseStateForUnknown(),
			},
		},
		"protocol": schema.SingleNestedAttribute{
			MarkdownDescription: "Protocol configuration",
			Optional:            true,
			Computed:            true,
			Default:             objectdefault.StaticValue(defaultProtocolValue),
			Attributes:          ProtocolAttributes(),
		},
		"geo_fencing": schema.SingleNestedAttribute{
			MarkdownDescription: "Geo-fencing configuration — restrict access by viewer country " +
				"(allow-list or deny-list). Block is fully optional; omit it to apply no restriction. " +
				"When present, `mode` is required and `countries` defaults to `[]`.",
			Optional:   true,
			Attributes: GeoFencingAttributes(),
		},
		"domains": schema.ListNestedAttribute{
			MarkdownDescription: "Domain configuration",
			Optional:            true,
			Computed:            true,
			Default:             listdefault.StaticValue(types.ListValueMust(types.ObjectType{AttrTypes: DomainAttrTypes()}, []attr.Value{})),
			PlanModifiers: []planmodifier.List{
				DomainListPlanModifier(),
			},
			NestedObject: schema.NestedAttributeObject{
				Attributes: DomainAttributes(),
			},
		},
		"origins": schema.ListNestedAttribute{
			MarkdownDescription: "Origin configuration",
			Optional:            true,
			PlanModifiers: []planmodifier.List{
				NamedListPlanModifier("name"),
			},
			NestedObject: schema.NestedAttributeObject{
				Attributes: OriginAttributes(),
			},
		},
		"origin_sets": schema.ListNestedAttribute{
			MarkdownDescription: "Origin set configuration",
			Optional:            true,
			PlanModifiers: []planmodifier.List{
				OriginSetListPlanModifier(),
			},
			NestedObject: schema.NestedAttributeObject{
				Attributes: OriginSetAttributes(),
			},
		},
		"behaviors": schema.SingleNestedAttribute{
			MarkdownDescription: "Behavior configuration — contains the default behavior and the list of specific (custom) behaviors",
			Optional:            true,
			Computed:            true,
			Default:             objectdefault.StaticValue(defaultBehaviorsBlockObjectValue),
			Attributes: map[string]schema.Attribute{
				"custom": schema.ListNestedAttribute{
					MarkdownDescription: "Specific behaviors — evaluated in order, after the default Behavior",
					Optional:            true,
					Computed:            true,
					PlanModifiers: []planmodifier.List{
						ListNullClearsStateModifier(),
					},
					NestedObject: schema.NestedAttributeObject{
						Attributes: BehaviorAttributes(),
					},
				},
				"default": schema.SingleNestedAttribute{
					MarkdownDescription: "Default behavior — applies to all requests. \n" +
						"  - Optional and computed: omit to let the backend apply platform defaults.\n  - ",
					Optional:   true,
					Computed:   true,
					Default:    objectdefault.StaticValue(defaultBehaviorObjectValue),
					Attributes: DefaultBehaviorAttributes(),
				},
			},
		},
		"log_destinations": schema.ListNestedAttribute{
			MarkdownDescription: "Log destination configuration",
			Optional:            true,
			PlanModifiers: []planmodifier.List{
				NamedListPlanModifier("name"),
			},
			NestedObject: schema.NestedAttributeObject{
				Attributes: LogDestinationAttributes(),
			},
		},
		"security": schema.SingleNestedAttribute{
			MarkdownDescription: "Security configuration (WAF, custom rules, rate limiting)",
			Optional:            true,
			Computed:            true,
			Default:             objectdefault.StaticValue(defaultSecurityValue),
			Attributes:          SecurityAttributes(),
		},
		"compute": schema.SingleNestedAttribute{
			MarkdownDescription: "Compute configuration",
			Optional:            true,
			// Computed:            true,
			Attributes: ComputeAttributes(),
		},
	}
}

// Convert from terraform resource model to backend request format
func (c *ServiceConfigModel) ModelToMap(ctx context.Context, updateTransformCtx *ServiceTransformContext) (map[string]interface{}, error) {
	if c == nil {
		return nil, nil
	}

	configMap := make(map[string]interface{})

	if !c.UUId.IsNull() && c.UUId.ValueString() != "" {
		configMap["uuid"] = c.UUId.ValueString()
	}
	if !c.ServiceUid.IsNull() && c.ServiceUid.ValueString() != "" {
		configMap["service_uid"] = c.ServiceUid.ValueString()
	}
	if !c.Name.IsNull() && c.Name.ValueString() != "" {
		configMap["name"] = c.Name.ValueString()
	}

	// Convert Protocol
	if protocolMap := c.Protocol.ModelToMap(); c.Protocol != nil && protocolMap != nil {
		configMap["protocol"] = protocolMap
		tflog.Debug(ctx, fmt.Sprintf("[ModelToMap] ✓ Protocol converted: %+v\n", protocolMap))
	}

	// Convert GeoFencing — fully optional block; omit the key entirely when nil
	// so the backend's `optional=True` semantics are preserved (no block ≠ empty
	// block). Wire JSON key remains "geo_restriction" because the backend has not
	// been renamed (see geo_fencing_model.go header for the asymmetry rationale).
	if geoMap := c.GeoFencing.ModelToMap(ctx); c.GeoFencing != nil && geoMap != nil {
		configMap["geo_restriction"] = geoMap
		tflog.Debug(ctx, fmt.Sprintf("[ModelToMap] ✓ GeoFencing converted: %+v\n", geoMap))
	}

	// Convert Log Destinations - before behaviors (UUID must be known before behavior translation)
	logDestArray := []interface{}{}
	if c.LogDestinations != nil {
		logDestMaps, err := LogDestinationsToMap(ctx, c.LogDestinations, updateTransformCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to convert log destinations: %w", err)
		}
		for _, m := range logDestMaps {
			logDestArray = append(logDestArray, m)
		}
	}
	configMap["log_destinations"] = logDestArray
	tflog.Debug(ctx, fmt.Sprintf("[ModelToMap] ✓ Log Destinations converted: %+v", logDestArray))

	// Record whether the user explicitly set origins/origin_sets (even as [])
	// so ServiceConfigMapToModel can return [] instead of null when API returns nothing.
	if updateTransformCtx != nil {
		updateTransformCtx.OriginsExplicitlySet = !c.Origins.IsNull()
		updateTransformCtx.OriginSetsExplicitlySet = c.OriginSets != nil
	}

	// Convert Origins and add UUID to each
	var originsSlice *[]OriginModel
	if !c.Origins.IsNull() && !c.Origins.IsUnknown() {
		var singulars []OriginModel
		if diags := c.Origins.ElementsAs(ctx, &singulars, false); !diags.HasError() {
			originsSlice = &singulars
		}
	}
	originsArray, err := OriginsToMap(ctx, originsSlice, updateTransformCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to convert origins: %w", err)
	}
	if originsArray == nil {
		originsArray = []interface{}{}
	}
	configMap["origins"] = originsArray
	tflog.Debug(ctx, fmt.Sprintf("[ModelToMap] ✓ Origins converted: %+v\n", originsArray))

	// Convert OriginSets — must happen before Domains so namesToUUIDs is available
	originSetsArray, originSetNamesToUUIDs, err := OriginSetsToMap(ctx, c.OriginSets, updateTransformCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to convert origin_sets: %w", err)
	}
	configMap["origin_sets"] = originSetsArray
	tflog.Debug(ctx, fmt.Sprintf("[ModelToMap] ✓ OriginSets converted: %+v\n", originSetsArray))

	// Convert Domains — pass both origin and origin-set name→UUID maps
	domainsArray, err := DomainsToMap(ctx, c.Domains, updateTransformCtx, originSetNamesToUUIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to convert domains: %w", err)
	}
	if domainsArray == nil {
		domainsArray = []interface{}{}
	}
	configMap["domains"] = domainsArray
	tflog.Debug(ctx, fmt.Sprintf("[ModelToMap] ✓ Domains converted: %+v\n", domainsArray))

	// Convert Behaviors — extract default and custom from the behaviors block
	var defaultBehavior *DefaultBehaviorModel
	var specificBehaviors types.List
	if !c.Behaviors.IsNull() && !c.Behaviors.IsUnknown() {
		var b BehaviorsBlockModel
		diags := c.Behaviors.As(ctx, &b, basetypes.ObjectAsOptions{})
		if diags.HasError() {
			return nil, fmt.Errorf("failed to unmarshal behaviors: %v", diags)
		}
		if !b.Default.IsNull() && !b.Default.IsUnknown() {
			var m DefaultBehaviorModel
			if diags := b.Default.As(ctx, &m, basetypes.ObjectAsOptions{}); !diags.HasError() {
				defaultBehavior = &m
			}
		}
		specificBehaviors = b.Custom
	}
	behaviorsDict, err := BehaviorsToMap(ctx, defaultBehavior, specificBehaviors, updateTransformCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to convert behaviors: %w", err)
	}
	configMap["behaviors"] = behaviorsDict
	tflog.Debug(ctx, fmt.Sprintf("[ModelToMap] ✓ Behaviors converted: %+v\n", behaviorsDict))

	// Required fields - all collections must be arrays, never null
	addRequiredFieldsEmpty(configMap)

	// Security (WAF) - serialise only when set.
	// The backend stores everything under the "waf" key (custom, enabled,
	// checkpoint, limit_body_size, rate_limit all live inside "waf").
	if !c.Security.IsNull() && !c.Security.IsUnknown() {
		var sec SecurityModel
		if diags := c.Security.As(ctx, &sec, basetypes.ObjectAsOptions{}); !diags.HasError() {
			if wafApiMap := sec.SecurityModelToMap(ctx); wafApiMap != nil {
				configMap["waf"] = wafApiMap
			}
			// bot_management is a sibling of waf on the wire (matches the Python Config schema).
			if bmMap := sec.BotManagementToMap(ctx); bmMap != nil {
				configMap["bot_management"] = bmMap
			}
		}
	}

	return configMap, nil
}

func addRequiredFieldsEmpty(configMap map[string]interface{}) {
	// TODO - impl them when done having basic service running
	configMap["service_type"] = "generic"
	configMap["log_based_stats_enabled"] = true
	// configMap["compute"] = map[string]interface{}{}
	configMap["internal"] = map[string]interface{}{}
}

// Convert from backend response to terraform resource
func ServiceConfigMapToModel(
	ctx context.Context,
	configMap map[string]interface{},
	updateTransformCtx *ServiceTransformContext,
	planConfig *ServiceConfigModel) (*ServiceConfigModel, error) {

	if configMap == nil {
		return nil, nil
	}

	if planConfig != nil {
		tflog.Debug(ctx, fmt.Sprintf("[MapToModel] plan config: %+v\n", planConfig))
	}

	config := &ServiceConfigModel{}

	// Convert computed string fields
	if uuid, ok := configMap["uuid"].(string); ok {
		config.UUId = types.StringValue(uuid)
	}
	if uid, ok := configMap["service_uid"].(string); ok {
		config.ServiceUid = types.StringValue(uid)
	}
	if name, ok := configMap["name"].(string); ok {
		config.Name = types.StringValue(name)
	}

	// Origins, Domains, LogDestinations: always return &[] (never nil) so that
	// empty API responses produce [] in state.  This means:
	//   - import produces [] matching generated HCL → no diff after import
	//   - ListUseStateForUnknown copies [] from state to plan when user omits → no diff
	//   - null vs [] has no semantic difference for these fields at the backend level
	originsRaw, _ := configMap["origins"].([]interface{})
	if len(originsRaw) > 0 {
		originModels, err := OriginsFromMap(ctx, originsRaw, updateTransformCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to convert origins: %w", err)
		}

		// credentials_version is TF-only and never returned by the API.
		// Restore it from the prior config so state stays consistent with the plan.
		if planConfig != nil && !planConfig.Origins.IsNull() && !planConfig.Origins.IsUnknown() {
			var priorOrigins []OriginModel
			if diags := planConfig.Origins.ElementsAs(ctx, &priorOrigins, false); !diags.HasError() {
				priorVerByName := make(map[string]types.Int64)
				for _, o := range priorOrigins {
					if o.S3Origin != nil && !o.Name.IsNull() {
						priorVerByName[o.Name.ValueString()] = o.S3Origin.CredentialsVersion
					}
				}
				for i := range *originModels {
					o := &(*originModels)[i]
					if o.S3Origin != nil && !o.Name.IsNull() {
						if ver, ok := priorVerByName[o.Name.ValueString()]; ok {
							o.S3Origin.CredentialsVersion = ver
						}
					}
				}
			}
		}

		originsList, diags := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: GetOriginAttrTypes()}, *originModels)
		if diags.HasError() {
			return nil, fmt.Errorf("failed to build origins list")
		}
		config.Origins = originsList
	} else {
		// API returned no origins. Use null unless the user explicitly wrote
		// origins = [] — OriginsExplicitlySet is set by ModelToMap from the plan.
		if updateTransformCtx != nil && updateTransformCtx.OriginsExplicitlySet {
			emptyList, _ := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: GetOriginAttrTypes()}, []OriginModel{})
			config.Origins = emptyList
		} else {
			config.Origins = types.ListNull(types.ObjectType{AttrTypes: GetOriginAttrTypes()})
		}
	}

	// Convert OriginSets — must happen before Domains so uuidToOriginSetName is available
	uuidToOriginSetName := map[string]string{}
	originSetsRaw, _ := configMap["origin_sets"].([]interface{})
	if len(originSetsRaw) > 0 {
		originSetModels, uuidMap, err := OriginSetsFromMap(ctx, originSetsRaw, updateTransformCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to convert origin_sets: %w", err)
		}
		config.OriginSets = originSetModels
		uuidToOriginSetName = uuidMap
	} else {
		// nil (null) unless the user explicitly wrote origin_sets = [].
		if updateTransformCtx != nil && updateTransformCtx.OriginSetsExplicitlySet {
			config.OriginSets = []OriginSetModel{}
		} else {
			config.OriginSets = nil
		}
	}

	domainsRaw, _ := configMap["domains"].([]interface{})
	if len(domainsRaw) > 0 {
		domainModels, err := DomainsFromMap(ctx, domainsRaw, updateTransformCtx, uuidToOriginSetName)
		if err != nil {
			return nil, fmt.Errorf("failed to convert domains: %w", err)
		}
		config.Domains = domainModels
	} else {
		empty := []DomainModel{}
		config.Domains = &empty
	}

	// Convert LogDestinations FIRST — must populate LogDestNamesToUUIDs before behaviors
	// are parsed (translateStreamLogsToName needs uuid→name lookup during import too).
	logDestRaw, _ := configMap["log_destinations"].([]interface{})
	if len(logDestRaw) > 0 {
		logDestModels, err := LogDestinationsFromMap(ctx, logDestRaw, updateTransformCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to convert log destinations: %w", err)
		}

		// credentials_version is TF-only and never returned by the API.
		// Restore it from the prior config so state stays consistent with the plan.
		if planConfig != nil && planConfig.LogDestinations != nil {
			priorAwsByName := make(map[string]types.Int64)
			priorCompatByName := make(map[string]types.Int64)
			for _, ld := range *planConfig.LogDestinations {
				name := ld.Name.ValueString()
				if ld.AwsS3 != nil {
					priorAwsByName[name] = ld.AwsS3.CredentialsVersion
				}
				if ld.CompatibleS3 != nil {
					priorCompatByName[name] = ld.CompatibleS3.CredentialsVersion
				}
			}
			for i := range *logDestModels {
				ld := &(*logDestModels)[i]
				name := ld.Name.ValueString()
				if ld.AwsS3 != nil {
					if ver, ok := priorAwsByName[name]; ok {
						ld.AwsS3.CredentialsVersion = ver
					}
				}
				if ld.CompatibleS3 != nil {
					if ver, ok := priorCompatByName[name]; ok {
						ld.CompatibleS3.CredentialsVersion = ver
					}
				}
			}
		}

		config.LogDestinations = logDestModels
	} else {
		config.LogDestinations = nil
	}

	// Convert Behaviors - API returns dict, we need array.
	// ObjectNullClearsStateModifier handles the plan-time null when user omits the block.
	if behaviorsDict, ok := configMap["behaviors"].(map[string]interface{}); ok {
		tflog.Debug(ctx, fmt.Sprintf("[MapToModel] 🔍 Behaviors dict: %+v\n", behaviorsDict))
		behaviorModels, err := BehaviorsModelFromMap(ctx, behaviorsDict, updateTransformCtx, planConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to convert behaviors: %w", err)
		}

		// Build the default sub-object — only populate when the user explicitly configured it.
		var defaultObj types.Object
		if behaviorModels.Default != nil {
			objVal, diags := types.ObjectValueFrom(ctx, DefaultBehaviorAttrTypes(), behaviorModels.Default)
			if !diags.HasError() {
				defaultObj = objVal
			} else {
				defaultObj = types.ObjectNull(DefaultBehaviorAttrTypes())
			}
		} else {
			return nil, fmt.Errorf("failed to receive default behavior object from backend response")
		}

		// Assemble the behaviors block.
		behaviorsBlock := BehaviorsBlockModel{
			Default: defaultObj,
			Custom:  behaviorModels.AllMatch,
		}
		behaviorsObjVal, diags := types.ObjectValueFrom(ctx, BehaviorsBlockAttrTypes(), behaviorsBlock)
		if !diags.HasError() {
			config.Behaviors = behaviorsObjVal
		} else {
			config.Behaviors = types.ObjectNull(BehaviorsBlockAttrTypes())
		}
	} else {
		config.Behaviors = types.ObjectNull(BehaviorsBlockAttrTypes())
		tflog.Debug(ctx, "[MapToModel] ⚠️  No behaviors dict found or wrong type\n")
	}
	tflog.Debug(ctx, fmt.Sprintf("[MapToModel] ✓ Behaviors converted: %+v\n", config.Behaviors))

	// Convert Protocol
	if protocolMap, ok := configMap["protocol"].(map[string]interface{}); ok {
		tflog.Debug(ctx, fmt.Sprintf("[MapToModel] Received protocol map: %+v\n", protocolMap))
		config.Protocol = ProtocolConfigMapToModel(ctx, protocolMap)
	}
	tflog.Debug(ctx, fmt.Sprintf("[MapToModel] ✓ Protocol converted: %+v\n", config.Protocol))

	// Convert GeoFencing — absent in API response ⇢ keep nil in state (no diff
	// vs. HCL omitting the block). Only populate when the backend actually sent
	// a map. Wire JSON key remains "geo_restriction" — see geo_fencing_model.go
	// header for the TF/wire naming asymmetry.
	if geoMap, ok := configMap["geo_restriction"].(map[string]interface{}); ok {
		tflog.Debug(ctx, fmt.Sprintf("[MapToModel] Received geo_restriction map: %+v\n", geoMap))
		config.GeoFencing = GeoFencingMapToModel(ctx, geoMap)
	}
	tflog.Debug(ctx, fmt.Sprintf("[MapToModel] ✓ GeoFencing converted: %+v\n", config.GeoFencing))

	// TODO: Convert Compute similarly

	// Security (WAF).
	// The backend always returns a waf block (even with empty defaults).
	// Only populate Security when the user explicitly configured it (SecurityConfigured=true).
	// This prevents null→non-null inconsistency when the user omits the block entirely.
	if wafRaw, ok := configMap["waf"]; ok && wafRaw != nil {
		if updateTransformCtx != nil && updateTransformCtx.SecurityConfigured {
			var priorSec *SecurityModel
			if planConfig != nil && !planConfig.Security.IsNull() && !planConfig.Security.IsUnknown() {
				var s SecurityModel
				if diags := planConfig.Security.As(ctx, &s, basetypes.ObjectAsOptions{}); !diags.HasError() {
					priorSec = &s
				}
			}
			sec, err := securityMapToModelWithCtx(ctx, wafRaw, updateTransformCtx, priorSec)
			if err != nil {
				return nil, fmt.Errorf("failed to parse security map: %v", err)
			}
			if sec != nil {
				// bot_management is a top-level sibling of waf on the wire.
				if bmRaw, ok := configMap["bot_management"]; ok && bmRaw != nil {
					sec.BotManagement = BotManagementMapToModel(ctx, bmRaw)
				}
				objVal, diags := types.ObjectValueFrom(ctx, SecurityAttrTypes(), sec)
				if diags.HasError() {
					config.Security = types.ObjectNull(SecurityAttrTypes())
				} else {
					config.Security = objVal
				}
				tflog.Debug(ctx, fmt.Sprintf("[MapToModel] waf populated: enabled=%v custom=%d rate_limit=%d",
					sec.Enabled.ValueBool(), len(sec.CustomRules), len(sec.RateLimit)))
			} else {
				config.Security = types.ObjectNull(SecurityAttrTypes())
			}
		} else {
			config.Security = types.ObjectNull(SecurityAttrTypes())
			tflog.Debug(ctx, "[MapToModel] security not configured by user — skipping")
		}
	} else {
		config.Security = types.ObjectNull(SecurityAttrTypes())
	}

	return config, nil
}
