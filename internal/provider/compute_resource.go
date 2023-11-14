package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	ioriver "github.com/ioriver/ioriver-go"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &ComputeResource{}
var _ resource.ResourceWithImportState = &ComputeResource{}

func NewComputeResource() resource.Resource {
	return &ComputeResource{}
}

type ComputeResourceId struct {
	computeId string
	serviceId string
}

type ComputeResource struct {
	client *ioriver.IORiverClient
}

type ComputeRouteModel struct {
	Domain types.String `tfsdk:"domain"`
	Path   types.String `tfsdk:"path"`
}

type ComputeResourceModel struct {
	Id           types.String        `tfsdk:"id"`
	Service      types.String        `tfsdk:"service"`
	Name         types.String        `tfsdk:"name"`
	RequestCode  types.String        `tfsdk:"request_code"`
	ResponseCode types.String        `tfsdk:"response_code"`
	Routes       []ComputeRouteModel `tfsdk:"routes"`
}

func (r *ComputeResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_compute"
}

func (r *ComputeResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Compute resource",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Compute identifier",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"service": schema.StringAttribute{
				MarkdownDescription: "The id of the service this compute belongs to",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Compute function name",
				Required:            true,
			},
			"request_code": schema.StringAttribute{
				MarkdownDescription: "Compute code for request phase",
				Optional:            true,
				Computed:            true,
			},
			"response_code": schema.StringAttribute{
				MarkdownDescription: "Compute code for response phase",
				Optional:            true,
				Computed:            true,
			},
			"routes": schema.ListNestedAttribute{
				MarkdownDescription: "List of routes to apply the compute",
				Required:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"domain": schema.StringAttribute{
							MarkdownDescription: "Route domain name",
							Required:            true,
						},
						"path": schema.StringAttribute{
							MarkdownDescription: "Route path pattern",
							Required:            true,
						},
					},
				},
			},
		},
	}
}

// Configure resource and retrieve API client
func (r *ComputeResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	client := ConfigureBase(ctx, req, resp)
	if client == nil {
		return
	}
	r.client = client
}

// Create Compute resource
func (r *ComputeResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ComputeResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	newData := resourceCreate(r.client, ctx, req, resp, r, data, false)
	if newData == nil {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newData)...)
}

// Read Compute resource
func (r *ComputeResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ComputeResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	newData := resourceRead(r.client, ctx, req, resp, r, data)
	if newData == nil {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newData)...)
}

// Update Compute resource
func (r *ComputeResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data ComputeResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	newData := resourceUpdate(r.client, ctx, req, resp, r, data)
	if newData == nil {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newData)...)
}

// Delete Compute resource
func (r *ComputeResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ComputeResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resourceDelete(r.client, ctx, req, resp, r, data)
}

// Import Compute resource
func (r *ComputeResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	serviceResourceImport(ctx, req, resp)
}

// ------- Implement base Resource API ---------

func (ComputeResource) create(client *ioriver.IORiverClient, newObj interface{}) (interface{}, error) {
	return client.CreateCompute(newObj.(ioriver.Compute))
}

func (ComputeResource) read(client *ioriver.IORiverClient, id interface{}) (interface{}, error) {
	resourceId := id.(ComputeResourceId)
	return client.GetCompute(resourceId.serviceId, resourceId.computeId)
}

func (ComputeResource) update(client *ioriver.IORiverClient, obj interface{}) (interface{}, error) {
	return client.UpdateCompute(obj.(ioriver.Compute))
}

func (ComputeResource) delete(client *ioriver.IORiverClient, id interface{}) error {
	resourceId := id.(ComputeResourceId)
	return client.DeleteCompute(resourceId.serviceId, resourceId.computeId)
}

func (ComputeResource) getId(data interface{}) interface{} {
	d := data.(ComputeResourceModel)
	computeId := d.Id.ValueString()
	serviceId := d.Service.ValueString()
	return ComputeResourceId{computeId, serviceId}
}

// Convert Compute resource to Compute API object
func (ComputeResource) resourceToObj(ctx context.Context, data interface{}) (interface{}, error) {
	d := data.(ComputeResourceModel)

	// convert routes
	computeRoutes := []ioriver.ComputeRoute{}
	for _, route := range d.Routes {
		computeRoutes = append(computeRoutes,
			ioriver.ComputeRoute{
				Domain: route.Domain.ValueString(),
				Path:   route.Path.ValueString(),
			})
	}

	return ioriver.Compute{
		Id:           d.Id.ValueString(),
		Service:      d.Service.ValueString(),
		Name:         d.Name.ValueString(),
		RequestCode:  d.RequestCode.ValueString(),
		ResponseCode: d.ResponseCode.ValueString(),
		Routes:       computeRoutes,
	}, nil
}

// Convert Compute API object to Compute resource
func (ComputeResource) objToResource(ctx context.Context, obj interface{}) (interface{}, error) {
	compute := obj.(*ioriver.Compute)

	// convert actions
	modelRoutes := []ComputeRouteModel{}
	for _, route := range compute.Routes {
		modelRoutes = append(modelRoutes,
			ComputeRouteModel{
				Domain: types.StringValue(route.Domain),
				Path:   types.StringValue(route.Path),
			})
	}

	return ComputeResourceModel{
		Id:           types.StringValue(compute.Id),
		Service:      types.StringValue(compute.Service),
		Name:         types.StringValue(compute.Name),
		RequestCode:  types.StringValue(compute.RequestCode),
		ResponseCode: types.StringValue(compute.ResponseCode),
		Routes:       modelRoutes,
	}, nil
}
