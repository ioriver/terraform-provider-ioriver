package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	ioriver "github.com/ioriver/ioriver-go"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &OriginShieldResource{}
var _ resource.ResourceWithImportState = &OriginShieldResource{}
var _ resource.ResourceWithUpgradeState = &OriginShieldResource{}

func NewOriginShieldResource() resource.Resource {
	return &OriginShieldResource{}
}

type OriginShieldResource struct{}

func (r *OriginShieldResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_origin_shield"
}

func (r *OriginShieldResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		DeprecationMessage: "ioriver resource is deprecated, Please remove this resource from your configuration.\n" +
			"Any existing configuration remains set in ioriver, and can be imported to new resource.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Origin shield identifier",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"service": schema.StringAttribute{
				MarkdownDescription: "The id of the service this origin belongs to",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"origin": schema.StringAttribute{
				MarkdownDescription: "The id of the origin this origin-shield is related to",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"shield_location": schema.SingleNestedAttribute{
				MarkdownDescription: "Location of the origin shield",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"country": schema.StringAttribute{
						MarkdownDescription: "The country code in which the origin shield is located",
						Required:            true,
					},
					"subdivision": schema.StringAttribute{
						MarkdownDescription: "The subdivision code in which the origin shield is located. It is required when the country is US in order to specify US state",
						Optional:            true,
						Computed:            true,
					},
				},
			},
			"shield_providers": schema.ListNestedAttribute{
				MarkdownDescription: "List of service providers to enable origin-shield for",
				Optional:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"service_provider": schema.StringAttribute{
							MarkdownDescription: "Service provider Id",
							Required:            true,
						},
						"provider_location": schema.StringAttribute{
							MarkdownDescription: "Specific origin-shield location of the provider",
							Computed:            true,
						},
					},
				},
			},
		},
		// state version 1 - origin_shield resource is deprecated
		Version: 1,
	}
}

// Configure resource and retrieve API client
func (r *OriginShieldResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// no-op: resource is deprecated, no client needed
}

// Create OriginShield resource
func (r *OriginShieldResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	resp.Diagnostics.AddError(
		"ioriver resource is deprecated",
		"Please remove this resource from your configuration.\n"+
			"Any existing configuration remains set in ioriver, and can be imported to new resource.",
	)
}

// Read OriginShield resource
func (r *OriginShieldResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// resource is deprecated: remove from state so Terraform stops tracking it
	resp.State.RemoveResource(ctx)
}

// Update OriginShield resource
func (r *OriginShieldResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError(
		"ioriver resource is deprecated",
		"Please remove this resource from your configuration.\n"+
			"Any existing configuration remains set in ioriver, and can be imported to new resource.",
	)
}

// Delete OriginShield resource
func (r *OriginShieldResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// no-op: resource is deprecated, Terraform will remove it from state automatically
}

// Import OriginShield resource
func (r *OriginShieldResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.AddError(
		"ioriver resource is deprecated",
		"Please remove this resource from your configuration.\n"+
			"Any existing configuration remains set in ioriver, and can be imported to new resource.",
	)
}

func (r *OriginShieldResource) UpgradeState(ctx context.Context) map[int64]resource.StateUpgrader {
	return map[int64]resource.StateUpgrader{
		// State upgrade from 0 to 1: ioriver_origin_shield is deprecated, just remove from state
		0: {
			PriorSchema: &schema.Schema{
				Attributes: map[string]schema.Attribute{
					"id": schema.StringAttribute{
						Computed: true,
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.UseStateForUnknown(),
						},
					},
					"service": schema.StringAttribute{
						Required: true,
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.RequiresReplace(),
						},
					},
					"origin": schema.StringAttribute{
						Required: true,
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.RequiresReplace(),
						},
					},
					"shield_location": schema.SingleNestedAttribute{
						Optional: true,
						Attributes: map[string]schema.Attribute{
							"country": schema.StringAttribute{
								Required: true,
							},
							"subdivision": schema.StringAttribute{
								Optional: true,
								Computed: true,
							},
						},
					},
					"shield_providers": schema.ListNestedAttribute{
						Optional: true,
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"service_provider": schema.StringAttribute{
									Required: true,
								},
								"provider_location": schema.StringAttribute{
									Computed: true,
								},
							},
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

func (OriginShieldResource) create(ctx context.Context, client *ioriver.IORiverClient, newObj interface{}) (interface{}, error) {
	return nil, nil
}

func (OriginShieldResource) read(ctx context.Context, client *ioriver.IORiverClient, id interface{}) (interface{}, error) {
	return nil, nil
}

func (OriginShieldResource) update(ctx context.Context, client *ioriver.IORiverClient, obj interface{}) (interface{}, error) {
	return nil, nil
}

func (OriginShieldResource) delete(ctx context.Context, client *ioriver.IORiverClient, id interface{}) error {
	return nil
}

func (OriginShieldResource) getId(data interface{}) interface{} {
	return nil
}

func (OriginShieldResource) resourceToObj(ctx context.Context, data interface{}) (interface{}, error) {
	return nil, nil
}

func (OriginShieldResource) objToResource(ctx context.Context, obj interface{}, data interface{}) (interface{}, error) {
	return nil, nil
}
