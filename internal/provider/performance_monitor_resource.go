package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	ioriver "github.com/ioriver/ioriver-go"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &PerformanceMonitorResource{}
var _ resource.ResourceWithImportState = &PerformanceMonitorResource{}

func NewPerformanceMonitorResource() resource.Resource {
	return &PerformanceMonitorResource{}
}

type PerformanceMonitorResourceId struct {
	performanceMonitorId string
	serviceId            string
}

type PerformanceMonitorResource struct {
	client *ioriver.IORiverClient
}

type PerformanceMonitorResourceModel struct {
	Id      types.String `tfsdk:"id"`
	Service types.String `tfsdk:"service"`
	Name    types.String `tfsdk:"name"`
	Url     types.String `tfsdk:"url"`
	Enabled types.Bool   `tfsdk:"enabled"`
}

func (r *PerformanceMonitorResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_performance_monitor"
}

func (r *PerformanceMonitorResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "PerformanceMonitor resource",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "PerformanceMonitor identifier",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"service": schema.StringAttribute{
				MarkdownDescription: "The id of the service this monitor belongs to",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Health monitor name",
				Required:            true,
			},
			"url": schema.StringAttribute{
				MarkdownDescription: "URL to monitor",
				Required:            true,
			},
			"enabled": schema.BoolAttribute{
				MarkdownDescription: "Health monitor port",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
			},
		},
	}
}

// Configure resource and retrieve API client
func (r *PerformanceMonitorResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	client := ConfigureBase(ctx, req, resp)
	if client == nil {
		return
	}
	r.client = client
}

// Create PerformanceMonitor resource
func (r *PerformanceMonitorResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data PerformanceMonitorResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	newData := resourceCreate(r.client, ctx, req, resp, r, data, false)
	if newData == nil {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newData)...)
}

// Read PerformanceMonitor resource
func (r *PerformanceMonitorResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data PerformanceMonitorResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	newData := resourceRead(r.client, ctx, req, resp, r, data)
	if newData == nil {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newData)...)
}

// Update PerformanceMonitor resource
func (r *PerformanceMonitorResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data PerformanceMonitorResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	newData := resourceUpdate(r.client, ctx, req, resp, r, data)
	if newData == nil {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newData)...)
}

// Delete PerformanceMonitor resource
func (r *PerformanceMonitorResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data PerformanceMonitorResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resourceDelete(r.client, ctx, req, resp, r, data)
}

// Import PerformanceMonitor resource
func (r *PerformanceMonitorResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	serviceResourceImport(ctx, req, resp)
}

// ------- Implement base Resource API ---------

func (PerformanceMonitorResource) create(ctx context.Context, client *ioriver.IORiverClient, newObj interface{}) (interface{}, error) {
	return client.CreatePerformanceMonitor(newObj.(ioriver.PerformanceMonitor))
}

func (PerformanceMonitorResource) read(ctx context.Context, client *ioriver.IORiverClient, id interface{}) (interface{}, error) {
	resourceId := id.(PerformanceMonitorResourceId)
	return client.GetPerformanceMonitor(resourceId.serviceId, resourceId.performanceMonitorId)
}

func (PerformanceMonitorResource) update(ctx context.Context, client *ioriver.IORiverClient, obj interface{}) (interface{}, error) {
	return client.UpdatePerformanceMonitor(obj.(ioriver.PerformanceMonitor))
}

func (PerformanceMonitorResource) delete(ctx context.Context, client *ioriver.IORiverClient, id interface{}) error {
	resourceId := id.(PerformanceMonitorResourceId)
	return client.DeletePerformanceMonitor(resourceId.serviceId, resourceId.performanceMonitorId)
}

func (PerformanceMonitorResource) getId(data interface{}) interface{} {
	d := data.(PerformanceMonitorResourceModel)
	performanceMonitorId := d.Id.ValueString()
	serviceId := d.Service.ValueString()
	return PerformanceMonitorResourceId{performanceMonitorId, serviceId}
}

// Convert PerformanceMonitor resource to PerformanceMonitor API object
func (PerformanceMonitorResource) resourceToObj(ctx context.Context, data interface{}) (interface{}, error) {
	d := data.(PerformanceMonitorResourceModel)

	return ioriver.PerformanceMonitor{
		Id:      d.Id.ValueString(),
		Service: d.Service.ValueString(),
		Name:    d.Name.ValueString(),
		Url:     d.Url.ValueString(),
		Enabled: d.Enabled.ValueBool(),
	}, nil
}

// Convert PerformanceMonitor API object to PerformanceMonitor resource
func (PerformanceMonitorResource) objToResource(ctx context.Context, obj interface{}) (interface{}, error) {
	performanceMonitor := obj.(*ioriver.PerformanceMonitor)

	return PerformanceMonitorResourceModel{
		Id:      types.StringValue(performanceMonitor.Id),
		Service: types.StringValue(performanceMonitor.Service),
		Name:    types.StringValue(performanceMonitor.Name),
		Url:     types.StringValue(performanceMonitor.Url),
		Enabled: types.BoolValue(performanceMonitor.Enabled),
	}, nil
}
