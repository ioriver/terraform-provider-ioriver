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
var _ resource.Resource = &UrlSigningKeyResource{}
var _ resource.ResourceWithImportState = &UrlSigningKeyResource{}

func NewUrlSigningKeyResource() resource.Resource {
	return &UrlSigningKeyResource{}
}

type UrlSigningKeyResource struct{}

type UrlSigningKeyResourceModel struct {
	Id            types.String `tfsdk:"id"`
	Service       types.String `tfsdk:"service"`
	Name          types.String `tfsdk:"name"`
	PublicKey     types.String `tfsdk:"public_key"`
	EncryptionKey types.String `tfsdk:"encryption_key"`
	ProviderKeys  types.Map    `tfsdk:"provider_keys"`
}

func (r *UrlSigningKeyResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_url_signing_key"
}

func (r *UrlSigningKeyResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "UrlSigningKey resource",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "UrlSigningKey identifier",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"service": schema.StringAttribute{
				MarkdownDescription: "The id of the service this key belongs to",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Signing key name",
				Required:            true,
			},
			"public_key": schema.StringAttribute{
				MarkdownDescription: "Public key for URL signing",
				Required:            true,
				Sensitive:           true,
			},
			"encryption_key": schema.StringAttribute{
				MarkdownDescription: "Encryption key for URL signing",
				Required:            true,
				Sensitive:           true,
			},
			"provider_keys": schema.MapAttribute{
				MarkdownDescription: "Keys for each provider to be used by the backend to sign URLs.",
				Computed:            true,
				ElementType:         types.StringType,
			},
		},
	}
}

// Configure resource and retrieve API client
func (r *UrlSigningKeyResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// no-op: resource is deprecated, no client needed
}

// Create UrlSigningKey resource
func (r *UrlSigningKeyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	resp.Diagnostics.AddError(
		"ioriver resource is deprecated",
		"Please remove this resource from your configuration.\n"+
			"Any existing configuration remains set in ioriver, and can be imported to new resource.",
	)
}

// Read UrlSigningKey resource
func (r *UrlSigningKeyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// resource is deprecated: remove from state so Terraform stops tracking it
	resp.State.RemoveResource(ctx)
}

// Update UrlSigningKey resource
func (r *UrlSigningKeyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError(
		"ioriver resource is deprecated",
		"Please remove this resource from your configuration.\n"+
			"Any existing configuration remains set in ioriver, and can be imported to new resource.",
	)
}

// Delete UrlSigningKey resource
func (r *UrlSigningKeyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// no-op: resource is deprecated, Terraform will remove it from state automatically
}

// Import UrlSigningKey resource
func (r *UrlSigningKeyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.AddError(
		"ioriver resource is deprecated",
		"Please remove this resource from your configuration.\n"+
			"Any existing configuration remains set in ioriver, and can be imported to new resource.",
	)
}

// ------- Implement base Resource API ---------

func (UrlSigningKeyResource) create(ctx context.Context, client *ioriver.IORiverClient, newObj interface{}) (interface{}, error) {
	return nil, nil
}

func (UrlSigningKeyResource) read(ctx context.Context, client *ioriver.IORiverClient, id interface{}) (interface{}, error) {
	return nil, nil
}

func (UrlSigningKeyResource) update(ctx context.Context, client *ioriver.IORiverClient, obj interface{}) (interface{}, error) {
	return nil, nil
}

func (UrlSigningKeyResource) delete(ctx context.Context, client *ioriver.IORiverClient, id interface{}) error {
	return nil
}

func (UrlSigningKeyResource) getId(data interface{}) interface{} {
	return nil
}

// Convert UrlSigningKey resource to UrlSigningKey API object
func (UrlSigningKeyResource) resourceToObj(ctx context.Context, data interface{}) (interface{}, error) {
	return nil, nil
}

// Convert UrlSigningKey API object to UrlSigningKey resource
func (UrlSigningKeyResource) objToResource(ctx context.Context, obj interface{}, data interface{}) (interface{}, error) {
	return nil, nil
}
