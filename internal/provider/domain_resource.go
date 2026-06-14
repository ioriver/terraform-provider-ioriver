package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	ioriver "github.com/ioriver/ioriver-go"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &DomainResource{}
var _ resource.ResourceWithImportState = &DomainResource{}

func NewDomainResource() resource.Resource {
	return &DomainResource{}
}

type DomainResource struct{}

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
	// no-op: resource is deprecated, no client needed
}

// Create Domain resource
func (r *DomainResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	resp.Diagnostics.AddError(
		"ioriver resource is deprecated",
		"Please remove this resource from your configuration.\n"+
			"Any existing configuration remains set in ioriver, and can be imported to new resource.",
	)
}

// Read Domain resource
func (r *DomainResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// resource is deprecated: remove from state so Terraform stops tracking it
	resp.State.RemoveResource(ctx)
}

// Update Domain resource
func (r *DomainResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError(
		"ioriver resource is deprecated",
		"Please remove this resource from your configuration.\n"+
			"Any existing configuration remains set in ioriver, and can be imported to new resource.",
	)
}

// Delete Domain resource
func (r *DomainResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// no-op: resource is deprecated, Terraform will remove it from state automatically
}

// Import Domain resource
func (r *DomainResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.AddError(
		"ioriver resource is deprecated",
		"Please remove this resource from your configuration.\n"+
			"Any existing configuration remains set in ioriver, and can be imported to new resource.",
	)
}

func (r *DomainResource) UpgradeState(ctx context.Context) map[int64]resource.StateUpgrader {
	return map[int64]resource.StateUpgrader{
		// State upgrade from 0 to 1: ioriver_domain is deprecated, just remove from state
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
				resp.Diagnostics.AddError(
					"ioriver resource is deprecated",
					"Please remove this resource from your configuration.\n"+
						"Any existing configuration remains set in ioriver, and can be imported to new resource.",
				)
			},
		},
	}
}

// ------- Implement base Resource API ---------

func (DomainResource) create(ctx context.Context, client *ioriver.IORiverClient, newObj interface{}) (interface{}, error) {
	return nil, nil
}

func (DomainResource) read(ctx context.Context, client *ioriver.IORiverClient, id interface{}) (interface{}, error) {
	return nil, nil
}

func (DomainResource) update(ctx context.Context, client *ioriver.IORiverClient, obj interface{}) (interface{}, error) {
	return nil, nil
}

func (DomainResource) delete(ctx context.Context, client *ioriver.IORiverClient, id interface{}) error {
	return nil
}

func (DomainResource) getId(data interface{}) interface{} {
	return nil
}

// Convert Domain resource to Domain API object
func (DomainResource) resourceToObj(ctx context.Context, data interface{}) (interface{}, error) {
	return nil, nil
}

// Convert Domain API object to Domain resource
func (DomainResource) objToResource(ctx context.Context, obj interface{}, data interface{}) (interface{}, error) {
	return nil, nil
}
