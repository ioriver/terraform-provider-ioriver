package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/objectvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	ioriver "github.com/ioriver/ioriver-go"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &LogDestinationResource{}
var _ resource.ResourceWithImportState = &LogDestinationResource{}

func NewLogDestinationResource() resource.Resource {
	return &LogDestinationResource{}
}

type LogDestinationResourceId struct {
	logDestinationId string
	serviceId        string
}

type LogDestinationResource struct {
	client *ioriver.IORiverClient
}

type AwsS3LogDestinationModel struct {
	Name        types.String  `tfsdk:"name"`
	Path        types.String  `tfsdk:"path"`
	Region      types.String  `tfsdk:"region"`
	Credentials AwsCredsModel `tfsdk:"credentials"`
}

type CompatibleS3LogDestinationModel struct {
	Name        types.String      `tfsdk:"name"`
	Path        types.String      `tfsdk:"path"`
	Region      types.String      `tfsdk:"region"`
	Domain      types.String      `tfsdk:"domain"`
	Credentials AwsAccessKeyModel `tfsdk:"credentials"`
}

type LogDestinationResourceModel struct {
	Id           types.String                     `tfsdk:"id"`
	Service      types.String                     `tfsdk:"service"`
	Name         types.String                     `tfsdk:"name"`
	AwsS3        *AwsS3LogDestinationModel        `tfsdk:"aws_s3"`
	CompatibleS3 *CompatibleS3LogDestinationModel `tfsdk:"compatible_s3"`
}

func (r *LogDestinationResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_log_destination"
}

func (r *LogDestinationResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Log Destination resource",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Log Destination identifier",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"service": schema.StringAttribute{
				MarkdownDescription: "The id of the service this log destination belongs to",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Log destination name",
				Required:            true,
			},
			"aws_s3": schema.SingleNestedAttribute{
				MarkdownDescription: "Properties of AWS S3 bucket log destination",
				Optional:            true,
				Validators: []validator.Object{
					objectvalidator.ExactlyOneOf(path.Expressions{
						path.MatchRelative().AtParent().AtName("compatible_s3"),
					}...),
				},
				Attributes: map[string]schema.Attribute{
					"name": schema.StringAttribute{
						MarkdownDescription: "Name of the bucket",
						Required:            true,
					},
					"path": schema.StringAttribute{
						MarkdownDescription: "The path in the bucket where the logs will be written",
						Optional:            true,
						Computed:            true,
						Default:             stringdefault.StaticString("/"),
					},
					"region": schema.StringAttribute{
						MarkdownDescription: "Bucket region",
						Required:            true,
					},
					"credentials": schema.SingleNestedAttribute{
						MarkdownDescription: "Either AWS role or access-key credentials",
						Required:            true,
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
										Sensitive:           true,
									},
									"secret_key": schema.StringAttribute{
										MarkdownDescription: "AWS access-key secret",
										Required:            true,
										Sensitive:           true,
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
				},
			},
			"compatible_s3": schema.SingleNestedAttribute{
				MarkdownDescription: "Properties of S3 compatible bucket log destination",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"name": schema.StringAttribute{
						MarkdownDescription: "Name of the bucket",
						Required:            true,
					},
					"path": schema.StringAttribute{
						MarkdownDescription: "The path in the bucket where the logs will be written",
						Optional:            true,
						Computed:            true,
						Default:             stringdefault.StaticString("/"),
					},
					"domain": schema.StringAttribute{
						MarkdownDescription: "Domain of the bucket",
						Required:            true,
					},
					"region": schema.StringAttribute{
						MarkdownDescription: "Bucket region",
						Required:            true,
					},
					"credentials": schema.SingleNestedAttribute{
						MarkdownDescription: "Access-key credentials",
						Required:            true,
						Attributes: map[string]schema.Attribute{
							"access_key": schema.StringAttribute{
								MarkdownDescription: "Access-key ID",
								Required:            true,
								Sensitive:           true,
							},
							"secret_key": schema.StringAttribute{
								MarkdownDescription: "Access-key secret",
								Required:            true,
								Sensitive:           true,
							},
						},
					},
				},
			},
		},
	}
}

