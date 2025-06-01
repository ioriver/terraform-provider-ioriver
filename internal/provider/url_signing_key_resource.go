package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
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

type UrlSigningKeyResourceId struct {
	urlSigningKeyId string
	serviceId       string
}

type UrlSigningKeyResource struct {
	client *ioriver.IORiverClient
}

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
	client := ConfigureBase(ctx, req, resp)
	if client == nil {
		return
	}
	r.client = client
}

// Create UrlSigningKey resource
func (r *UrlSigningKeyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data UrlSigningKeyResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	newData := resourceCreate(r.client, ctx, req, resp, r, data, false)
	if newData == nil {
		return
	}

	// This resource has a couple of write-only fields which we need to preserve from original request
	newKey := newData.(UrlSigningKeyResourceModel)
	newKey.PublicKey = data.PublicKey
	newKey.EncryptionKey = data.EncryptionKey

	resp.Diagnostics.Append(resp.State.Set(ctx, &newKey)...)
}

// Read UrlSigningKey resource
func (r *UrlSigningKeyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data UrlSigningKeyResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	newData := resourceRead(r.client, ctx, req, resp, r, data)
	if newData == nil {
		return
	}

	// This resource has a couple of write-only fields which we need to preserve from original request
	newKey := newData.(UrlSigningKeyResourceModel)
	newKey.PublicKey = data.PublicKey
	newKey.EncryptionKey = data.EncryptionKey

	resp.Diagnostics.Append(resp.State.Set(ctx, &newKey)...)
}

// Update UrlSigningKey resource
func (r *UrlSigningKeyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data UrlSigningKeyResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	newData := resourceUpdate(r.client, ctx, req, resp, r, data)
	if newData == nil {
		return
	}

	// This resource has a couple of write-only fields which we need to preserve from original request
	newKey := newData.(UrlSigningKeyResourceModel)
	newKey.PublicKey = data.PublicKey
	newKey.EncryptionKey = data.EncryptionKey

	resp.Diagnostics.Append(resp.State.Set(ctx, &newKey)...)
}

// Delete UrlSigningKey resource
func (r *UrlSigningKeyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data UrlSigningKeyResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resourceDelete(r.client, ctx, req, resp, r, data)
}

// Import UrlSigningKey resource
func (r *UrlSigningKeyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	serviceResourceImport(ctx, req, resp)
}

// ------- Implement base Resource API ---------

func (UrlSigningKeyResource) create(ctx context.Context, client *ioriver.IORiverClient, newObj interface{}) (interface{}, error) {
	return client.CreateUrlSigningKey(newObj.(ioriver.UrlSigningKey))
}

func (UrlSigningKeyResource) read(ctx context.Context, client *ioriver.IORiverClient, id interface{}) (interface{}, error) {
	resourceId := id.(UrlSigningKeyResourceId)
	return client.GetUrlSigningKey(resourceId.serviceId, resourceId.urlSigningKeyId)
}

func (UrlSigningKeyResource) update(ctx context.Context, client *ioriver.IORiverClient, obj interface{}) (interface{}, error) {
	return client.UpdateUrlSigningKey(obj.(ioriver.UrlSigningKey))
}

func (UrlSigningKeyResource) delete(ctx context.Context, client *ioriver.IORiverClient, id interface{}) error {
	resourceId := id.(UrlSigningKeyResourceId)
	return client.DeleteUrlSigningKey(resourceId.serviceId, resourceId.urlSigningKeyId)
}

func (UrlSigningKeyResource) getId(data interface{}) interface{} {
	d := data.(UrlSigningKeyResourceModel)
	urlSigningKeyId := d.Id.ValueString()
	serviceId := d.Service.ValueString()
	return UrlSigningKeyResourceId{urlSigningKeyId, serviceId}
}

// Convert UrlSigningKey resource to UrlSigningKey API object
func (UrlSigningKeyResource) resourceToObj(ctx context.Context, data interface{}) (interface{}, error) {
	d := data.(UrlSigningKeyResourceModel)

	return ioriver.UrlSigningKey{
		Id:            d.Id.ValueString(),
		Service:       d.Service.ValueString(),
		Name:          d.Name.ValueString(),
		PublicKey:     d.PublicKey.ValueString(),
		EncryptionKey: d.PublicKey.ValueString(),
	}, nil
}

// Convert UrlSigningKey API object to UrlSigningKey resource
func (UrlSigningKeyResource) objToResource(ctx context.Context, obj interface{}) (interface{}, error) {
	urlSigningKey := obj.(*ioriver.UrlSigningKey)

	providerKeysMap := make(map[string]attr.Value, len(urlSigningKey.ProviderKeys))
	for k, v := range urlSigningKey.ProviderKeys {
		providerKeysMap[k] = types.StringValue(v)
	}

	providerKeys, diag := types.MapValue(types.StringType, providerKeysMap)
	if diag.HasError() {
		return nil, fmt.Errorf("error converting provider keys map")
	}

	return UrlSigningKeyResourceModel{
		Id:            types.StringValue(urlSigningKey.Id),
		Service:       types.StringValue(urlSigningKey.Service),
		Name:          types.StringValue(urlSigningKey.Name),
		PublicKey:     types.StringValue(""),
		EncryptionKey: types.StringValue(""),
		ProviderKeys:  providerKeys,
	}, nil
}
