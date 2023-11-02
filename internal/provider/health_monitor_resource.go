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
var _ resource.Resource = &HealthMonitorResource{}
var _ resource.ResourceWithImportState = &HealthMonitorResource{}

func NewHealthMonitorResource() resource.Resource {
	return &HealthMonitorResource{}
}

type HealthMonitorResourceId struct {
	healthMonitorId string
	serviceId       string
}

type HealthMonitorResource struct {
	client *ioriver.IORiverClient
}

type HealthMonitorResourceModel struct {
	Id      types.String `tfsdk:"id"`
	Service types.String `tfsdk:"service"`
	Name    types.String `tfsdk:"name"`
	Url     types.String `tfsdk:"url"`
	Enabled types.Bool   `tfsdk:"enabled"`
}

func (r *HealthMonitorResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_health_monitor"
}

func (r *HealthMonitorResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "HealthMonitor resource",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "HealthMonitor identifier",
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
func (r *HealthMonitorResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	client := ConfigureBase(ctx, req, resp)
	if client == nil {
		return
	}
	r.client = client
}

// Create HealthMonitor resource
func (r *HealthMonitorResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data HealthMonitorResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	newData := resourceCreate(r.client, ctx, req, resp, r, data)
	if newData == nil {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newData)...)
}

// Read HealthMonitor resource
func (r *HealthMonitorResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data HealthMonitorResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	newData := resourceRead(r.client, ctx, req, resp, r, data)
	if newData == nil {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newData)...)
}

// Update HealthMonitor resource
func (r *HealthMonitorResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data HealthMonitorResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	newData := resourceUpdate(r.client, ctx, req, resp, r, data)
	if newData == nil {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newData)...)
}

// Delete HealthMonitor resource
func (r *HealthMonitorResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data HealthMonitorResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resourceDelete(r.client, ctx, req, resp, r, data)
}

// Import HealthMonitor resource
func (r *HealthMonitorResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	serviceResourceImport(ctx, req, resp)
}

// ------- Implement base Resource API ---------

func (HealthMonitorResource) create(client *ioriver.IORiverClient, newObj interface{}) (interface{}, error) {
	return client.CreateHealthMonitor(newObj.(ioriver.HealthMonitor))
}

func (HealthMonitorResource) read(client *ioriver.IORiverClient, id interface{}) (interface{}, error) {
	resourceId := id.(HealthMonitorResourceId)
	return client.GetHealthMonitor(resourceId.serviceId, resourceId.healthMonitorId)
}

func (HealthMonitorResource) update(client *ioriver.IORiverClient, obj interface{}) (interface{}, error) {
	return client.UpdateHealthMonitor(obj.(ioriver.HealthMonitor))
}

func (HealthMonitorResource) delete(client *ioriver.IORiverClient, id interface{}) error {
	resourceId := id.(HealthMonitorResourceId)
	return client.DeleteHealthMonitor(resourceId.serviceId, resourceId.healthMonitorId)
}

func (HealthMonitorResource) getId(data interface{}) interface{} {
	d := data.(HealthMonitorResourceModel)
	healthMonitorId := d.Id.ValueString()
	serviceId := d.Service.ValueString()
	return HealthMonitorResourceId{healthMonitorId, serviceId}
}

// Convert HealthMonitor resource to HealthMonitor API object
func (HealthMonitorResource) resourceToObj(ctx context.Context, data interface{}) (interface{}, error) {
	d := data.(HealthMonitorResourceModel)

	return ioriver.HealthMonitor{
		Id:      d.Id.ValueString(),
		Service: d.Service.ValueString(),
		Name:    d.Name.ValueString(),
		Url:     d.Url.ValueString(),
		Enabled: d.Enabled.ValueBool(),
	}, nil
}

// Convert HealthMonitor API object to HealthMonitor resource
func (HealthMonitorResource) objToResource(ctx context.Context, obj interface{}) (interface{}, error) {
	healthMonitor := obj.(*ioriver.HealthMonitor)

	return HealthMonitorResourceModel{
		Id:      types.StringValue(healthMonitor.Id),
		Service: types.StringValue(healthMonitor.Service),
		Name:    types.StringValue(healthMonitor.Name),
		Url:     types.StringValue(healthMonitor.Url),
		Enabled: types.BoolValue(healthMonitor.Enabled),
	}, nil
}