// Configure resource and retrieve API client
func (r *LogDestinationResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	client := ConfigureBase(ctx, req, resp)
	if client == nil {
		return
	}
	r.client = client
}

// Create LogDestination resource
func (r *LogDestinationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data LogDestinationResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	newData := resourceCreate(r.client, ctx, req, resp, r, data, false)
	if newData == nil {
		return
	}

	// LogDestination credential field is write-only which we need to preserve from original request
	newLogDestination := newData.(LogDestinationResourceModel)
	if newLogDestination.AwsS3 != nil && data.AwsS3 != nil {
		newLogDestination.AwsS3.Credentials = data.AwsS3.Credentials
	}
	if newLogDestination.CompatibleS3 != nil && data.CompatibleS3 != nil {
		newLogDestination.CompatibleS3.Credentials = data.CompatibleS3.Credentials
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newData)...)
}

// Read LogDestination resource
func (r *LogDestinationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data LogDestinationResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	newData := resourceRead(r.client, ctx, req, resp, r, data)
	if newData == nil {
		return
	}

	// LogDestination credential field is write-only which we need to preserve from original request
	newLogDestination := newData.(LogDestinationResourceModel)
	if newLogDestination.AwsS3 != nil && data.AwsS3 != nil {
		newLogDestination.AwsS3.Credentials = data.AwsS3.Credentials
	}
	if newLogDestination.CompatibleS3 != nil && data.CompatibleS3 != nil {
		newLogDestination.CompatibleS3.Credentials = data.CompatibleS3.Credentials
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newData)...)
}

// Update LogDestination resource
func (r *LogDestinationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data LogDestinationResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	newData := resourceUpdate(r.client, ctx, req, resp, r, data)
	if newData == nil {
		return
	}

	// LogDestination credential field is write-only which we need to preserve from original request
	newLogDestination := newData.(LogDestinationResourceModel)
	if newLogDestination.AwsS3 != nil && data.AwsS3 != nil {
		newLogDestination.AwsS3.Credentials = data.AwsS3.Credentials
	}
	if newLogDestination.CompatibleS3 != nil && data.CompatibleS3 != nil {
		newLogDestination.CompatibleS3.Credentials = data.CompatibleS3.Credentials
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newData)...)
}

// Delete LogDestination resource
func (r *LogDestinationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data LogDestinationResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resourceDelete(r.client, ctx, req, resp, r, data)
}

// Import LogDestination resource
func (r *LogDestinationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	serviceResourceImport(ctx, req, resp)
}

// ------- Implement base Resource API ---------

func (LogDestinationResource) create(ctx context.Context, client *ioriver.IORiverClient, newObj interface{}) (interface{}, error) {
	return client.CreateLogDestination(newObj.(ioriver.LogDestination))
}

func (LogDestinationResource) read(ctx context.Context, client *ioriver.IORiverClient, id interface{}) (interface{}, error) {
	resourceId := id.(LogDestinationResourceId)
	return client.GetLogDestination(resourceId.serviceId, resourceId.logDestinationId)
}

func (LogDestinationResource) update(ctx context.Context, client *ioriver.IORiverClient, obj interface{}) (interface{}, error) {
	return client.UpdateLogDestination(obj.(ioriver.LogDestination))
}

func (LogDestinationResource) delete(ctx context.Context, client *ioriver.IORiverClient, id interface{}) error {
	resourceId := id.(LogDestinationResourceId)
	return client.DeleteLogDestination(resourceId.serviceId, resourceId.logDestinationId)
}

func (LogDestinationResource) getId(data interface{}) interface{} {
	d := data.(LogDestinationResourceModel)
	logDestinationId := d.Id.ValueString()
	serviceId := d.Service.ValueString()
	return LogDestinationResourceId{logDestinationId, serviceId}
}

