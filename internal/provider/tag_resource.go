package provider

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	ioriver "github.com/ioriver/ioriver-go"
)

var tagColors = map[string]string{
	"green": "#0AA636", "blue": "#0065FF", "purple": "#8232D6",
	"orange": "#D6781A", "red": "#D63838", "turquoise": "#1AA0C8", "gray": "#5A697A",
}

var tagColorNames = map[string]string{
	"#0AA636": "green", "#0065FF": "blue", "#8232D6": "purple",
	"#D6781A": "orange", "#D63838": "red", "#1AA0C8": "turquoise", "#5A697A": "gray",
}

var (
	_ Resource                         = &TagResource{}
	_ resource.Resource                = &TagResource{}
	_ resource.ResourceWithConfigure   = &TagResource{}
	_ resource.ResourceWithImportState = &TagResource{}
)

type TagResource struct {
	client *ioriver.IORiverClient
}

type TagResourceModel struct {
	Id              types.String `tfsdk:"id"`
	Name            types.String `tfsdk:"name"`
	Color           types.String `tfsdk:"color"`
	Services        types.Set    `tfsdk:"services"`
	membershipError error
}

type TagWithServices struct {
	Tag             ioriver.Tag
	Services        []string
	MembershipError error
}

func NewTagResource() resource.Resource { return &TagResource{} }

func tagColorToAPI(name string) (string, error) {
	color, ok := tagColors[name]
	if !ok {
		return "", fmt.Errorf("unsupported tag color name %q; use green, blue, purple, orange, red, turquoise, or gray", name)
	}
	return color, nil
}

func tagColorFromAPI(color string) (string, error) {
	name, ok := tagColorNames[strings.ToUpper(color)]
	if !ok {
		return "", fmt.Errorf("unsupported backend tag color %q; change the tag to a supported palette color before reading or importing it", color)
	}
	return name, nil
}

func (r *TagResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tag"
}

func (r *TagResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an account tag and its complete set of service memberships visible to the API token. Membership changes are applied individually.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Tag identifier",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Tag name, unique within the account.",
				Validators:          []validator.String{stringvalidator.LengthBetween(1, 255)},
			},
			"color": schema.StringAttribute{
				Optional: true, Computed: true,
				Default:             stringdefault.StaticString("gray"),
				MarkdownDescription: "Tag color: green, blue, purple, orange, red, turquoise, or gray. Defaults to gray.",
				Validators:          []validator.String{stringvalidator.OneOf("green", "blue", "purple", "orange", "red", "turquoise", "gray")},
			},
			"services": schema.SetAttribute{
				Optional: true, Computed: true, ElementType: types.StringType,
				Default:             setdefault.StaticValue(types.SetValueMust(types.StringType, []attr.Value{})),
				MarkdownDescription: "Authoritative set of service UUIDs for this tag within the API token's visibility. Omitting this attribute or setting an empty set detaches the tag from all visible services.",
			},
		},
	}
}

func (r *TagResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = ConfigureBase(ctx, req, resp)
}

