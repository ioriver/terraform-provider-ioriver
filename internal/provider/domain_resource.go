package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	ioriver "github.com/ioriver/ioriver-go"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &DomainResource{}
var _ resource.ResourceWithImportState = &DomainResource{}

func NewDomainResource() resource.Resource {
	return &DomainResource{}
}

type DomainResourceId struct {
	domainId  string
	serviceId string
}

type DomainResource struct {
	client *ioriver.IORiverClient
}

type DomainResourceModelV0 struct {
	Id           types.String `tfsdk:"id"`
	Service      types.String `tfsdk:"service"`
	Domain       types.String `tfsdk:"domain"`
	PathPattern  types.String `tfsdk:"path_pattern"`
	Origin       types.String `tfsdk:"origin"`
	LoadBalancer types.String `tfsdk:"load_balancer"`
}

type DomainMappingModel struct {
	PathPattern types.String `tfsdk:"path_pattern"`
	TargetId    types.String `tfsdk:"target_id"`
	TargetType  types.String `tfsdk:"target_type"`
}

type DomainResourceModel struct {
	Id       types.String         `tfsdk:"id"`
	Service  types.String         `tfsdk:"service"`
	Domain   types.String         `tfsdk:"domain"`
	Aliases  types.List           `tfsdk:"aliases"`
	Mappings []DomainMappingModel `tfsdk:"mappings"`
}

func (r *DomainResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_domain"
}

func (r *DomainResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Domain resource",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Domain identifier",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"service": schema.StringAttribute{
				MarkdownDescription: "The id of the service this domain belongs to",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"domain": schema.StringAttribute{
				MarkdownDescription: "Domain name",
				Required:            true,
			},
			"aliases": schema.ListAttribute{
				MarkdownDescription: "A list of domain aliases",
				ElementType:         types.StringType,
				Optional:            true,
				Computed:            true,
			},
			"mappings": schema.ListNestedAttribute{
				MarkdownDescription: "A list of mappings between path pattern and target (origin/load-balancer)",
				Required:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"path_pattern": schema.StringAttribute{
							MarkdownDescription: "Path pattern within the domain to be mapped with the Domain",
							Optional:            true,
							Computed:            true,
							Default:             stringdefault.StaticString("/*"),
						},
						"target_id": schema.StringAttribute{
							MarkdownDescription: "Id of the target (Id of origin/load-balancer)",
							Required:            true,
						},
						"target_type": schema.StringAttribute{
							MarkdownDescription: "Type of the taget: origin or load-balancer",
							Optional:            true,
							Computed:            true,
							Default:             stringdefault.StaticString("origin"),
							Validators: []validator.String{
								stringvalidator.OneOf([]string{"origin", "load-balancer"}...),
							},
						},
					},
				},
			},
		},
		// state version 1 - domain resource with aliases and all mappings included
		Version: 1,
	}
}

// Configure resource and retrieve API client
func (r *DomainResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	client := ConfigureBase(ctx, req, resp)
	if client == nil {
		return
	}
	r.client = client
}

// Create Domain resource
func (r *DomainResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data DomainResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	newData := resourceCreate(r.client, ctx, req, resp, r, data, false)
	if newData == nil {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newData)...)
}

// Read Domain resource
func (r *DomainResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data DomainResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	newData := resourceRead(r.client, ctx, req, resp, r, data)
	if newData == nil {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newData)...)
}

// Update Domain resource
func (r *DomainResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data DomainResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	newData := resourceUpdate(r.client, ctx, req, resp, r, data)
	if newData == nil {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newData)...)
}

// Delete Domain resource
func (r *DomainResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data DomainResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resourceDelete(r.client, ctx, req, resp, r, data)
}

// Import Domain resource
func (r *DomainResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	serviceResourceImport(ctx, req, resp)
}

