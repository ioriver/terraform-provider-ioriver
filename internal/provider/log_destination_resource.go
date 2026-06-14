package provider

import (
	"context"

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

type LogDestinationResource struct{}

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
	// no-op: resource is deprecated, no client needed
}

// Create LogDestination resource
func (r *LogDestinationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	resp.Diagnostics.AddError(
		"ioriver resource is deprecated",
		"Please remove this resource from your configuration.\n"+
			"Any existing configuration remains set in ioriver, and can be imported to new resource.",
	)
}

// Read LogDestination resource
func (r *LogDestinationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	resp.State.RemoveResource(ctx)
}

// Update LogDestination resource
func (r *LogDestinationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError(
		"ioriver resource is deprecated",
		"Please remove this resource from your configuration.\n"+
			"Any existing configuration remains set in ioriver, and can be imported to new resource.",
	)
}

// Delete LogDestination resource
func (r *LogDestinationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// no-op: resource is deprecated, Terraform will remove it from state automatically
}

// Import LogDestination resource
func (r *LogDestinationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.AddError(
		"ioriver resource is deprecated",
		"Please remove this resource from your configuration.\n"+
			"Any existing configuration remains set in ioriver, and can be imported to new resource.",
	)
}

// ------- Implement base Resource API ---------

func (LogDestinationResource) create(ctx context.Context, client *ioriver.IORiverClient, newObj interface{}) (interface{}, error) {
	return nil, nil
}

func (LogDestinationResource) read(ctx context.Context, client *ioriver.IORiverClient, id interface{}) (interface{}, error) {
	return nil, nil
}

func (LogDestinationResource) update(ctx context.Context, client *ioriver.IORiverClient, obj interface{}) (interface{}, error) {
	return nil, nil
}

func (LogDestinationResource) delete(ctx context.Context, client *ioriver.IORiverClient, id interface{}) error {
	return nil
}

func (LogDestinationResource) getId(data interface{}) interface{} {
	return nil
}

// Convert LogDestination resource to LogDestination API object
func (LogDestinationResource) resourceToObj(ctx context.Context, data interface{}) (interface{}, error) {
	return nil, nil
}

// Convert LogDestination API object to LogDestination resource
func (LogDestinationResource) objToResource(ctx context.Context, obj interface{}, data interface{}) (interface{}, error) {
	return nil, nil
}
