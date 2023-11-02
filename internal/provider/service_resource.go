package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	ioriver "github.com/ioriver/ioriver-go"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &ServiceResource{}
var _ resource.ResourceWithImportState = &ServiceResource{}

func NewServiceResource() resource.Resource {
	return &ServiceResource{}
}

type ServiceResourceId = string
type ServiceResource struct {
	client *ioriver.IORiverClient
}

type ServiceResourceModel struct {
	Id          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	Certificate types.String `tfsdk:"certificate"`
}

func (r *ServiceResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_service"
}

func (r *ServiceResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Service resource",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Service identifier",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Service name",
				Required:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Description of the service",
				Optional:            true,
			},
			"certificate": schema.StringAttribute{
				MarkdownDescription: "Certificate to be used with the service (must match the domain)",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

// Configure resource and retrieve API client
func (r *ServiceResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	client := ConfigureBase(ctx, req, resp)
	if client == nil {
		return
	}
	r.client = client
}

// Create Service resource
func (r *ServiceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ServiceResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	newData := resourceCreate(r.client, ctx, req, resp, r, data)
	if newData == nil {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newData)...)
}

// Read Service resource
func (r *ServiceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ServiceResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	newData := resourceRead(r.client, ctx, req, resp, r, data)
	if newData == nil {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newData)...)
}

// Update Service resource
func (r *ServiceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data ServiceResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	newData := resourceUpdate(r.client, ctx, req, resp, r, data)
	if newData == nil {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newData)...)
}

// Delete Service resource
func (r *ServiceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ServiceResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resourceDelete(r.client, ctx, req, resp, r, data)
}

// Import Service resource
func (r *ServiceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// ------- Implement base Resource API ---------

func (ServiceResource) create(client *ioriver.IORiverClient, newObj interface{}) (interface{}, error) {
	return client.CreateService(newObj.(ioriver.Service))
}

func (ServiceResource) read(client *ioriver.IORiverClient, id interface{}) (interface{}, error) {
	return client.GetService(id.(ServiceResourceId))
}

func (ServiceResource) update(client *ioriver.IORiverClient, obj interface{}) (interface{}, error) {
	return client.UpdateService(obj.(ioriver.Service))
}

func (ServiceResource) delete(client *ioriver.IORiverClient, id interface{}) error {
	return client.DeleteService(id.(ServiceResourceId))
}

func (ServiceResource) getId(data interface{}) interface{} {
	d := data.(ServiceResourceModel)
	return d.Id.ValueString()
}

// Convert Service resource to Service API object
func (ServiceResource) resourceToObj(ctx context.Context, data interface{}) (interface{}, error) {
	d := data.(ServiceResourceModel)

	return ioriver.Service{
		Id:          d.Id.ValueString(),
		Name:        d.Name.ValueString(),
		Description: d.Description.ValueString(),
		Certificate: d.Certificate.ValueString(),
	}, nil
}

// Convert Service API object to Service resource
func (ServiceResource) objToResource(ctx context.Context, obj interface{}) (interface{}, error) {
	service := obj.(*ioriver.Service)

	return ServiceResourceModel{
		Id:          types.StringValue(service.Id),
		Name:        types.StringValue(service.Name),
		Description: types.StringValue(service.Description),
		Certificate: types.StringValue(service.Certificate),
	}, nil
}