// Convert LogDestination resource to LogDestination API object
func (LogDestinationResource) resourceToObj(ctx context.Context, data interface{}) (interface{}, error) {
	d := data.(LogDestinationResourceModel)

	var destinationType ioriver.DestinationType
	s3BucketName := ""
	s3Domain := ""
	s3Path := ""
	s3Region := ""
	var credentials interface{}

	if d.AwsS3 != nil {
		destinationType = ioriver.AWS_S3
		s3BucketName = d.AwsS3.Name.ValueString()
		s3Path = d.AwsS3.Path.ValueString()
		s3Region = d.AwsS3.Region.ValueString()
		if d.AwsS3.Credentials.AccessKey != nil {
			credentials = fmt.Sprintf("{\"accessKey\":\"%s\",\"accessSecret\":\"%s\"}",
				d.AwsS3.Credentials.AccessKey.AccessKey.ValueString(),
				d.AwsS3.Credentials.AccessKey.SecretKey.ValueString())
		}
		if d.AwsS3.Credentials.AssumeRole != nil {
			credentials = fmt.Sprintf("{\"assume_role_arn\":\"%s\",\"external_id\":\"%s\"}",
				d.AwsS3.Credentials.AssumeRole.RoleArn.ValueString(),
				d.AwsS3.Credentials.AssumeRole.ExternalId.ValueString())
		}
	} else if d.CompatibleS3 != nil {
		destinationType = "S3_COMPATIBLE"
		s3BucketName = d.CompatibleS3.Name.ValueString()
		s3Path = d.CompatibleS3.Path.ValueString()
		s3Domain = d.CompatibleS3.Domain.ValueString()
		s3Region = d.CompatibleS3.Region.ValueString()
		credentials = fmt.Sprintf("{\"accessKey\":\"%s\",\"accessSecret\":\"%s\"}",
			d.CompatibleS3.Credentials.AccessKey.ValueString(),
			d.CompatibleS3.Credentials.SecretKey.ValueString())
	} else {
		return nil, fmt.Errorf("unsupported destination type")
	}

	return ioriver.LogDestination{
		Id:          d.Id.ValueString(),
		Service:     d.Service.ValueString(),
		Name:        d.Name.ValueString(),
		Type:        destinationType,
		Credentials: credentials,
		S3Bucket:    s3BucketName,
		S3Domain:    s3Domain,
		S3Path:      s3Path,
		S3Region:    s3Region,
	}, nil
}

// Convert LogDestination API object to LogDestination resource
func (LogDestinationResource) objToResource(ctx context.Context, obj interface{}) (interface{}, error) {
	logDestination := obj.(*ioriver.LogDestination)

	var awsS3 *AwsS3LogDestinationModel
	var compatibleS3 *CompatibleS3LogDestinationModel

	if logDestination.Type == ioriver.AWS_S3 {
		awsS3 = &AwsS3LogDestinationModel{
			Name:   types.StringValue(logDestination.S3Bucket),
			Path:   types.StringValue(logDestination.S3Path),
			Region: types.StringValue(logDestination.S3Region),
		}
	} else if logDestination.Type == ioriver.S3_COMPATIBLE {
		compatibleS3 = &CompatibleS3LogDestinationModel{
			Name:   types.StringValue(logDestination.S3Bucket),
			Path:   types.StringValue(logDestination.S3Path),
			Domain: types.StringValue(logDestination.S3Domain),
			Region: types.StringValue(logDestination.S3Region),
		}
	} else {
		return nil, fmt.Errorf("unsupported destination type %s", logDestination.Type)
	}

	return LogDestinationResourceModel{
		Id:           types.StringValue(logDestination.Id),
		Service:      types.StringValue(logDestination.Service),
		Name:         types.StringValue(logDestination.Name),
		AwsS3:        awsS3,
		CompatibleS3: compatibleS3,
	}, nil
}
