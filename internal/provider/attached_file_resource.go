package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
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

var (
	_ Resource                         = &AttachedFileResource{}
	_ resource.Resource                = &AttachedFileResource{}
	_ resource.ResourceWithConfigure   = &AttachedFileResource{}
	_ resource.ResourceWithImportState = &AttachedFileResource{}
)

type AttachedFileResource struct {
	client *ioriver.IORiverClient
}

type AttachedFileResourceModel struct {
	ID       types.String `tfsdk:"id"`
	FileType types.String `tfsdk:"file_type"`
	Contents types.String `tfsdk:"contents"`
}

func NewAttachedFileResource() resource.Resource {
	return &AttachedFileResource{}
}

func (r *AttachedFileResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_attached_file"
}

func (r *AttachedFileResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an immutable, account-scoped response attachment. Changes to contents (excluding outer whitespace) or file_type require replacement. " +
			"Use create_before_destroy when referenced by a service. Deletion is rejected while any service configuration version, including historical versions, references the attachment. " +
			"The API deduplicates identical contents and file types within an account: do not manage the same attachment with multiple resource addresses.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Attachment UUID returned by the backend.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"file_type": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Attachment type. Currently only HTML is supported. Changes require replacement.",
				Validators:          []validator.String{stringvalidator.OneOf("HTML")},
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"contents": schema.StringAttribute{
				Required: true,
				MarkdownDescription: "Text contents, supplied inline with a heredoc or using `file(\"${path.module}/blocked.html\")` for an existing UTF-8 file. Stored in Terraform state. " +
					"Maximum 2 MiB of UTF-8 data and 2,000,000 characters; whitespace-only contents are not accepted. The API trims outer whitespace. Content changes require replacement; outer-whitespace-only changes update state without an API call.",
				Validators: []validator.String{
					stringvalidator.UTF8LengthBetween(1, 2000000),
					stringvalidator.LengthAtMost(2 * 1024 * 1024),
					stringvalidator.RegexMatches(regexp.MustCompile(`\S`), "must contain non-whitespace characters"),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplaceIf(
						attachedFileContentsRequireReplace,
						"Replace when contents change after trimming outer whitespace.",
						"Replace when contents change after trimming outer whitespace.",
					),
				},
			},
		},
	}
}

func (r *AttachedFileResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = ConfigureBase(ctx, req, resp)
}

func (r *AttachedFileResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data AttachedFileResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if newData := resourceCreate(r.client, ctx, req, resp, r, data, false); newData != nil {
		resp.Diagnostics.Append(resp.State.Set(ctx, newData)...)
	}
}

func (r *AttachedFileResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data AttachedFileResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	obj, err := performOperation(ctx, func() (interface{}, error) {
		return r.read(ctx, r.client, data.ID.ValueString())
	})
	var apiErr *ioriver.APIError
	if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error reading attachment", err.Error())
		return
	}
	model, err := r.objToResource(ctx, obj, data)
	if err != nil {
		resp.Diagnostics.AddError("Error reading attachment", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}

func (r *AttachedFileResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state AttachedFileResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !plan.FileType.Equal(state.FileType) ||
		strings.TrimSpace(plan.Contents.ValueString()) != strings.TrimSpace(state.Contents.ValueString()) {
		resp.Diagnostics.AddError("Immutable attachment", "Changing contents or file_type requires replacing the attachment.")
		return
	}
	// Avoid replacing a deduplicated attachment with itself for whitespace-only edits.
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func attachedFileContentsRequireReplace(ctx context.Context, req planmodifier.StringRequest, resp *stringplanmodifier.RequiresReplaceIfFuncResponse) {
	resp.RequiresReplace = req.PlanValue.IsUnknown() ||
		strings.TrimSpace(req.PlanValue.ValueString()) != strings.TrimSpace(req.StateValue.ValueString())
}

func (r *AttachedFileResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data AttachedFileResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	resourceDelete(r.client, ctx, req, resp, r, data)
}

func (r *AttachedFileResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (AttachedFileResource) create(ctx context.Context, client *ioriver.IORiverClient, obj interface{}) (interface{}, error) {
	file := obj.(ioriver.AttachedFile)
	return client.CreateAttachedFile(file.FileType, file.Contents)
}

func (AttachedFileResource) read(ctx context.Context, client *ioriver.IORiverClient, id interface{}) (interface{}, error) {
	return client.GetAttachedFile(id.(string))
}

func (AttachedFileResource) update(ctx context.Context, client *ioriver.IORiverClient, obj interface{}) (interface{}, error) {
	return nil, errors.New("attachments are immutable")
}

func (AttachedFileResource) delete(ctx context.Context, client *ioriver.IORiverClient, id interface{}) error {
	err := client.DeleteAttachedFile(id.(string))
	var apiErr *ioriver.APIError
	if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
		return nil
	}
	if err != nil {
		return fmt.Errorf("delete attachment %s (all service configuration versions must release it first): %w", id, err)
	}
	return nil
}

func (AttachedFileResource) getId(data interface{}) interface{} {
	return data.(AttachedFileResourceModel).ID.ValueString()
}

func (AttachedFileResource) resourceToObj(ctx context.Context, data interface{}) (interface{}, error) {
	model := data.(AttachedFileResourceModel)
	return ioriver.AttachedFile{FileType: model.FileType.ValueString(), Contents: model.Contents.ValueString()}, nil
}

func (AttachedFileResource) objToResource(ctx context.Context, obj interface{}, data interface{}) (interface{}, error) {
	file := obj.(*ioriver.AttachedFile)
	if file.ID == "" {
		return nil, errors.New("attachment API returned an empty id")
	}
	prior := data.(AttachedFileResourceModel)
	model := AttachedFileResourceModel{
		ID: types.StringValue(file.ID), FileType: types.StringValue(file.FileType), Contents: types.StringValue(file.Contents),
	}
	// DRF trims outer whitespace; preserve equivalent configured file contents.
	if !prior.Contents.IsNull() && !prior.Contents.IsUnknown() &&
		strings.TrimSpace(prior.Contents.ValueString()) == file.Contents {
		model.Contents = prior.Contents
	}
	return model, nil
}