func (r *TagResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data TagResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	newData := resourceCreate(r.client, ctx, req, resp, r, data, false)
	if newData == nil || resp.Diagnostics.HasError() {
		return
	}
	data = newData.(TagResourceModel)
	if data.Services.IsNull() {
		data.Services = types.SetValueMust(types.StringType, []attr.Value{})
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	if !resp.Diagnostics.HasError() && data.membershipError != nil {
		resp.Diagnostics.AddError("Error setting tag services", data.membershipError.Error())
	}
}

func (r *TagResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data TagResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	newData := resourceRead(r.client, ctx, req, resp, r, data)
	if newData == nil || resp.Diagnostics.HasError() {
		return
	}
	updated := newData.(TagResourceModel)
	if updated.membershipError != nil {
		resp.Diagnostics.AddError("Error reading tag services", updated.membershipError.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &updated)...)
}

func (r *TagResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data, prior TagResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}
	newData := resourceUpdate(r.client, ctx, req, resp, r, data)
	if newData == nil || resp.Diagnostics.HasError() {
		return
	}
	data = newData.(TagResourceModel)
	if data.Services.IsNull() {
		data.Services = prior.Services
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	if !resp.Diagnostics.HasError() && data.membershipError != nil {
		resp.Diagnostics.AddError("Error setting tag services", data.membershipError.Error())
	}
}

func (r *TagResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data TagResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resourceDelete(r.client, ctx, req, resp, r, data)
}

func (r *TagResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (TagResource) create(_ context.Context, client *ioriver.IORiverClient, obj interface{}) (interface{}, error) {
	request := obj.(TagWithServices)
	tag, err := client.CreateTag(request.Tag)
	if err != nil {
		return nil, err
	}
	return updateTagServices(client, *tag, request.Services), nil
}

func (TagResource) read(_ context.Context, client *ioriver.IORiverClient, id interface{}) (interface{}, error) {
	tag, err := client.GetTag(id.(string))
	if err != nil {
		return nil, err
	}
	result := &TagWithServices{Tag: *tag}
	result.Services, result.MembershipError = listTagServiceIDs(client, tag.Id)
	return result, nil
}

func (TagResource) update(_ context.Context, client *ioriver.IORiverClient, obj interface{}) (interface{}, error) {
	request := obj.(TagWithServices)
	tag, err := client.UpdateTag(request.Tag)
	if err != nil {
		return nil, err
	}
	return updateTagServices(client, *tag, request.Services), nil
}

func (TagResource) delete(_ context.Context, client *ioriver.IORiverClient, id interface{}) error {
	return client.DeleteTag(id.(string))
}

func (TagResource) getId(data interface{}) interface{} {
	return data.(TagResourceModel).Id.ValueString()
}

func (TagResource) resourceToObj(ctx context.Context, data interface{}) (interface{}, error) {
	model := data.(TagResourceModel)
	color, err := tagColorToAPI(model.Color.ValueString())
	if err != nil {
		return nil, err
	}
	var services []string
	if !model.Services.IsNull() && !model.Services.IsUnknown() {
		if diags := model.Services.ElementsAs(ctx, &services, false); diags.HasError() {
			return nil, fmt.Errorf("decode tag services: %v", diags.Errors())
		}
	}
	return TagWithServices{Tag: ioriver.Tag{Id: model.Id.ValueString(), Name: model.Name.ValueString(), Color: color}, Services: services}, nil
}

func (TagResource) objToResource(ctx context.Context, obj interface{}, _ interface{}) (interface{}, error) {
	result := obj.(*TagWithServices)
	color, err := tagColorFromAPI(result.Tag.Color)
	if err != nil {
		return nil, err
	}
	services := types.SetNull(types.StringType)
	if result.Services != nil {
		value, diags := types.SetValueFrom(ctx, types.StringType, result.Services)
		if diags.HasError() {
			return nil, fmt.Errorf("convert tag services: %v", diags.Errors())
		}
		services = value
	}
	return TagResourceModel{
		Id: types.StringValue(result.Tag.Id), Name: types.StringValue(result.Tag.Name),
		Color: types.StringValue(color), Services: services, membershipError: result.MembershipError,
	}, nil
}

func listTagServiceIDs(client *ioriver.IORiverClient, tagID string) ([]string, error) {
	services, err := client.ListServicesByTagId(tagID)
	if err != nil {
		return nil, fmt.Errorf("read services for tag %q: %w", tagID, err)
	}
	ids := make([]string, len(services))
	for i, service := range services {
		ids[i] = service.Id
	}
	return ids, nil
}

func updateTagServices(client *ioriver.IORiverClient, tag ioriver.Tag, desired []string) *TagWithServices {
	result := &TagWithServices{Tag: tag}
	current, err := listTagServiceIDs(client, tag.Id)
	if err != nil {
		result.MembershipError = err
		return result
	}
	result.MembershipError = reconcileTagServices(client, tag.Id, tag.Name, desired, current)
	result.Services, err = listTagServiceIDs(client, tag.Id)
	if err != nil {
		result.Services = current
		result.MembershipError = errors.Join(result.MembershipError, err)
	}
	return result
}

func reconcileTagServices(client *ioriver.IORiverClient, tagID, tagName string, desired, current []string) error {
	add, remove := diffTagServices(desired, current)
	for _, id := range add {
		if _, err := client.AddServiceTags(id, []string{tagName}); err != nil {
			return fmt.Errorf("attach tag %s to service %s: %w", tagID, id, err)
		}
	}
	for _, id := range remove {
		if _, err := client.RemoveServiceTag(id, tagName); err != nil {
			return fmt.Errorf("detach tag %s from service %s: %w", tagID, id, err)
		}
	}
	return nil
}

func diffTagServices(desired, current []string) (add, remove []string) {
	desiredSet, currentSet := make(map[string]bool, len(desired)), make(map[string]bool, len(current))
	for _, id := range desired {
		desiredSet[id] = true
	}
	for _, id := range current {
		currentSet[id] = true
	}
	for id := range desiredSet {
		if !currentSet[id] {
			add = append(add, id)
		}
	}
	for id := range currentSet {
		if !desiredSet[id] {
			remove = append(remove, id)
		}
	}
	sort.Strings(add)
	sort.Strings(remove)
	return add, remove
}
