package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	ioriver "github.com/ioriver/ioriver-go"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &OriginResource{}
var _ resource.ResourceWithImportState = &OriginResource{}

func NewOriginResource() resource.Resource {
	return &OriginResource{}
}

type OriginResourceId struct {
	originId  string
	serviceId string
}

type OriginResource struct {
	client *ioriver.IORiverClient
}

type PrivateS3BucketCredentialsModel struct {
	AccessKey types.String `tfsdk:"access_key"`
	SecretKey types.String `tfsdk:"secret_key"`
}

type PrivateS3BucketModel struct {
	BucketName   types.String                    `tfsdk:"bucket_name"`
	BucketRegion types.String                    `tfsdk:"bucket_region"`
	Credentials  PrivateS3BucketCredentialsModel `tfsdk:"credentials"`
}

type OriginResourceModel struct {
	Id        types.String          `tfsdk:"id"`
	Service   types.String          `tfsdk:"service"`
	Host      types.String          `tfsdk:"host"`
	Protocol  types.String          `tfsdk:"protocol"`
	HttpsPort types.Int64           `tfsdk:"https_port"`
	HttpPort  types.Int64           `tfsdk:"http_port"`
	Path      types.String          `tfsdk:"path"`
	IsS3      types.Bool            `tfsdk:"is_s3"`
	PrivateS3 *PrivateS3BucketModel `tfsdk:"private_s3"`
	TimeoutMs types.Int64           `tfsdk:"timeout_ms"`
	VerifyTLS types.Bool            `tfsdk:"verify_tls"`
}

func (r *OriginResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_origin"
}

func (r *OriginResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Origin resource",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Origin identifier",
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
			"host": schema.StringAttribute{
				MarkdownDescription: "Origin host",
				Required:            true,
			},
			"protocol": schema.StringAttribute{
				MarkdownDescription: "Origin protocol scheme (HTTP/HTTPS)",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("HTTPS"),
				Validators: []validator.String{
					stringvalidator.OneOf([]string{"HTTP", "HTTPS"}...),
				},
			},
			"https_port": schema.Int64Attribute{
				MarkdownDescription: "Origin HTTPS port",
				Optional:            true,
				Computed:            true,
				Validators: []validator.Int64{
					int64validator.AtLeast(1),
					int64validator.AtMost(65535),
				},
			},
			"http_port": schema.Int64Attribute{
				MarkdownDescription: "Origin HTTP port",
				Optional:            true,
				Computed:            true,
				Validators: []validator.Int64{
					int64validator.AtLeast(1),
					int64validator.AtMost(65535),
				},
			},
			"path": schema.StringAttribute{
				MarkdownDescription: "Prefix path to be added to the origin request",
				Optional:            true,
				Computed:            true,
			},
			"is_s3": schema.BoolAttribute{
				MarkdownDescription: "Is this origin a S3 bucket",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"private_s3": schema.SingleNestedAttribute{
				MarkdownDescription: "Attributes for a private S3 bucket",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"bucket_name": schema.StringAttribute{
						MarkdownDescription: "Name of the S3 bucket",
						Required:            true,
					},
					"bucket_region": schema.StringAttribute{
						MarkdownDescription: "Region of the S3 bucket",
						Required:            true,
					},
					"credentials": schema.SingleNestedAttribute{
						MarkdownDescription: "AWS access-key credentials for accessing the private bucket",
						Required:            true,
						Attributes: map[string]schema.Attribute{
							"access_key": schema.StringAttribute{
								MarkdownDescription: "AWS access-key ID",
								Required:            true,
								Sensitive:           true,
							},
							"secret_key": schema.StringAttribute{
								MarkdownDescription: "AWS access-key secret",
								Required:            true,
								Sensitive:           true,
							},
						},
					},
				},
				Validators: []validator.Object{
					PrivateS3Validator{},
				},
			},
			"timeout_ms": schema.Int64Attribute{
				MarkdownDescription: "Origin timeout",
				Optional:            true,
				Computed:            true,
			},
			"verify_tls": schema.BoolAttribute{
				MarkdownDescription: "Should verify origin TLS certificate",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
			},
		},
	}
}

// Configure resource and retrieve API client
func (r *OriginResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	client := ConfigureBase(ctx, req, resp)
	if client == nil {
		return
	}
	r.client = client
}

// Create Origin resource
func (r *OriginResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data OriginResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	newData := resourceCreate(r.client, ctx, req, resp, r, data, false)
	if newData == nil {
		return
	}

	// Origin private bucket credential field is write-only which we need to preserve from original request
	newOrigin := newData.(OriginResourceModel)
	if newOrigin.PrivateS3 != nil {
		newOrigin.PrivateS3.Credentials = data.PrivateS3.Credentials
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newData)...)
}

// Read Origin resource
func (r *OriginResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data OriginResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	newData := resourceRead(r.client, ctx, req, resp, r, data)
	if newData == nil {
		return
	}

	// Origin private bucket credential field is write-only which we need to preserve from original request
	newOrigin := newData.(OriginResourceModel)
	if newOrigin.PrivateS3 != nil && data.PrivateS3 != nil {
		newOrigin.PrivateS3.Credentials = data.PrivateS3.Credentials
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newData)...)
}

// Update Origin resource
func (r *OriginResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data OriginResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	newData := resourceUpdate(r.client, ctx, req, resp, r, data)
	if newData == nil {
		return
	}

	// Origin private bucket credential field is write-only which we need to preserve from original request
	newOrigin := newData.(OriginResourceModel)
	if newOrigin.PrivateS3 != nil {
		newOrigin.PrivateS3.Credentials = data.PrivateS3.Credentials
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newData)...)
}

