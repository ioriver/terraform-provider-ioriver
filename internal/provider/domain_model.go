package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

type DomainMappingModelV1 struct {
	PathPattern   types.String `tfsdk:"path_pattern"`
	TargetMapping types.String `tfsdk:"target_mapping"`
	TargetType    types.String `tfsdk:"target_type"`
}

type DomainModel struct {
	UUId     types.String           `tfsdk:"uuid"`
	Domain   types.String           `tfsdk:"domain"`
	Aliases  types.List             `tfsdk:"aliases"`
	Mappings []DomainMappingModelV1 `tfsdk:"mappings"`
}

func (d DomainModel) GetName() string {
	return d.Domain.ValueString()
}

func (m DomainMappingModelV1) GetName() string {
	return m.PathPattern.ValueString()
}

func DomainMappingAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"path_pattern":   types.StringType,
		"target_mapping": types.StringType,
		"target_type":    types.StringType,
	}
}

func DomainAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"uuid":     types.StringType,
		"domain":   types.StringType,
		"aliases":  types.ListType{ElemType: types.StringType},
		"mappings": types.ListType{ElemType: types.ObjectType{AttrTypes: DomainMappingAttrTypes()}},
	}
}

func DomainAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"uuid": schema.StringAttribute{
			Computed: true,
			// We Do NOT use UseStateForUnknown() here.
			// The DomainListPlanModifier resolves uuid from state by domain name
		},
		"domain": schema.StringAttribute{
			MarkdownDescription: "Domain name",
			Required:            true,
			Validators: []validator.String{
				stringvalidator.LengthBetween(1, 253),
			},
		},
		"aliases": schema.ListAttribute{
			MarkdownDescription: "A list of domain aliases",
			ElementType:         types.StringType,
			Optional:            true,
			Computed:            true,
			Default:             listdefault.StaticValue(types.ListValueMust(types.StringType, []attr.Value{})),
			PlanModifiers: []planmodifier.List{
				ListNullClearsStateModifier(),
			},
		},
		"mappings": schema.ListNestedAttribute{
			MarkdownDescription: "A list of mappings between path pattern and target.\n" +
				"  - Order of paths are performed by ioriver internally.",
			Required: true,
			PlanModifiers: []planmodifier.List{
				NamedListPlanModifier("target_mapping"),
			},
			NestedObject: schema.NestedAttributeObject{
				Attributes: map[string]schema.Attribute{
					"path_pattern": schema.StringAttribute{
						MarkdownDescription: "Path pattern within the domain",
						Optional:            true,
						Computed:            true,
						Default:             stringdefault.StaticString("/*"),
					},
					"target_mapping": schema.StringAttribute{
						MarkdownDescription: "Id of the origin / origin-set",
						Required:            true,
					},
					"target_type": schema.StringAttribute{
						MarkdownDescription: "Type of the target: origin or origin_set",
						Optional:            true,
						Computed:            true,
						Default:             stringdefault.StaticString("origin"),
						Validators: []validator.String{
							stringvalidator.OneOf("origin", "origin_set"),
						},
					},
				},
			},
		},
	}
}

func DomainsToMap(ctx context.Context, domains *[]DomainModel, updateTransformCtx *ServiceTransformContext, originSetNamesToUUIDs map[string]string) ([]interface{}, error) {
	if domains == nil {
		return nil, nil
	}

	domainsArray := make([]interface{}, 0)
	newDesiredOrder := []string{}
	if updateTransformCtx.DesiredMappingOrder == nil {
		updateTransformCtx.DesiredMappingOrder = map[string][]string{}
	}
	for _, domain := range *domains {
		domainApiMap, err := domain.ModelToMap(ctx, updateTransformCtx.OriginNamesToUUIDs, originSetNamesToUUIDs)
		if err != nil {
			return nil, fmt.Errorf("failed to convert domain: %w", err)
		}
		domainsArray = append(domainsArray, domainApiMap)
		newDesiredOrder = append(newDesiredOrder, domain.Domain.ValueString())
		// Record the desired mapping order for this domain (by path_pattern — unique per mapping).
		mappingOrder := make([]string, 0, len(domain.Mappings))
		for _, m := range domain.Mappings {
			mappingOrder = append(mappingOrder, m.PathPattern.ValueString())
		}
		updateTransformCtx.DesiredMappingOrder[domain.Domain.ValueString()] = mappingOrder
	}
	updateTransformCtx.DesiredDomainOrder = newDesiredOrder

	return domainsArray, nil
}