func (r *DomainResource) UpgradeState(ctx context.Context) map[int64]resource.StateUpgrader {
	return map[int64]resource.StateUpgrader{
		// State upgrade implementation from 0 (prior state version) to 1 (Schema.Version)
		0: {
			PriorSchema: &schema.Schema{
				Attributes: map[string]schema.Attribute{
					"id": schema.StringAttribute{
						Computed:            true,
						MarkdownDescription: "Domain identifier",
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.UseStateForUnknown(),
						},
					},
					"service": schema.StringAttribute{
						MarkdownDescription: "The id of the service this domain belongs to",
						Required:            true,
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.RequiresReplace(),
						},
					},
					"domain": schema.StringAttribute{
						MarkdownDescription: "Domain name",
						Required:            true,
					},
					"path_pattern": schema.StringAttribute{
						MarkdownDescription: "Path pattern within the domain to be mapped with the Domain",
						Optional:            true,
						Computed:            true,
					},
					"origin": schema.StringAttribute{
						MarkdownDescription: "Origin id to forward traffic to",
						Optional:            true,
						Computed:            true,
						Validators: []validator.String{
							stringvalidator.ExactlyOneOf(path.Expressions{
								path.MatchRoot("load_balancer"),
							}...),
						},
					},
					"load_balancer": schema.StringAttribute{
						MarkdownDescription: "Load balancer id to forward traffic to",
						Optional:            true,
						Computed:            true,
						Validators: []validator.String{
							stringvalidator.ExactlyOneOf(path.Expressions{
								path.MatchRoot("origin"),
							}...),
						},
					},
				},
			},
			StateUpgrader: func(ctx context.Context, req resource.UpgradeStateRequest, resp *resource.UpgradeStateResponse) {
				var priorStateData DomainResourceModelV0

				resp.Diagnostics.Append(req.State.Get(ctx, &priorStateData)...)

				if resp.Diagnostics.HasError() {
					return
				}

				serviceId := priorStateData.Service.ValueString()

				tflog.Info(ctx, fmt.Sprintf("Upgrade service: %s", serviceId))

				aliases, _ := types.ListValueFrom(ctx, types.StringType, []string{})

				// The domain has a different id than the old domain mapping instance
				// Need to identify of the domain by comparing the mapping and get the id.
				var modelMappings []DomainMappingModel
				serviceDomains, err := r.client.ListDomains(serviceId)
				if err != nil {
					resp.Diagnostics.AddError("Failed to list domains", err.Error())
					return
				}

				domainId := ""
				for _, d := range serviceDomains {
					for _, m := range d.Mappings {
						if m.PathPattern == priorStateData.PathPattern.ValueString() &&
							m.TargetId == priorStateData.Origin.ValueString() {
							domainId = d.Id
							break
						}
					}

					if domainId != "" {
						break
					}
				}

				if domainId == "" {
					resp.Diagnostics.AddError("Failed to find matching domain for domain mapping ", priorStateData.Id.String())
					return
				}

				upgradedStateData := DomainResourceModel{
					Id:       types.StringValue(domainId),
					Service:  priorStateData.Service,
					Domain:   priorStateData.Domain,
					Aliases:  aliases,
					Mappings: modelMappings,
				}

				resp.Diagnostics.Append(resp.State.Set(ctx, upgradedStateData)...)
			},
		},
	}
}

// ------- Implement base Resource API ---------

func (DomainResource) create(ctx context.Context, client *ioriver.IORiverClient, newObj interface{}) (interface{}, error) {
	return client.CreateDomain(newObj.(ioriver.Domain))
}

func (DomainResource) read(ctx context.Context, client *ioriver.IORiverClient, id interface{}) (interface{}, error) {
	resourceId := id.(DomainResourceId)
	return client.GetDomain(resourceId.serviceId, resourceId.domainId)
}

func (DomainResource) update(ctx context.Context, client *ioriver.IORiverClient, obj interface{}) (interface{}, error) {
	return client.UpdateDomain(obj.(ioriver.Domain))
}

func (DomainResource) delete(ctx context.Context, client *ioriver.IORiverClient, id interface{}) error {
	resourceId := id.(DomainResourceId)
	return client.DeleteDomain(resourceId.serviceId, resourceId.domainId)
}

func (DomainResource) getId(data interface{}) interface{} {
	d := data.(DomainResourceModel)
	domainId := d.Id.ValueString()
	serviceId := d.Service.ValueString()
	return DomainResourceId{domainId, serviceId}
}

// Convert Domain resource to Domain API object
func (DomainResource) resourceToObj(ctx context.Context, data interface{}) (interface{}, error) {
	d := data.(DomainResourceModel)

	mappings := []ioriver.DomainMappings{}
	for _, mapping := range d.Mappings {
		mappings = append(mappings,
			ioriver.DomainMappings{
				PathPattern: mapping.PathPattern.ValueString(),
				TargetId:    mapping.TargetId.ValueString(),
				TargetType:  mapping.TargetType.ValueString(),
			})
	}

	aliases := make([]string, 0, len(d.Aliases.Elements()))
	d.Aliases.ElementsAs(ctx, &aliases, false)

	return ioriver.Domain{
		Id:       d.Id.ValueString(),
		Service:  d.Service.ValueString(),
		Domain:   d.Domain.ValueString(),
		Mappings: mappings,
		Aliases:  aliases,
	}, nil
}

// Convert Domain API object to Domain resource
func (DomainResource) objToResource(ctx context.Context, obj interface{}) (interface{}, error) {
	domain := obj.(*ioriver.Domain)

	var modelMappings []DomainMappingModel
	for _, mapping := range domain.Mappings {
		modelMappings = append(modelMappings,
			DomainMappingModel{
				PathPattern: types.StringValue(mapping.PathPattern),
				TargetId:    types.StringValue(mapping.TargetId),
				TargetType:  types.StringValue(mapping.TargetType),
			})
	}

	aliases, _ := types.ListValueFrom(ctx, types.StringType, domain.Aliases)

	domainModel := DomainResourceModel{
		Id:       types.StringValue(domain.Id),
		Service:  types.StringValue(domain.Service),
		Domain:   types.StringValue(domain.Domain),
		Aliases:  aliases,
		Mappings: modelMappings,
	}

	return domainModel, nil
}