// Delete Origin resource
func (r *OriginResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data OriginResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resourceDelete(r.client, ctx, req, resp, r, data)
}

// Import Origin resource
func (r *OriginResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	serviceResourceImport(ctx, req, resp)
}

// ------- Implement base Resource API ---------

func (OriginResource) create(client *ioriver.IORiverClient, newObj interface{}) (interface{}, error) {
	return client.CreateOrigin(newObj.(ioriver.Origin))
}

func (OriginResource) read(client *ioriver.IORiverClient, id interface{}) (interface{}, error) {
	resourceId := id.(OriginResourceId)
	return client.GetOrigin(resourceId.serviceId, resourceId.originId)
}

func (OriginResource) update(client *ioriver.IORiverClient, obj interface{}) (interface{}, error) {
	return client.UpdateOrigin(obj.(ioriver.Origin))
}

func (OriginResource) delete(client *ioriver.IORiverClient, id interface{}) error {
	resourceId := id.(OriginResourceId)
	return client.DeleteOrigin(resourceId.serviceId, resourceId.originId)
}

func (OriginResource) getId(data interface{}) interface{} {
	d := data.(OriginResourceModel)
	originId := d.Id.ValueString()
	serviceId := d.Service.ValueString()
	return OriginResourceId{originId, serviceId}
}

// Convert Origin resource to Origin API object
func (OriginResource) resourceToObj(ctx context.Context, data interface{}) (interface{}, error) {
	d := data.(OriginResourceModel)

	// convert private-s3
	isPrivateS3 := false
	s3BucketName := ""
	s3BucketRegion := ""
	s3AwsKey := ""
	s3AwsSecret := ""
	if d.PrivateS3 != nil {
		isPrivateS3 = true
		s3BucketName = d.PrivateS3.BucketName.ValueString()
		s3BucketRegion = d.PrivateS3.BucketRegion.ValueString()
		s3AwsKey = d.PrivateS3.Credentials.AccessKey.ValueString()
		s3AwsSecret = d.PrivateS3.Credentials.SecretKey.ValueString()
	}

	return ioriver.Origin{
		Id:           d.Id.ValueString(),
		Service:      d.Service.ValueString(),
		Host:         d.Host.ValueString(),
		Protocol:     d.Protocol.ValueString(),
		HttpsPort:    int(d.HttpsPort.ValueInt64()),
		HttpPort:     int(d.HttpPort.ValueInt64()),
		Path:         d.Path.ValueString(),
		IsS3:         d.IsS3.ValueBool(),
		IsPrivateS3:  isPrivateS3,
		S3BucketName: s3BucketName,
		S3AwsRegion:  s3BucketRegion,
		S3AwsKey:     s3AwsKey,
		S3AwsSecret:  s3AwsSecret,
		TimeoutMs:    int(d.TimeoutMs.ValueInt64()),
		VerifyTLS:    d.VerifyTLS.ValueBool(),
	}, nil
}

// Convert Origin API object to Origin resource
func (OriginResource) objToResource(ctx context.Context, obj interface{}) (interface{}, error) {
	origin := obj.(*ioriver.Origin)

	var privateS3 *PrivateS3BucketModel
	if origin.IsPrivateS3 {
		privateS3 = &PrivateS3BucketModel{
			BucketName:   types.StringValue(origin.S3BucketName),
			BucketRegion: types.StringValue(origin.S3AwsRegion),
			Credentials: PrivateS3BucketCredentialsModel{
				AccessKey: types.StringValue(origin.S3AwsKey),
				SecretKey: types.StringValue(origin.S3AwsSecret),
			},
		}
	}

	return OriginResourceModel{
		Id:        types.StringValue(origin.Id),
		Service:   types.StringValue(origin.Service),
		Host:      types.StringValue(origin.Host),
		Protocol:  types.StringValue(origin.Protocol),
		HttpsPort: types.Int64Value((int64(origin.HttpsPort))),
		HttpPort:  types.Int64Value((int64(origin.HttpPort))),
		Path:      types.StringValue(origin.Path),
		IsS3:      types.BoolValue(origin.IsS3),
		PrivateS3: privateS3,
		TimeoutMs: types.Int64Value((int64(origin.TimeoutMs))),
		VerifyTLS: types.BoolValue(origin.VerifyTLS),
	}, nil
}

// Validators

var _ validator.Object = PrivateS3Validator{}

type PrivateS3Validator struct{}

func (v PrivateS3Validator) Description(ctx context.Context) string {
	return "Ensures private_s3 is only set when is_s3 is true."
}

func (v PrivateS3Validator) MarkdownDescription(ctx context.Context) string {
	return "Ensures `private_s3` is set only if `is_s3` is `true`."
}

func (v PrivateS3Validator) ValidateObject(ctx context.Context, req validator.ObjectRequest, resp *validator.ObjectResponse) {
	// Extract is_s3 from config
	var isS3 types.Bool
	diags := req.Config.GetAttribute(ctx, path.Root("is_s3"), &isS3)
	resp.Diagnostics.Append(diags...)

	// Check if private_s3_details exists
	if !isS3.ValueBool() && !req.ConfigValue.IsNull() {
		resp.Diagnostics.AddError(
			"Invalid Configuration",
			"`private_s3` can only be set when `is_s3` is true.",
		)
	}
}
