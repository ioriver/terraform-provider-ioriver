package provider

import (
	"context"

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
	originShieldId string
	serviceId      string
}

type OriginShieldResource struct {
	client *ioriver.IORiverClient
}

type ProviderOriginShieldModel struct {
	ServiceProvider  types.String `tfsdk:"service_provider"`
	ProviderLocation types.String `tfsdk:"provider_location"`
}

type OriginShieldLocationModel struct {
	Country     types.String `tfsdk:"country"`
	Subdivision types.String `tfsdk:"subdivision"`
}

type OriginShieldResourceModel struct {
	Id        types.String                `tfsdk:"id"`
	Service   types.String                `tfsdk:"service"`
	Location  OriginShieldLocationModel   `tfsdk:"location"`
	Providers []ProviderOriginShieldModel `tfsdk:"providers"`
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
				MarkdownDescription: "OriginShield identifier",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"service": schema.StringAttribute{
				MarkdownDescription: "The id of the service this OriginShield belongs to",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"location": schema.SingleNestedAttribute{
				MarkdownDescription: "Location of the origin",
				Required:            true,
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
			"providers": schema.ListNestedAttribute{
				MarkdownDescription: "List of service provider within this policy",
				Required:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"service_provider": schema.StringAttribute{
							MarkdownDescription: "Service provider Id",
							Required:            true,
						},
						"provider_location": schema.StringAttribute{
							MarkdownDescription: "Origin-shield location for the provider",
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
	newData := resourceCreate(r.client, ctx, req, resp, r, data, false)
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

// Update OriginShield resource
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
	resourceDelete(r.client, ctx, req, resp, r, data)
}

// Import OriginShield resource
func (r *OriginShieldResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	serviceResourceImport(ctx, req, resp)
}

// ------- Implement base Resource API ---------

func (OriginShieldResource) create(client *ioriver.IORiverClient, newObj interface{}) (interface{}, error) {
	return client.CreateOriginShield(newObj.(ioriver.OriginShield))
}

func (OriginShieldResource) read(client *ioriver.IORiverClient, id interface{}) (interface{}, error) {
	resourceId := id.(OriginShieldResourceId)
	return client.GetOriginShield(resourceId.serviceId, resourceId.originShieldId)
}

func (OriginShieldResource) update(client *ioriver.IORiverClient, obj interface{}) (interface{}, error) {
	return client.UpdateOriginShield(obj.(ioriver.OriginShield))
}

func (OriginShieldResource) delete(client *ioriver.IORiverClient, id interface{}) error {
	resourceId := id.(OriginShieldResourceId)
	return client.DeleteOriginShield(resourceId.serviceId, resourceId.originShieldId)
}

func (OriginShieldResource) getId(data interface{}) interface{} {
	d := data.(OriginShieldResourceModel)
	originShieldId := d.Id.ValueString()
	serviceId := d.Service.ValueString()
	return OriginShieldResourceId{originShieldId, serviceId}
}

// Convert OriginShield resource to OriginShield API object
func (OriginShieldResource) resourceToObj(ctx context.Context, data interface{}) (interface{}, error) {
	d := data.(OriginShieldResourceModel)

	// convert location
	originLocation := ioriver.OriginShieldLocation{
		Country:     d.Location.Country.ValueString(),
		Subdivision: d.Location.Subdivision.ValueString(),
	}

	// convert providers
	originShieldProviders := []ioriver.ProviderOriginShield{}
	for _, provider := range d.Providers {
		originShieldProviders = append(originShieldProviders,
			ioriver.ProviderOriginShield{
				ServiceProvider: provider.ServiceProvider.ValueString(),
			})
	}

	return ioriver.OriginShield{
		Id:        d.Id.ValueString(),
		Service:   d.Service.ValueString(),
		Location:  originLocation,
		Providers: originShieldProviders,
	}, nil
}

// Convert OriginShield API object to OriginShield resource
func (OriginShieldResource) objToResource(ctx context.Context, obj interface{}) (interface{}, error) {
	originShield := obj.(*ioriver.OriginShield)

	// convert providers
	modelProviders := []ProviderOriginShieldModel{}
	for _, provider := range originShield.Providers {
		modelProviders = append(modelProviders,
			ProviderOriginShieldModel{
				ServiceProvider:  types.StringValue(provider.ServiceProvider),
				ProviderLocation: types.StringValue(provider.ProviderLocation),
			})
	}

	return OriginShieldResourceModel{
		Id:      types.StringValue(originShield.Id),
		Service: types.StringValue(originShield.Service),
		Location: OriginShieldLocationModel{
			Country:     types.StringValue(originShield.Location.Country),
			Subdivision: types.StringValue(originShield.Location.Subdivision),
		},
		Providers: modelProviders,
	}, nil
}