func (d *DomainModel) ModelToMap(ctx context.Context, originNamesToUUIDs map[string]string, originSetNamesToUUIDs map[string]string) (map[string]interface{}, error) {
	if d == nil {
		return nil, nil
	}

	domainMap := make(map[string]interface{})

	// Send the UUID so the backend can update the existing domain (not create a new one).
	if !d.UUId.IsNull() && !d.UUId.IsUnknown() && d.UUId.ValueString() != "" {
		domainMap["uuid"] = d.UUId.ValueString()
	}

	// convert domain - domain is required so should always have a value
	domainMap["domain"] = d.Domain.ValueString()

	// convert aliases from types.List to []string
	aliasesArray := []string{}
	if !d.Aliases.IsNull() && !d.Aliases.IsUnknown() {
		diags := d.Aliases.ElementsAs(ctx, &aliasesArray, false)
		if diags.HasError() {
			return nil, fmt.Errorf("failed to convert aliases: %v", diags.Errors())
		}
	}
	// Always include aliases (even if empty array)
	domainMap["aliases"] = aliasesArray

	// convert mappings
	mappingsArray := make([]interface{}, 0)
	for _, mapping := range d.Mappings {
		mappingMap, err := mapping.ModelToMap(ctx, originNamesToUUIDs, originSetNamesToUUIDs)
		if err != nil {
			return nil, fmt.Errorf("failed to convert mapping for domain %q: %w", d.Domain.ValueString(), err)
		}
		mappingsArray = append(mappingsArray, mappingMap)
	}
	domainMap["mappings"] = mappingsArray

	return domainMap, nil
}

// DomainMappingModel.ModelToMap - convert mapping to map
func (m *DomainMappingModelV1) ModelToMap(ctx context.Context, originNamesToUUIDs map[string]string, originSetNamesToUUIDs map[string]string) (map[string]interface{}, error) {
	if m == nil {
		return nil, nil
	}

	mappingMap := make(map[string]interface{})

	targetMapping := m.TargetMapping.ValueString()
	targetType := m.TargetType.ValueString()

	// Choose the right name→UUID map based on target_type
	var uuid string
	var found bool
	if targetType == "origin_set" {
		uuid, found = originSetNamesToUUIDs[targetMapping]
		if !found {
			return nil, fmt.Errorf("target_mapping %q not found in origin_sets", targetMapping)
		}
	} else {
		uuid, found = originNamesToUUIDs[targetMapping]
		if !found {
			return nil, fmt.Errorf("target_mapping %q not found in origins", targetMapping)
		}
	}
	mappingMap["target_id"] = uuid

	if pattern := m.PathPattern.ValueString(); pattern != "" {
		mappingMap["path_pattern"] = pattern
	}
	mappingMap["target_type"] = targetType

	return mappingMap, nil
}

func DomainsFromMap(ctx context.Context, domainsArray []interface{}, updateTransformCtx *ServiceTransformContext, uuidToOriginSetName map[string]string) (*[]DomainModel, error) {
	domains := []DomainModel{}
	desiredDomainOrder := &updateTransformCtx.DesiredDomainOrder

	// Build reverse lookup: UUID -> origin name
	uuidToOriginName := make(map[string]string)
	for name, uuid := range updateTransformCtx.OriginNamesToUUIDs {
		uuidToOriginName[uuid] = name
	}

	for _, domainMap := range domainsArray {
		domainModel, err := domainFromMap(ctx, domainMap.(map[string]interface{}), uuidToOriginName, uuidToOriginSetName, updateTransformCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to convert domain: %w", err)
		}
		domains = append(domains, domainModel)
	}

	reordered := alignItems(domains, *desiredDomainOrder)
	newDesiredOrder := make([]string, 0, len(reordered))
	for _, d := range reordered {
		newDesiredOrder = append(newDesiredOrder, d.Domain.ValueString())
	}
	*desiredDomainOrder = newDesiredOrder

	return &reordered, nil
}

