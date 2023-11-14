package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	ioriver "ioriver.io/ioriver/ioriver-go"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &CertificateResource{}
var _ resource.ResourceWithImportState = &CertificateResource{}

func NewCertificateResource() resource.Resource {
	return &CertificateResource{}
}

type CertificateResourceId = string
type CertificateResource struct {
	client *ioriver.IORiverClient
}

type CertificateResourceModel struct {
	Id               types.String `tfsdk:"id"`
	Name             types.String `tfsdk:"name"`
	Type             types.String `tfsdk:"type"`
	Cn               types.String `tfsdk:"cn"`
	NotValidAfter    types.String `tfsdk:"not_valid_after"`
	Certificate      types.String `tfsdk:"certificate"`
	PrivateKey       types.String `tfsdk:"private_key"`
	CertificateChain types.String `tfsdk:"certificate_chain"`
	Challenges       types.String `tfsdk:"challenges"`
	Status           types.String `tfsdk:"status"`
}

func (r *CertificateResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_certificate"
}

func (r *CertificateResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Certificate resource",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Certificate identifier",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Certificate name",
				Required:            true,
			},
			"type": schema.StringAttribute{
				MarkdownDescription: "Certificate type (MANAGED/SELF_MANAGED/EXTERNAL)",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.OneOf([]string{"MANAGED", "SELF_MANAGED", "EXTERNAL"}...),
				},
			},
			"cn": schema.StringAttribute{
				MarkdownDescription: "Certificate CN",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"not_valid_after": schema.StringAttribute{
				MarkdownDescription: "Certificate expiration date",
				Computed:            true,
			},
			"certificate": schema.StringAttribute{
				MarkdownDescription: "Certificate content",
				Optional:            true,
				Computed:            true,
				Sensitive:           true,
				Default:             stringdefault.StaticString(""),
			},
			"private_key": schema.StringAttribute{
				MarkdownDescription: "Certificate private key",
				Optional:            true,
				Computed:            true,
				Sensitive:           true,
				Default:             stringdefault.StaticString(""),
			},
			"certificate_chain": schema.StringAttribute{
				MarkdownDescription: "Certificate chain",
				Optional:            true,
				Computed:            true,
				Sensitive:           true,
				Default:             stringdefault.StaticString(""),
			},
			"challenges": schema.StringAttribute{
				MarkdownDescription: "Required DNS challenges",
				Computed:            true,
			},
			"status": schema.StringAttribute{
				MarkdownDescription: "Certificate status",
				Computed:            true,
			},
		},
	}
}

// Configure resource and retrieve API client
func (r *CertificateResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	client := ConfigureBase(ctx, req, resp)
	if client == nil {
		return
	}
	r.client = client
}

// Create Certificate resource
func (r *CertificateResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data CertificateResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	newData := resourceCreate(r.client, ctx, req, resp, r, data, false)
	if newData == nil {
		return
	}

	// Certificate has couple of write-only fields which we need to preserve from original request
	newCert := newData.(CertificateResourceModel)
	newCert.Certificate = data.Certificate
	newCert.PrivateKey = data.PrivateKey
	newCert.CertificateChain = data.CertificateChain

	resp.Diagnostics.Append(resp.State.Set(ctx, &newCert)...)
}

// Read Certificate resource
func (r *CertificateResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data CertificateResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	newData := resourceRead(r.client, ctx, req, resp, r, data)
	if newData == nil {
		return
	}

	// Certificate has couple of write-only fields which we need to preserve from original request
	newCert := newData.(CertificateResourceModel)
	newCert.Certificate = data.Certificate
	newCert.PrivateKey = data.PrivateKey
	newCert.CertificateChain = data.CertificateChain

	resp.Diagnostics.Append(resp.State.Set(ctx, &newCert)...)
}

// Update Certificate resource
func (r *CertificateResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data CertificateResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	newData := resourceUpdate(r.client, ctx, req, resp, r, data)
	if newData == nil {
		return
	}

	// Certificate has couple of write-only fields which we need to preserve from original request
	updatedCert := newData.(CertificateResourceModel)
	updatedCert.Certificate = data.Certificate
	updatedCert.PrivateKey = data.PrivateKey
	updatedCert.CertificateChain = data.CertificateChain

	resp.Diagnostics.Append(resp.State.Set(ctx, &updatedCert)...)
}

// Delete Certificate resource
func (r *CertificateResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data CertificateResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resourceDelete(r.client, ctx, req, resp, r, data)
}

// Import Certificate resource
func (r *CertificateResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// ------- Implement base Resource API ---------

func (CertificateResource) create(client *ioriver.IORiverClient, newObj interface{}) (interface{}, error) {
	return client.CreateCertificate(newObj.(ioriver.Certificate))
}

func (CertificateResource) read(client *ioriver.IORiverClient, id interface{}) (interface{}, error) {
	return client.GetCertificate(id.(CertificateResourceId))
}

func (CertificateResource) update(client *ioriver.IORiverClient, obj interface{}) (interface{}, error) {
	return client.UpdateCertificate(obj.(ioriver.Certificate))
}

func (CertificateResource) delete(client *ioriver.IORiverClient, id interface{}) error {
	return client.DeleteCertificate(id.(CertificateResourceId))
}

func (CertificateResource) getId(data interface{}) interface{} {
	d := data.(CertificateResourceModel)
	return d.Id.ValueString()
}

// Convert Certificate resource to Certificate API object
func (CertificateResource) resourceToObj(ctx context.Context, data interface{}) (interface{}, error) {
	d := data.(CertificateResourceModel)
	return ioriver.Certificate{
		Id:               d.Id.ValueString(),
		Name:             d.Name.ValueString(),
		Type:             ioriver.CertificateType(d.Type.ValueString()),
		Cn:               d.Cn.ValueString(),
		Certificate:      d.Certificate.ValueString(),
		PrivateKey:       d.PrivateKey.ValueString(),
		CertificateChain: d.Certificate.ValueString()}, nil
}

// Convert Certificate API object to Certificate resource
func (CertificateResource) objToResource(ctx context.Context, obj interface{}) (interface{}, error) {
	cert := obj.(*ioriver.Certificate)

	return CertificateResourceModel{
		Id:               types.StringValue(cert.Id),
		Name:             types.StringValue(cert.Name),
		Type:             types.StringValue(string(cert.Type)),
		Cn:               types.StringValue(cert.Cn),
		NotValidAfter:    types.StringValue(cert.NotValidAfter),
		Certificate:      types.StringValue(cert.Certificate),
		PrivateKey:       types.StringValue(cert.PrivateKey),
		CertificateChain: types.StringValue(cert.CertificateChain),
		Challenges:       types.StringValue(cert.Challenges),
		Status:           types.StringValue(string(cert.Status)),
	}, nil
}
