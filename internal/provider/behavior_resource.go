package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	ioriver "github.com/ioriver/ioriver-go"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &BehaviorResource{}
var _ resource.ResourceWithImportState = &BehaviorResource{}

func NewBehaviorResource() resource.Resource {
	return &BehaviorResource{}
}

type BehaviorResourceId struct {
	behaviorId string
	serviceId  string
}

type BehaviorResource struct {
	client *ioriver.IORiverClient
}

type BehaviorResourceModel struct {
	Id          types.String                  `tfsdk:"id"`
	Service     types.String                  `tfsdk:"service"`
	Name        types.String                  `tfsdk:"name"`
	PathPattern types.String                  `tfsdk:"path_pattern"`
	Actions     []BehaviorActionResourceModel `tfsdk:"actions"`
}

type BehaviorActionResourceModel struct {
	Type                      types.String `tfsdk:"type"`
	MaxTTL                    types.Int64  `tfsdk:"max_ttl"`
	ResponseHeaderName        types.String `tfsdk:"response_header_name"`
	ResponseHeaderValue       types.String `tfsdk:"response_header_value"`
	CacheBehaviorValue        types.String `tfsdk:"cache_behavior_value"`
	RedirectURL               types.String `tfsdk:"redirect_url"`
	OriginCacheControlEnabled types.Bool   `tfsdk:"origin_cache_control_enabled"`
	Pattern                   types.String `tfsdk:"pattern"`
	Cookie                    types.String `tfsdk:"cookie"`
	AutoMinify                types.String `tfsdk:"auto_minify"`
	HostHeader                types.String `tfsdk:"host_header"`
	Origin                    types.String `tfsdk:"origin"`
	Enabled                   types.Bool   `tfsdk:"enabled"`
	CacheKey                  types.String `tfsdk:"cache_key"`
	ClientHeaderName          types.String `tfsdk:"client_header_name"`
	ActionDisabled            types.Bool   `tfsdk:"action_disabled"`
}

func (r *BehaviorResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_behavior"
}

func (r *BehaviorResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Behavior resource",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Behavior identifier",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"service": schema.StringAttribute{
				MarkdownDescription: "The id of the service this behavior belongs to",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Behavior name",
				Required:            true,
			},
			"path_pattern": schema.StringAttribute{
				MarkdownDescription: "Path pattern to apply the behavior",
				Required:            true,
			},
			"actions": schema.ListNestedAttribute{
				MarkdownDescription: "List of actions to apply",
				Required:            true,
				// TODO: need to replace it with more granular plan modifier. If action was modified, only this action should be modified.
				PlanModifiers: []planmodifier.List{
					listplanmodifier.RequiresReplace(),
				},
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"type": schema.StringAttribute{
							MarkdownDescription: "Type of the action",
							Required:            true,
						},
						"max_ttl": schema.Int64Attribute{
							MarkdownDescription: "TTL value",
							Optional:            true,
						},
						"response_header_name": schema.StringAttribute{
							MarkdownDescription: "Name of the header to be added in the response",
							Optional:            true,
						},
						"response_header_value": schema.StringAttribute{
							MarkdownDescription: "Value of the header to be added in the response",
							Optional:            true,
						},
						"cache_behavior_value": schema.StringAttribute{
							MarkdownDescription: "Value of cache behavior (CACHE/BYPASS)",
							Optional:            true,
						},
						"redirect_url": schema.StringAttribute{
							MarkdownDescription: "Value of redirect URL",
							Optional:            true,
						},
						"origin_cache_control_enabled": schema.BoolAttribute{
							MarkdownDescription: "Value of origin cache control",
							Optional:            true,
						},
						"pattern": schema.StringAttribute{
							MarkdownDescription: "Value of pattern",
							Optional:            true,
						},
						"cookie": schema.StringAttribute{
							MarkdownDescription: "Value of cookie",
							Optional:            true,
						},
						"auto_minify": schema.StringAttribute{
							MarkdownDescription: "Value of auto-minify",
							Optional:            true,
						},
						"host_header": schema.StringAttribute{
							MarkdownDescription: "Value of Host header",
							Optional:            true,
						},
						"origin": schema.StringAttribute{
							MarkdownDescription: "Value of origin id",
							Optional:            true,
						},
						"enabled": schema.BoolAttribute{
							MarkdownDescription: "Is action the type of this action enabled",
							Optional:            true,
						},
						"cache_key": schema.StringAttribute{
							MarkdownDescription: "Cache key configuration",
							Optional:            true,
						},
						"client_header_name": schema.StringAttribute{
							MarkdownDescription: "Value of client header name",
							Optional:            true,
						},
						"action_disabled": schema.BoolAttribute{
							MarkdownDescription: "Is this action disabled",
							Optional:            true,
						},
					},
				},
			},
		},
	}
}

