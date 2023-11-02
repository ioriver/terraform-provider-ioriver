package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	ioriver "github.com/ioriver/ioriver-go"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &ServiceProviderResource{}
var _ resource.ResourceWithImportState = &ServiceProviderResource{}

func NewServiceProviderResource() resource.Resource {
	return &ServiceProviderResource{}
}

type ServiceProviderResourceId struct {
	serviceProviderId string
	serviceId         string
}

type ServiceProviderResource struct {
	client *ioriver.IORiverClient
}

type ServiceProviderResourceModel struct {
	Id              types.String `tfsdk:"id"`
	Service         types.String `tfsdk:"service"`
	AccountProvider types.String `tfsdk:"account_provider"`
	IsUnmanaged     types.Bool   `tfsdk:"is_unmanaged"`
	CName           types.String `tfsdk:"cname"`
	DisplayName     types.String `tfsdk:"display_name"`
	IsFailed        types.Bool   `tfsdk:"is_failed"`
	Status          types.String `tfsdk:"status"`
	StatusDetails   types.String `tfsdk:"status_details"`
	Restored        types.Bool   `tfsdk:"restored"`
	Name            types.String `tfsdk:"name"`
}

func (r *ServiceProviderResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_service_provider"
}

func (r *ServiceProviderResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Service Provider resource",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "ServiceProvider identifier",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"service": schema.StringAttribute{
				MarkdownDescription: "The id of the service this service provider belongs to",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"account_provider": schema.StringAttribute{
				MarkdownDescription: "ServiceProvider associated account provider",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"is_unmanaged": schema.BoolAttribute{
				MarkdownDescription: "Is this an unmanaged ServiceProvider",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false), // has default since this is a write-only field
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.RequiresReplace(),
				},
			},
			"cname": schema.StringAttribute{
				MarkdownDescription: "CName for the ServiceProvider",
				Optional:            true,
				Computed:            true,
			},
			"display_name": schema.StringAttribute{
				MarkdownDescription: "Display name for the ServiceProvider",
				Optional:            true,
				Computed:            true,
			},
			"is_failed": schema.BoolAttribute{
				MarkdownDescription: "Is ServiceProvider in a failed state",
				Computed:            true,
			},
			"status": schema.StringAttribute{
				MarkdownDescription: "ServiceProvider status",
				Computed:            true,
			},
			"status_details": schema.StringAttribute{
				MarkdownDescription: "ServiceProvider detailed status",
				Computed:            true,
			},
			"restored": schema.BoolAttribute{
				MarkdownDescription: "Is ServiceProvider restored",
				Computed:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Name of the provider",
				Computed:            true,
			},
		},
	}
}

// Configure resource and retrieve API client
func (r *ServiceProviderResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	client := ConfigureBase(ctx, req, resp)
	if client == nil {
		return
	}
	r.client = client
}

// Create ServiceProvider resource
func (r *ServiceProviderResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ServiceProviderResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	newData := resourceCreate(r.client, ctx, req, resp, r, data)
	if newData == nil {
		return
	}

	// is_unamanged is a write-only field which we need to preserve from original request
	newSp := newData.(ServiceProviderResourceModel)
	newSp.IsUnmanaged = data.IsUnmanaged

	resp.Diagnostics.Append(resp.State.Set(ctx, &newSp)...)
}

// Read ServiceProvider resource
func (r *ServiceProviderResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ServiceProviderResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	newData := resourceRead(r.client, ctx, req, resp, r, data)
	if newData == nil {
		return
	}

	// is_unamanged is a write-only field which we need to preserve from original request
	newSp := newData.(ServiceProviderResourceModel)
	newSp.IsUnmanaged = data.IsUnmanaged

	resp.Diagnostics.Append(resp.State.Set(ctx, &newSp)...)
}

// Update ServiceProvider resource
func (r *ServiceProviderResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data ServiceProviderResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	newData := resourceUpdate(r.client, ctx, req, resp, r, data)
	if newData == nil {
		return
	}

	// is_unamanged is a write-only field which we need to preserve from original request
	newSp := newData.(ServiceProviderResourceModel)
	newSp.IsUnmanaged = data.IsUnmanaged

	resp.Diagnostics.Append(resp.State.Set(ctx, &newSp)...)
}

// Delete ServiceProvider resource
func (r *ServiceProviderResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ServiceProviderResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resourceDelete(r.client, ctx, req, resp, r, data)
}

// Import ServiceProvider resource
func (r *ServiceProviderResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	serviceResourceImport(ctx, req, resp)
}

// ------- Implement base Resource API ---------

func (ServiceProviderResource) create(client *ioriver.IORiverClient, newObj interface{}) (interface{}, error) {
	return client.CreateServiceProvider(newObj.(ioriver.ServiceProvider))
}

func (ServiceProviderResource) read(client *ioriver.IORiverClient, id interface{}) (interface{}, error) {
	resourceId := id.(ServiceProviderResourceId)
	return client.GetServiceProvider(resourceId.serviceId, resourceId.serviceProviderId)
}

func (ServiceProviderResource) update(client *ioriver.IORiverClient, obj interface{}) (interface{}, error) {
	return client.UpdateServiceProvider(obj.(ioriver.ServiceProvider))
}

func (ServiceProviderResource) delete(client *ioriver.IORiverClient, id interface{}) error {
	resourceId := id.(ServiceProviderResourceId)
	return client.DeleteServiceProvider(resourceId.serviceId, resourceId.serviceProviderId, "disconnect")
}

func (ServiceProviderResource) getId(data interface{}) interface{} {
	d := data.(ServiceProviderResourceModel)
	serviceProviderId := d.Id.ValueString()
	serviceId := d.Service.ValueString()
	return ServiceProviderResourceId{serviceProviderId, serviceId}
}

// Convert ServiceProvider resource to ServiceProvider API object
func (ServiceProviderResource) resourceToObj(ctx context.Context, data interface{}) (interface{}, error) {
	d := data.(ServiceProviderResourceModel)

	return ioriver.ServiceProvider{
		Id:              d.Id.ValueString(),
		Service:         d.Service.ValueString(),
		AccountProvider: d.AccountProvider.ValueString(),
		IsUnmanaged:     d.IsUnmanaged.ValueBool(),
		CName:           d.CName.ValueString(),
		DisplayName:     d.DisplayName.ValueString(),
		IsFailed:        d.IsFailed.ValueBool(),
		Status:          d.Status.ValueString(),
		StatusDetails:   d.StatusDetails.ValueString(),
		Restored:        d.Restored.ValueBool(),
		Name:            d.Name.ValueString(),
	}, nil
}

// Convert ServiceProvider API object to ServiceProvider resource
func (ServiceProviderResource) objToResource(ctx context.Context, obj interface{}) (interface{}, error) {
	serviceProvider := obj.(*ioriver.ServiceProvider)

	return ServiceProviderResourceModel{
		Id:              types.StringValue(serviceProvider.Id),
		Service:         types.StringValue(serviceProvider.Service),
		AccountProvider: types.StringValue(serviceProvider.AccountProvider),
		IsUnmanaged:     types.BoolValue(serviceProvider.IsUnmanaged),
		CName:           types.StringValue(serviceProvider.CName),
		DisplayName:     types.StringValue(serviceProvider.DisplayName),
		IsFailed:        types.BoolValue(serviceProvider.IsFailed),
		Status:          types.StringValue(serviceProvider.Status),
		StatusDetails:   types.StringValue(serviceProvider.StatusDetails),
		Restored:        types.BoolValue(serviceProvider.Restored),
		Name:            types.StringValue(serviceProvider.Name),
	}, nil
}
