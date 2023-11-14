package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	ioriver "ioriver.io/ioriver/ioriver-go"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &AccountProviderResource{}
var _ resource.ResourceWithImportState = &AccountProviderResource{}

func NewAccountProviderResource() resource.Resource {
	return &AccountProviderResource{}
}

type AccountProviderResourceId = string
type AccountProviderResource struct {
	client *ioriver.IORiverClient
}

type CloudfrontCredsModel struct {
	AccessKey    types.String `tfsdk:"access_key"`
	AccessSecret types.String `tfsdk:"access_secret"`
}

type CredentialsModel struct {
	Fastly     types.String          `tfsdk:"fastly"`
	Cloudflare types.String          `tfsdk:"cloudflare"`
	Cloudfront *CloudfrontCredsModel `tfsdk:"cloudfront"`
}

type AccountProviderResourceModel struct {
	Id           types.String      `tfsdk:"id"`
	ProviderName types.String      `tfsdk:"provider_name"`
	Credentials  *CredentialsModel `tfsdk:"credentials"`
}

func (r *AccountProviderResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_account_provider"
}

func (r *AccountProviderResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "AccountProvider resource",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Account-Provider identifier",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"provider_name": schema.StringAttribute{
				MarkdownDescription: "Account-Provider provider name",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.OneOf([]string{"fastly", "cloudflare", "cloudfront", "azure_cdn", "akamai"}...),
				},
			},
			"credentials": schema.SingleNestedAttribute{
				MarkdownDescription: "Account-Provider credentials",
				Required:            true,
				Sensitive:           true,
				Attributes: map[string]schema.Attribute{
					"fastly": schema.StringAttribute{
						Optional: true,
					},
					"cloudflare": schema.StringAttribute{
						Optional: true,
					},
					"cloudfront": schema.SingleNestedAttribute{
						Optional: true,
						Attributes: map[string]schema.Attribute{
							"access_key": schema.StringAttribute{
								Required: true,
							},
							"access_secret": schema.StringAttribute{
								Required: true,
							},
						},
					},
				},
			},
		},
	}
}

// Configure resource and retrieve API client
func (r *AccountProviderResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	client := ConfigureBase(ctx, req, resp)
	if client == nil {
		return
	}
	r.client = client
}

// Create AccountProvider resource
func (r *AccountProviderResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data AccountProviderResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	newData := resourceCreate(r.client, ctx, req, resp, r, data, false)
	if newData == nil {
		return
	}

	// AccountProvider credential field is write-only which we need to preserve from original request
	newAC := newData.(AccountProviderResourceModel)
	newAC.Credentials = data.Credentials

	resp.Diagnostics.Append(resp.State.Set(ctx, &newAC)...)
}

// Read AccountProvider resource
func (r *AccountProviderResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data AccountProviderResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	newData := resourceRead(r.client, ctx, req, resp, r, data)
	if newData == nil {
		return
	}

	// AccountProvider credential field is write-only which we need to preserve from original request
	newAC := newData.(AccountProviderResourceModel)
	newAC.Credentials = data.Credentials

	resp.Diagnostics.Append(resp.State.Set(ctx, &newAC)...)
}

// Update AccountProvider resource
func (r *AccountProviderResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data AccountProviderResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	newData := resourceUpdate(r.client, ctx, req, resp, r, data)
	if newData == nil {
		return
	}

	// AccountProvider credential field is write-only which we need to preserve from original request
	updatedAC := newData.(AccountProviderResourceModel)
	updatedAC.Credentials = data.Credentials

	resp.Diagnostics.Append(resp.State.Set(ctx, &updatedAC)...)
}

// Delete AccountProvider resource
func (r *AccountProviderResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data AccountProviderResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resourceDelete(r.client, ctx, req, resp, r, data)
}

// Import AccountProvider resource
func (r *AccountProviderResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// ------- Implement base Resource API ---------

func (AccountProviderResource) create(client *ioriver.IORiverClient, newObj interface{}) (interface{}, error) {
	return client.CreateAccountProvider(newObj.(ioriver.AccountProvider))
}

func (AccountProviderResource) read(client *ioriver.IORiverClient, id interface{}) (interface{}, error) {
	return client.GetAccountProvider(id.(AccountProviderResourceId))
}

func (AccountProviderResource) update(client *ioriver.IORiverClient, obj interface{}) (interface{}, error) {
	return client.UpdateAccountProvider(obj.(ioriver.AccountProvider))
}

func (AccountProviderResource) delete(client *ioriver.IORiverClient, id interface{}) error {
	return client.DeleteAccountProvider(id.(AccountProviderResourceId))
}

func (AccountProviderResource) getId(data interface{}) interface{} {
	d := data.(AccountProviderResourceModel)
	return d.Id.ValueString()
}

// Convert AccountProvider resource to AccountProvider API object
func (AccountProviderResource) resourceToObj(ctx context.Context, data interface{}) (interface{}, error) {
	d := data.(AccountProviderResourceModel)
	providerName := d.ProviderName.ValueString()
	credentials := d.Credentials

	return ioriver.AccountProvider{
		Id:          d.Id.ValueString(),
		Provider:    convertProviderName(providerName),
		Credentials: convertCredentials(providerName, *credentials),
	}, nil
}

// Convert AccountProvider API object to AccountProvider resource
func (AccountProviderResource) objToResource(ctx context.Context, obj interface{}) (interface{}, error) {
	accountProvider := obj.(*ioriver.AccountProvider)

	return AccountProviderResourceModel{
		Id:           types.StringValue(accountProvider.Id),
		ProviderName: types.StringValue(convertProviderId(accountProvider.Provider)),
	}, nil
}

func convertProviderName(name string) int {
	providerId := -1
	switch name {
	case "fastly":
		providerId = ioriver.Fastly
	case "cloudflare":
		providerId = ioriver.Cloudflare
	case "cloudfront":
		providerId = ioriver.Cloudfront
	case "azure_cdn":
		providerId = ioriver.AzureCDN
	case "akamai":
		providerId = ioriver.Akamai
	}
	return providerId
}

func convertProviderId(id int) string {
	name := ""
	switch id {
	case ioriver.Fastly:
		name = "fastly"
	case ioriver.Cloudflare:
		name = "cloudflare"
	case ioriver.Cloudfront:
		name = "cloudfront"
	case ioriver.AzureCDN:
		name = "azure_cdn"
	case ioriver.Akamai:
		name = "akamai"
	}
	return name
}

func convertCredentials(providerName string, credsMap CredentialsModel) (credentials interface{}) {
	switch providerName {
	case "fastly":
		credentials = credsMap.Fastly.ValueString()
	case "cloudflare":
		credentials = credsMap.Cloudflare.ValueString()
	case "cloudfront":
		credentials = fmt.Sprintf("{\"accessKey\":\"%s\",\"accessSecret\":\"%s\"}",
			credsMap.Cloudfront.AccessKey.ValueString(),
			credsMap.Cloudfront.AccessSecret.ValueString())
	default:
		credentials = nil
	}

	return credentials
}