func domainFromMap(ctx context.Context, domainMap map[string]interface{}, uuidToOriginName map[string]string, uuidToOriginSetName map[string]string, updateTransformCtx *ServiceTransformContext) (DomainModel, error) {
	tflog.Debug(ctx, fmt.Sprintf("[domainFromMap] Received domain map: %+v", domainMap))
	var domainModel DomainModel

	// Convert uuid
	if uuid, ok := domainMap["uuid"].(string); ok {
		domainModel.UUId = types.StringValue(uuid)
	}

	// Convert domain
	var domain = ""
	if domain, ok := domainMap["domain"].(string); ok {
		domainModel.Domain = types.StringValue(domain)
	} else {
		return domainModel, fmt.Errorf("missing or invalid domain")
	}

	// Convert aliases
	if aliases, ok := domainMap["aliases"].([]interface{}); ok {
		aliasElements := []types.String{}
		for _, alias := range aliases {
			if aliasStr, ok := alias.(string); ok {
				aliasElements = append(aliasElements, types.StringValue(aliasStr))
			}
		}
		aliasListValue, diags := types.ListValueFrom(ctx, types.StringType, aliasElements)
		if diags.HasError() {
			return domainModel, fmt.Errorf("failed to convert aliases: %v", diags.Errors())
		}
		domainModel.Aliases = aliasListValue
	}

	// Convert mappings
	if mappings, ok := domainMap["mappings"].([]interface{}); ok {
		for _, mapping := range mappings {
			if mappingMap, ok := mapping.(map[string]interface{}); ok {
				mappingModel, err := domainMappingFromMap(ctx, mappingMap, uuidToOriginName, uuidToOriginSetName, domain)
				if err != nil {
					return domainModel, fmt.Errorf("failed to convert mapping: %w", err)
				}
				domainModel.Mappings = append(domainModel.Mappings, mappingModel)
			}
		}
	}

	// Reorder mappings to match the HCL-declared order (by path_pattern — unique within a domain).
	if updateTransformCtx != nil && updateTransformCtx.DesiredMappingOrder != nil {
		if desiredOrder, ok := updateTransformCtx.DesiredMappingOrder[domainModel.Domain.ValueString()]; ok {
			domainModel.Mappings = alignItems(domainModel.Mappings, desiredOrder)
		}
	}

	tflog.Debug(ctx, fmt.Sprintf("[domainFromMap] Converted domain: %+v", domainModel))
	return domainModel, nil
}

func domainMappingFromMap(ctx context.Context, mappingMap map[string]interface{}, uuidToOriginName map[string]string, uuidToOriginSetName map[string]string, domain string) (DomainMappingModelV1, error) {
	var mapping DomainMappingModelV1

	// Extract UUID if present (ignored — not tracked in state)
	// if uuid, ok := mappingMap["uuid"].(string); ok {
	// 	mapping.UUId = types.StringValue(uuid)
	// }

	// Extract path_pattern; default to "/*" when the backend omits it (matches schema Default)
	if pathPattern, ok := mappingMap["path_pattern"].(string); ok {
		mapping.PathPattern = types.StringValue(pathPattern)
	} else {
		mapping.PathPattern = types.StringValue("/*")
	}

	// Extract target_type first so we know which map to look up
	targetType := "origin"
	if tt, ok := mappingMap["target_type"].(string); ok {
		targetType = tt
	}
	mapping.TargetType = types.StringValue(targetType)

	// Resolve target_id UUID back to the user-facing name
	if targetID, ok := mappingMap["target_id"].(string); ok {
		if targetType == "origin_set" {
			if name, found := uuidToOriginSetName[targetID]; found {
				mapping.TargetMapping = types.StringValue(name)
			} else {
				return mapping, fmt.Errorf("origin_set with uuid %q not found in origin_sets mapping", targetID)
			}
		} else {
			if name, found := uuidToOriginName[targetID]; found {
				mapping.TargetMapping = types.StringValue(name)
			} else {
				return mapping, fmt.Errorf("origin with uuid %q not found in origins mapping", targetID)
			}
		}
	} else {
		return mapping, fmt.Errorf("field target_id not found in API response for domain %q", domain)
	}

	return mapping, nil
}
