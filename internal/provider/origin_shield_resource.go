package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	ioriver "github.com/ioriver/ioriver-go"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &OriginShieldResource{}
var _ resource.ResourceWithImportState = &OriginShieldResource{}

func NewOriginShieldResource() resource.Resource {
	return &OriginShieldResource{}
}

type OriginShieldResourceId struct {
	originId  string
	serviceId string
}

type OriginShieldResource struct {
	client *ioriver.IORiverClient
}

type OriginShieldLocationModel struct {
	Country     types.String `tfsdk:"country"`
	Subdivision types.String `tfsdk:"subdivision"`
}

type OriginShieldProviderModel struct {
	ServiceProvider  types.String `tfsdk:"service_provider"`
	ProviderLocation types.String `tfsdk:"provider_location"`
}

type OriginShieldResourceModel struct {
	Id              types.String                `tfsdk:"id"`
	Service         types.String                `tfsdk:"service"`
	Origin          types.String                `tfsdk:"origin"`
	ShieldLocation  *OriginShieldLocationModel  `tfsdk:"shield_location"`
	ShieldProviders []OriginShieldProviderModel `tfsdk:"shield_providers"`
}

func (r *OriginShieldResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_origin_shield"
}

func (r *OriginShieldResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "OriginShield resource",

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
	}
}

// Configure resource and retrieve API client
func (r *OriginShieldResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	client := ConfigureBase(ctx, req, resp)
	if client == nil {
		return
	}
	r.client = client
}

// Create OriginShield resource
func (r *OriginShieldResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data OriginShieldResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	doUpdate := true
	newData := resourceCreate(r.client, ctx, req, resp, r, data, doUpdate)
	if newData == nil {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newData)...)
}

// Read OriginShield resource
func (r *OriginShieldResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data OriginShieldResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	newData := resourceRead(r.client, ctx, req, resp, r, data)
	if newData == nil {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newData)...)
}

// Update Origin resource
func (r *OriginShieldResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data OriginShieldResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	newData := resourceUpdate(r.client, ctx, req, resp, r, data)
	if newData == nil {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newData)...)
}

// Delete OriginShield resource
func (r *OriginShieldResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data OriginShieldResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// deleting origin shield means removing it from the origin
	origin, err := r.getOrigin(data.Service.ValueString(), data.Origin.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error getting origin", "Unexpected error: "+err.Error())
		return
	}

	origin.ShieldLocation = nil
	origin.ShieldProviders = nil

	// the update operation must be called under the global mutex
	updateOp := func() (interface{}, error) {
		return r.client.UpdateOrigin(*origin)
	}
	_, err = performOperation(func() (interface{}, error) { return updateOp() })
	if err != nil {
		resp.Diagnostics.AddError("Error updating origin", "Unexpected error: "+err.Error())
	}
}

// Import OriginShield resource
func (r *OriginShieldResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	idParts := strings.Split(req.ID, ",")
	if len(idParts) != 2 || idParts[0] == "" || idParts[1] == "" {
		resp.Diagnostics.AddError(
			"Unexpected Import Identifier",
			fmt.Sprintf("Expected import identifier with format: service-id,origin-id. Got: %q", req.ID),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("service"), idParts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("origin"), idParts[1])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), idParts[1])...)
}

// ------- Implement base Resource API ---------

func (OriginShieldResource) create(ctx context.Context, client *ioriver.IORiverClient, newObj interface{}) (interface{}, error) {
	return nil, fmt.Errorf("unexpected create of origin shield object")
}

func (OriginShieldResource) read(ctx context.Context, client *ioriver.IORiverClient, id interface{}) (interface{}, error) {
	resourceId := id.(OriginShieldResourceId)
	return client.GetOrigin(resourceId.serviceId, resourceId.originId)
}

func (OriginShieldResource) update(ctx context.Context, client *ioriver.IORiverClient, obj interface{}) (interface{}, error) {
	return client.UpdateOrigin(obj.(ioriver.Origin))
}

func (OriginShieldResource) delete(ctx context.Context, client *ioriver.IORiverClient, id interface{}) error {
	return fmt.Errorf("unexpected delete of origin shield object")
}

func (OriginShieldResource) getId(data interface{}) interface{} {
	d := data.(OriginShieldResourceModel)
	originId := d.Origin.ValueString()
	serviceId := d.Service.ValueString()
	return OriginShieldResourceId{originId, serviceId}
}

// Convert OriginShield resource to OriginShield API object
func (r OriginShieldResource) resourceToObj(ctx context.Context, data interface{}) (interface{}, error) {
	d := data.(OriginShieldResourceModel)

	originId := d.Origin.ValueString()
	serviceId := d.Service.ValueString()

	origin, err := r.getOrigin(serviceId, originId)
	if err != nil {
		return nil, err
	}

	// convert shield-location
	var shieldLocation *ioriver.OriginShieldLocation
	if d.ShieldLocation != nil {
		shieldLocation = &ioriver.OriginShieldLocation{
			Country:     d.ShieldLocation.Country.ValueString(),
			Subdivision: d.ShieldLocation.Subdivision.ValueString(),
		}
	}

	// convert shield-providers
	shieldProviders := []ioriver.OriginShieldProvider{}
	for _, provider := range d.ShieldProviders {
		shieldProviders = append(shieldProviders,
			ioriver.OriginShieldProvider{
				ServiceProvider: provider.ServiceProvider.ValueString(),
			})
	}

	origin.ShieldLocation = shieldLocation
	origin.ShieldProviders = shieldProviders

	return *origin, nil
}

// Convert Origin API object to OriginShield resource
func (OriginShieldResource) objToResource(ctx context.Context, obj interface{}) (interface{}, error) {
	origin := obj.(*ioriver.Origin)

	// convert providers
	var modelShieldProviders []OriginShieldProviderModel
	for _, provider := range origin.ShieldProviders {
		modelShieldProviders = append(modelShieldProviders,
			OriginShieldProviderModel{
				ServiceProvider:  types.StringValue(provider.ServiceProvider),
				ProviderLocation: types.StringValue(provider.ProviderLocation),
			})
	}

	var shieldLocation *OriginShieldLocationModel
	if origin.ShieldLocation != nil {
		shieldLocation = &OriginShieldLocationModel{
			Country:     types.StringValue(origin.ShieldLocation.Country),
			Subdivision: types.StringValue(origin.ShieldLocation.Subdivision),
		}
	}

	return OriginShieldResourceModel{
		Id:              types.StringValue(origin.Id), // use origin id since shield doesn't have an id
		Service:         types.StringValue(origin.Service),
		Origin:          types.StringValue(origin.Id),
		ShieldLocation:  shieldLocation,
		ShieldProviders: modelShieldProviders,
	}, nil
}

func (r OriginShieldResource) getOrigin(serviceId string, originId string) (*ioriver.Origin, error) {
	origin, err := r.client.GetOrigin(serviceId, originId)
	if err != nil {
		return nil, err
	}

	return origin, nil
}