// Configure resource and retrieve API client
func (r *BehaviorResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	client := ConfigureBase(ctx, req, resp)
	if client == nil {
		return
	}
	r.client = client
}

// Create Behavior resource
func (r *BehaviorResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data BehaviorResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	newData := resourceCreate(r.client, ctx, req, resp, r, data)
	if newData == nil {
		tflog.Error(ctx, fmt.Sprintf("Failed to IORiver object"))
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newData)...)
}

// Read Behavior resource
func (r *BehaviorResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data BehaviorResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	newData := resourceRead(r.client, ctx, req, resp, r, data)
	if newData == nil {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newData)...)
}

// Update Behavior resource
func (r *BehaviorResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data BehaviorResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	newData := resourceUpdate(r.client, ctx, req, resp, r, data)
	if newData == nil {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newData)...)
}

// Delete Behavior resource
func (r *BehaviorResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data BehaviorResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resourceDelete(r.client, ctx, req, resp, r, data)
}

// Import Behavior resource
func (r *BehaviorResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	serviceResourceImport(ctx, req, resp)
}

// ------- Implement base Resource API ---------

func (BehaviorResource) create(client *ioriver.IORiverClient, newObj interface{}) (interface{}, error) {
	return client.CreateBehavior(newObj.(ioriver.Behavior))
}

func (BehaviorResource) read(client *ioriver.IORiverClient, id interface{}) (interface{}, error) {
	resourceId := id.(BehaviorResourceId)
	return client.GetBehavior(resourceId.serviceId, resourceId.behaviorId)
}

func (BehaviorResource) update(client *ioriver.IORiverClient, obj interface{}) (interface{}, error) {
	return client.UpdateBehavior(obj.(ioriver.Behavior))
}

func (BehaviorResource) delete(client *ioriver.IORiverClient, id interface{}) error {
	resourceId := id.(BehaviorResourceId)
	return client.DeleteBehavior(resourceId.serviceId, resourceId.behaviorId)
}

func (BehaviorResource) getId(data interface{}) interface{} {
	d := data.(BehaviorResourceModel)
	behaviorId := d.Id.ValueString()
	serviceId := d.Service.ValueString()
	return BehaviorResourceId{behaviorId, serviceId}
}

// Convert Behavior resource to Behavior API object
func (BehaviorResource) resourceToObj(ctx context.Context, data interface{}) (interface{}, error) {
	d := data.(BehaviorResourceModel)

	// convert actions
	behaviorActions := []ioriver.BehaviorAction{}
	for _, action := range d.Actions {
		ba, err := modelToBehaviorAction(action)
		if err != nil {
			return nil, err
		}
		ba.ActionDisabled = action.ActionDisabled.ValueBool()
		behaviorActions = append(behaviorActions, *ba)
	}

	return ioriver.Behavior{
		Id:          d.Id.ValueString(),
		Service:     d.Service.ValueString(),
		Name:        d.Name.ValueString(),
		PathPattern: d.PathPattern.ValueString(),
		Actions:     behaviorActions,
	}, nil
}

