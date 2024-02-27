package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/objectvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	ioriver "github.com/ioriver/ioriver-go"
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

type AwsAccessKeyModel struct {
	AccessKey types.String `tfsdk:"access_key"`
	SecretKey types.String `tfsdk:"secret_key"`
}

type AwsAssumeRoleModel struct {
	RoleArn    types.String `tfsdk:"role_arn"`
	ExternalId types.String `tfsdk:"external_id"`
}

type CloudfrontCredsModel struct {
	AccessKey  *AwsAccessKeyModel  `tfsdk:"access_key"`
	AssumeRole *AwsAssumeRoleModel `tfsdk:"assume_role"`
}

type EdgioCredsModel struct {
	CliendId       types.String `tfsdk:"client_id"`
	ClientSecret   types.String `tfsdk:"client_secret"`
	OrganizationId types.String `tfsdk:"organization_id"`
}

type CredentialsModel struct {
	Fastly     types.String          `tfsdk:"fastly"`
	Cloudflare types.String          `tfsdk:"cloudflare"`
	Cloudfront *CloudfrontCredsModel `tfsdk:"cloudfront"`
	Edgio      *EdgioCredsModel      `tfsdk:"edgio"`
}

type AccountProviderResourceModel struct {
	Id          types.String      `tfsdk:"id"`
	Credentials *CredentialsModel `tfsdk:"credentials"`
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
			"credentials": schema.SingleNestedAttribute{
				MarkdownDescription: "Account-Provider credentials",
				Required:            true,
				Sensitive:           true,
				Attributes: map[string]schema.Attribute{
					"fastly": schema.StringAttribute{
						MarkdownDescription: "Fastly API access token",
						Optional:            true,
						Validators: []validator.String{
							stringvalidator.ExactlyOneOf(path.Expressions{
								path.MatchRelative().AtParent().AtName("cloudflare"),
								path.MatchRelative().AtParent().AtName("cloudfront"),
								path.MatchRelative().AtParent().AtName("edgio"),
							}...),
						},
					},
					"cloudflare": schema.StringAttribute{
						MarkdownDescription: "Cloudflare API access token",
						Optional:            true,
					},
					"cloudfront": schema.SingleNestedAttribute{
						MarkdownDescription: "Either AWS role or access-key credentials",
						Optional:            true,
						Attributes: map[string]schema.Attribute{
							"access_key": schema.SingleNestedAttribute{
								MarkdownDescription: "AWS access-key credentials",
								Optional:            true,
								Validators: []validator.Object{
									objectvalidator.ExactlyOneOf(path.Expressions{
										path.MatchRelative().AtParent().AtName("assume_role"),
									}...),
								},
								Attributes: map[string]schema.Attribute{
									"access_key": schema.StringAttribute{
										MarkdownDescription: "AWS access-key ID",
										Required:            true,
									},
									"secret_key": schema.StringAttribute{
										MarkdownDescription: "AWS access-key secret",
										Required:            true,
									},
								},
							},
							"assume_role": schema.SingleNestedAttribute{
								MarkdownDescription: "AWS role credentials",
								Optional:            true,
								Attributes: map[string]schema.Attribute{
									"role_arn": schema.StringAttribute{
										MarkdownDescription: "AWS role ARN",
										Required:            true,
									},
									"external_id": schema.StringAttribute{
										MarkdownDescription: "AWS role external ID",
										Required:            true,
									},
								},
							},
						},
					},
					"edgio": schema.SingleNestedAttribute{
						MarkdownDescription: "Edgio API credentials",
						Optional:            true,
						Attributes: map[string]schema.Attribute{
							"client_id": schema.StringAttribute{
								MarkdownDescription: "Edgio API client ID",
								Required:            true,
							},
							"client_secret": schema.StringAttribute{
								MarkdownDescription: "Edgio API client secret",
								Required:            true,
							},
							"organization_id": schema.StringAttribute{
								MarkdownDescription: "Edgio organization ID",
								Required:            true,
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
	credentials := d.Credentials
	convertedCreds, providerName := convertCredentials(*credentials)

	return ioriver.AccountProvider{
		Id:          d.Id.ValueString(),
		Provider:    convertProviderName(providerName),
		Credentials: convertedCreds,
	}, nil
}

// Convert AccountProvider API object to AccountProvider resource
func (AccountProviderResource) objToResource(ctx context.Context, obj interface{}) (interface{}, error) {
	accountProvider := obj.(*ioriver.AccountProvider)

	return AccountProviderResourceModel{
		Id: types.StringValue(accountProvider.Id),
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
	case "edgio":
		providerId = ioriver.Edgio
	}
	return providerId
}

func convertCredentials(credsMap CredentialsModel) (credentials interface{}, name string) {
	credentials = nil
	name = ""

	if !credsMap.Fastly.IsNull() {
		credentials = credsMap.Fastly.ValueString()
		name = "fastly"
	} else if !credsMap.Cloudflare.IsNull() {
		credentials = credsMap.Cloudflare.ValueString()
		name = "cloudflare"
	} else if credsMap.Cloudfront != nil {
		name = "cloudfront"
		if credsMap.Cloudfront.AccessKey != nil {
			credentials = fmt.Sprintf("{\"accessKey\":\"%s\",\"accessSecret\":\"%s\"}",
				credsMap.Cloudfront.AccessKey.AccessKey.ValueString(),
				credsMap.Cloudfront.AccessKey.SecretKey.ValueString())
		} else if credsMap.Cloudfront.AssumeRole != nil {
			credentials = fmt.Sprintf("{\"assume_role_arn\":\"%s\",\"external_id\":\"%s\"}",
				credsMap.Cloudfront.AssumeRole.RoleArn.ValueString(),
				credsMap.Cloudfront.AssumeRole.ExternalId.ValueString())
		}
	} else if credsMap.Edgio != nil {
		name = "edgio"
		credentials = fmt.Sprintf("{\"client_id\":\"%s\",\"client_secret\":\"%s\",\"organization_id\":\"%s\"}",
			credsMap.Edgio.CliendId.ValueString(),
			credsMap.Edgio.ClientSecret.ValueString(),
			credsMap.Edgio.OrganizationId.ValueString())
	}

	return credentials, name
}
