package provider

import (
	"context"
	"strings"

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
	ioriver "github.com/ioriver/ioriver-go"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &BehaviorResource{}
var _ resource.ResourceWithImportState = &BehaviorResource{}

func NewBehaviorResource() resource.Resource {
	return &BehaviorResource{}
}

type BehaviorResourceId struct {
}

type BehaviorResource struct{}

type HeaderNameValueModel struct {
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
	AllowedMethods *[]MethodModel `tfsdk:"allowed_methods"`
	MaxAge         types.Int64    `tfsdk:"max_age"`
}

type StreamLogsModel struct {
	UnifiedLogDestination  types.String `tfsdk:"unified_log_destination"`
	UnifiedLogSamplingRate types.Int64  `tfsdk:"unified_log_sampling_rate"`
}

type GenerateResponseModel struct {
	StatusCode       types.String `tfsdk:"status_code"`
	ResponsePagePath types.String `tfsdk:"response_page_path"`
}

type IPModel struct {
	IP types.String `tfsdk:"ip"`
}

type BehaviorResourceModel struct {
	Id          types.String                  `tfsdk:"id"`
	Service     types.String                  `tfsdk:"service"`
	Name        types.String                  `tfsdk:"name"`
	PathPattern types.String                  `tfsdk:"path_pattern"`
	IsDefault   types.Bool                    `tfsdk:"is_default"`
	Actions     []BehaviorActionResourceModel `tfsdk:"actions"`
}

type MethodModel struct {
	Method types.String `tfsdk:"method"`
}

type HeaderModel struct {
	Header types.String `tfsdk:"header"`
}

type HostHeaderModel struct {
	HeaderValue   types.String `tfsdk:"header_value"`
	UseOriginHost types.Bool   `tfsdk:"use_origin_host"`
}

type CookieModel struct {
	Cookie types.String `tfsdk:"cookie"`
}

type ParamModel struct {
	Param types.String `tfsdk:"param"`
}
type QueryStringsModel struct {
	ParamsList []ParamModel `tfsdk:"list"`
	ListType   types.String `tfsdk:"type"`
}

type CacheKeyModel struct {
	Headers      []HeaderModel     `tfsdk:"headers"`
	Cookies      []CookieModel     `tfsdk:"cookies"`
	QueryStrings QueryStringsModel `tfsdk:"query_strings"`
	Country      types.Bool        `tfsdk:"country"`
	DeviceType   types.Bool        `tfsdk:"device_type"`
}

type QueryStringsData struct {
	ParamsList []string `json:"list"`
	ListType   string   `json:"type"`
}

type CacheKeyData struct {
	Headers      []string         `json:"headers"`
	Cookies      []string         `json:"cookies"`
	QueryStrings QueryStringsData `json:"query_strings"`
}

type BehaviorActionResourceModel struct {
	ResponseHeader            *HeaderNameValueModel           `tfsdk:"response_header"`
	DeleteResponseHeader      types.String                    `tfsdk:"delete_response_header"`
	RequestHeader             *HeaderNameValueModel           `tfsdk:"request_header"`
	DeleteRequestHeader       types.String                    `tfsdk:"delete_request_header"`
	CacheTTL                  types.Int64                     `tfsdk:"cache_ttl"`
	CacheBehavior             types.String                    `tfsdk:"cache_behavior"`
	BrowserCacheTtl           types.Int64                     `tfsdk:"browser_cache_ttl"`
	ViewerProtocol            types.String                    `tfsdk:"viewer_protocol"`
	Redirect                  types.String                    `tfsdk:"redirect"`
	OriginCacheControl        types.Bool                      `tfsdk:"origin_cache_control"`
	BypassCacheOnCookie       types.String                    `tfsdk:"bypass_cache_on_cookie"`
	CacheKey                  *CacheKeyModel                  `tfsdk:"cache_key"`
	HostHeader                *HostHeaderModel                `tfsdk:"host_header"`
	CorsHeader                *HeaderNameValueModel           `tfsdk:"cors_header"`
	OverrideOrigin            types.String                    `tfsdk:"override_origin"`
	OriginErrorPassThrough    types.Bool                      `tfsdk:"origin_error_pass_through"`
	ForwardClientHeader       types.String                    `tfsdk:"forward_client_header"`
	FollowRedirects           types.Bool                      `tfsdk:"follow_redirects"`
	StatusCodeCache           *StatusCodeCacheModel           `tfsdk:"status_code_cache"`
	StatusCodeBrowserCache    *StatusCodeBrowserCacheModel    `tfsdk:"status_code_browser_cache"`
	GeneratePreflightResponse *GeneratePreflightResponseModel `tfsdk:"generate_preflight_response"`
	StaleTtl                  types.Int64                     `tfsdk:"stale_ttl"`
	StreamLogs                *StreamLogsModel                `tfsdk:"stream_logs"`
	AllowedMethods            *[]MethodModel                  `tfsdk:"allowed_methods"`
	Compression               types.Bool                      `tfsdk:"compression"`
	LargeFilesOptimization    types.Bool                      `tfsdk:"large_files_optimization"`
	GenerateResponse          *GenerateResponseModel          `tfsdk:"generate_response"`
	CachedMethods             *[]MethodModel                  `tfsdk:"cached_methods"`
	UrlSigning                types.Bool                      `tfsdk:"url_signing"`
	AllowAccessOnlyFromIP     *[]IPModel                      `tfsdk:"allow_access_only_from_ip"`
}

func (r *BehaviorResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_behavior"
}

func (r *BehaviorResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		DeprecationMessage: "ioriver resource is deprecated, Please remove this resource from your configuration.\n" +
			"Any existing configuration remains set in ioriver, and can be imported to new resource.",
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
			"actions": schema.SetNestedAttribute{
				MarkdownDescription: "Set of actions to apply for matching requests. Each element in the set defines a single action.",
				Required:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"response_header": schema.SingleNestedAttribute{
							MarkdownDescription: "Header to be added to the response",
							Optional:            true,
							Attributes: map[string]schema.Attribute{
								"header_name": schema.StringAttribute{
									MarkdownDescription: "Name of the header to be added to the response",
									Required:            true,
									Sensitive:           false,
								},
								"header_value": schema.StringAttribute{
									MarkdownDescription: "Value of the header to be added to the response",
									Required:            true,
									Sensitive:           false,
								},
							},
						},
						"delete_response_header": schema.StringAttribute{
							MarkdownDescription: "Header name to be deleted from the response",
							Optional:            true,
						},
						"request_header": schema.SingleNestedAttribute{
							MarkdownDescription: "Header to be added to the request sent to the origin",
							Optional:            true,
							Attributes: map[string]schema.Attribute{
								"header_name": schema.StringAttribute{
									MarkdownDescription: "Name of the header to be added to the request",
									Required:            true,
									Sensitive:           false,
								},
								"header_value": schema.StringAttribute{
									MarkdownDescription: "Value of the header to be added to the request",
									Required:            true,
									Sensitive:           false,
								},
							},
						},
						"delete_request_header": schema.StringAttribute{
							MarkdownDescription: "Header name to be deleted from the request",
							Optional:            true,
						},
						"cache_ttl": schema.Int64Attribute{
							MarkdownDescription: "Set the value of the edge cache TTL",
							Optional:            true,
							Validators: []validator.Int64{
								int64validator.AtLeast(0),
							},
						},
						"cache_behavior": schema.StringAttribute{
							MarkdownDescription: "Whether to bypass cache for this behavior. Valid values: `" + strings.Join(cacheBehaviorValues, "`, `") + "`",
							Optional:            true,
							Validators: []validator.String{
								stringvalidator.OneOf(cacheBehaviorValues...),
							},
						},
						"browser_cache_ttl": schema.Int64Attribute{
							MarkdownDescription: "Set the value of the browser cache TTL (Cache-Control)",
							Optional:            true,
							Validators: []validator.Int64{
								int64validator.AtLeast(0),
							},
						},
						"viewer_protocol": schema.StringAttribute{
							MarkdownDescription: "Allowed viewer protocol - can be one of the following: HTTPS_ONLY, HTTP_AND_HTTPS, or REDIRECT_HTTP_TO_HTTPS.",
							Optional:            true,
							Validators: []validator.String{
								stringvalidator.OneOf([]string{"HTTPS_ONLY", "HTTP_AND_HTTPS", "REDIRECT_HTTP_TO_HTTPS"}...),
							},
						},
						"redirect": schema.StringAttribute{
							MarkdownDescription: "Send a redirect response",
							Optional:            true,
							// The validation below makes sure each action contains only single type. It needs to be applied on each
							// action seperately. I could not find a way to place this validator in the list element scope, since then
							// AtAnyListIndex() should be used and that validates all elements together instead of each one sperately
							Validators: []validator.String{
								stringvalidator.ExactlyOneOf(path.Expressions{
									path.MatchRelative().AtParent().AtName("response_header"),
									path.MatchRelative().AtParent().AtName("delete_response_header"),
									path.MatchRelative().AtParent().AtName("request_header"),
									path.MatchRelative().AtParent().AtName("delete_request_header"),
									path.MatchRelative().AtParent().AtName("cache_ttl"),
									path.MatchRelative().AtParent().AtName("cache_behavior"),
									path.MatchRelative().AtParent().AtName("browser_cache_ttl"),
									path.MatchRelative().AtParent().AtName("redirect"),
									path.MatchRelative().AtParent().AtName("origin_cache_control"),
									path.MatchRelative().AtParent().AtName("bypass_cache_on_cookie"),
									path.MatchRelative().AtParent().AtName("cache_key"),
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
									path.MatchRelative().AtParent().AtName("stream_logs"),
									path.MatchRelative().AtParent().AtName("allowed_methods"),
									path.MatchRelative().AtParent().AtName("compression"),
									path.MatchRelative().AtParent().AtName("large_files_optimization"),
									path.MatchRelative().AtParent().AtName("generate_response"),
									path.MatchRelative().AtParent().AtName("cached_methods"),
									path.MatchRelative().AtParent().AtName("viewer_protocol"),
									path.MatchRelative().AtParent().AtName("url_signing"),
									path.MatchRelative().AtParent().AtName("allow_access_only_from_ip"),
								}...),
							},
						},
						"origin_cache_control": schema.BoolAttribute{
							MarkdownDescription: "Enable origin cache control",
							Optional:            true,
						},
						"bypass_cache_on_cookie": schema.StringAttribute{
							MarkdownDescription: "Bypass cache if the provided cookie exists",
							Optional:            true,
						},
						"cache_key": schema.SingleNestedAttribute{
							MarkdownDescription: "Custom cache key configuration",
							Optional:            true,
							Attributes: map[string]schema.Attribute{
								"headers": schema.SetNestedAttribute{
									MarkdownDescription: "Set of headers to include in the cache key",
									Required:            true,
									NestedObject: schema.NestedAttributeObject{
										Attributes: map[string]schema.Attribute{
											"header": schema.StringAttribute{
												MarkdownDescription: "Header name",
												Required:            true,
											},
										},
									},
								},
								"cookies": schema.SetNestedAttribute{
									MarkdownDescription: "Set of cookies to include in the cache key",
									Required:            true,
									NestedObject: schema.NestedAttributeObject{
										Attributes: map[string]schema.Attribute{
											"cookie": schema.StringAttribute{
												MarkdownDescription: "Cookie name",
												Required:            true,
											},
										},
									},
								},
								"query_strings": schema.SingleNestedAttribute{
									MarkdownDescription: "Cache key query strings configuration",
									Required:            true,
									Attributes: map[string]schema.Attribute{
										"list": schema.SetNestedAttribute{
											MarkdownDescription: "Set of query strings to include or exclude in the cache key",
											Required:            true,
											NestedObject: schema.NestedAttributeObject{
												Attributes: map[string]schema.Attribute{
													"param": schema.StringAttribute{
														MarkdownDescription: "Query param to include/exclude",
														Required:            true,
													},
												},
											},
										},
										"type": schema.StringAttribute{
											MarkdownDescription: "Type of the set: include, exclude, all or none",
											Required:            true,
											Validators: []validator.String{
												stringvalidator.OneOf([]string{"include", "exclude", "all", "none"}...),
											},
										},
									},
								},
								"country": schema.BoolAttribute{
									MarkdownDescription: "Include the client country in the cache key",
									Optional:            true,
								},
								"device_type": schema.BoolAttribute{
									MarkdownDescription: "Include the client device type (mobile/desktop) in the cache key",
									Optional:            true,
								},
							},
						},
						"host_header": schema.SingleNestedAttribute{
							MarkdownDescription: "Override the Host header sent to the origin with the specified value",
							Optional:            true,
							Attributes: map[string]schema.Attribute{
								"header_value": schema.StringAttribute{
									MarkdownDescription: "Value of the host header",
									Optional:            true,
								},
								"use_origin_host": schema.BoolAttribute{
									MarkdownDescription: "Use the origin domain name as the Host header for the origin",
									Optional:            true,
								},
							},
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
									MarkdownDescription: "Cache behavior for this status code. Valid values: `" + strings.Join(cacheBehaviorValues, "`, `") + "`",
									Required:            true,
									Validators: []validator.String{
										stringvalidator.OneOf(cacheBehaviorValues...),
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
								"allowed_methods": schema.SetNestedAttribute{
									MarkdownDescription: "Set of allowed HTTP methods",
									Optional:            true,
									NestedObject: schema.NestedAttributeObject{
										Attributes: map[string]schema.Attribute{
											"method": schema.StringAttribute{
												MarkdownDescription: "Allowed HTTP Method. Valid values: `" + strings.Join(httpMethodValues, "`, `") + "`",
												Required:            true,
												Validators: []validator.String{
													stringvalidator.OneOf(httpMethodValues...),
												},
											},
										},
									},
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
						"stream_logs": schema.SingleNestedAttribute{
							MarkdownDescription: "Define streaming of unifield logs",
							Optional:            true,
							Attributes: map[string]schema.Attribute{
								"unified_log_destination": schema.StringAttribute{
									MarkdownDescription: "Destination for the logs streaming",
									Required:            true,
								},
								"unified_log_sampling_rate": schema.Int64Attribute{
									MarkdownDescription: "Sampling rate for the logs (1-100)",
									Required:            true,
									Validators: []validator.Int64{
										int64validator.AtLeast(1),
										int64validator.AtMost(100),
									},
								},
							},
						},
						"allowed_methods": schema.SetNestedAttribute{
							MarkdownDescription: "Set of allowed HTTP methods",
							Optional:            true,
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"method": schema.StringAttribute{
										MarkdownDescription: "Allowed HTTP Method. Valid values: `" + strings.Join(httpMethodValues, "`, `") + "`",
										Required:            true,
										Validators: []validator.String{
											stringvalidator.OneOf(httpMethodValues...),
										},
									},
								},
							},
						},
						"compression": schema.BoolAttribute{
							MarkdownDescription: "Enable or disable compression",
							Optional:            true,
						},
						"large_files_optimization": schema.BoolAttribute{
							MarkdownDescription: "Enable cache optimization for large files. This is required for files larger than 20MB",
							Optional:            true,
						},
						"generate_response": schema.SingleNestedAttribute{
							MarkdownDescription: "Generate a custome response for specific status code(s)",
							Optional:            true,
							Attributes: map[string]schema.Attribute{
								"status_code": schema.StringAttribute{
									MarkdownDescription: "Status code to generate custome response for (1xx,2xx,.. can be used for ranges)",
									Required:            true,
								},
								"response_page_path": schema.StringAttribute{
									MarkdownDescription: "Path of the custom response page",
									Required:            true,
								},
							},
						},
						"cached_methods": schema.SetNestedAttribute{
							MarkdownDescription: "Set of HTTP methods which will be cached",
							Optional:            true,
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"method": schema.StringAttribute{
										MarkdownDescription: "Method to be cached. Valid values: `" + strings.Join(httpMethodValues, "`, `") + "`",
										Required:            true,
										Validators: []validator.String{
											stringvalidator.OneOf(httpMethodValues...),
										},
									},
								},
							},
						},
						"url_signing": schema.BoolAttribute{
							MarkdownDescription: "Enable URL signing for secure access to resources",
							Optional:            true,
						},
						"allow_access_only_from_ip": schema.SetNestedAttribute{
							MarkdownDescription: "Allow access only from specified IP addresses",
							Optional:            true,
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"ip": schema.StringAttribute{
										MarkdownDescription: "IP address or CIDR block to allow",
										Required:            true,
									},
								},
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
	// no-op: ioriver is deprecated, no client needed
}

// Create Behavior resource
func (r *BehaviorResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	resp.Diagnostics.AddError(
		"ioriver resource is deprecated",
		"Please remove this resource from your configuration.\n"+
			"Any existing configuration remains set in ioriver, and can be imported to new resource.",
	)
}

// Read Behavior resource
func (r *BehaviorResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// resource is deprecated: remove from state so Terraform stops tracking it
	resp.State.RemoveResource(ctx)
}

// Update Behavior resource
func (r *BehaviorResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError(
		"ioriver resource is deprecated",
		"Please remove this resource from your configuration.\n"+
			"Any existing configuration remains set in ioriver, and can be imported to new resource.",
	)
}

// Delete Behavior resource
func (r *BehaviorResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// no-op: resource is deprecated, Terraform will remove it from state automatically
}

// Import Behavior resource
func (r *BehaviorResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.AddError(
		"ioriver resource is deprecated",
		"Please remove this resource from your configuration.\n"+
			"Any existing configuration remains set in ioriver, and can be imported to new resource.",
	)
}

// ------- Implement base Resource API (stubs to satisfy interface) ---------

func (BehaviorResource) create(ctx context.Context, client *ioriver.IORiverClient, newObj interface{}) (interface{}, error) {
	return nil, nil
}

func (BehaviorResource) read(ctx context.Context, client *ioriver.IORiverClient, id interface{}) (interface{}, error) {
	return nil, nil
}

func (BehaviorResource) update(ctx context.Context, client *ioriver.IORiverClient, obj interface{}) (interface{}, error) {
	return nil, nil
}

func (BehaviorResource) delete(ctx context.Context, client *ioriver.IORiverClient, id interface{}) error {
	return nil
}

func (BehaviorResource) getId(data interface{}) interface{} {
	return nil
}

func (BehaviorResource) resourceToObj(ctx context.Context, data interface{}) (interface{}, error) {
	return nil, nil
}

func (BehaviorResource) objToResource(ctx context.Context, obj interface{}, data interface{}) (interface{}, error) {
	return nil, nil
}
