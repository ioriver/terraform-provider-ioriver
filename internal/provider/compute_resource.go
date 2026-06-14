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

type ComputeResource struct{}

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
	// no-op: resource is deprecated, no client needed
}

// Create Compute resource
func (r *ComputeResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	resp.Diagnostics.AddError(
		"ioriver resource is deprecated",
		"Please remove this resource from your configuration.\n"+
			"Any existing configuration remains set in ioriver, and can be imported to new resource.",
	)
}

// Read Compute resource
func (r *ComputeResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// resource is deprecated: remove from state so Terraform stops tracking it
	resp.State.RemoveResource(ctx)
}

// Update Compute resource
func (r *ComputeResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError(
		"ioriver resource is deprecated",
		"Please remove this resource from your configuration.\n"+
			"Any existing configuration remains set in ioriver, and can be imported to new resource.",
	)
}

// Delete Compute resource
func (r *ComputeResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// no-op: resource is deprecated, Terraform will remove it from state automatically
}

// Import Compute resource
func (r *ComputeResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.AddError(
		"ioriver resource is deprecated",
		"Please remove this resource from your configuration.\n"+
			"Any existing configuration remains set in ioriver, and can be imported to new resource.",
	)
}

// ------- Implement base Resource API ---------

func (ComputeResource) create(ctx context.Context, client *ioriver.IORiverClient, newObj interface{}) (interface{}, error) {
	return nil, nil
}

func (ComputeResource) read(ctx context.Context, client *ioriver.IORiverClient, id interface{}) (interface{}, error) {
	return nil, nil
}

func (ComputeResource) update(ctx context.Context, client *ioriver.IORiverClient, obj interface{}) (interface{}, error) {
	return nil, nil
}

func (ComputeResource) delete(ctx context.Context, client *ioriver.IORiverClient, id interface{}) error {
	return nil
}

func (ComputeResource) getId(data interface{}) interface{} {
	return nil
}

// Convert Compute resource to Compute API object
func (ComputeResource) resourceToObj(ctx context.Context, data interface{}) (interface{}, error) {
	return nil, nil
}

// Convert Compute API object to Compute resource
func (ComputeResource) objToResource(ctx context.Context, obj interface{}, data interface{}) (interface{}, error) {
	return nil, nil
}
