package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
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

type OriginResourceModel struct {
	Id        types.String `tfsdk:"id"`
	Service   types.String `tfsdk:"service"`
	Host      types.String `tfsdk:"host"`
	Protocol  types.String `tfsdk:"protocol"`
	Path      types.String `tfsdk:"path"`
	IsS3      types.Bool   `tfsdk:"is_s3"`
	TimeoutMs types.Int64  `tfsdk:"timeout_ms"`
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
			"timeout_ms": schema.Int64Attribute{
				MarkdownDescription: "Origin timeout",
				Optional:            true,
				Computed:            true,
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

	return ioriver.Origin{
		Id:        d.Id.ValueString(),
		Service:   d.Service.ValueString(),
		Host:      d.Host.ValueString(),
		Protocol:  d.Protocol.ValueString(),
		Path:      d.Path.ValueString(),
		IsS3:      d.IsS3.ValueBool(),
		TimeoutMs: int(d.TimeoutMs.ValueInt64()),
	}, nil
}

// Convert Origin API object to Origin resource
func (OriginResource) objToResource(ctx context.Context, obj interface{}) (interface{}, error) {
	origin := obj.(*ioriver.Origin)

	return OriginResourceModel{
		Id:        types.StringValue(origin.Id),
		Service:   types.StringValue(origin.Service),
		Host:      types.StringValue(origin.Host),
		Protocol:  types.StringValue(origin.Protocol),
		Path:      types.StringValue(origin.Path),
		IsS3:      types.BoolValue(origin.IsS3),
		TimeoutMs: types.Int64Value((int64(origin.TimeoutMs))),
	}, nil
}