// Convert Behavior API object to Behavior resource
func (BehaviorResource) objToResource(ctx context.Context, obj interface{}) (interface{}, error) {
	behavior := obj.(*ioriver.Behavior)

	// convert actions
	modelActions := []BehaviorActionResourceModel{}
	for _, action := range behavior.Actions {
		modelAction, err := behaviorActionToModel(action)
		if err != nil {
			return nil, err
		}
		modelActions = append(modelActions, *modelAction)
	}

	return BehaviorResourceModel{
		Id:          types.StringValue(behavior.Id),
		Service:     types.StringValue(behavior.Service),
		Name:        types.StringValue(behavior.Name),
		PathPattern: types.StringValue(behavior.PathPattern),
		Actions:     modelActions,
	}, nil
}

func modelToBehaviorAction(action BehaviorActionResourceModel) (*ioriver.BehaviorAction, error) {
	actionType := ioriver.ActionType(action.Type.ValueString())

	if actionType == ioriver.SET_RESPONSE_HEADER {
		return &ioriver.BehaviorAction{
			Type:                actionType,
			ResponseHeaderName:  action.ResponseHeaderName.ValueString(),
			ResponseHeaderValue: action.ResponseHeaderValue.ValueString(),
		}, nil
	} else if actionType == ioriver.CACHE_TTL {
		return &ioriver.BehaviorAction{
			Type:   actionType,
			MaxTTL: int(action.MaxTTL.ValueInt64()),
		}, nil
	} else if actionType == ioriver.REDIRECT_HTTP_TO_HTTPS {
		return &ioriver.BehaviorAction{
			Type: actionType,
		}, nil
	} else if actionType == ioriver.CACHE_BEHAVIOR {
		return &ioriver.BehaviorAction{
			Type:               actionType,
			CacheBehaviorValue: action.CacheBehaviorValue.ValueString(),
		}, nil
	} else if actionType == ioriver.BROWSER_CACHE_TTL {
		return &ioriver.BehaviorAction{
			Type:   actionType,
			MaxTTL: int(action.MaxTTL.ValueInt64()),
		}, nil
	} else if actionType == ioriver.REDIRECT {
		return &ioriver.BehaviorAction{
			Type:        actionType,
			RedirectURL: action.RedirectURL.ValueString(),
		}, nil
	} else if actionType == ioriver.ORIGIN_CACHE_CONTROL {
		return &ioriver.BehaviorAction{
			Type:                      actionType,
			OriginCacheControlEnabled: action.OriginCacheControlEnabled.ValueBool(),
		}, nil
	} else if actionType == ioriver.DISABLE_WAF {
		return &ioriver.BehaviorAction{
			Type: actionType,
		}, nil
	} else if actionType == ioriver.BYPASS_CACHE_ON_COOKIE {
		return &ioriver.BehaviorAction{
			Type:   actionType,
			Cookie: action.Cookie.ValueString(),
		}, nil
	} else if actionType == ioriver.CACHE_KEY {
		return &ioriver.BehaviorAction{
			Type:     actionType,
			CacheKey: action.CacheKey.ValueString(),
		}, nil
	} else if actionType == ioriver.AUTO_MINIFY {
		return &ioriver.BehaviorAction{
			Type:       actionType,
			AutoMinify: action.AutoMinify.ValueString(),
		}, nil
	} else if actionType == ioriver.HOST_HEADER_OVERRIDE {
		return &ioriver.BehaviorAction{
			Type:       actionType,
			HostHeader: action.HostHeader.ValueString(),
		}, nil
	} else if actionType == ioriver.SET_CORS_HEADER {
		return &ioriver.BehaviorAction{
			Type:                actionType,
			ResponseHeaderName:  action.ResponseHeaderName.ValueString(),
			ResponseHeaderValue: action.ResponseHeaderValue.ValueString(),
		}, nil
	} else if actionType == ioriver.ORIGIN_ERRORS_PASS_THRU {
		return &ioriver.BehaviorAction{
			Type:    actionType,
			Enabled: action.Enabled.ValueBool(),
		}, nil
	}

	return nil, fmt.Errorf("Unsupported action type %s", actionType)
}

