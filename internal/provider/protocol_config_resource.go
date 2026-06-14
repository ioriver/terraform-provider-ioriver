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

type ProtocolConfigResource struct{}

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
	// no-op: resource is deprecated, no client needed
}

// Create ProtocolConfig resource
func (r *ProtocolConfigResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	resp.Diagnostics.AddError(
		"ioriver resource is deprecated",
		"Please remove this resource from your configuration.\n"+
			"Any existing configuration remains set in ioriver, and can be imported to new resource.",
	)
}

// Read ProtocolConfig resource
func (r *ProtocolConfigResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// resource is deprecated: remove from state so Terraform stops tracking it
	resp.State.RemoveResource(ctx)
}

// Update ProtocolConfig resource
func (r *ProtocolConfigResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError(
		"ioriver resource is deprecated",
		"Please remove this resource from your configuration.\n"+
			"Any existing configuration remains set in ioriver, and can be imported to new resource.",
	)
}

// Delete ProtocolConfig resource
func (r *ProtocolConfigResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// no-op: resource is deprecated, Terraform will remove it from state automatically
}

// Import ProtocolConfig resource
func (r *ProtocolConfigResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.AddError(
		"ioriver resource is deprecated",
		"Please remove this resource from your configuration.\n"+
			"Any existing configuration remains set in ioriver, and can be imported to new resource.",
	)
}

// ------- Implement base Resource API ---------

func (ProtocolConfigResource) create(ctx context.Context, client *ioriver.IORiverClient, newObj interface{}) (interface{}, error) {
	return nil, nil
}

func (ProtocolConfigResource) read(ctx context.Context, client *ioriver.IORiverClient, id interface{}) (interface{}, error) {
	return nil, nil
}

func (ProtocolConfigResource) update(ctx context.Context, client *ioriver.IORiverClient, obj interface{}) (interface{}, error) {
	return nil, nil
}

func (ProtocolConfigResource) delete(ctx context.Context, client *ioriver.IORiverClient, id interface{}) error {
	return nil
}

func (ProtocolConfigResource) getId(data interface{}) interface{} {
	return nil
}

// Convert ProtocolConfig resource to ProtocolConfig API object
func (ProtocolConfigResource) resourceToObj(ctx context.Context, data interface{}) (interface{}, error) {
	return nil, nil
}

// Convert ProtocolConfig API object to ProtocolConfig resource
func (ProtocolConfigResource) objToResource(ctx context.Context, obj interface{}, data interface{}) (interface{}, error) {
	return nil, nil
}
