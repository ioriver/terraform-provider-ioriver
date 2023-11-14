package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	ioriver "ioriver.io/ioriver/ioriver-go"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &DomainResource{}
var _ resource.ResourceWithImportState = &DomainResource{}

func NewDomainResource() resource.Resource {
	return &DomainResource{}
}

type DomainResourceId struct {
	domainId  string
	serviceId string
}

type DomainResource struct {
	client *ioriver.IORiverClient
}

type DomainResourceModel struct {
	Id           types.String `tfsdk:"id"`
	Service      types.String `tfsdk:"service"`
	Domain       types.String `tfsdk:"domain"`
	PathPattern  types.String `tfsdk:"path_pattern"`
	Origin       types.String `tfsdk:"origin"`
	LoadBalancer types.String `tfsdk:"load_balancer"`
}

func (r *DomainResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_domain"
}

func (r *DomainResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Domain resource",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Domain identifier",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"service": schema.StringAttribute{
				MarkdownDescription: "The id of the service this domain belongs to",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"domain": schema.StringAttribute{
				MarkdownDescription: "Domain name",
				Required:            true,
			},
			"path_pattern": schema.StringAttribute{
				MarkdownDescription: "Path pattern within the domain to be mapped with the Domain",
				Optional:            true,
				Computed:            true,
			},
			"origin": schema.StringAttribute{
				MarkdownDescription: "Origin id to forward traffic to",
				Optional:            true,
				Computed:            true,
				Validators: []validator.String{
					stringvalidator.ExactlyOneOf(path.Expressions{
						path.MatchRoot("load_balancer"),
					}...),
				},
			},
			"load_balancer": schema.StringAttribute{
				MarkdownDescription: "Load balancer id to forward traffic to",
				Optional:            true,
				Computed:            true,
				Validators: []validator.String{
					stringvalidator.ExactlyOneOf(path.Expressions{
						path.MatchRoot("origin"),
					}...),
				},
			},
		},
	}
}

// Configure resource and retrieve API client
func (r *DomainResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	client := ConfigureBase(ctx, req, resp)
	if client == nil {
		return
	}
	r.client = client
}

// Create Domain resource
func (r *DomainResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data DomainResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	newData := resourceCreate(r.client, ctx, req, resp, r, data, false)
	if newData == nil {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newData)...)
}

// Read Domain resource
func (r *DomainResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data DomainResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	newData := resourceRead(r.client, ctx, req, resp, r, data)
	if newData == nil {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newData)...)
}

// Update Domain resource
func (r *DomainResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data DomainResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	newData := resourceUpdate(r.client, ctx, req, resp, r, data)
	if newData == nil {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newData)...)
}

// Delete Domain resource
func (r *DomainResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data DomainResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resourceDelete(r.client, ctx, req, resp, r, data)
}

// Import Domain resource
func (r *DomainResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	serviceResourceImport(ctx, req, resp)
}

// ------- Implement base Resource API ---------

func (DomainResource) create(client *ioriver.IORiverClient, newObj interface{}) (interface{}, error) {
	return client.CreateDomain(newObj.(ioriver.Domain))
}

func (DomainResource) read(client *ioriver.IORiverClient, id interface{}) (interface{}, error) {
	resourceId := id.(DomainResourceId)
	return client.GetDomain(resourceId.serviceId, resourceId.domainId)
}

func (DomainResource) update(client *ioriver.IORiverClient, obj interface{}) (interface{}, error) {
	return client.UpdateDomain(obj.(ioriver.Domain))
}

func (DomainResource) delete(client *ioriver.IORiverClient, id interface{}) error {
	resourceId := id.(DomainResourceId)
	return client.DeleteDomain(resourceId.serviceId, resourceId.domainId)
}

func (DomainResource) getId(data interface{}) interface{} {
	d := data.(DomainResourceModel)
	domainId := d.Id.ValueString()
	serviceId := d.Service.ValueString()
	return DomainResourceId{domainId, serviceId}
}

// Convert Domain resource to Domain API object
func (DomainResource) resourceToObj(ctx context.Context, data interface{}) (interface{}, error) {
	d := data.(DomainResourceModel)

	return ioriver.Domain{
		Id:           d.Id.ValueString(),
		Service:      d.Service.ValueString(),
		Domain:       d.Domain.ValueString(),
		PathPattern:  d.PathPattern.ValueString(),
		Origin:       d.Origin.ValueString(),
		LoadBalancer: d.LoadBalancer.ValueString(),
	}, nil
}

// Convert Domain API object to Domain resource
func (DomainResource) objToResource(ctx context.Context, obj interface{}) (interface{}, error) {
	domain := obj.(*ioriver.Domain)

	return DomainResourceModel{
		Id:           types.StringValue(domain.Id),
		Service:      types.StringValue(domain.Service),
		Domain:       types.StringValue(domain.Domain),
		PathPattern:  types.StringValue(domain.PathPattern),
		Origin:       types.StringValue(domain.Origin),
		LoadBalancer: types.StringValue(domain.LoadBalancer),
	}, nil
}