func behaviorActionToModel(behaviorAction ioriver.BehaviorAction) (*BehaviorActionResourceModel, error) {
	actionType := types.StringValue(string(behaviorAction.Type))

	if behaviorAction.Type == ioriver.SET_RESPONSE_HEADER {
		return &BehaviorActionResourceModel{
			Type:                actionType,
			ResponseHeaderName:  types.StringValue(behaviorAction.ResponseHeaderName),
			ResponseHeaderValue: types.StringValue(behaviorAction.ResponseHeaderValue),
		}, nil
	}
	if behaviorAction.Type == ioriver.CACHE_TTL {
		return &BehaviorActionResourceModel{
			Type:   actionType,
			MaxTTL: types.Int64Value(int64(behaviorAction.MaxTTL)),
		}, nil
	}
	if behaviorAction.Type == ioriver.REDIRECT_HTTP_TO_HTTPS {
		return &BehaviorActionResourceModel{
			Type: actionType,
		}, nil
	}
	if behaviorAction.Type == ioriver.CACHE_BEHAVIOR {
		return &BehaviorActionResourceModel{
			Type:               actionType,
			CacheBehaviorValue: types.StringValue(behaviorAction.CacheBehaviorValue),
		}, nil
	}
	if behaviorAction.Type == ioriver.BROWSER_CACHE_TTL {
		return &BehaviorActionResourceModel{
			Type:   actionType,
			MaxTTL: types.Int64Value(int64(behaviorAction.MaxTTL)),
		}, nil
	}
	if behaviorAction.Type == ioriver.REDIRECT {
		return &BehaviorActionResourceModel{
			Type:        actionType,
			RedirectURL: types.StringValue(behaviorAction.RedirectURL),
		}, nil
	}
	if behaviorAction.Type == ioriver.ORIGIN_CACHE_CONTROL {
		return &BehaviorActionResourceModel{
			Type:                      actionType,
			OriginCacheControlEnabled: types.BoolValue(behaviorAction.OriginCacheControlEnabled),
		}, nil
	}
	if behaviorAction.Type == ioriver.DISABLE_WAF {
		return &BehaviorActionResourceModel{
			Type: actionType,
		}, nil
	}
	if behaviorAction.Type == ioriver.BYPASS_CACHE_ON_COOKIE {
		return &BehaviorActionResourceModel{
			Type:   actionType,
			Cookie: types.StringValue(behaviorAction.Cookie),
		}, nil
	}
	if behaviorAction.Type == ioriver.CACHE_KEY {
		return &BehaviorActionResourceModel{
			Type:     actionType,
			CacheKey: types.StringValue(behaviorAction.CacheKey),
		}, nil
	}
	if behaviorAction.Type == ioriver.AUTO_MINIFY {
		return &BehaviorActionResourceModel{
			Type:       actionType,
			AutoMinify: types.StringValue(behaviorAction.AutoMinify),
		}, nil
	}
	if behaviorAction.Type == ioriver.HOST_HEADER_OVERRIDE {
		return &BehaviorActionResourceModel{
			Type:       actionType,
			HostHeader: types.StringValue(behaviorAction.HostHeader),
		}, nil
	}
	if behaviorAction.Type == ioriver.SET_CORS_HEADER {
		return &BehaviorActionResourceModel{
			Type:                actionType,
			ResponseHeaderName:  types.StringValue(behaviorAction.ResponseHeaderName),
			ResponseHeaderValue: types.StringValue(behaviorAction.ResponseHeaderValue),
		}, nil
	}
	if behaviorAction.Type == ioriver.ORIGIN_ERRORS_PASS_THRU {
		return &BehaviorActionResourceModel{
			Type:    actionType,
			Enabled: types.BoolValue(behaviorAction.Enabled),
		}, nil
	}
	return nil, fmt.Errorf("Unsupported action type %s", actionType)
}
