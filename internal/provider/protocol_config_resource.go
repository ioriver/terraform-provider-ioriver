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
var _ resource.Resource = &ProtocolConfigResource{}
var _ resource.ResourceWithImportState = &ProtocolConfigResource{}

func NewProtocolConfigResource() resource.Resource {
	return &ProtocolConfigResource{}
}

type ProtocolConfigResourceId struct {
	protocolConfigId string
	serviceId        string
}

type ProtocolConfigResource struct {
	client *ioriver.IORiverClient
}

type ProtocolConfigShieldLocationModel struct {
	Country     types.String `tfsdk:"country"`
	Subdivision types.String `tfsdk:"subdivision"`
}

type ProtocolConfigShieldProviderModel struct {
	ServiceProvider  types.String `tfsdk:"service_provider"`
	ProviderLocation types.String `tfsdk:"provider_location"`
}

type ProtocolConfigResourceModel struct {
	Id           types.String `tfsdk:"id"`
	Service      types.String `tfsdk:"service"`
	Http2Enabled types.Bool   `tfsdk:"http2_enabled"`
	Http3Enabled types.Bool   `tfsdk:"http3_enabled"`
	Ipv6Enabled  types.Bool   `tfsdk:"ipv6_enabled"`
}

func (r *ProtocolConfigResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_protocol_config"
}

func (r *ProtocolConfigResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "ProtocolConfig resource",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "ProtocolConfig identifier",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"service": schema.StringAttribute{
				MarkdownDescription: "The id of the service this protocolConfig belongs to",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"http2_enabled": schema.BoolAttribute{
				MarkdownDescription: "Is HTTP/2 enabled for this service",
				Required:            true,
			},
			"http3_enabled": schema.BoolAttribute{
				MarkdownDescription: "Is HTTP/3 enabled for this service",
				Required:            true,
			},
			"ipv6_enabled": schema.BoolAttribute{
				MarkdownDescription: "Is IPv6 enabled for this service",
				Required:            true,
			},
		},
	}
}

// Configure resource and retrieve API client
func (r *ProtocolConfigResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	client := ConfigureBase(ctx, req, resp)
	if client == nil {
		return
	}
	r.client = client
}

// Create ProtocolConfig resource
func (r *ProtocolConfigResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ProtocolConfigResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	newData := resourceCreate(r.client, ctx, req, resp, r, data, false)
	if newData == nil {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newData)...)
}

// Read ProtocolConfig resource
func (r *ProtocolConfigResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ProtocolConfigResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	newData := resourceRead(r.client, ctx, req, resp, r, data)
	if newData == nil {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newData)...)
}

// Update ProtocolConfig resource
func (r *ProtocolConfigResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data ProtocolConfigResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	newData := resourceUpdate(r.client, ctx, req, resp, r, data)
	if newData == nil {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newData)...)
}

// Delete ProtocolConfig resource
func (r *ProtocolConfigResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ProtocolConfigResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resourceDelete(r.client, ctx, req, resp, r, data)
}

// Import ProtocolConfig resource
func (r *ProtocolConfigResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	serviceResourceImport(ctx, req, resp)
}

// ------- Implement base Resource API ---------

func (ProtocolConfigResource) create(client *ioriver.IORiverClient, newObj interface{}) (interface{}, error) {
	return client.CreateProtocolConfig(newObj.(ioriver.ProtocolConfig))
}

func (ProtocolConfigResource) read(client *ioriver.IORiverClient, id interface{}) (interface{}, error) {
	resourceId := id.(ProtocolConfigResourceId)
	return client.GetProtocolConfig(resourceId.serviceId, resourceId.protocolConfigId)
}

func (ProtocolConfigResource) update(client *ioriver.IORiverClient, obj interface{}) (interface{}, error) {
	return client.UpdateProtocolConfig(obj.(ioriver.ProtocolConfig))
}

func (ProtocolConfigResource) delete(client *ioriver.IORiverClient, id interface{}) error {
	resourceId := id.(ProtocolConfigResourceId)
	return client.DeleteProtocolConfig(resourceId.serviceId, resourceId.protocolConfigId)
}

func (ProtocolConfigResource) getId(data interface{}) interface{} {
	d := data.(ProtocolConfigResourceModel)
	protocolConfigId := d.Id.ValueString()
	serviceId := d.Service.ValueString()
	return ProtocolConfigResourceId{protocolConfigId, serviceId}
}

// Convert ProtocolConfig resource to ProtocolConfig API object
func (ProtocolConfigResource) resourceToObj(ctx context.Context, data interface{}) (interface{}, error) {
	d := data.(ProtocolConfigResourceModel)

	return ioriver.ProtocolConfig{
		Id:           d.Id.ValueString(),
		Service:      d.Service.ValueString(),
		Http2Enabled: d.Http2Enabled.ValueBool(),
		Http3Enabled: d.Http3Enabled.ValueBool(),
		Ipv6Enabled:  d.Ipv6Enabled.ValueBool(),
	}, nil
}

// Convert ProtocolConfig API object to ProtocolConfig resource
func (ProtocolConfigResource) objToResource(ctx context.Context, obj interface{}) (interface{}, error) {
	protocolConfig := obj.(*ioriver.ProtocolConfig)

	return ProtocolConfigResourceModel{
		Id:           types.StringValue(protocolConfig.Id),
		Service:      types.StringValue(protocolConfig.Service),
		Http2Enabled: types.BoolValue(protocolConfig.Http2Enabled),
		Http3Enabled: types.BoolValue(protocolConfig.Http3Enabled),
		Ipv6Enabled:  types.BoolValue(protocolConfig.Ipv6Enabled),
	}, nil
}
