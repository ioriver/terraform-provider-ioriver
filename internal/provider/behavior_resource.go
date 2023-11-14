package provider

import (
	"context"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework-validators/boolvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	ioriver "ioriver.io/ioriver/ioriver-go"
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

type ResponseHeaderModel struct {
	HeaderName  types.String `tfsdk:"header_name"`
	HeaderValue types.String `tfsdk:"header_value"`
}

type StatusCodeCacheModel struct {
	StatusCode    types.String `tfsdk:"status_code"`
	CacheBehavior types.String `tfsdk:"cache_behavior"`
	CacheTTL      types.Int64  `tfsdk:"cache_ttl"`
}

type StatusCodeBrowserCacheModel struct {
	StatusCode      types.String `tfsdk:"status_code"`
	BrowserCacheTtl types.Int64  `tfsdk:"browser_cache_ttl"`
}

type GeneratePreflightResponseModel struct {
	AllowedMethods types.String `tfsdk:"allowed_methods"`
	MaxAge         types.Int64  `tfsdk:"max_age"`
}

type BehaviorResourceModel struct {
	Id          types.String                  `tfsdk:"id"`
	Service     types.String                  `tfsdk:"service"`
	Name        types.String                  `tfsdk:"name"`
	PathPattern types.String                  `tfsdk:"path_pattern"`
	IsDefault   types.Bool                    `tfsdk:"is_default"`
	Actions     []BehaviorActionResourceModel `tfsdk:"actions"`
}

type BehaviorActionResourceModel struct {
	ResponseHeader            *ResponseHeaderModel            `tfsdk:"response_header"`
	CacheTTL                  types.Int64                     `tfsdk:"cache_ttl"`
	RedirectHttpToHttps       types.Bool                      `tfsdk:"redirect_http_to_https"`
	CacheBehavior             types.String                    `tfsdk:"cache_behavior"`
	BrowserCacheTtl           types.Int64                     `tfsdk:"browser_cache_ttl"`
	Redirect                  types.String                    `tfsdk:"redirect"`
	OriginCacheControl        types.Bool                      `tfsdk:"origin_cache_control"`
	BypassCacheOnCookie       types.String                    `tfsdk:"bypass_cache_on_cookie"`
	CacheKey                  types.String                    `tfsdk:"cache_key"`
	AutoMinify                types.String                    `tfsdk:"auto_minify"`
	HostHeader                types.String                    `tfsdk:"host_header"`
	CorsHeader                *ResponseHeaderModel            `tfsdk:"cors_header"`
	OverrideOrigin            types.String                    `tfsdk:"override_origin"`
	OriginErrorPassThrough    types.Bool                      `tfsdk:"origin_error_pass_through"`
	ForwardClientHeader       types.String                    `tfsdk:"forward_client_header"`
	FollowRedirects           types.Bool                      `tfsdk:"follow_redirects"`
	StatusCodeCache           *StatusCodeCacheModel           `tfsdk:"status_code_cache"`
	StatusCodeBrowserCache    *StatusCodeBrowserCacheModel    `tfsdk:"status_code_browser_cache"`
	GeneratePreflightResponse *GeneratePreflightResponseModel `tfsdk:"generate_preflight_response"`
	StaleTtl                  types.Int64                     `tfsdk:"stale_ttl"`
}

func (r *BehaviorResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_behavior"
}

func (r *BehaviorResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Behavior resource which includes a list of behavior of actions to apply",

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
			"is_default": schema.BoolAttribute{
				MarkdownDescription: "Is this the default behavior",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.RequiresReplace(),
				},
			},
			"actions": schema.ListNestedAttribute{
				MarkdownDescription: "List of actions to apply on the path pattern. Each element in the list defines a single action.",
				Required:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"response_header": schema.SingleNestedAttribute{
							MarkdownDescription: "Header to be added within the response",
							Optional:            true,
							Attributes: map[string]schema.Attribute{
								"header_name": schema.StringAttribute{
									MarkdownDescription: "Name of the header to be added in the response",
									Required:            true,
								},
								"header_value": schema.StringAttribute{
									MarkdownDescription: "Value of the header to be added in the response",
									Required:            true,
								},
							},
						},
						"cache_ttl": schema.Int64Attribute{
							MarkdownDescription: "Set value of edge cache TTL",
							Optional:            true,
							Validators: []validator.Int64{
								int64validator.AtLeast(0),
							},
						},
						"redirect_http_to_https": schema.BoolAttribute{
							MarkdownDescription: "Enable redirect of HTTP requests to HTTPS",
							Optional:            true,
							// The validation below makes sure each action contains only single type. It needs to be applied on each
							// action seperately. I could not find a way to place this validator in the list element scope, since then
							// AtAnyListIndex() should be used and that validates all elements together instead of each one sperately
							Validators: []validator.Bool{
								boolvalidator.ExactlyOneOf(path.Expressions{
									path.MatchRelative().AtParent().AtName("response_header"),
									path.MatchRelative().AtParent().AtName("cache_ttl"),
									path.MatchRelative().AtParent().AtName("redirect_http_to_https"),
									path.MatchRelative().AtParent().AtName("cache_behavior"),
									path.MatchRelative().AtParent().AtName("browser_cache_ttl"),
									path.MatchRelative().AtParent().AtName("redirect"),
									path.MatchRelative().AtParent().AtName("origin_cache_control"),
									path.MatchRelative().AtParent().AtName("bypass_cache_on_cookie"),
									path.MatchRelative().AtParent().AtName("cache_key"),
									path.MatchRelative().AtParent().AtName("auto_minify"),
									path.MatchRelative().AtParent().AtName("host_header"),
									path.MatchRelative().AtParent().AtName("cors_header"),
									path.MatchRelative().AtParent().AtName("override_origin"),
									path.MatchRelative().AtParent().AtName("origin_error_pass_through"),
									path.MatchRelative().AtParent().AtName("forward_client_header"),
									path.MatchRelative().AtParent().AtName("follow_redirects"),
									path.MatchRelative().AtParent().AtName("status_code_cache"),
									path.MatchRelative().AtParent().AtName("generate_preflight_response"),
									path.MatchRelative().AtParent().AtName("status_code_browser_cache"),
									path.MatchRelative().AtParent().AtName("stale_ttl"),
								}...),
							},
						},
						"cache_behavior": schema.StringAttribute{
							MarkdownDescription: "Cache behavior type: CACHE or BYPASS",
							Optional:            true,
							Validators: []validator.String{
								stringvalidator.OneOf([]string{"CACHE", "BYPASS"}...),
							},
						},
						"browser_cache_ttl": schema.Int64Attribute{
							MarkdownDescription: "Set value of browser cache TTL",
							Optional:            true,
							Validators: []validator.Int64{
								int64validator.AtLeast(0),
							},
						},
						"redirect": schema.StringAttribute{
							MarkdownDescription: "Send redirect response",
							Optional:            true,
						},
						"origin_cache_control": schema.BoolAttribute{
							MarkdownDescription: "Enable origin cache control",
							Optional:            true,
						},
						"bypass_cache_on_cookie": schema.StringAttribute{
							MarkdownDescription: "Bypass cache if the provided cookie exists",
							Optional:            true,
						},
						"cache_key": schema.StringAttribute{
							MarkdownDescription: "Use custom cache key configuration",
							Optional:            true,
						},
						"auto_minify": schema.StringAttribute{
							MarkdownDescription: "Use the provided auto-minify configuration",
							Optional:            true,
						},
						"host_header": schema.StringAttribute{
							MarkdownDescription: "Override host header with the provided value",
							Optional:            true,
						},
						"cors_header": schema.SingleNestedAttribute{
							MarkdownDescription: "CORS header to be added within the response",
							Optional:            true,
							Attributes: map[string]schema.Attribute{
								"header_name": schema.StringAttribute{
									MarkdownDescription: "Name of the CORS header to be added in the response",
									Required:            true,
									Validators: []validator.String{
										stringvalidator.OneOfCaseInsensitive([]string{
											"access-control-allow-credentials",
											"access-control-allow-headers",
											"access-control-allow-methods",
											"access-control-allow-origin",
											"access-control-expose-headers",
											"access-control-max-age",
										}...),
									},
								},
								"header_value": schema.StringAttribute{
									MarkdownDescription: "Value of the header to be added in the response",
									Required:            true,
								},
							},
						},
						"override_origin": schema.StringAttribute{
							MarkdownDescription: "Value of origin id",
							Optional:            true,
						},
						"origin_error_pass_through": schema.BoolAttribute{
							MarkdownDescription: "Enable origin error pass through",
							Optional:            true,
						},
						"forward_client_header": schema.StringAttribute{
							MarkdownDescription: "Header to be forwarded to the origin",
							Optional:            true,
						},
						"follow_redirects": schema.BoolAttribute{
							MarkdownDescription: "Enable follow redirect in case origin returns a redirect response",
							Optional:            true,
						},
						"status_code_cache": schema.SingleNestedAttribute{
							MarkdownDescription: "Define edge cache configuration for status code(s)",
							Optional:            true,
							Attributes: map[string]schema.Attribute{
								"status_code": schema.StringAttribute{
									MarkdownDescription: "Status code to apply the configuratoin for (1xx,2xx,.. can be used for ranges)",
									Required:            true,
								},
								"cache_behavior": schema.StringAttribute{
									MarkdownDescription: "Cache behavior type: CACHE or BYPASS",
									Required:            true,
									Validators: []validator.String{
										stringvalidator.OneOf([]string{"CACHE", "BYPASS"}...),
									},
								},
								"cache_ttl": schema.Int64Attribute{
									MarkdownDescription: "Value of edge cache TTL",
									Required:            true,
									Validators: []validator.Int64{
										int64validator.AtLeast(0),
									},
								},
							},
						},
						"generate_preflight_response": schema.SingleNestedAttribute{
							MarkdownDescription: "Define auto generate preflight response",
							Optional:            true,
							Attributes: map[string]schema.Attribute{
								"allowed_methods": schema.StringAttribute{
									MarkdownDescription: "Comma separated allowed methods (value of `Access-Control-Allow-Methods` response header)",
									Required:            true,
								},
								"max_age": schema.Int64Attribute{
									MarkdownDescription: "Response cache TTL (value of `Access-Control-Max-Age` response header)",
									Required:            true,
									Validators: []validator.Int64{
										int64validator.AtLeast(0),
									},
								},
							},
						},
						"status_code_browser_cache": schema.SingleNestedAttribute{
							MarkdownDescription: "Define browser cache configuration for status code(s)",
							Optional:            true,
							Attributes: map[string]schema.Attribute{
								"status_code": schema.StringAttribute{
									MarkdownDescription: "Status code to apply the configuratoin for (1xx,2xx,.. can be used for ranges)",
									Required:            true,
								},
								"browser_cache_ttl": schema.Int64Attribute{
									MarkdownDescription: "Value of browser cache TTL",
									Required:            true,
									Validators: []validator.Int64{
										int64validator.AtLeast(0),
									},
								},
							},
						},
						"stale_ttl": schema.Int64Attribute{
							MarkdownDescription: "Set value of stale TTL (in case of origin issue, the CDN will serve stale content for that period of time)",
							Optional:            true,
							Validators: []validator.Int64{
								int64validator.AtLeast(0),
							},
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
	var doUpdate = false

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	// in case this is a default behavior, it needs to be udpated instead of created
	if data.IsDefault.ValueBool() {
		id, err := r.getDefaultBehaviorId(r.client, data.Service.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Error creating default behavior", "Unexpected error: "+err.Error())
			return
		}
		data.Id = types.StringValue(id)
		doUpdate = true
	}

	newData := resourceCreate(r.client, ctx, req, resp, r, data, doUpdate)
	if newData == nil {
		tflog.Error(ctx, fmt.Sprintf("Failed to create IORiver object"))
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

	// in case this is a default behavior, it needs to be udpated to default instead of deleted
	if data.IsDefault.ValueBool() {
		err := r.client.ResetDefaultBehavior(data.Service.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Error deleting default behavior", "Unexpected error: "+err.Error())
		}
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

func (BehaviorResource) getDefaultBehaviorId(client *ioriver.IORiverClient, serviceId string) (string, error) {
	behaviors, err := client.ListBehaviors(serviceId)
	if err != nil {
		return "", err
	}

	for _, behavior := range behaviors {
		if behavior.IsDefault {
			return behavior.Id, nil
		}
	}
	return "", fmt.Errorf("unable to find default behavior for service %s", serviceId)
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
		behaviorActions = append(behaviorActions, *ba)
	}

	return ioriver.Behavior{
		Id:          d.Id.ValueString(),
		Service:     d.Service.ValueString(),
		Name:        d.Name.ValueString(),
		PathPattern: d.PathPattern.ValueString(),
		IsDefault:   d.IsDefault.ValueBool(),
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
		IsDefault:   types.BoolValue(behavior.IsDefault),
		Actions:     modelActions,
	}, nil
}

func modelToBehaviorAction(action BehaviorActionResourceModel) (*ioriver.BehaviorAction, error) {

	if action.ResponseHeader != nil {
		return &ioriver.BehaviorAction{
			Type:                ioriver.SET_RESPONSE_HEADER,
			ResponseHeaderName:  action.ResponseHeader.HeaderName.ValueString(),
			ResponseHeaderValue: action.ResponseHeader.HeaderValue.ValueString(),
		}, nil
	}
	if !action.CacheTTL.IsNull() {
		return &ioriver.BehaviorAction{
			Type:   ioriver.CACHE_TTL,
			MaxTTL: int(action.CacheTTL.ValueInt64()),
		}, nil
	}
	if !action.RedirectHttpToHttps.IsNull() {
		return &ioriver.BehaviorAction{
			Type: ioriver.REDIRECT_HTTP_TO_HTTPS,
		}, nil
	}
	if !action.CacheBehavior.IsNull() {
		return &ioriver.BehaviorAction{
			Type:               ioriver.CACHE_BEHAVIOR,
			CacheBehaviorValue: action.CacheBehavior.ValueString(),
		}, nil
	}
	if !action.BrowserCacheTtl.IsNull() {
		return &ioriver.BehaviorAction{
			Type:   ioriver.BROWSER_CACHE_TTL,
			MaxTTL: int(action.BrowserCacheTtl.ValueInt64()),
		}, nil
	}
	if !action.Redirect.IsNull() {
		return &ioriver.BehaviorAction{
			Type:        ioriver.REDIRECT,
			RedirectURL: action.Redirect.ValueString(),
		}, nil
	}
	if !action.OriginCacheControl.IsNull() {
		return &ioriver.BehaviorAction{
			Type:                      ioriver.ORIGIN_CACHE_CONTROL,
			OriginCacheControlEnabled: action.OriginCacheControl.ValueBool(),
		}, nil
	}
	if !action.BypassCacheOnCookie.IsNull() {
		return &ioriver.BehaviorAction{
			Type:   ioriver.BYPASS_CACHE_ON_COOKIE,
			Cookie: action.BypassCacheOnCookie.ValueString(),
		}, nil
	}
	if !action.CacheKey.IsNull() {
		return &ioriver.BehaviorAction{
			Type:     ioriver.CACHE_KEY,
			CacheKey: action.CacheKey.ValueString(),
		}, nil
	}
	if !action.AutoMinify.IsNull() {
		return &ioriver.BehaviorAction{
			Type:       ioriver.AUTO_MINIFY,
			AutoMinify: action.AutoMinify.ValueString(),
		}, nil
	}
	if !action.HostHeader.IsNull() {
		return &ioriver.BehaviorAction{
			Type:       ioriver.HOST_HEADER_OVERRIDE,
			HostHeader: action.HostHeader.ValueString(),
		}, nil
	}
	if action.CorsHeader != nil {
		return &ioriver.BehaviorAction{
			Type:                ioriver.SET_CORS_HEADER,
			ResponseHeaderName:  action.CorsHeader.HeaderName.ValueString(),
			ResponseHeaderValue: action.CorsHeader.HeaderValue.ValueString(),
		}, nil
	}
	if !action.OverrideOrigin.IsNull() {
		return &ioriver.BehaviorAction{
			Type:   ioriver.OVERRIDE_ORIGIN,
			Origin: action.OverrideOrigin.ValueString(),
		}, nil
	}
	if !action.OriginErrorPassThrough.IsNull() {
		return &ioriver.BehaviorAction{
			Type:    ioriver.ORIGIN_ERRORS_PASS_THRU,
			Enabled: action.OriginErrorPassThrough.ValueBool(),
		}, nil
	}
	if !action.ForwardClientHeader.IsNull() {
		return &ioriver.BehaviorAction{
			Type:             ioriver.FORWARD_CLIENT_HEADER,
			ClientHeaderName: action.ForwardClientHeader.ValueString(),
		}, nil
	}
	if !action.FollowRedirects.IsNull() {
		return &ioriver.BehaviorAction{
			Type: ioriver.FOLLOW_REDIRECTS,
		}, nil
	}
	if action.StatusCodeCache != nil {
		statusCode, err := statusCodeToInt(action.StatusCodeCache.StatusCode.ValueString())
		if err != nil {
			return nil, fmt.Errorf("Invalid status code %s", action.StatusCodeCache.StatusCode.ValueString())
		}
		return &ioriver.BehaviorAction{
			Type:               ioriver.STATUS_CODE_CACHE,
			StatusCode:         statusCode,
			CacheBehaviorValue: action.StatusCodeCache.CacheBehavior.ValueString(),
			MaxTTL:             int(action.StatusCodeCache.CacheTTL.ValueInt64()),
		}, nil
	}
	if action.GeneratePreflightResponse != nil {
		return &ioriver.BehaviorAction{
			Type:                ioriver.GENERATE_PREFLIGHT_RESPONSE,
			ResponseHeaderValue: action.GeneratePreflightResponse.AllowedMethods.ValueString(),
			MaxTTL:              int(action.GeneratePreflightResponse.MaxAge.ValueInt64()),
		}, nil
	}
	if action.StatusCodeBrowserCache != nil {
		statusCode, err := statusCodeToInt(action.StatusCodeBrowserCache.StatusCode.ValueString())
		if err != nil {
			return nil, fmt.Errorf("Invalid status code %s", action.StatusCodeCache.StatusCode.ValueString())
		}
		return &ioriver.BehaviorAction{
			Type:       ioriver.STATUS_CODE_BROWSER_CACHE,
			StatusCode: statusCode,
			MaxTTL:     int(action.StatusCodeBrowserCache.BrowserCacheTtl.ValueInt64()),
		}, nil
	}
	if !action.StaleTtl.IsNull() {
		return &ioriver.BehaviorAction{
			Type:   ioriver.STALE_TTL,
			MaxTTL: int(action.StaleTtl.ValueInt64()),
		}, nil
	}

	return nil, fmt.Errorf("Unsupported action type")
}

func behaviorActionToModel(behaviorAction ioriver.BehaviorAction) (*BehaviorActionResourceModel, error) {
	actionType := types.StringValue(string(behaviorAction.Type))

	if behaviorAction.Type == ioriver.CACHE_TTL {
		return &BehaviorActionResourceModel{
			CacheTTL: types.Int64Value(int64(behaviorAction.MaxTTL)),
		}, nil
	}
	if behaviorAction.Type == ioriver.SET_RESPONSE_HEADER {
		responseHeader := &ResponseHeaderModel{
			HeaderName:  types.StringValue(behaviorAction.ResponseHeaderName),
			HeaderValue: types.StringValue(behaviorAction.ResponseHeaderValue),
		}
		return &BehaviorActionResourceModel{
			ResponseHeader: responseHeader,
		}, nil
	}
	if behaviorAction.Type == ioriver.REDIRECT_HTTP_TO_HTTPS {
		return &BehaviorActionResourceModel{
			RedirectHttpToHttps: types.BoolValue(true),
		}, nil
	}
	if behaviorAction.Type == ioriver.CACHE_BEHAVIOR {
		return &BehaviorActionResourceModel{
			CacheBehavior: types.StringValue(behaviorAction.CacheBehaviorValue),
		}, nil
	}
	if behaviorAction.Type == ioriver.BROWSER_CACHE_TTL {
		return &BehaviorActionResourceModel{
			BrowserCacheTtl: types.Int64Value(int64(behaviorAction.MaxTTL)),
		}, nil
	}
	if behaviorAction.Type == ioriver.REDIRECT {
		return &BehaviorActionResourceModel{
			Redirect: types.StringValue(behaviorAction.RedirectURL),
		}, nil
	}
	if behaviorAction.Type == ioriver.ORIGIN_CACHE_CONTROL {
		return &BehaviorActionResourceModel{
			OriginCacheControl: types.BoolValue(behaviorAction.OriginCacheControlEnabled),
		}, nil
	}
	if behaviorAction.Type == ioriver.BYPASS_CACHE_ON_COOKIE {
		return &BehaviorActionResourceModel{
			BypassCacheOnCookie: types.StringValue(behaviorAction.Cookie),
		}, nil
	}
	if behaviorAction.Type == ioriver.CACHE_KEY {
		return &BehaviorActionResourceModel{
			CacheKey: types.StringValue(behaviorAction.CacheKey),
		}, nil
	}
	if behaviorAction.Type == ioriver.AUTO_MINIFY {
		return &BehaviorActionResourceModel{
			AutoMinify: types.StringValue(behaviorAction.AutoMinify),
		}, nil
	}
	if behaviorAction.Type == ioriver.HOST_HEADER_OVERRIDE {
		return &BehaviorActionResourceModel{
			HostHeader: types.StringValue(behaviorAction.HostHeader),
		}, nil
	}
	if behaviorAction.Type == ioriver.SET_CORS_HEADER {
		responseHeader := &ResponseHeaderModel{
			HeaderName:  types.StringValue(behaviorAction.ResponseHeaderName),
			HeaderValue: types.StringValue(behaviorAction.ResponseHeaderValue),
		}
		return &BehaviorActionResourceModel{
			CorsHeader: responseHeader,
		}, nil
	}
	if behaviorAction.Type == ioriver.ORIGIN_ERRORS_PASS_THRU {
		return &BehaviorActionResourceModel{
			OriginErrorPassThrough: types.BoolValue(behaviorAction.Enabled),
		}, nil
	}
	if behaviorAction.Type == ioriver.FORWARD_CLIENT_HEADER {
		return &BehaviorActionResourceModel{
			ForwardClientHeader: types.StringValue(behaviorAction.ClientHeaderName),
		}, nil
	}
	if behaviorAction.Type == ioriver.FOLLOW_REDIRECTS {
		return &BehaviorActionResourceModel{
			FollowRedirects: types.BoolValue(true),
		}, nil
	}
	if behaviorAction.Type == ioriver.STATUS_CODE_CACHE {
		statusCodeCache := &StatusCodeCacheModel{
			StatusCode:    types.StringValue(statusCodeToString(behaviorAction.StatusCode)),
			CacheBehavior: types.StringValue(behaviorAction.CacheBehaviorValue),
			CacheTTL:      types.Int64Value(int64(behaviorAction.MaxTTL)),
		}
		return &BehaviorActionResourceModel{
			StatusCodeCache: statusCodeCache,
		}, nil
	}
	if behaviorAction.Type == ioriver.GENERATE_PREFLIGHT_RESPONSE {
		genPreflightResp := &GeneratePreflightResponseModel{
			AllowedMethods: types.StringValue(behaviorAction.ResponseHeaderValue),
			MaxAge:         types.Int64Value(int64(behaviorAction.MaxTTL)),
		}
		return &BehaviorActionResourceModel{
			GeneratePreflightResponse: genPreflightResp,
		}, nil
	}
	if behaviorAction.Type == ioriver.STATUS_CODE_BROWSER_CACHE {
		statusCodeBrowserCache := &StatusCodeBrowserCacheModel{
			StatusCode:      types.StringValue(statusCodeToString(behaviorAction.StatusCode)),
			BrowserCacheTtl: types.Int64Value(int64(behaviorAction.MaxTTL)),
		}
		return &BehaviorActionResourceModel{
			StatusCodeBrowserCache: statusCodeBrowserCache,
		}, nil
	}
	if behaviorAction.Type == ioriver.STALE_TTL {
		return &BehaviorActionResourceModel{
			StaleTtl: types.Int64Value(int64(behaviorAction.MaxTTL)),
		}, nil
	}
	return nil, fmt.Errorf("Unsupported action type %s", actionType)
}

func statusCodeToInt(statusCode string) (int, error) {
	switch statusCode {
	case "1xx":
		return 1, nil
	case "2xx":
		return 2, nil
	case "3xx":
		return 3, nil
	case "4xx":
		return 4, nil
	case "5xx":
		return 5, nil
	}
	return strconv.Atoi(statusCode)
}

func statusCodeToString(statusCode int) string {
	switch statusCode {
	case 1:
		return "1xx"
	case 2:
		return "2xx"
	case 3:
		return "4xx"
	case 4:
		return "4xx"
	case 5:
		return "5xx"
	}
	return fmt.Sprintf("%d", statusCode)
}
