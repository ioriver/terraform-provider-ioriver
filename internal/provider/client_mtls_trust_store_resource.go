package provider

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	ioriver "github.com/ioriver/ioriver-go"
)

var _ resource.Resource = &ClientMTLSTrustStoreResource{}
var _ resource.ResourceWithConfigure = &ClientMTLSTrustStoreResource{}
var _ resource.ResourceWithImportState = &ClientMTLSTrustStoreResource{}
var _ Resource = &ClientMTLSTrustStoreResource{}

func NewClientMTLSTrustStoreResource() resource.Resource {
	return &ClientMTLSTrustStoreResource{}
}

type ClientMTLSTrustStoreResource struct {
	client *ioriver.IORiverClient
}

type ClientMTLSTrustStoreResourceModel struct {
	Id     types.String `tfsdk:"id"`
	Name   types.String `tfsdk:"name"`
	Bundle types.String `tfsdk:"bundle"`
}

func (r *ClientMTLSTrustStoreResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_client_mtls_trust_store"
}

func (r *ClientMTLSTrustStoreResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Account-scoped CA trust bundle for client mTLS. Trust stores are immutable; changing name or bundle replaces the resource. " +
			"Use `lifecycle { create_before_destroy = true }` when referenced by a service so the service can switch to the replacement before the old trust store is deleted. " +
			"Deletion fails while any enabled service references the trust store.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Client mTLS trust store identifier.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Trust store name. Changing this value requires replacement.",
				Validators:          []validator.String{stringvalidator.LengthBetween(1, 255)},
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"bundle": schema.StringAttribute{
				Required: true,
				MarkdownDescription: "PEM-encoded CA certificate bundle used to authenticate clients. Do not include private keys. Stored in Terraform state. " +
					"Changing this value requires replacement. Import reads the backend's bundle with outer whitespace trimmed.",
				Validators:    []validator.String{stringvalidator.LengthAtLeast(1)},
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
		},
	}
}

func (r *ClientMTLSTrustStoreResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = ConfigureBase(ctx, req, resp)
}

func (r *ClientMTLSTrustStoreResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ClientMTLSTrustStoreResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if newData := resourceCreate(r.client, ctx, req, resp, r, data, false); newData != nil {
		resp.Diagnostics.Append(resp.State.Set(ctx, newData)...)
	}
}

func (r *ClientMTLSTrustStoreResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ClientMTLSTrustStoreResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if newData := resourceRead(r.client, ctx, req, resp, r, data); newData != nil {
		resp.Diagnostics.Append(resp.State.Set(ctx, newData)...)
	}
}

func (r *ClientMTLSTrustStoreResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Immutable client mTLS trust store", "Changing name or bundle requires replacing the trust store.")
}

func (r *ClientMTLSTrustStoreResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ClientMTLSTrustStoreResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	resourceDelete(r.client, ctx, req, resp, r, data)
}

func (r *ClientMTLSTrustStoreResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (ClientMTLSTrustStoreResource) create(ctx context.Context, client *ioriver.IORiverClient, obj interface{}) (interface{}, error) {
	return client.CreateViewerMTLSTrustStore(obj.(ioriver.ViewerMTLSTrustStore))
}

func (ClientMTLSTrustStoreResource) read(ctx context.Context, client *ioriver.IORiverClient, id interface{}) (interface{}, error) {
	return client.GetViewerMTLSTrustStore(id.(string))
}

func (ClientMTLSTrustStoreResource) update(ctx context.Context, client *ioriver.IORiverClient, obj interface{}) (interface{}, error) {
	return nil, errors.New("client mTLS trust stores are immutable")
}

func (ClientMTLSTrustStoreResource) delete(ctx context.Context, client *ioriver.IORiverClient, id interface{}) error {
	err := client.DeleteViewerMTLSTrustStore(id.(string))
	var apiErr *ioriver.APIError
	if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
		return nil
	}
	return err
}

func (ClientMTLSTrustStoreResource) getId(data interface{}) interface{} {
	return data.(ClientMTLSTrustStoreResourceModel).Id.ValueString()
}

func (ClientMTLSTrustStoreResource) resourceToObj(ctx context.Context, data interface{}) (interface{}, error) {
	model := data.(ClientMTLSTrustStoreResourceModel)
	return ioriver.ViewerMTLSTrustStore{
		Id: model.Id.ValueString(), Name: model.Name.ValueString(), Bundle: model.Bundle.ValueString(),
	}, nil
}

func (ClientMTLSTrustStoreResource) objToResource(ctx context.Context, obj interface{}, data interface{}) (interface{}, error) {
	store := obj.(*ioriver.ViewerMTLSTrustStore)
	prior := data.(ClientMTLSTrustStoreResourceModel)
	model := ClientMTLSTrustStoreResourceModel{
		Id: types.StringValue(store.Id), Name: types.StringValue(store.Name), Bundle: types.StringValue(store.Bundle),
	}
	// DRF trims outer whitespace; keep equivalent configured values, including PEM's final newline.
	if !prior.Name.IsNull() && !prior.Name.IsUnknown() && strings.TrimSpace(prior.Name.ValueString()) == strings.TrimSpace(store.Name) {
		model.Name = prior.Name
	}
	if !prior.Bundle.IsNull() && !prior.Bundle.IsUnknown() && strings.TrimSpace(prior.Bundle.ValueString()) == strings.TrimSpace(store.Bundle) {
		model.Bundle = prior.Bundle
	}
	return model, nil
}
