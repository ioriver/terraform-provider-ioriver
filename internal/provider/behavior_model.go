package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework-validators/boolvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// ---------------------------------------------------------------------------
// Concrete backend defaults for the 5 default_behavior.actions fields.
// Values observed from a real backend API call with no default behavior set.
// ---------------------------------------------------------------------------

func methodObj(m string) attr.Value {
	return types.ObjectValueMust(map[string]attr.Type{"method": types.StringType},
		map[string]attr.Value{"method": types.StringValue(m)})
}

// Default values for behavior actions
var DefaultCacheTTL int64 = 86400
var defaultCacheTTLValue = types.Int64Value(DefaultCacheTTL)

var methodElemType = types.ObjectType{AttrTypes: map[string]attr.Type{"method": types.StringType}}

var defaultAllowedMethodsValue = types.SetValueMust(methodElemType, []attr.Value{
	methodObj("DELETE"), methodObj("GET"), methodObj("HEAD"), methodObj("OPTIONS"),
	methodObj("PATCH"), methodObj("POST"), methodObj("PUT"),
})

var defaultCachedMethodsValue = types.SetValueMust(methodElemType, []attr.Value{
	methodObj("GET"), methodObj("HEAD"),
})

// CACHE KEY DEFAULTS
var cacheKeyHeaderElemType = types.ObjectType{AttrTypes: map[string]attr.Type{"header": types.StringType}}
var defaultCacheKeyHeaderValue = types.SetValueMust(cacheKeyHeaderElemType, []attr.Value{})

var cacheKeyParamElemType = types.ObjectType{AttrTypes: map[string]attr.Type{"param": types.StringType}}
var cacheKeyQueryStringAttrType = map[string]attr.Type{
	"type":   types.StringType,
	"params": types.SetType{ElemType: cacheKeyParamElemType},
}
var defaultCacheKeyQueryStringValue = types.ObjectValueMust(
	cacheKeyQueryStringAttrType,
	map[string]attr.Value{
		"type":   types.StringValue("all"),
		"params": types.SetNull(cacheKeyParamElemType),
	},
)

var cacheKeyCookieElemType = types.ObjectType{AttrTypes: map[string]attr.Type{"cookie": types.StringType}}
var defaultCacheKeyCookieValue = types.SetValueMust(cacheKeyCookieElemType, []attr.Value{})

var defaultCacheKeyDeviceType = false
var defaultCacheKeyCountry = false

var defaultCacheKeyValue = types.ObjectValueMust(
	map[string]attr.Type{
		"headers":       types.SetType{ElemType: cacheKeyHeaderElemType},
		"cookies":       types.SetType{ElemType: cacheKeyCookieElemType},
		"query_strings": types.ObjectType{AttrTypes: cacheKeyQueryStringAttrType},
		"country":       types.BoolType,
		"device_type":   types.BoolType,
	},
	map[string]attr.Value{
		// non default behaviors get empty list, while default behavior will get "host" in the headers list
		"headers": types.SetValueMust(cacheKeyHeaderElemType, []attr.Value{
			types.ObjectValueMust(cacheKeyHeaderElemType.AttrTypes, map[string]attr.Value{"header": types.StringValue("host")}),
		}),
		"cookies":       defaultCacheKeyCookieValue,
		"query_strings": defaultCacheKeyQueryStringValue,
		"country":       types.BoolValue(defaultCacheKeyCountry),
		"device_type":   types.BoolValue(defaultCacheKeyDeviceType),
	},
)

var defaultStatusCodesTtlValue = func() types.List {
	elemType := types.ObjectType{AttrTypes: map[string]attr.Type{
		"status_code": types.StringType, "cache_behavior": types.StringType, "cache_ttl": types.Int64Type,
	}}
	mk := func(code, behavior string, ttl int64) attr.Value {
		return types.ObjectValueMust(elemType.AttrTypes, map[string]attr.Value{
			"status_code": types.StringValue(code), "cache_behavior": types.StringValue(behavior), "cache_ttl": types.Int64Value(ttl),
		})
	}
	return types.ListValueMust(elemType, []attr.Value{mk("4xx", "CACHE", 10), mk("5xx", "CACHE", 10)})
}()

var defaultCompression bool = true
var defaultCompressionValue = types.BoolValue(defaultCompression)

var defaultCacheBehavior string = "CACHE"
var defaultCacheBehaviorValue = types.StringValue(defaultCacheBehavior)

// DefaultActionsObject is applied to the actions attribute itself so that omitting
// actions {} entirely still gets the backend defaults filled in.
func GetDefaultActionsValue() types.Object {
	at := BehaviorActionAttrTypes()

	vals := make(map[string]attr.Value, len(at))
	for k, t := range at {
		vals[k] = nullValueForType(t)
	}

	// Fields the backend always fills in for the default behavior
	vals["cache_ttl"] = defaultCacheTTLValue
	vals["cache_key"] = defaultCacheKeyValue
	vals["allowed_methods"] = defaultAllowedMethodsValue
	vals["cached_methods"] = defaultCachedMethodsValue
	vals["status_codes_ttl"] = defaultStatusCodesTtlValue
	vals["compression"] = defaultCompressionValue
	vals["cache_behavior"] = defaultCacheBehaviorValue

	return types.ObjectValueMust(at, vals)
}

var cacheBehaviorValues = []string{"BYPASS", "CACHE"}

var httpMethodValues = []string{"GET", "HEAD", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}

var viewerProtocolValues = []string{"HTTPS_ONLY", "HTTP_AND_HTTPS", "REDIRECT_HTTP_TO_HTTPS"}

// ---------------------------------------------------------------------------
// corsValuesRequireSpecificMode is a list validator that rejects non-empty
// values/origins when the sibling `mode` attribute is not "specific".
// Use it on any CORS `values` or `origins` list attribute.
// ---------------------------------------------------------------------------

type corsValuesRequireSpecificModeValidator struct{}

func (v corsValuesRequireSpecificModeValidator) Description(_ context.Context) string {
	return `values/origins may only be set when mode is "specific"`
}

func (v corsValuesRequireSpecificModeValidator) MarkdownDescription(_ context.Context) string {
	return "values/origins may only be set when `mode` is `specific`"
}

func (v corsValuesRequireSpecificModeValidator) ValidateList(ctx context.Context, req validator.ListRequest, resp *validator.ListResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() || len(req.ConfigValue.Elements()) == 0 {
		return
	}
	modePath := req.Path.ParentPath().AtName("mode")
	var mode types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, modePath, &mode)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !mode.IsNull() && !mode.IsUnknown() && mode.ValueString() != "specific" {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Conflicting CORS Configuration",
			fmt.Sprintf(`values/origins must not be set when mode is %q — they are only used with mode = "specific"`, mode.ValueString()),
		)
	}
}

// corsValuesRequireSpecificMode returns the validator as a validator.List.
func corsValuesRequireSpecificMode() validator.List {
	return corsValuesRequireSpecificModeValidator{}
}

// ---------------------------------------------------------------------------
// corsSpecificModeRequiresValues is a string validator on the `mode` attribute.
// When mode is "specific", the sibling `values` (or `origins`) list must be
// non-empty.  The sibling attribute name is configurable via valuesAttr.
// ---------------------------------------------------------------------------

type corsSpecificModeRequiresValuesValidator struct {
	valuesAttr string // "values" or "origins"
}

func (v corsSpecificModeRequiresValuesValidator) Description(_ context.Context) string {
	return fmt.Sprintf(`when mode is "specific", %s must be non-empty`, v.valuesAttr)
}

func (v corsSpecificModeRequiresValuesValidator) MarkdownDescription(_ context.Context) string {
	return fmt.Sprintf("when `mode` is `specific`, `%s` must be non-empty", v.valuesAttr)
}

func (v corsSpecificModeRequiresValuesValidator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	if req.ConfigValue.ValueString() != "specific" {
		return
	}
	valuesPath := req.Path.ParentPath().AtName(v.valuesAttr)
	var values types.List
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, valuesPath, &values)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if values.IsNull() || values.IsUnknown() || len(values.Elements()) == 0 {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Missing CORS Configuration",
			fmt.Sprintf(`mode is "specific" but %s is empty — provide at least one value`, v.valuesAttr),
		)
	}
}

// corsSpecificRequiresValues returns a string validator for the `mode` attribute
// that enforces a non-empty sibling list when mode = "specific".
func corsSpecificRequiresValues(valuesAttr string) validator.String {
	return corsSpecificModeRequiresValuesValidator{valuesAttr: valuesAttr}
}

// ---------------------------------------------------------------------------
// queryStringTypeValidator is a string validator on the `type` attribute
// inside cache_key.query_strings.
// It enforces both directions of the list/type relationship:
//   - "include" / "exclude" → sibling `list` must be non-empty
//   - "all" / "none"        → sibling `list` must be absent/empty
// ---------------------------------------------------------------------------

type queryStringTypeValidator struct{}

func (v queryStringTypeValidator) Description(_ context.Context) string {
	return `include/exclude require a non-empty params; all/none forbid it`
}

func (v queryStringTypeValidator) MarkdownDescription(_ context.Context) string {
	return "- `include` / `exclude`: `params` must be non-empty\n- `all` / `none`: `params` must be omitted"
}

func (v queryStringTypeValidator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	t := req.ConfigValue.ValueString()

	listPath := req.Path.ParentPath().AtName("params")
	var list types.Set
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, listPath, &list)...)
	if resp.Diagnostics.HasError() {
		return
	}
	isEmpty := list.IsNull() || list.IsUnknown() || len(list.Elements()) == 0
	isAbsent := list.IsNull() || list.IsUnknown()

	switch t {
	case "include", "exclude":
		if isEmpty {
			resp.Diagnostics.AddAttributeError(
				req.Path,
				"Missing Query String Configuration",
				fmt.Sprintf(`type is %q but params is empty — provide at least one param`, t),
			)
		}
	case "all", "none":
		if !isAbsent {
			resp.Diagnostics.AddAttributeError(
				req.Path,
				"Conflicting Query String Configuration",
				fmt.Sprintf(`params must not be set when type is %q — omit the params block or set params = null`, t),
			)
		}
	}
}

// queryStringType returns the unified query-string validator as a validator.String.
func queryStringType() validator.String {
	return queryStringTypeValidator{}
}

// ValidateBehaviorModel validates that exactly one of path_pattern or condition is set,
// and that at least one action field is populated.
func ValidateBehaviorModel(b *BehaviorModel, prefix string) []string {
	hasPathPattern := !b.PathPattern.IsNull() && !b.PathPattern.IsUnknown() && b.PathPattern.ValueString() != ""
	hasCondition := b.Condition != nil

	if hasPathPattern && hasCondition {
		return []string{fmt.Sprintf("%s: only one of 'path_pattern' or 'condition' may be set", prefix)}
	}
	if !hasPathPattern && !hasCondition {
		return []string{fmt.Sprintf("%s: one of 'path_pattern' or 'condition' must be set", prefix)}
	}

	var errs []string
	if isBehaviorActionsEmpty(b.Actions) {
		errs = append(errs, fmt.Sprintf("%s: actions must have at least one field set", prefix))
	}
	if hasCondition {
		errs = append(errs, ValidateConditionModel(b.Condition, prefix, BehaviorConditionSpec)...)
	}
	if b.Actions != nil {
		errs = append(errs, validateHeaderActions(b.Actions.RequestHeaders, prefix+".actions.request_headers")...)
		errs = append(errs, validateHeaderActions(b.Actions.ResponseHeaders, prefix+".actions.response_headers")...)
		errs = append(errs, validateHeaderActions(b.Actions.OriginResponseHeaders, prefix+".actions.origin_response_headers")...)
	}
	return errs
}

// validateHeaderActions checks that each header action entry is consistent:
// - action = "delete" must not provide values
// - action = "set" or "add" must provide at least one value
func validateHeaderActions(entries *[]HeaderActionModelV2, prefix string) []string {
	if entries == nil {
		return nil
	}
	var errs []string
	for i, h := range *entries {
		action := h.Action.ValueString()
		hasValues := len(h.Values) > 0
		loc := fmt.Sprintf("%s[%d] (name=%q)", prefix, i, h.Name.ValueString())
		switch action {
		case "delete":
			if hasValues {
				errs = append(errs, fmt.Sprintf("%s: action \"delete\" must not include values", loc))
			}
		case "set", "add":
			if !hasValues {
				errs = append(errs, fmt.Sprintf("%s: action %q requires at least one value", loc, action))
			}
		}
	}
	return errs
}

// isBehaviorActionsEmpty returns true when no field inside the actions block is set.
// Used by ValidateBehaviorModel to reject an empty actions = {} block.
func isBehaviorActionsEmpty(a *BehaviorActionV2ResourceModel) bool {
	if a == nil {
		return true
	}
	// Pointer/slice fields — nil or empty means unset
	if a.CacheKey != nil ||
		a.HostHeader != nil ||
		a.Cors != nil ||
		a.Redirect != nil ||
		a.GeneratePreflightResponse != nil ||
		len(a.StatusCodeBrowserCache) > 0 ||
		a.StreamLogs != nil ||
		len(a.StatusCodeCustomResponse) > 0 ||
		len(a.ProviderSpecific) > 0 ||
		a.AllowedMethods != nil ||
		a.CachedMethods != nil ||
		a.AllowAccessOnlyFromIP != nil ||
		a.DenyAccessByIP != nil ||
		a.DenyAccessByTime != nil ||
		a.UrlRewrites != nil ||
		a.RequestHeaders != nil ||
		a.ResponseHeaders != nil ||
		a.OriginResponseHeaders != nil ||
		len(a.StatusCodeCache) > 0 {
		return false
	}
	// Scalar types — null/unknown means unset
	if !a.CacheTTL.IsNull() && !a.CacheTTL.IsUnknown() {
		return false
	}
	if !a.CacheBehavior.IsNull() && !a.CacheBehavior.IsUnknown() {
		return false
	}
	if !a.BrowserCacheTtl.IsNull() && !a.BrowserCacheTtl.IsUnknown() {
		return false
	}
	if !a.ViewerProtocol.IsNull() && !a.ViewerProtocol.IsUnknown() {
		return false
	}
	if !a.OriginCacheControl.IsNull() && !a.OriginCacheControl.IsUnknown() {
		return false
	}
	if !a.FollowRedirects.IsNull() && !a.FollowRedirects.IsUnknown() {
		return false
	}
	if !a.StaleTtl.IsNull() && !a.StaleTtl.IsUnknown() {
		return false
	}
	if !a.Compression.IsNull() && !a.Compression.IsUnknown() {
		return false
	}
	if !a.LargeFilesOptimization.IsNull() && !a.LargeFilesOptimization.IsUnknown() {
		return false
	}
	if !a.UrlSigning.IsNull() && !a.UrlSigning.IsUnknown() {
		return false
	}
	if !a.TrueClientIP.IsNull() && !a.TrueClientIP.IsUnknown() {
		return false
	}
	if !a.DenyAccess.IsNull() && !a.DenyAccess.IsUnknown() {
		return false
	}
	return true
}

// Service Config API response structures
type ServiceConfigAPIBehavior struct {
	Name        string                 `json:"name,omitempty"`
	PathPattern string                 `json:"path_pattern,omitempty"`
	Condition   interface{}            `json:"condition,omitempty"`
	Action      ServiceConfigAPIAction `json:"action"`
	Children    []interface{}          `json:"children"`
	UUID        string                 `json:"uuid,omitempty"`
}

type ServiceConfigAPIRedirect struct {
	Destination string  `json:"destination"`
	Source      *string `json:"source,omitempty"`
}

type ServiceConfigAPIAction struct {
	// Simple scalar actions
	CacheTTL               *int                           `json:"cache_ttl,omitempty"`
	CacheBehavior          *ServiceConfigAPICacheBehavior `json:"cache_behavior,omitempty"`
	BrowserCacheTTL        *int                           `json:"browser_cache_ttl,omitempty"`
	ViewerProtocol         *string                        `json:"viewer_protocol,omitempty"`
	Redirect               *ServiceConfigAPIRedirect      `json:"redirect,omitempty"`
	OriginCacheControl     *bool                          `json:"origin_cache_control,omitempty"`
	FollowRedirects        *bool                          `json:"follow_redirects,omitempty"`
	StaleTTL               *int                           `json:"stale_ttl,omitempty"`
	Compression            *bool                          `json:"compression,omitempty"`
	LargeFilesOptimization *bool                          `json:"large_files_optimization,omitempty"`
	URLSigning             *bool                          `json:"url_signing,omitempty"`
	TrueClientIP           *bool                          `json:"true_client_ip,omitempty"`
	DenyAccess             *bool                          `json:"deny_access,omitempty"`
	// Host header: backend uses flat fields, not nested object
	HostHeaderOverride  *string `json:"host_header_override,omitempty"`
	HostHeaderUseOrigin *bool   `json:"host_header_use_origin,omitempty"`

	// Complex nested actions
	CacheKey                  *ServiceConfigAPICacheKey                  `json:"cache_key,omitempty"`
	StatusCodeCache           []ServiceConfigAPIStatusCodeCache          `json:"status_code_cache,omitempty"`
	GeneratePreflightResponse *ServiceConfigAPIPreflightResponse         `json:"generate_preflight,omitempty"`
	StatusCodeBrowserCache    []ServiceConfigAPIStatusCodeBrowserCache   `json:"status_code_browser_cache,omitempty"`
	StreamLogs                *ServiceConfigAPIStreamLogs                `json:"logs_streaming,omitempty"`
	Cors                      *ServiceConfigAPICors                      `json:"cors,omitempty"`
	AllowedMethods            []string                                   `json:"allowed_methods,omitempty"`
	AllowAccessOnlyFromIP     []ServiceConfigAPIIP                       `json:"allow_access_only_from_ip,omitempty"`
	DenyAccessByIP            []string                                   `json:"deny_access_by_ip,omitempty"`
	DenyAccessByTime          []ServiceConfigAPITimeConstraint           `json:"deny_access_by_time,omitempty"`
	UrlRewrites               []ServiceConfigAPIUrlRewrite               `json:"url_rewrites,omitempty"`
	StatusCodeCustomResponse  []ServiceConfigAPIStatusCodeCustomResponse `json:"status_code_custom_response,omitempty"`
	ProviderSpecific          []ServiceConfigAPIProviderSpecific         `json:"unmanaged_behavior,omitempty"`
	// Header modification actions — shape matches the Python backend model:
	// each entry: { name, values, override, delete }
	RequestHeaders        []ServiceConfigAPIHeaderAction `json:"request_headers,omitempty"`
	ResponseHeaders       []ServiceConfigAPIHeaderAction `json:"response_headers,omitempty"`
	OriginResponseHeaders []ServiceConfigAPIHeaderAction `json:"origin_response_headers,omitempty"`
	Children              []interface{}                  `json:"children,omitempty"`
}

type ServiceConfigAPIUrlRewrite struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
}

// ServiceConfigAPIHeaderAction matches the Python HttpHeaderAction shape:
// the header name is stored as "name" (the dict key in Python → list element in JSON).
type ServiceConfigAPIHeaderAction struct {
	Name     string   `json:"name"`
	Values   []string `json:"values,omitempty"`
	Override *bool    `json:"override,omitempty"`
	Delete   *bool    `json:"delete,omitempty"`
}

type ServiceConfigAPIDateTimeWindow struct {
	StartDate int64 `json:"start_date"`
	EndDate   int64 `json:"end_date"`
}

type ServiceConfigAPITimePeriodic struct {
	StartDate         int64  `json:"start_date"`
	Duration          int64  `json:"duration"`
	DurationUnits     string `json:"duration_units"`
	RepeatPeriod      int64  `json:"repeat_period"`
	RepeatPeriodUnits string `json:"repeat_period_units"`
}

type ServiceConfigAPITimeConstraint struct {
	DateTimeWindow *ServiceConfigAPIDateTimeWindow `json:"date_time_window,omitempty"`
	TimePeriodic   *ServiceConfigAPITimePeriodic   `json:"time_periodic,omitempty"`
}

type ServiceConfigAPIHeaderValue struct {
	HeaderName  string `json:"header_name"`
	HeaderValue string `json:"header_value"`
}

type ServiceConfigAPICacheBehavior struct {
	BypassCache   bool     `json:"bypass_cache"`
	CachedMethods []string `json:"cached_methods,omitempty"`
}

type ServiceConfigAPICacheKey struct {
	Headers         []string `json:"headers"`
	Cookies         []string `json:"cookies"`
	QueryParams     []string `json:"query_params"`
	QueryParamsMode string   `json:"query_params_mode"`
	Country         bool     `json:"country"`
	DeviceType      bool     `json:"device_type"`
}

type ServiceConfigAPIHeader struct {
	Header string `json:"header"`
}

type ServiceConfigAPICookie struct {
	Cookie string `json:"cookie"`
}

type ServiceConfigAPIParam struct {
	Param string `json:"param"`
}

type ServiceConfigAPIHostHeader struct {
	HeaderValue   string `json:"header_value,omitempty"`
	UseOriginHost *bool  `json:"use_origin_host,omitempty"`
}

type ServiceConfigAPICorsAllowOrigin struct {
	Mode     string   `json:"mode"`
	Origins  []string `json:"origins,omitempty"`
	Override *bool    `json:"override,omitempty"`
}

type ServiceConfigAPICorsValueList struct {
	Mode     string   `json:"mode"`
	Values   []string `json:"values,omitempty"`
	Override *bool    `json:"override,omitempty"`
}

type ServiceConfigAPICors struct {
	AllowOrigin      *ServiceConfigAPICorsAllowOrigin `json:"allow_origin,omitempty"`
	AllowHeaders     *ServiceConfigAPICorsValueList   `json:"allow_headers,omitempty"`
	ExposeHeaders    *ServiceConfigAPICorsValueList   `json:"expose_headers,omitempty"`
	AllowMethods     *ServiceConfigAPICorsValueList   `json:"allow_methods,omitempty"`
	MaxAge           *int                             `json:"max_age,omitempty"`
	OverrideMaxAge   *bool                            `json:"override_max_age,omitempty"`
	AllowCredentials *bool                            `json:"allow_credentials,omitempty"`
}

type ServiceConfigAPIStatusCodeCache struct {
	Code        string `json:"code"`
	TTL         int    `json:"ttl"`
	BypassCache bool   `json:"bypass_cache"`
}

type ServiceConfigAPIPreflightResponse struct {
	AllowedMethods []string `json:"allowed_methods,omitempty"`
	AllowedHeaders []string `json:"allowed_headers,omitempty"`
	MaxTTL         *int     `json:"max_ttl,omitempty"`
}

type ServiceConfigAPIMethod struct {
	Method string `json:"method"`
}

type ServiceConfigAPIStatusCodeBrowserCache struct {
	Code string `json:"code"`
	TTL  int    `json:"ttl"`
}

type ServiceConfigAPIStreamLogs struct {
	UnifiedLogDestination  string `json:"destination"`
	UnifiedLogSamplingRate int    `json:"sampling_percentage"`
}

type ServiceConfigAPIStatusCodeCustomResponse struct {
	Code        string `json:"code"`
	ResponseURL string `json:"response_url"`
}

type ServiceConfigAPIIP struct {
	IP string `json:"ip"`
}

type ServiceConfigAPIProviderSpecific struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// ========== V2 Model Structs (self-contained, independent of behavior_resource.go) ==========

type HeaderNameValueModelV2 struct {
	HeaderName  types.String `tfsdk:"header_name"`
	HeaderValue types.String `tfsdk:"header_value"`
}

type MethodModelV2 struct {
	Method types.String `tfsdk:"method"`
}

type HeaderModelV2 struct {
	Header types.String `tfsdk:"header"`
}

type DeleteHeaderModelV2 struct {
	HeaderName types.String `tfsdk:"header_name"`
}

type CorsAllowOriginModelV2 struct {
	Mode     types.String   `tfsdk:"mode"`
	Origins  []types.String `tfsdk:"origins"`
	Override types.Bool     `tfsdk:"override"`
}

type CorsHeaderListModelV2 struct {
	Mode     types.String   `tfsdk:"mode"`
	Values   []types.String `tfsdk:"values"`
	Override types.Bool     `tfsdk:"override"`
}

type CorsMethodListModelV2 struct {
	Mode     types.String   `tfsdk:"mode"`
	Values   []types.String `tfsdk:"values"`
	Override types.Bool     `tfsdk:"override"`
}

type CorsMaxAgeModelV2 struct {
	Value    types.Int64 `tfsdk:"value"`
	Override types.Bool  `tfsdk:"override"`
}

type CorsConfigModelV2 struct {
	AllowOrigin      *CorsAllowOriginModelV2 `tfsdk:"allow_origin"`
	AllowHeaders     *CorsHeaderListModelV2  `tfsdk:"allow_headers"`
	ExposeHeaders    *CorsHeaderListModelV2  `tfsdk:"expose_headers"`
	AllowMethods     *CorsMethodListModelV2  `tfsdk:"allow_methods"`
	AllowCredentials types.Bool              `tfsdk:"allow_credentials"`
	MaxAge           *CorsMaxAgeModelV2      `tfsdk:"max_age"`
}

type HostHeaderModelV2 struct {
	HeaderValue   types.String `tfsdk:"header_value"`
	UseOriginHost types.Bool   `tfsdk:"use_origin_host"`
}

type CookieModelV2 struct {
	Cookie types.String `tfsdk:"cookie"`
}

type ParamModelV2 struct {
	Param types.String `tfsdk:"param"`
}

type QueryStringsModelV2 struct {
	ParamsList []ParamModelV2 `tfsdk:"params"`
	ListType   types.String   `tfsdk:"type"`
}

type CacheKeyModelV2 struct {
	Headers      []HeaderModelV2      `tfsdk:"headers"`
	Cookies      []CookieModelV2      `tfsdk:"cookies"`
	QueryStrings *QueryStringsModelV2 `tfsdk:"query_strings"`
	Country      types.Bool           `tfsdk:"country"`
	DeviceType   types.Bool           `tfsdk:"device_type"`
}

type QueryStringsDataV2 struct {
	ParamsList []string `json:"params"`
	ListType   string   `json:"type"`
}

type CacheKeyDataV2 struct {
	Headers      []string           `json:"headers"`
	Cookies      []string           `json:"cookies"`
	QueryStrings QueryStringsDataV2 `json:"query_strings"`
}

type StatusCodeCacheModelV2 struct {
	StatusCode    types.String `tfsdk:"status_code"`
	CacheBehavior types.String `tfsdk:"cache_behavior"`
	CacheTTL      types.Int64  `tfsdk:"cache_ttl"`
}

type StatusCodeBrowserCacheModelV2 struct {
	StatusCode types.String `tfsdk:"status_code"`
	CacheTtl   types.Int64  `tfsdk:"cache_ttl"`
}

type GeneratePreflightResponseModelV2 struct {
	AllowedMethods *[]MethodModelV2 `tfsdk:"allowed_methods"`
	AllowedHeaders []types.String   `tfsdk:"allowed_headers"`
	MaxAge         types.Int64      `tfsdk:"max_age"`
}

type StreamLogsModelV2 struct {
	UnifiedLogDestination  types.String `tfsdk:"log_destination"`
	UnifiedLogSamplingRate types.Int64  `tfsdk:"log_sampling_rate"`
}

type StatusCodeCustomResponseModelV2 struct {
	StatusCode  types.String `tfsdk:"status_code"`
	ResponseURL types.String `tfsdk:"response_url"`
}

type IPModelV2 struct {
	IP types.String `tfsdk:"ip"`
}

type ProviderSpecificModel struct {
	Provider types.String         `tfsdk:"provider"`
	Code     jsontypes.Normalized `tfsdk:"code"`
}

// HeaderActionModelV2 matches the Python HttpHeaderAction dict-keyed shape.
// "name" is the header name (the dict key in Python → list element in JSON).
// action: "set" (override=true), "add" (override=false), "delete" (delete=true).
type HeaderActionModelV2 struct {
	Name   types.String   `tfsdk:"name"`
	Values []types.String `tfsdk:"values"`
	Action types.String   `tfsdk:"action"`
}

type RedirectModelV2 struct {
	Destination types.String `tfsdk:"destination"`
	Source      types.String `tfsdk:"source"`
}

type UrlRewriteModelV2 struct {
	Source      types.String `tfsdk:"source"`
	Destination types.String `tfsdk:"destination"`
}

type DateTimeWindowModelV2 struct {
	StartDate types.Int64 `tfsdk:"start_date"`
	EndDate   types.Int64 `tfsdk:"end_date"`
}

type TimePeriodicModelV2 struct {
	StartDate         types.Int64  `tfsdk:"start_date"`
	Duration          types.Int64  `tfsdk:"duration"`
	DurationUnits     types.String `tfsdk:"duration_units"`
	RepeatPeriod      types.Int64  `tfsdk:"repeat_period"`
	RepeatPeriodUnits types.String `tfsdk:"repeat_period_units"`
}

type TimeConstraintModelV2 struct {
	DateTimeWindow *DateTimeWindowModelV2 `tfsdk:"date_time_window"`
	TimePeriodic   *TimePeriodicModelV2   `tfsdk:"time_periodic"`
}

// ========== Behavior Models ==========

// Terraform models
type BehaviorsModel struct {
	Default  *DefaultBehaviorModel
	AllMatch types.List
}

type DefaultBehaviorModel struct {
	Actions *BehaviorActionV2ResourceModel `tfsdk:"actions" json:"actions,omitempty"`
}

type BehaviorModel struct {
	Name        types.String                      `tfsdk:"name" json:"name"`
	PathPattern types.String                      `tfsdk:"path_pattern"`
	Condition   *BehaviorConditionExpressionModel `tfsdk:"condition"`
	Actions     *BehaviorActionV2ResourceModel    `tfsdk:"actions" json:"actions,omitempty"`
}

// V2 action model - matches the v2 schema exactly
type BehaviorActionV2ResourceModel struct {
	CacheTTL                  types.Int64                       `tfsdk:"cache_ttl"`
	CacheBehavior             types.String                      `tfsdk:"cache_behavior"`
	BrowserCacheTtl           types.Int64                       `tfsdk:"browser_cache_ttl"`
	ViewerProtocol            types.String                      `tfsdk:"viewer_protocol"`
	Redirect                  *RedirectModelV2                  `tfsdk:"redirect"`
	OriginCacheControl        types.Bool                        `tfsdk:"origin_cache_control"`
	CacheKey                  *CacheKeyModelV2                  `tfsdk:"cache_key"`
	HostHeader                *HostHeaderModelV2                `tfsdk:"host_header"`
	Cors                      *CorsConfigModelV2                `tfsdk:"cors"`
	FollowRedirects           types.Bool                        `tfsdk:"follow_redirects"`
	StatusCodeBrowserCache    []StatusCodeBrowserCacheModelV2   `tfsdk:"status_code_browser_cache"`
	GeneratePreflightResponse *GeneratePreflightResponseModelV2 `tfsdk:"generate_preflight_response"`
	StaleTtl                  types.Int64                       `tfsdk:"stale_ttl"`
	StreamLogs                *StreamLogsModelV2                `tfsdk:"stream_logs"`
	AllowedMethods            *[]MethodModelV2                  `tfsdk:"allowed_methods"`
	Compression               types.Bool                        `tfsdk:"compression"`
	LargeFilesOptimization    types.Bool                        `tfsdk:"large_files_optimization"`
	StatusCodeCustomResponse  []StatusCodeCustomResponseModelV2 `tfsdk:"generate_response"`
	CachedMethods             *[]MethodModelV2                  `tfsdk:"cached_methods"`
	UrlSigning                types.Bool                        `tfsdk:"url_signing"`
	TrueClientIP              types.Bool                        `tfsdk:"true_client_ip"`
	DenyAccess                types.Bool                        `tfsdk:"deny_access"`
	AllowAccessOnlyFromIP     *[]IPModelV2                      `tfsdk:"allow_access_only_from_ip"`
	DenyAccessByIP            *[]IPModelV2                      `tfsdk:"deny_access_by_ip"`
	DenyAccessByTime          *[]TimeConstraintModelV2          `tfsdk:"deny_access_by_time"`
	UrlRewrites               *[]UrlRewriteModelV2              `tfsdk:"url_rewrites"`
	RequestHeaders            *[]HeaderActionModelV2            `tfsdk:"request_headers"`
	ResponseHeaders           *[]HeaderActionModelV2            `tfsdk:"response_headers"`
	OriginResponseHeaders     *[]HeaderActionModelV2            `tfsdk:"origin_response_headers"`
	StatusCodeCache           []StatusCodeCacheModelV2          `tfsdk:"status_codes_ttl"`
	ProviderSpecific          []ProviderSpecificModel           `tfsdk:"provider_specific"`
}

var behaviorPathAllowedChars = regexp.MustCompile(`^/[A-Za-z0-9_\-\.\*\$/~"'\@:\+]*$`)

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
		return "3xx"
	case 4:
		return "4xx"
	case 5:
		return "5xx"
	}
	return fmt.Sprintf("%d", statusCode)
}

func nullValueForType(t attr.Type) attr.Value {
	switch t := t.(type) {
	case types.ObjectType:
		return types.ObjectNull(t.AttrTypes)
	case types.ListType:
		return types.ListNull(t.ElemType)
	case types.SetType:
		return types.SetNull(t.ElemType)
	case basetypes.StringType:
		return types.StringNull()
	case basetypes.Int64Type:
		return types.Int64Null()
	case basetypes.BoolType:
		return types.BoolNull()
	default:
		panic(fmt.Sprintf("unsupported attr.Type: %T", t))
	}
}

func BehaviorAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"name": schema.StringAttribute{
			MarkdownDescription: "Behavior name",
			Required:            true,
		},
		"path_pattern": schema.StringAttribute{
			MarkdownDescription: "Simple path pattern shorthand (e.g. '/api/*').\n" +
				"  - Mutually exclusive with `condition`.\n" +
				"  - Internally expanded to a single `http.request.path` / `match` condition.",
			Optional: true,
			Validators: []validator.String{
				stringvalidator.LengthAtLeast(1),
			},
		},
		"condition": schema.SingleNestedAttribute{
			MarkdownDescription: "Full match condition (OR-of-ANDs expression).\n" +
				"  - Mutually exclusive with `path_pattern`. \n  -",
			Optional:   true,
			Attributes: behaviorConditionExpressionAttributes(),
		},
		"actions": schema.SingleNestedAttribute{
			MarkdownDescription: "Set of actions to apply for matching requests. Each element in the set defines a single action.",
			Required:            true,
			Attributes:          BehaviorActionAttributes(),
		},
	}
}

func DefaultBehaviorAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"actions": schema.SingleNestedAttribute{
			MarkdownDescription: "Set of actions to apply for matching requests. Each element in the set defines a single action.",
			Optional:            true,
			Computed:            true,
			Default:             objectdefault.StaticValue(GetDefaultActionsValue()),
			Attributes:          DefaultBehaviorActionsAttributes(),
		},
	}
}

func DefaultBehaviorAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"actions": types.ObjectType{AttrTypes: BehaviorActionAttrTypes()},
	}
}

// defaultBehaviorObjectValue is the static types.Object for the default behavior
// block — computed once at startup.
var defaultBehaviorObjectValue = types.ObjectValueMust(
	map[string]attr.Type{
		"actions": types.ObjectType{AttrTypes: BehaviorActionAttrTypes()},
	},
	map[string]attr.Value{
		"actions": GetDefaultActionsValue(),
	},
)

// BehaviorsBlockModel is the TF-facing struct for the top-level behaviors block,
// combining the default behavior and the list of specific (custom) behaviors.
type BehaviorsBlockModel struct {
	Default types.Object `tfsdk:"default"`
	Custom  types.List   `tfsdk:"custom"`
}

// BehaviorsBlockAttrTypes returns the attr.Type map for BehaviorsBlockModel,
// used when constructing/inspecting types.Object values for the behaviors block.
func BehaviorsBlockAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"default": types.ObjectType{AttrTypes: DefaultBehaviorAttrTypes()},
		"custom":  types.ListType{ElemType: types.ObjectType{AttrTypes: BehaviorAttrTypes()}},
	}
}

func BehaviorAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"name":         types.StringType,
		"path_pattern": types.StringType,
		"condition":    conditionExpressionAttrType(),
		"actions":      types.ObjectType{AttrTypes: BehaviorActionAttrTypes()},
	}
}

func BehaviorActionAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		// Simple Scalars
		"cache_ttl":                types.Int64Type,
		"cache_behavior":           types.StringType,
		"browser_cache_ttl":        types.Int64Type,
		"viewer_protocol":          types.StringType,
		"origin_cache_control":     types.BoolType,
		"follow_redirects":         types.BoolType,
		"stale_ttl":                types.Int64Type,
		"compression":              types.BoolType,
		"large_files_optimization": types.BoolType,
		"url_signing":              types.BoolType,
		"true_client_ip":           types.BoolType,
		"deny_access":              types.BoolType,

		// 3. Simple Sets (ListType in Framework for SetNestedAttribute)
		"cached_methods": types.SetType{ElemType: types.ObjectType{AttrTypes: map[string]attr.Type{
			"method": types.StringType,
		}}},
		"allowed_methods": types.SetType{ElemType: types.ObjectType{AttrTypes: map[string]attr.Type{
			"method": types.StringType,
		}}},
		"allow_access_only_from_ip": types.SetType{ElemType: types.ObjectType{AttrTypes: map[string]attr.Type{
			"ip": types.StringType,
		}}},
		"deny_access_by_ip": types.SetType{ElemType: types.ObjectType{AttrTypes: map[string]attr.Type{
			"ip": types.StringType,
		}}},
		"deny_access_by_time": types.ListType{ElemType: types.ObjectType{AttrTypes: map[string]attr.Type{
			"date_time_window": types.ObjectType{AttrTypes: map[string]attr.Type{
				"start_date": types.Int64Type,
				"end_date":   types.Int64Type,
			}},
			"time_periodic": types.ObjectType{AttrTypes: map[string]attr.Type{
				"start_date":          types.Int64Type,
				"duration":            types.Int64Type,
				"duration_units":      types.StringType,
				"repeat_period":       types.Int64Type,
				"repeat_period_units": types.StringType,
			}},
		}}},
		"url_rewrites": types.ListType{ElemType: types.ObjectType{AttrTypes: map[string]attr.Type{
			"source":      types.StringType,
			"destination": types.StringType,
		}}},
		// Header modification actions — { name, values, action }
		"request_headers": types.SetType{ElemType: types.ObjectType{AttrTypes: map[string]attr.Type{
			"name":   types.StringType,
			"values": types.ListType{ElemType: types.StringType},
			"action": types.StringType,
		}}},
		"response_headers": types.SetType{ElemType: types.ObjectType{AttrTypes: map[string]attr.Type{
			"name":   types.StringType,
			"values": types.ListType{ElemType: types.StringType},
			"action": types.StringType,
		}}},
		"origin_response_headers": types.SetType{ElemType: types.ObjectType{AttrTypes: map[string]attr.Type{
			"name":   types.StringType,
			"values": types.ListType{ElemType: types.StringType},
			"action": types.StringType,
		}}},

		// 4. Nested Objects (SingleNestedAttribute)
		"cache_key": types.ObjectType{AttrTypes: map[string]attr.Type{
			"headers": types.SetType{ElemType: types.ObjectType{AttrTypes: map[string]attr.Type{"header": types.StringType}}},
			"cookies": types.SetType{ElemType: types.ObjectType{AttrTypes: map[string]attr.Type{"cookie": types.StringType}}},
			"query_strings": types.ObjectType{AttrTypes: map[string]attr.Type{
				"type":   types.StringType,
				"params": types.SetType{ElemType: types.ObjectType{AttrTypes: map[string]attr.Type{"param": types.StringType}}},
			}},
			"country":     types.BoolType,
			"device_type": types.BoolType,
		}},
		"host_header": types.ObjectType{AttrTypes: map[string]attr.Type{
			"header_value":    types.StringType,
			"use_origin_host": types.BoolType,
		}},
		"cors": types.ObjectType{AttrTypes: map[string]attr.Type{
			"allow_origin": types.ObjectType{AttrTypes: map[string]attr.Type{
				"mode":     types.StringType,
				"origins":  types.ListType{ElemType: types.StringType},
				"override": types.BoolType,
			}},
			"allow_headers": types.ObjectType{AttrTypes: map[string]attr.Type{
				"mode":     types.StringType,
				"values":   types.ListType{ElemType: types.StringType},
				"override": types.BoolType,
			}},
			"expose_headers": types.ObjectType{AttrTypes: map[string]attr.Type{
				"mode":     types.StringType,
				"values":   types.ListType{ElemType: types.StringType},
				"override": types.BoolType,
			}},
			"allow_methods": types.ObjectType{AttrTypes: map[string]attr.Type{
				"mode":     types.StringType,
				"values":   types.ListType{ElemType: types.StringType},
				"override": types.BoolType,
			}},
			"allow_credentials": types.BoolType,
			"max_age": types.ObjectType{AttrTypes: map[string]attr.Type{
				"value":    types.Int64Type,
				"override": types.BoolType,
			}},
		}},
		"generate_preflight_response": types.ObjectType{AttrTypes: map[string]attr.Type{
			"max_age": types.Int64Type,
			"allowed_methods": types.SetType{ElemType: types.ObjectType{AttrTypes: map[string]attr.Type{
				"method": types.StringType,
			}}},
			"allowed_headers": types.SetType{ElemType: types.StringType},
		}},
		"status_code_browser_cache": types.ListType{ElemType: types.ObjectType{AttrTypes: map[string]attr.Type{
			"status_code": types.StringType,
			"cache_ttl":   types.Int64Type,
		}}},
		"stream_logs": types.ObjectType{AttrTypes: map[string]attr.Type{
			"log_destination":   types.StringType,
			"log_sampling_rate": types.Int64Type,
		}},
		"generate_response": types.ListType{ElemType: types.ObjectType{AttrTypes: map[string]attr.Type{
			"status_code":  types.StringType,
			"response_url": types.StringType,
		}}},
		"redirect": types.ObjectType{AttrTypes: map[string]attr.Type{
			"destination": types.StringType,
			"source":      types.StringType,
		}},
		"status_codes_ttl": types.ListType{ElemType: types.ObjectType{AttrTypes: map[string]attr.Type{
			"status_code":    types.StringType,
			"cache_behavior": types.StringType,
			"cache_ttl":      types.Int64Type,
		}}},
		"provider_specific": types.ListType{ElemType: types.ObjectType{AttrTypes: map[string]attr.Type{
			"provider": types.StringType,
			"code":     jsontypes.NormalizedType{},
		}}},
	}
}

// headerActionAttributes returns the shared schema for request_headers,
// response_headers, and origin_response_headers list entries.
// action: "set" (override, replace value), "add" (append value), "delete" (remove header).
func headerActionAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"name": schema.StringAttribute{
			MarkdownDescription: "Header name",
			Required:            true,
		},
		"values": schema.ListAttribute{
			MarkdownDescription: "One or more header values to set.\n" +
				"  - Not required when `action = \"delete\"`.",
			Optional:    true,
			ElementType: types.StringType,
		},
		"action": schema.StringAttribute{
			MarkdownDescription: "Header action: `set` replaces the header value, `add` appends it, `delete` removes the header.",
			Required:            true,
			Validators: []validator.String{
				stringvalidator.OneOf("set", "add", "delete"),
			},
		},
	}
}

func BehaviorActionAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"cache_ttl": schema.Int64Attribute{
			MarkdownDescription: "Set the value of the edge cache TTL in seconds",
			Optional:            true,
			Validators: []validator.Int64{
				int64validator.AtLeast(0),
			},
		},
		"cache_behavior": schema.StringAttribute{
			MarkdownDescription: "Controls whether content should be cached by the CDN, possible values: `" + strings.Join(cacheBehaviorValues, "`, `") + "`",
			Optional:            true,
			Validators: []validator.String{
				stringvalidator.OneOf(cacheBehaviorValues...),
			},
		},
		"cached_methods": schema.SetNestedAttribute{
			MarkdownDescription: "Controls the list of HTTP methods whose responses the CDN will cache",
			Optional:            true,
			NestedObject: schema.NestedAttributeObject{
				Attributes: map[string]schema.Attribute{
					"method": schema.StringAttribute{
						MarkdownDescription: "Method to be cached. Valid values: `" + strings.Join(httpMethodValues, "`, `") + "`",
						Optional:            true,
						Validators: []validator.String{
							stringvalidator.OneOf(httpMethodValues...),
						},
					},
				},
			},
		},
		"allowed_methods": schema.SetNestedAttribute{
			MarkdownDescription: "Set of HTTP methods the CDN will accept (top-level allowed methods action)",
			Optional:            true,
			NestedObject: schema.NestedAttributeObject{
				Attributes: map[string]schema.Attribute{
					"method": schema.StringAttribute{
						MarkdownDescription: "HTTP method. Valid values: `" + strings.Join(httpMethodValues, "`, `") + "`",
						Optional:            true,
						Validators: []validator.String{
							stringvalidator.OneOf(httpMethodValues...),
						},
					},
				},
			},
		},
		"browser_cache_ttl": schema.Int64Attribute{
			MarkdownDescription: "Controls how long the browser is allowed to cache the content.\n" +
				"  - The CDN will add a Cache-Control header to the response sent to end-users with the configured cache TTL.\n" +
				"  - Set the value of the browser cache TTL in seconds.",
			Optional: true,
			Validators: []validator.Int64{
				int64validator.AtLeast(0),
			},
		},
		"viewer_protocol": schema.StringAttribute{
			MarkdownDescription: "Controls how the CDN should respond to HTTP requests.\n" +
				"  - The CDN can either accept both HTTP and HTTPS, redirect HTTP to HTTPS, or restrict access to HTTPS only.\n" +
				"  - If HTTPS-only is selected, the CDN will respond with a 403 status code to HTTP requests.\n" +
				"  - Allowed viewer protocol - can be one of the following: `" + strings.Join(viewerProtocolValues, "`, `") + "`.",
			Optional: true,
			Validators: []validator.String{
				stringvalidator.OneOf(viewerProtocolValues...),
			},
		},
		"redirect": schema.SingleNestedAttribute{
			MarkdownDescription: "This action enables sending a redirect response with a specified URL.",
			Optional:            true,
			Attributes: map[string]schema.Attribute{
				"destination": schema.StringAttribute{
					MarkdownDescription: "Redirect destination URL",
					Required:            true,
				},
				"source": schema.StringAttribute{
					MarkdownDescription: "Redirect source pattern (optional)",
					Optional:            true,
				},
			},
		},
		"origin_cache_control": schema.BoolAttribute{
			MarkdownDescription: "Controls whether the CDN should honor the Cache-Control header sent by the origin.\n" +
				"  - By default, origin Cache-Control headers are not honored by the CDN.",
			Optional: true,
		},
		"cache_key": schema.SingleNestedAttribute{
			MarkdownDescription: "Custom cache key configuration",
			Optional:            true,
			Attributes: map[string]schema.Attribute{
				"headers": schema.SetNestedAttribute{
					MarkdownDescription: "Set of headers to include in the cache key",
					Optional:            true,
					Computed:            true,
					Default:             setdefault.StaticValue(defaultCacheKeyHeaderValue),
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
					Optional:            true,
					Computed:            true,
					Default:             setdefault.StaticValue(defaultCacheKeyCookieValue),
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
					Optional:            true,
					Computed:            true,
					Default:             objectdefault.StaticValue(defaultCacheKeyQueryStringValue),
					Attributes: map[string]schema.Attribute{
						"params": schema.SetNestedAttribute{
							MarkdownDescription: "Set of query string params to include or exclude in the cache key.\n" +
								"  - Required when `type` is `include` or `exclude`.\n" +
								"  - Must be omitted when `type` is `all` or `none`.",
							Optional: true,
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
							MarkdownDescription: "Controls which query string params are part of the cache key.\n" +
								"  - `include` / `exclude`: use the params listed in `params`.\n" +
								"  - `all`: include every query string param (no `params` allowed).\n" +
								"  - `none`: ignore all query string params (no `params` allowed).",
							Required: true,
							Validators: []validator.String{
								stringvalidator.OneOf([]string{"include", "exclude", "all", "none"}...),
								queryStringType(),
							},
						},
					},
				},
				"country": schema.BoolAttribute{
					MarkdownDescription: "Include the client country in the cache key",
					Optional:            true,
					Computed:            true,
					Default:             booldefault.StaticBool(defaultCacheKeyCountry),
				},
				"device_type": schema.BoolAttribute{
					MarkdownDescription: "Include the client device type (mobile/desktop) in the cache key",
					Optional:            true,
					Computed:            true,
					Default:             booldefault.StaticBool(defaultCacheKeyDeviceType),
				},
			},
		},
		"host_header": schema.SingleNestedAttribute{
			MarkdownDescription: "Override the Host header sent to the origin.\n" +
				"  - Set `header_value` to a specific hostname, or set `use_origin_host = true` to use the origin's own hostname.\n" +
				"  - The two fields are mutually exclusive. \n  -",
			Optional: true,
			Attributes: map[string]schema.Attribute{
				"header_value": schema.StringAttribute{
					MarkdownDescription: "Value of the host header",
					Optional:            true,
					Validators: []validator.String{
						stringvalidator.ConflictsWith(path.MatchRelative().AtParent().AtName("use_origin_host")),
					},
				},
				"use_origin_host": schema.BoolAttribute{
					MarkdownDescription: "Use the origin domain name as the Host header for the origin. Must be set to `true`.",
					Optional:            true,
					Validators: []validator.Bool{
						boolvalidator.Equals(true),
						boolvalidator.ConflictsWith(path.MatchRelative().AtParent().AtName("header_value")),
					},
				},
			},
		},
		"cors": schema.SingleNestedAttribute{
			MarkdownDescription: "CORS configuration for the behavior",
			Optional:            true,
			Attributes: map[string]schema.Attribute{
				"allow_origin": schema.SingleNestedAttribute{
					MarkdownDescription: "Access-Control-Allow-Origin configuration",
					Optional:            true,
					Attributes: map[string]schema.Attribute{
						"mode": schema.StringAttribute{
							MarkdownDescription: "Origin mode. Valid values: `all`, `specific`, `from_request`",
							Required:            true,
							Validators: []validator.String{
								stringvalidator.OneOf("all", "specific", "from_request"),
								corsSpecificRequiresValues("origins"),
							},
						},
						"origins": schema.ListAttribute{
							MarkdownDescription: "List of allowed origins\n" +
								"  - Required when mode is `specific`, must be empty otherwise.",
							Optional:    true,
							ElementType: types.StringType,
							Validators: []validator.List{
								corsValuesRequireSpecificMode(),
							},
						},
						"override": schema.BoolAttribute{
							MarkdownDescription: "Override the origin value",
							Optional:            true,
						},
					},
				},
				"allow_headers": schema.SingleNestedAttribute{
					MarkdownDescription: "Access-Control-Allow-Headers configuration",
					Optional:            true,
					Attributes: map[string]schema.Attribute{
						"mode": schema.StringAttribute{
							MarkdownDescription: "Header mode. Valid values: `all`, `specific`",
							Required:            true,
							Validators: []validator.String{
								stringvalidator.OneOf("all", "specific"),
								corsSpecificRequiresValues("values"),
							},
						},
						"values": schema.ListAttribute{
							MarkdownDescription: "List of allowed headers\n" +
								"  - Required when mode is `specific`, must be empty otherwise.",
							Optional:    true,
							ElementType: types.StringType,
							Validators: []validator.List{
								corsValuesRequireSpecificMode(),
							},
						},
						"override": schema.BoolAttribute{
							MarkdownDescription: "Override the headers value",
							Optional:            true,
						},
					},
				},
				"expose_headers": schema.SingleNestedAttribute{
					MarkdownDescription: "Access-Control-Expose-Headers configuration",
					Optional:            true,
					Attributes: map[string]schema.Attribute{
						"mode": schema.StringAttribute{
							MarkdownDescription: "Header mode. Valid values: `all`, `specific`",
							Required:            true,
							Validators: []validator.String{
								stringvalidator.OneOf("all", "specific"),
								corsSpecificRequiresValues("values"),
							},
						},
						"values": schema.ListAttribute{
							MarkdownDescription: "List of exposed headers \n" +
								"  - Required when mode is `specific`, must be empty otherwise.",
							Optional:    true,
							ElementType: types.StringType,
							Validators: []validator.List{
								corsValuesRequireSpecificMode(),
							},
						},
						"override": schema.BoolAttribute{
							MarkdownDescription: "Override the expose headers value",
							Optional:            true,
						},
					},
				},
				"allow_methods": schema.SingleNestedAttribute{
					MarkdownDescription: "Access-Control-Allow-Methods configuration",
					Optional:            true,
					Attributes: map[string]schema.Attribute{
						"mode": schema.StringAttribute{
							MarkdownDescription: "Method mode. Valid values: `all`, `specific`",
							Required:            true,
							Validators: []validator.String{
								stringvalidator.OneOf("all", "specific"),
								corsSpecificRequiresValues("values"),
							},
						},
						"values": schema.ListAttribute{
							MarkdownDescription: "List of allowed methods\n" +
								"  - Required when mode is `specific`, must be empty otherwise.\n" +
								"  - Valid values: `" + strings.Join(httpMethodValues, "`, `") + "`",
							Optional:    true,
							ElementType: types.StringType,
							Validators: []validator.List{
								corsValuesRequireSpecificMode(),
							},
						},
						"override": schema.BoolAttribute{
							MarkdownDescription: "Override the methods value",
							Optional:            true,
						},
					},
				},
				"allow_credentials": schema.BoolAttribute{
					MarkdownDescription: "Access-Control-Allow-Credentials value",
					Optional:            true,
					Validators: []validator.Bool{
						boolvalidator.Equals(true),
					},
				},
				"max_age": schema.SingleNestedAttribute{
					MarkdownDescription: "Access-Control-Max-Age configuration",
					Optional:            true,
					Attributes: map[string]schema.Attribute{
						"value": schema.Int64Attribute{
							MarkdownDescription: "Max-Age value in seconds",
							Optional:            true,
							Validators: []validator.Int64{
								int64validator.AtLeast(0),
							},
						},
						"override": schema.BoolAttribute{
							MarkdownDescription: "Override the max_age value",
							Optional:            true,
						},
					},
				},
			},
		},
		"follow_redirects": schema.BoolAttribute{
			MarkdownDescription: "This action enables the CDN to follow a redirect response from the origin.\n" +
				"  - If the origin responds with a redirect, the CDN will follow it and return the response to the end user.\n" +
				"  - **Limitations**:\n" +
				"    - The host in the `Location` header must be defined as an origin in the service.\n" +
				"    - The scheme in the `Location` header must be HTTPS.",
			Optional: true,
		},
		"generate_preflight_response": schema.SingleNestedAttribute{
			MarkdownDescription: "Controls how the CDN should respond to a preflight request.\n" +
				"  - The CDN can respond to a preflight request without forwarding it to the origin.\n" +
				"  - You can configure the headers to include in the preflight response. \n  -",
			Optional: true,
			Attributes: map[string]schema.Attribute{
				"allowed_methods": schema.SetNestedAttribute{
					MarkdownDescription: "Value for the `Access-Control-Allow-Methods` header.\n" +
						"  - Valid values: `" + strings.Join(httpMethodValues, "`, `") + "` \n  -",
					Required: true,
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
				"allowed_headers": schema.SetAttribute{
					MarkdownDescription: "Value for the `Access-Control-Allow-Headers` header.\n" +
						"  - List of request headers the browser is allowed to send in the actual request.",
					Optional:    true,
					ElementType: types.StringType,
				},
				"max_age": schema.Int64Attribute{
					MarkdownDescription: "Value for the `Access-Control-Max-Age` header in seconds.\n" +
						"  - Controls how long the browser may cache the preflight response.",
					Required: true,
					Validators: []validator.Int64{
						int64validator.AtLeast(0),
					},
				},
			},
		},
		"status_code_browser_cache": schema.ListNestedAttribute{
			MarkdownDescription: "Define browser cache configuration for status code(s)",
			Optional:            true,
			NestedObject: schema.NestedAttributeObject{
				Attributes: map[string]schema.Attribute{
					"status_code": schema.StringAttribute{
						MarkdownDescription: "Status code to apply the configuratoin for (1xx,2xx,.. can be used for ranges)",
						Required:            true,
					},
					"cache_ttl": schema.Int64Attribute{
						MarkdownDescription: "Value of browser cache TTL - in seconds",
						Required:            true,
						Validators: []validator.Int64{
							int64validator.AtLeast(0),
						},
					},
				},
			},
		},
		"stale_ttl": schema.Int64Attribute{
			MarkdownDescription: "This action sets the time duration the CDN can serve expired content from the cache\n" +
				"  - If the content is expired and there is a problem fetching it from the origin, the CDN can serve stale content for the specified duration.\n " +
				"  - Set the cache TTL value in seconds.",
			Optional: true,
			Validators: []validator.Int64{
				int64validator.AtLeast(0),
			},
		},
		"stream_logs": schema.SingleNestedAttribute{
			MarkdownDescription: "Stream CDN access logs to a configured logging destination (e.g., an S3 bucket).\n" +
				"  - All logs are delivered in a unified format, regardless of which CDN provider generated them.\n" +
				"  - See IO River Documentation for details on how to configure a destination.",
			Optional: true,
			Attributes: map[string]schema.Attribute{
				"log_destination": schema.StringAttribute{
					MarkdownDescription: "Name of the log destination to stream logs to.\n" +
						"  - The destination must be configured in the service's `log_destinations` block.",
					Required: true,
				},
				"log_sampling_rate": schema.Int64Attribute{
					MarkdownDescription: "Percentage of requests whose logs should be streamed (1–100).\n" +
						"  - Use `100` to stream all logs, or a lower value to sample.",
					Required: true,
					Validators: []validator.Int64{
						int64validator.AtLeast(1),
						int64validator.AtMost(100),
					},
				},
			},
		},
		"generate_response": schema.ListNestedAttribute{
			MarkdownDescription: "Return a custom response page for specific status code(s)",
			Optional:            true,
			NestedObject: schema.NestedAttributeObject{
				Attributes: map[string]schema.Attribute{
					"status_code": schema.StringAttribute{
						MarkdownDescription: "HTTP status code or range (e.g. `404`, `4xx`, `5xx`)",
						Required:            true,
					},
					"response_url": schema.StringAttribute{
						MarkdownDescription: "URL of the custom response page",
						Required:            true,
					},
				},
			},
		},
		"url_signing": schema.BoolAttribute{
			MarkdownDescription: "Controls whether the CDN should verify the URL signature before allowing access to the content.",
			Optional:            true,
		},
		"compression": schema.BoolAttribute{
			MarkdownDescription: "Enable compression of responses.",
			Optional:            true,
		},
		"large_files_optimization": schema.BoolAttribute{
			MarkdownDescription: "Enable large files optimization.",
			Optional:            true,
		},
		"true_client_ip": schema.BoolAttribute{
			MarkdownDescription: "Controls whether the CDN should add a `True-Client-IP` header when forwarding the request to the origin.\n" +
				"  - When enabled, the header will contain the real IP address of the end user.",
			Optional: true,
		},
		"deny_access": schema.BoolAttribute{
			MarkdownDescription: "Controls whether the CDN should deny access to requests that meet the behavior condition.\n" +
				"  - When enabled, the CDN will return a 403 Forbidden response.",
			Optional: true,
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
		"deny_access_by_ip": schema.SetNestedAttribute{
			MarkdownDescription: "Controls whether the CDN should deny access to a specific set of IP addresses.",
			Optional:            true,
			NestedObject: schema.NestedAttributeObject{
				Attributes: map[string]schema.Attribute{
					"ip": schema.StringAttribute{
						MarkdownDescription: "IP address or CIDR block to deny",
						Required:            true,
					},
				},
			},
		},
		"deny_access_by_time": schema.ListNestedAttribute{
			MarkdownDescription: "Controls whether the CDN should deny access during a specific time period.\n" +
				"  - The time period can be either a fixed interval (`date_time_window`) or a recurring one (`time_periodic`). \n  -",
			Optional: true,
			NestedObject: schema.NestedAttributeObject{
				Attributes: map[string]schema.Attribute{
					"date_time_window": schema.SingleNestedAttribute{
						MarkdownDescription: "A fixed UTC time interval during which access is denied.",
						Optional:            true,
						Attributes: map[string]schema.Attribute{
							"start_date": schema.Int64Attribute{
								MarkdownDescription: "Start of the interval as a Unix timestamp (UTC).",
								Required:            true,
							},
							"end_date": schema.Int64Attribute{
								MarkdownDescription: "End of the interval as a Unix timestamp (UTC).",
								Required:            true,
							},
						},
					},
					"time_periodic": schema.SingleNestedAttribute{
						MarkdownDescription: "A recurring time interval during which access is denied.",
						Optional:            true,
						Attributes: map[string]schema.Attribute{

							"start_date": schema.Int64Attribute{
								MarkdownDescription: "Start of the first interval as a Unix timestamp (UTC).",
								Required:            true,
							},
							"duration": schema.Int64Attribute{
								MarkdownDescription: "Duration of the deny access window (numeric value, paired with `duration_units`).",
								Required:            true,
							},
							"duration_units": schema.StringAttribute{
								MarkdownDescription: "Units for `duration`. Valid values: `s` (seconds), `m` (minutes), `h` (hours), `d` (days).",
								Required:            true,
								Validators: []validator.String{
									stringvalidator.OneOf("s", "m", "h", "d"),
								},
							},
							"repeat_period": schema.Int64Attribute{
								MarkdownDescription: "Interval at which the deny access rule repeats (numeric value, paired with `repeat_period_units`).",
								Required:            true,
							},
							"repeat_period_units": schema.StringAttribute{
								MarkdownDescription: "Units for `repeat_period`. Valid values: `s` (seconds), `m` (minutes), `h` (hours), `d` (days).",
								Required:            true,
								Validators: []validator.String{
									stringvalidator.OneOf("s", "m", "h", "d"),
								},
							},
						},
					},
				},
			},
		},
		"url_rewrites": schema.ListNestedAttribute{
			MarkdownDescription: "This action enables rewriting the request path.\n" +
				"  - The destination path can be a static string or can be derived from a regular expression.\n" +
				"  - This action takes place before the content is retrieved from the cache.\n" +
				"  - Example: The following removes the static prefix from the request path: " +
				"    Source `/static/(.*)`, Destination `/$1` \n  -",
			Optional: true,
			NestedObject: schema.NestedAttributeObject{
				Attributes: map[string]schema.Attribute{
					"source": schema.StringAttribute{
						MarkdownDescription: "The source request path, which can be either a static string or a regex.",
						Required:            true,
					},
					"destination": schema.StringAttribute{
						MarkdownDescription: "The new path, which can be either a static string or a regex.",
						Required:            true,
					},
				},
			},
		},
		"request_headers": schema.SetNestedAttribute{
			MarkdownDescription: "Add, modify, or remove headers on the request forwarded to the origin.\n" +
				"  - Set `values` to add/override a header, or set `delete = true` to remove it. \n  -",
			Optional: true,
			NestedObject: schema.NestedAttributeObject{
				Attributes: headerActionAttributes(),
			},
		},
		"response_headers": schema.SetNestedAttribute{
			MarkdownDescription: "Add, modify, or remove headers on the response sent to the client.\n" +
				"  - Set `values` to add/override a header, or set `delete = true` to remove it. \n  -",
			Optional: true,
			NestedObject: schema.NestedAttributeObject{
				Attributes: headerActionAttributes(),
			},
		},
		"origin_response_headers": schema.SetNestedAttribute{
			MarkdownDescription: "Add, modify, or remove headers on the response received from the origin (before caching).\n" +
				"  - Set `values` to add/override a header, or set `delete = true` to remove it. \n  -",
			Optional: true,
			NestedObject: schema.NestedAttributeObject{
				Attributes: headerActionAttributes(),
			},
		},
		"status_codes_ttl": schema.ListNestedAttribute{
			MarkdownDescription: "Cache TTL configuration per status code",
			Optional:            true,
			NestedObject: schema.NestedAttributeObject{
				Attributes: map[string]schema.Attribute{
					"status_code": schema.StringAttribute{
						MarkdownDescription: "HTTP status code or range (e.g. `200`, `4xx`, `5xx`)",
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
						MarkdownDescription: "Cache TTL in seconds",
						Optional:            true,
						Computed:            true,
						Default:             int64default.StaticInt64(0),
						Validators: []validator.Int64{
							int64validator.AtLeast(0),
						},
					},
				},
			},
		},
		"provider_specific": schema.ListNestedAttribute{
			MarkdownDescription: "Provider-specific configuration for the behavior",
			Optional:            true,
			NestedObject: schema.NestedAttributeObject{
				Attributes: map[string]schema.Attribute{
					"provider": schema.StringAttribute{
						MarkdownDescription: "CDN provider name, must be one of(" + strings.Join(ProviderNames, ", ") + ")",
						Required:            true,
						Validators: []validator.String{
							stringvalidator.OneOf(ProviderNames...),
						},
					},
					"code": schema.StringAttribute{
						MarkdownDescription: "Provider-specific configuration code, must be a valid JSON,\n" +
							"For guidance per provider please refer to the documentation - https://www.ioriver.io/docs/Guides/Behaviors/Behavior%20Actions/#provider-specific-code",
						Required:   true,
						CustomType: jsontypes.NormalizedType{},
					},
				},
			},
		},
	}
}

func DefaultBehaviorActionsAttributes() map[string]schema.Attribute {
	// Start from the shared action attributes and override the 5 fields that the
	// backend always fills in for default_behavior with Computed:true + plan modifiers.
	// This prevents "inconsistent result after apply" when the user partially sets
	// default_behavior and the backend fills in the rest.
	attrs := BehaviorActionAttributes()

	// cache_ttl — backend always sets a value on default_behavior
	attrs["cache_ttl"] = schema.Int64Attribute{
		MarkdownDescription: "Set the value of the edge cache TTL in seconds",
		Optional:            true,
		Computed:            true,
		Default:             int64default.StaticInt64(DefaultCacheTTL),
		Validators: []validator.Int64{
			int64validator.AtLeast(0),
		},
	}

	// cache_key — backend always returns a cache_key object on default_behavior
	// We rebuild cache_key's inner attributes so that the Computed sub-fields
	// `country` and `device_type` each have a Default (false), preventing the
	// framework's MarkComputedNilsAsUnknown from marking them Unknown when the
	// user omits the actions block. Without Defaults on these children the
	// Unknown propagates upward and makes the entire `actions` object Unknown.
	cacheKeyInner := make(map[string]schema.Attribute)
	for k, v := range BehaviorActionAttributes()["cache_key"].(schema.SingleNestedAttribute).Attributes {
		cacheKeyInner[k] = v
	}
	cacheKeyInner["country"] = schema.BoolAttribute{
		MarkdownDescription: "Include the client country in the cache key",
		Optional:            true,
		Computed:            true,
		Default:             booldefault.StaticBool(false),
	}
	cacheKeyInner["device_type"] = schema.BoolAttribute{
		MarkdownDescription: "Include the client device type (mobile/desktop) in the cache key",
		Optional:            true,
		Computed:            true,
		Default:             booldefault.StaticBool(false),
	}
	attrs["cache_key"] = schema.SingleNestedAttribute{
		MarkdownDescription: "Custom cache key configuration",
		Optional:            true,
		Computed:            true,
		Default:             objectdefault.StaticValue(defaultCacheKeyValue),
		Attributes:          cacheKeyInner,
	}

	// allowed_methods — backend always returns a value on default_behavior
	attrs["allowed_methods"] = schema.SetNestedAttribute{
		MarkdownDescription: "Set of allowed HTTP methods",
		Optional:            true,
		Computed:            true,
		Default:             setdefault.StaticValue(defaultAllowedMethodsValue),
		NestedObject: schema.NestedAttributeObject{
			Attributes: map[string]schema.Attribute{
				"method": schema.StringAttribute{
					MarkdownDescription: "HTTP method. Valid values: `" + strings.Join(httpMethodValues, "`, `") + "`",
					Optional:            true,
					Validators: []validator.String{
						stringvalidator.OneOf(httpMethodValues...),
					},
				},
			},
		},
	}

	// cached_methods — backend always returns a value on default_behavior
	attrs["cached_methods"] = schema.SetNestedAttribute{
		MarkdownDescription: "Controls the list of HTTP methods whose responses the CDN will cache",
		Optional:            true,
		Computed:            true,
		Default:             setdefault.StaticValue(defaultCachedMethodsValue),
		NestedObject: schema.NestedAttributeObject{
			Attributes: map[string]schema.Attribute{
				"method": schema.StringAttribute{
					MarkdownDescription: "Method to be cached. Valid values: `" + strings.Join(httpMethodValues, "`, `") + "`",
					Optional:            true,
					Validators: []validator.String{
						stringvalidator.OneOf(httpMethodValues...),
					},
				},
			},
		},
	}

	// status_code_cache — backend always returns entries for default_behavior
	attrs["status_codes_ttl"] = schema.ListNestedAttribute{
		MarkdownDescription: "Cache TTL configuration per status code",
		Optional:            true,
		Computed:            true,
		Default:             listdefault.StaticValue(defaultStatusCodesTtlValue),
		NestedObject:        BehaviorActionAttributes()["status_codes_ttl"].(schema.ListNestedAttribute).NestedObject,
	}

	// compression — backend always returns true on default_behavior
	attrs["compression"] = schema.BoolAttribute{
		MarkdownDescription: "Enable compression of responses.",
		Optional:            true,
		Computed:            true,
		Default:             booldefault.StaticBool(defaultCompression),
	}

	// cache_behavior — backend always returns "CACHE" on default_behavior
	attrs["cache_behavior"] = schema.StringAttribute{
		MarkdownDescription: "Controls whether content should be cached by the CDN, possible values: `" + strings.Join(cacheBehaviorValues, "`, `") + "`",
		Optional:            true,
		Computed:            true,
		Default:             stringdefault.StaticString(defaultCacheBehavior),
		Validators: []validator.String{
			stringvalidator.OneOf(cacheBehaviorValues...),
		},
	}

	return attrs
}

// BehaviorsModelToMap converts BehaviorsModel to API dict format
func BehaviorsToMap(ctx context.Context, defaultBehavior *DefaultBehaviorModel, allMatch types.List, transformCtx *ServiceTransformContext) (map[string]interface{}, error) {
	behaviorsDict := map[string]interface{}{}

	// Convert default behavior
	if defaultBehavior == nil {
		defaultBehavior = &DefaultBehaviorModel{}
	}
	defaultMap, err := defaultBehavior.ModelToMap()
	if err != nil {
		return nil, fmt.Errorf("failed to convert default behavior: %w", err)
	}
	translateStreamLogsToUUID(defaultMap, transformCtx)
	behaviorsDict["default"] = defaultMap

	if allMatch.IsNull() || allMatch.IsUnknown() {
		behaviorsDict["all_match"] = []interface{}{}
		return behaviorsDict, nil
	}

	// Convert all_match behaviors - extract from types.List
	var behaviors []BehaviorModel
	diags := allMatch.ElementsAs(ctx, &behaviors, false)
	if diags.HasError() {
		return nil, fmt.Errorf("failed to extract behaviors: %v", diags.Errors()[0])
	}

	// Record which representation each named behavior uses (for drift-free read-back).
	if transformCtx != nil {
		if transformCtx.BehaviorRepresentation == nil {
			transformCtx.BehaviorRepresentation = make(map[string]string)
		}
		for _, behavior := range behaviors {
			name := behavior.Name.ValueString()
			if name == "" {
				continue
			}
			if !behavior.PathPattern.IsNull() && behavior.PathPattern.ValueString() != "" {
				transformCtx.BehaviorRepresentation[name] = "path_pattern"
			} else if behavior.Condition != nil {
				transformCtx.BehaviorRepresentation[name] = "condition"
			}
		}
	}

	allMatchBehaviors := []interface{}{}
	for _, behavior := range behaviors {
		behaviorMap, err := behavior.ModelToMap()
		if err != nil {
			return nil, fmt.Errorf("failed to convert behavior: %w", err)
		}
		translateStreamLogsToUUID(behaviorMap, transformCtx)
		allMatchBehaviors = append(allMatchBehaviors, behaviorMap)
	}
	behaviorsDict["all_match"] = allMatchBehaviors

	return behaviorsDict, nil
}

// translateStreamLogsToUUID replaces the log destination name with its UUID in a behavior map.
func translateStreamLogsToUUID(behaviorMap map[string]interface{}, transformCtx *ServiceTransformContext) {
	if transformCtx == nil {
		return
	}
	action, ok := behaviorMap["action"].(map[string]interface{})
	if !ok {
		return
	}
	streamLogs, ok := action["logs_streaming"].(map[string]interface{})
	if !ok {
		return
	}
	if name, ok := streamLogs["destination"].(string); ok {
		if uuid, exists := transformCtx.LogDestNamesToUUIDs[name]; exists {
			streamLogs["destination"] = uuid
		}
	}
}

// translateStreamLogsToName replaces the log destination UUID with the user-facing name in a behavior action model.
func translateStreamLogsToName(action *BehaviorActionV2ResourceModel, uuidToName map[string]string) {
	if action == nil || action.StreamLogs == nil {
		return
	}
	uuid := action.StreamLogs.UnifiedLogDestination.ValueString()
	if name, exists := uuidToName[uuid]; exists {
		action.StreamLogs.UnifiedLogDestination = types.StringValue(name)
	}
}

// BehaviorsModelFromMap converts API dict to BehaviorsModel:
// Default: list to DefaultBehaviorModel
// AllMatch: list to []behaviorModel to types.List
//
// transformCtx.BehaviorRepresentation (set during write) is used to preserve
// the user's chosen representation (path_pattern vs condition) on refresh.
func BehaviorsModelFromMap(ctx context.Context, behaviorsDict map[string]interface{}, transformCtx *ServiceTransformContext, planConfig *ServiceConfigModel) (*BehaviorsModel, error) {
	if behaviorsDict == nil {
		return nil, nil
	}
	behaviors := &BehaviorsModel{}

	// Build reverse map: UUID → log dest name
	uuidToLogDestName := make(map[string]string)
	if transformCtx != nil {
		for name, uuid := range transformCtx.LogDestNamesToUUIDs {
			uuidToLogDestName[uuid] = name
		}
	}

	// Build a name-keyed lookup of representation preference from the transform context.
	var behaviorRepresentation map[string]string
	if transformCtx != nil && transformCtx.BehaviorRepresentation != nil {
		behaviorRepresentation = transformCtx.BehaviorRepresentation
	}

	// Extract default behavior
	if defaultBehavior, ok := behaviorsDict["default"].(map[string]interface{}); ok {
		behavior, err := defaultBehaviorModelFromMap(ctx, defaultBehavior, transformCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to convert default behavior: %w", err)
		}
		translateStreamLogsToName(behavior.Actions, uuidToLogDestName)
		behaviors.Default = behavior
	}

	// Extract non default behaviors - list of dicts -> []BehaviorModel
	BehaviorModelList := []BehaviorModel{}

	// Extract planFittingItem all_match behaviors for comparison
	var planAllMatch *[]BehaviorModel
	planAllMatch, err := extractCustomBehaviors(ctx, planConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to extract plan all_match behaviors: %v", err)
	}

	if allMatchBehaviors, ok := behaviorsDict["all_match"].([]interface{}); ok {
		for _, behaviorData := range allMatchBehaviors {
			behaviorMap, ok := behaviorData.(map[string]interface{})
			if !ok {
				continue
			}

			name := ""
			if n, ok := behaviorMap["name"].(string); ok {
				name = n
			}

			// Look up the representation preference recorded during the last write.
			preferPathPattern := true // default: collapse to path_pattern when possible
			if name != "" && behaviorRepresentation != nil {
				if rep, ok := behaviorRepresentation[name]; ok && rep == "condition" {
					preferPathPattern = false
				}
			}

			// TODO - use idx of loop instead of find.
			planBehavior := findBehaviorByName(ctx, planAllMatch, name)
			behavior, err := BehaviorModelfromMap(ctx, name, behaviorMap, preferPathPattern, planBehavior)
			if err != nil {
				return nil, fmt.Errorf("failed to convert behavior %s: %w", name, err)
			}
			translateStreamLogsToName(behavior.Actions, uuidToLogDestName)

			BehaviorModelList = append(BehaviorModelList, *behavior)
		}

		// Convert []BehaviorModel to types.List
		var behaviorObjectType = types.ObjectType{AttrTypes: BehaviorAttrTypes()}
		if len(BehaviorModelList) == 0 {
			// Return an empty list (not null) so that state stays consistent with
			// a prior empty-list value. ListNull ≠ ListValEmpty — Terraform treats
			// them as different and raises "provider produced inconsistent result".
			behaviors.AllMatch, _ = types.ListValueFrom(ctx, behaviorObjectType, []BehaviorModel{})
		} else {
			var diags diag.Diagnostics
			behaviors.AllMatch, diags = types.ListValueFrom(ctx, behaviorObjectType, BehaviorModelList)
			if diags.HasError() {
				return nil, fmt.Errorf("failed to convert specific behaviors: %v", diags)
			}
		}
	}

	tflog.Debug(ctx, "Converted behaviors from API map", map[string]interface{}{
		"behaviors": behaviors,
	})
	return behaviors, nil
}

// DefaultBehaviorModel.ModelToMap converts default behavior to API format
func (d *DefaultBehaviorModel) ModelToMap() (map[string]interface{}, error) {
	if d == nil {
		return nil, nil
	}

	// Convert actions - single object, not array
	actions := ServiceConfigAPIAction{}
	if d.Actions != nil {
		if err := behaviorActionModelToAPIStruct(*d.Actions, &actions); err != nil {
			return nil, err
		}
	}

	// Convert status_codes_ttl to children format — no longer needed,
	// status_code_cache is now a direct action field sent by behaviorActionModelToAPIStruct.
	children := []interface{}{}

	// The backend requires path_pattern on the default behavior node even though
	// it is not exposed in the Terraform schema (it is always "/*").
	apiStruct := ServiceConfigAPIBehavior{
		PathPattern: "/*",
		Children:    children,
		Action:      actions,
	}

	// Marshal to map
	jsonBytes, err := json.Marshal(apiStruct)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal default behavior: %w", err)
	}

	var behaviorMap map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &behaviorMap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal default behavior: %w", err)
	}

	return behaviorMap, nil
}

// ToAPIMap converts BehaviorModel to the map structure expected by service config API
func (b *BehaviorModel) ModelToMap() (map[string]interface{}, error) {
	return b.ModelToMapWithCtx(context.Background())
}

// ModelToMapWithCtx converts BehaviorModel to the map structure expected by service config API.
func (b *BehaviorModel) ModelToMapWithCtx(ctx context.Context) (map[string]interface{}, error) {
	// Convert actions - single object, not array
	actions := ServiceConfigAPIAction{}
	if b.Actions != nil {
		if err := behaviorActionModelToAPIStruct(*b.Actions, &actions); err != nil {
			return nil, err
		}
	}

	// Serialise the condition expression.
	// path_pattern is a shorthand that expands to a simple condition map.
	var condition interface{}
	if !b.PathPattern.IsNull() && !b.PathPattern.IsUnknown() && b.PathPattern.ValueString() != "" {
		condition = pathPatternToConditionMap(b.PathPattern.ValueString())
	} else if b.Condition != nil {
		condition = behaviorConditionExpressionToMap(ctx, b.Condition)
	}

	apiStruct := ServiceConfigAPIBehavior{
		Condition: condition,
		Children:  []interface{}{},
		Action:    actions,
	}

	// Add name if present
	if !b.Name.IsNull() && !b.Name.IsUnknown() && b.Name.ValueString() != "" {
		apiStruct.Name = b.Name.ValueString()
	}

	// Marshal to map
	jsonBytes, err := json.Marshal(apiStruct)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal behavior: %w", err)
	}

	var behaviorMap map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &behaviorMap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal behavior: %w", err)
	}

	return behaviorMap, nil
}

func defaultBehaviorModelFromMap(ctx context.Context, apiMap map[string]interface{}, transformCtx *ServiceTransformContext) (*DefaultBehaviorModel, error) {
	// Output object
	behavior := &DefaultBehaviorModel{}

	// Start with defaults
	defaultActionsValue := GetDefaultActionsValue()
	var actionModelDefault BehaviorActionV2ResourceModel
	_ = defaultActionsValue.As(ctx, &actionModelDefault, basetypes.ObjectAsOptions{})

	// Get data from API
	jsonBytes, err := json.Marshal(apiMap)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal API map: %w", err)
	}
	var apiStruct ServiceConfigAPIBehavior
	if err := json.Unmarshal(jsonBytes, &apiStruct); err != nil {
		return nil, fmt.Errorf("failed to unmarshal to API struct: %w", err)
	}

	// Update fields from api response

	// Convert action struct to behavior action model
	actionModel, err := apiActionStructToModel(apiStruct.Action)
	if err != nil {
		return nil, fmt.Errorf("failed to parse actions: %w", err)
	}

	// Extract status_code_cache — now a direct action field, handled by apiActionStructToModel.

	// Propagate defaults if missing in the response:
	propagateMissingDefaultActions(actionModel, &actionModelDefault)

	behavior.Actions = actionModel

	return behavior, nil
}

// Direct conversion from Terraform model to API struct (bypassing ioriver.BehaviorAction)
// This function accumulates ALL fields from the model into the apiAction struct
func behaviorActionModelToAPIStruct(action BehaviorActionV2ResourceModel, apiAction *ServiceConfigAPIAction) error {
	// Process ALL fields (no early returns) to accumulate everything into apiAction

	if len(action.ProviderSpecific) > 0 {
		items := []ServiceConfigAPIProviderSpecific{}
		for _, item := range action.ProviderSpecific {
			providerName := item.Provider.ValueString()
			name, ok := ProviderNamesMapHCLToBackend[providerName]
			if !ok || name == "" {
				name = providerName
			}
			entry := ServiceConfigAPIProviderSpecific{
				Name:  name,
				Value: item.Code.ValueString(),
			}
			items = append(items, entry)
		}
		apiAction.ProviderSpecific = items
	}

	// Status Code Cache
	if len(action.StatusCodeCache) > 0 {
		items := []ServiceConfigAPIStatusCodeCache{}
		for _, item := range action.StatusCodeCache {
			entry := ServiceConfigAPIStatusCodeCache{
				Code:        item.StatusCode.ValueString(),
				TTL:         int(item.CacheTTL.ValueInt64()),
				BypassCache: item.CacheBehavior.ValueString() == "BYPASS",
			}
			items = append(items, entry)
		}
		apiAction.StatusCodeCache = items
	}

	// Cache Key — start from the same backend defaults used for default_behavior,
	// then override with whatever the user explicitly set.
	if action.CacheKey != nil {
		cacheKey := &ServiceConfigAPICacheKey{
			Headers:         []string{},
			Cookies:         []string{},
			QueryParams:     []string{},
			QueryParamsMode: "exclude",
			Country:         false,
			DeviceType:      false,
		}
		for _, h := range action.CacheKey.Headers {
			cacheKey.Headers = append(cacheKey.Headers, h.Header.ValueString())
		}
		for _, c := range action.CacheKey.Cookies {
			cacheKey.Cookies = append(cacheKey.Cookies, c.Cookie.ValueString())
		}
		if action.CacheKey.QueryStrings != nil {
			tfType := action.CacheKey.QueryStrings.ListType.ValueString()
			// The backend QueryParamsMode enum has only "include" and "exclude".
			// "none" = include nothing (no query params in cache key) → "include" + empty params
			// "all"  = exclude nothing (all query params in cache key) → "exclude" + empty params
			switch tfType {
			case "none":
				cacheKey.QueryParamsMode = "include"
			case "all":
				cacheKey.QueryParamsMode = "exclude"
			default:
				cacheKey.QueryParamsMode = tfType
			}
			for _, p := range action.CacheKey.QueryStrings.ParamsList {
				cacheKey.QueryParams = append(cacheKey.QueryParams, p.Param.ValueString())
			}
		}
		if !action.CacheKey.Country.IsNull() {
			cacheKey.Country = action.CacheKey.Country.ValueBool()
		}
		if !action.CacheKey.DeviceType.IsNull() {
			cacheKey.DeviceType = action.CacheKey.DeviceType.ValueBool()
		}
		apiAction.CacheKey = cacheKey
	}

	// Cached Methods
	if action.CachedMethods != nil {
		if apiAction.CacheBehavior == nil {
			apiAction.CacheBehavior = &ServiceConfigAPICacheBehavior{}
		}
		methods := []string{}
		for _, m := range *action.CachedMethods {
			methods = append(methods, m.Method.ValueString())
		}
		apiAction.CacheBehavior.CachedMethods = methods
	}

	// Cache TTL
	if !action.CacheTTL.IsNull() {
		ttl := int(action.CacheTTL.ValueInt64())
		apiAction.CacheTTL = &ttl
	}

	// Browser Cache TTL
	if !action.BrowserCacheTtl.IsNull() {
		ttl := int(action.BrowserCacheTtl.ValueInt64())
		apiAction.BrowserCacheTTL = &ttl
	}

	// Viewer Protocol
	if !action.ViewerProtocol.IsNull() {
		protocol := strings.ToLower(action.ViewerProtocol.ValueString())
		apiAction.ViewerProtocol = &protocol
	}

	// Redirect
	if action.Redirect != nil {
		src := (*string)(nil)
		if !action.Redirect.Source.IsNull() {
			s := action.Redirect.Source.ValueString()
			src = &s
		}
		apiAction.Redirect = &ServiceConfigAPIRedirect{
			Destination: action.Redirect.Destination.ValueString(),
			Source:      src,
		}
	}

	// Origin Cache Control
	if !action.OriginCacheControl.IsNull() {
		enabled := action.OriginCacheControl.ValueBool()
		apiAction.OriginCacheControl = &enabled
	}

	// Bypass Cache On Cookie - not supported in new service config API, skip

	// Host Header - maps to flat backend fields host_header_override and host_header_use_origin
	if action.HostHeader != nil {
		if !action.HostHeader.HeaderValue.IsNull() {
			v := action.HostHeader.HeaderValue.ValueString()
			apiAction.HostHeaderOverride = &v
		}
		if !action.HostHeader.UseOriginHost.IsNull() {
			useOrigin := action.HostHeader.UseOriginHost.ValueBool()
			apiAction.HostHeaderUseOrigin = &useOrigin
		}
	}

	// CORS
	if action.Cors != nil {
		cors := &ServiceConfigAPICors{}
		if action.Cors.AllowOrigin != nil {
			ao := &ServiceConfigAPICorsAllowOrigin{Mode: action.Cors.AllowOrigin.Mode.ValueString()}
			for _, o := range action.Cors.AllowOrigin.Origins {
				ao.Origins = append(ao.Origins, o.ValueString())
			}
			if !action.Cors.AllowOrigin.Override.IsNull() {
				v := action.Cors.AllowOrigin.Override.ValueBool()
				ao.Override = &v
			} else {
				ao.Override = nil
			}
			cors.AllowOrigin = ao
		}
		if action.Cors.AllowHeaders != nil {
			ah := &ServiceConfigAPICorsValueList{Mode: action.Cors.AllowHeaders.Mode.ValueString()}
			for _, v := range action.Cors.AllowHeaders.Values {
				ah.Values = append(ah.Values, v.ValueString())
			}
			if !action.Cors.AllowHeaders.Override.IsNull() {
				v := action.Cors.AllowHeaders.Override.ValueBool()
				ah.Override = &v
			} else {
				ah.Override = nil
			}
			cors.AllowHeaders = ah
		}
		if action.Cors.ExposeHeaders != nil {
			eh := &ServiceConfigAPICorsValueList{Mode: action.Cors.ExposeHeaders.Mode.ValueString()}
			for _, v := range action.Cors.ExposeHeaders.Values {
				eh.Values = append(eh.Values, v.ValueString())
			}
			if !action.Cors.ExposeHeaders.Override.IsNull() {
				v := action.Cors.ExposeHeaders.Override.ValueBool()
				eh.Override = &v
			} else {
				eh.Override = nil
			}
			cors.ExposeHeaders = eh
		}
		if action.Cors.AllowMethods != nil {
			am := &ServiceConfigAPICorsValueList{Mode: action.Cors.AllowMethods.Mode.ValueString()}
			for _, v := range action.Cors.AllowMethods.Values {
				am.Values = append(am.Values, v.ValueString())
			}
			if !action.Cors.AllowMethods.Override.IsNull() {
				v := action.Cors.AllowMethods.Override.ValueBool()
				am.Override = &v
			} else {
				am.Override = nil
			}
			cors.AllowMethods = am
		}
		if !action.Cors.AllowCredentials.IsNull() {
			v := action.Cors.AllowCredentials.ValueBool()
			cors.AllowCredentials = &v
		} else {
			cors.AllowCredentials = nil
		}
		if action.Cors.MaxAge != nil {
			if !action.Cors.MaxAge.Value.IsNull() {
				v := int(action.Cors.MaxAge.Value.ValueInt64())
				cors.MaxAge = &v
			}
			if !action.Cors.MaxAge.Override.IsNull() {
				v := action.Cors.MaxAge.Override.ValueBool()
				cors.OverrideMaxAge = &v
			}
		} else {
			cors.MaxAge = nil
			cors.OverrideMaxAge = nil
		}
		apiAction.Cors = cors
	}

	// Override Origin - obsolete in new service config API, skip

	// Header modification actions — request_headers, response_headers, origin_response_headers
	serializeHeaderActions := func(entries *[]HeaderActionModelV2) []ServiceConfigAPIHeaderAction {
		if entries == nil {
			return nil
		}
		out := make([]ServiceConfigAPIHeaderAction, 0, len(*entries))
		for _, h := range *entries {
			entry := ServiceConfigAPIHeaderAction{Name: h.Name.ValueString()}
			for _, v := range h.Values {
				entry.Values = append(entry.Values, v.ValueString())
			}
			switch h.Action.ValueString() {
			case "delete":
				t := true
				entry.Delete = &t
			case "add":
				f := false
				entry.Override = &f
			default: // "set"
				t := true
				entry.Override = &t
			}
			out = append(out, entry)
		}
		return out
	}
	if result := serializeHeaderActions(action.RequestHeaders); result != nil {
		apiAction.RequestHeaders = result
	}
	if result := serializeHeaderActions(action.ResponseHeaders); result != nil {
		apiAction.ResponseHeaders = result
	}
	if result := serializeHeaderActions(action.OriginResponseHeaders); result != nil {
		apiAction.OriginResponseHeaders = result
	}

	// Follow Redirects
	if !action.FollowRedirects.IsNull() && action.FollowRedirects.ValueBool() {
		followRedirects := true
		apiAction.FollowRedirects = &followRedirects
	}

	// Generate Preflight Response
	if action.GeneratePreflightResponse != nil {
		methods := []string{}
		if action.GeneratePreflightResponse.AllowedMethods != nil {
			for _, m := range *action.GeneratePreflightResponse.AllowedMethods {
				methods = append(methods, m.Method.ValueString())
			}
		}
		headers := []string{}
		for _, h := range action.GeneratePreflightResponse.AllowedHeaders {
			headers = append(headers, h.ValueString())
		}
		maxAge := int(action.GeneratePreflightResponse.MaxAge.ValueInt64())
		apiAction.GeneratePreflightResponse = &ServiceConfigAPIPreflightResponse{
			AllowedMethods: methods,
			AllowedHeaders: headers,
			MaxTTL:         &maxAge,
		}
	}

	// Status Code Browser Cache
	if len(action.StatusCodeBrowserCache) > 0 {
		for _, item := range action.StatusCodeBrowserCache {
			statusCode, err := statusCodeToInt(item.StatusCode.ValueString())
			if err != nil {
				return fmt.Errorf("invalid status code: %w", err)
			}
			apiAction.StatusCodeBrowserCache = append(apiAction.StatusCodeBrowserCache, ServiceConfigAPIStatusCodeBrowserCache{
				Code: statusCodeToString(statusCode),
				TTL:  int(item.CacheTtl.ValueInt64()),
			})
		}
	}

	// Stale TTL
	if !action.StaleTtl.IsNull() {
		ttl := int(action.StaleTtl.ValueInt64())
		apiAction.StaleTTL = &ttl
	}

	// Stream Logs
	if action.StreamLogs != nil {
		apiAction.StreamLogs = &ServiceConfigAPIStreamLogs{
			UnifiedLogDestination:  action.StreamLogs.UnifiedLogDestination.ValueString(),
			UnifiedLogSamplingRate: int(action.StreamLogs.UnifiedLogSamplingRate.ValueInt64()),
		}
	}

	// Allowed Methods
	if action.AllowedMethods != nil {
		methods := []string{}
		for _, m := range *action.AllowedMethods {
			methods = append(methods, m.Method.ValueString())
		}
		apiAction.AllowedMethods = methods
	}

	// Compression
	if !action.Compression.IsNull() {
		enabled := action.Compression.ValueBool()
		apiAction.Compression = &enabled
	}

	// Large Files Optimization
	if !action.LargeFilesOptimization.IsNull() {
		enabled := action.LargeFilesOptimization.ValueBool()
		apiAction.LargeFilesOptimization = &enabled
	}

	// URL Signing
	if !action.UrlSigning.IsNull() {
		enabled := action.UrlSigning.ValueBool()
		apiAction.URLSigning = &enabled
	}

	// True Client IP
	if !action.TrueClientIP.IsNull() {
		enabled := action.TrueClientIP.ValueBool()
		apiAction.TrueClientIP = &enabled
	}

	// Deny Access
	if !action.DenyAccess.IsNull() {
		enabled := action.DenyAccess.ValueBool()
		apiAction.DenyAccess = &enabled
	}

	// Status Code Custom Response
	if len(action.StatusCodeCustomResponse) > 0 {
		items := []ServiceConfigAPIStatusCodeCustomResponse{}
		for _, item := range action.StatusCodeCustomResponse {
			items = append(items, ServiceConfigAPIStatusCodeCustomResponse{
				Code:        item.StatusCode.ValueString(),
				ResponseURL: item.ResponseURL.ValueString(),
			})
		}
		apiAction.StatusCodeCustomResponse = items
	}

	// Allow Access Only From IP
	if action.AllowAccessOnlyFromIP != nil {
		ipList := []ServiceConfigAPIIP{}
		for _, ipModel := range *action.AllowAccessOnlyFromIP {
			ipList = append(ipList, ServiceConfigAPIIP{IP: ipModel.IP.ValueString()})
		}
		apiAction.AllowAccessOnlyFromIP = ipList
	}

	// Deny Access By IP
	if action.DenyAccessByIP != nil {
		ipList := []string{}
		for _, ipModel := range *action.DenyAccessByIP {
			ipList = append(ipList, ipModel.IP.ValueString())
		}
		apiAction.DenyAccessByIP = ipList
	}

	// Deny Access By Time
	if action.DenyAccessByTime != nil {
		constraints := []ServiceConfigAPITimeConstraint{}
		for _, tc := range *action.DenyAccessByTime {
			apiTC := ServiceConfigAPITimeConstraint{}
			if tc.DateTimeWindow != nil {
				apiTC.DateTimeWindow = &ServiceConfigAPIDateTimeWindow{
					StartDate: tc.DateTimeWindow.StartDate.ValueInt64(),
					EndDate:   tc.DateTimeWindow.EndDate.ValueInt64(),
				}
			}
			if tc.TimePeriodic != nil {
				apiTC.TimePeriodic = &ServiceConfigAPITimePeriodic{
					StartDate:         tc.TimePeriodic.StartDate.ValueInt64(),
					Duration:          tc.TimePeriodic.Duration.ValueInt64(),
					DurationUnits:     tc.TimePeriodic.DurationUnits.ValueString(),
					RepeatPeriod:      tc.TimePeriodic.RepeatPeriod.ValueInt64(),
					RepeatPeriodUnits: tc.TimePeriodic.RepeatPeriodUnits.ValueString(),
				}
			}
			constraints = append(constraints, apiTC)
		}
		apiAction.DenyAccessByTime = constraints
	}

	// URL Rewrites
	if action.UrlRewrites != nil {
		rewrites := []ServiceConfigAPIUrlRewrite{}
		for _, r := range *action.UrlRewrites {
			rewrites = append(rewrites, ServiceConfigAPIUrlRewrite{
				Source:      r.Source.ValueString(),
				Destination: r.Destination.ValueString(),
			})
		}
		apiAction.UrlRewrites = rewrites
	}

	// Cache Behavior - checked LAST because it has a default value
	if !action.CacheBehavior.IsNull() {
		if apiAction.CacheBehavior == nil {
			apiAction.CacheBehavior = &ServiceConfigAPICacheBehavior{}
		}
		apiAction.CacheBehavior.BypassCache = (action.CacheBehavior.ValueString() == "BYPASS")
	}

	return nil
}

// fromMap converts API map structure back to BehaviorModel.
// preferPathPattern controls whether a simple http.request.path/match condition
// is collapsed back to the path_pattern shorthand. Pass true (the default) to
// collapse, false to always return a full condition block.
func BehaviorModelfromMap(ctx context.Context, name string, apiMap map[string]interface{}, preferPathPattern bool, planBehavior *BehaviorModel) (*BehaviorModel, error) {
	// Marshal map to JSON then unmarshal to typed struct
	jsonBytes, err := json.Marshal(apiMap)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal API map: %w", err)
	}

	var apiStruct ServiceConfigAPIBehavior
	if err := json.Unmarshal(jsonBytes, &apiStruct); err != nil {
		return nil, fmt.Errorf("failed to unmarshal to API struct: %w", err)
	}

	behavior := &BehaviorModel{}

	// Set name if provided
	if name != "" {
		behavior.Name = types.StringValue(name)
	} else {
		behavior.Name = types.StringNull()
	}

	// Deserialise condition expression from the API map directly (not the re-marshalled struct,
	// since Condition is interface{} and loses type info through JSON round-trip).
	//
	// preferPathPattern (caller-supplied) controls whether a simple http.request.path/match
	// condition is collapsed back to the path_pattern shorthand (avoids drift).
	behavior.PathPattern = types.StringNull()
	if condMap, ok := apiMap["condition"].(map[string]interface{}); ok {
		pattern, isSimple := isSimplePathPattern(condMap)
		if preferPathPattern && isSimple {
			behavior.PathPattern = types.StringValue(pattern)
		} else {
			var planCond *BehaviorConditionExpressionModel
			if planBehavior != nil {
				planCond = planBehavior.Condition
			}
			var err error
			behavior.Condition, err = behaviorConditionExpressionFromMap(ctx, condMap, planCond)
			if err != nil {
				return nil, fmt.Errorf("failed to parse condition: %w", err)
			}
			tflog.Debug(ctx, fmt.Sprintf(" [BehaviorModelfromMap] 🔍 converted condition: %+v", behavior.Condition))
		}
	}

	// Convert action struct to behavior action model
	actionModel, err := apiActionStructToModel(apiStruct.Action)
	if err != nil {
		return nil, fmt.Errorf("failed to parse actions: %w", err)
	}
	behavior.Actions = actionModel

	return behavior, nil
}

// apiActionStructToModel converts API action struct to a single Terraform action model
func apiActionStructToModel(apiAction ServiceConfigAPIAction) (*BehaviorActionV2ResourceModel, error) {
	model := &BehaviorActionV2ResourceModel{}

	// Cache TTL
	if apiAction.CacheTTL != nil {
		model.CacheTTL = types.Int64Value(int64(*apiAction.CacheTTL))
	}

	// Cache Behavior
	if apiAction.CacheBehavior != nil {
		if apiAction.CacheBehavior.BypassCache {
			model.CacheBehavior = types.StringValue("BYPASS")
		} else {
			model.CacheBehavior = types.StringValue("CACHE")
		}

		// Cached Methods
		if len(apiAction.CacheBehavior.CachedMethods) > 0 {
			cachedMethods := []MethodModelV2{}
			for _, method := range apiAction.CacheBehavior.CachedMethods {
				cachedMethods = append(cachedMethods, MethodModelV2{Method: types.StringValue(method)})
			}
			model.CachedMethods = &cachedMethods
		}
	}

	// Browser Cache TTL
	if apiAction.BrowserCacheTTL != nil {
		model.BrowserCacheTtl = types.Int64Value(int64(*apiAction.BrowserCacheTTL))
	}

	// Compression
	if apiAction.Compression != nil {
		model.Compression = types.BoolValue(*apiAction.Compression)
	}

	// Cache Key
	if apiAction.CacheKey != nil {
		cacheKey := &CacheKeyModelV2{}

		// Headers - always initialize even if empty
		headers := []HeaderModelV2{}
		for _, h := range apiAction.CacheKey.Headers {
			headers = append(headers, HeaderModelV2{Header: types.StringValue(h)})
		}
		cacheKey.Headers = headers

		// Cookies - always initialize even if empty
		cookies := []CookieModelV2{}
		for _, c := range apiAction.CacheKey.Cookies {
			cookies = append(cookies, CookieModelV2{Cookie: types.StringValue(c)})
		}
		cacheKey.Cookies = cookies

		// Query Strings - always initialize
		// For "all"/"none" modes the backend returns an empty query_params list,
		// but the user does not write a `params` block — keep ParamsList nil so
		// Terraform does not produce a perpetual diff.
		// For "include"/"exclude" always initialise to at least an empty slice so
		// that a zero-param config stays as an empty set (not null) in state.
		var paramList []ParamModelV2
		apiMode := apiAction.CacheKey.QueryParamsMode
		if apiMode == "include" || apiMode == "exclude" {
			paramList = []ParamModelV2{}
			for _, p := range apiAction.CacheKey.QueryParams {
				paramList = append(paramList, ParamModelV2{Param: types.StringValue(p)})
			}
		}
		// "none" is stored on the backend as "include" with empty query_params
		// "all"  is stored on the backend as "exclude" with empty query_params
		// (the backend has no "none"/"all" enum values). Map them back so state
		// matches what the user wrote and there is no perpetual diff.
		// Also reset paramList to nil so Terraform does not render a params block.
		tfMode := apiMode
		if apiMode == "include" && len(apiAction.CacheKey.QueryParams) == 0 {
			tfMode = "none"
			paramList = nil
		} else if apiMode == "exclude" && len(apiAction.CacheKey.QueryParams) == 0 {
			tfMode = "all"
			paramList = nil
		}
		cacheKey.QueryStrings = &QueryStringsModelV2{
			ParamsList: paramList,
			ListType:   types.StringValue(tfMode),
		}
		cacheKey.Country = types.BoolValue(apiAction.CacheKey.Country)
		cacheKey.DeviceType = types.BoolValue(apiAction.CacheKey.DeviceType)

		model.CacheKey = cacheKey
	}

	// Allowed Methods
	if len(apiAction.AllowedMethods) > 0 {
		allowedMethods := []MethodModelV2{}
		for _, method := range apiAction.AllowedMethods {
			allowedMethods = append(allowedMethods, MethodModelV2{Method: types.StringValue(method)})
		}
		model.AllowedMethods = &allowedMethods
	}

	// Viewer Protocol
	if apiAction.ViewerProtocol != nil {
		model.ViewerProtocol = types.StringValue(strings.ToUpper(*apiAction.ViewerProtocol))
	}

	// Redirect
	if apiAction.Redirect != nil {
		var src types.String
		if apiAction.Redirect.Source != nil {
			src = types.StringValue(*apiAction.Redirect.Source)
		} else {
			src = types.StringNull()
		}
		model.Redirect = &RedirectModelV2{
			Destination: types.StringValue(apiAction.Redirect.Destination),
			Source:      src,
		}
	}

	// Origin Cache Control
	if apiAction.OriginCacheControl != nil {
		model.OriginCacheControl = types.BoolValue(*apiAction.OriginCacheControl)
	}

	// Bypass Cache On Cookie - not returned by new service config API

	// Host Header - backend returns host_header_override and host_header_use_origin as flat fields
	if apiAction.HostHeaderOverride != nil || apiAction.HostHeaderUseOrigin != nil {
		hostHeader := &HostHeaderModelV2{}
		if apiAction.HostHeaderOverride != nil {
			hostHeader.HeaderValue = types.StringValue(*apiAction.HostHeaderOverride)
		}
		if apiAction.HostHeaderUseOrigin != nil {
			hostHeader.UseOriginHost = types.BoolValue(*apiAction.HostHeaderUseOrigin)
		}
		model.HostHeader = hostHeader
	}

	// CORS
	if apiAction.Cors != nil {
		c := apiAction.Cors
		corsModel := &CorsConfigModelV2{}
		if c.AllowOrigin != nil {
			ao := &CorsAllowOriginModelV2{Mode: types.StringValue(c.AllowOrigin.Mode)}
			for _, o := range c.AllowOrigin.Origins {
				ao.Origins = append(ao.Origins, types.StringValue(o))
			}
			if c.AllowOrigin.Override != nil {
				ao.Override = types.BoolValue(*c.AllowOrigin.Override)
			} else {
				ao.Override = types.BoolNull()
			}
			corsModel.AllowOrigin = ao
		}
		if c.AllowHeaders != nil {
			ah := &CorsHeaderListModelV2{Mode: types.StringValue(c.AllowHeaders.Mode)}
			for _, v := range c.AllowHeaders.Values {
				ah.Values = append(ah.Values, types.StringValue(v))
			}
			if c.AllowHeaders.Override != nil {
				ah.Override = types.BoolValue(*c.AllowHeaders.Override)
			} else {
				ah.Override = types.BoolNull()
			}
			corsModel.AllowHeaders = ah
		}
		if c.ExposeHeaders != nil {
			eh := &CorsHeaderListModelV2{Mode: types.StringValue(c.ExposeHeaders.Mode)}
			for _, v := range c.ExposeHeaders.Values {
				eh.Values = append(eh.Values, types.StringValue(v))
			}
			if c.ExposeHeaders.Override != nil {
				eh.Override = types.BoolValue(*c.ExposeHeaders.Override)
			} else {
				eh.Override = types.BoolNull()
			}
			corsModel.ExposeHeaders = eh
		}
		if c.AllowMethods != nil {
			am := &CorsMethodListModelV2{Mode: types.StringValue(c.AllowMethods.Mode)}
			for _, v := range c.AllowMethods.Values {
				am.Values = append(am.Values, types.StringValue(v))
			}
			if c.AllowMethods.Override != nil {
				am.Override = types.BoolValue(*c.AllowMethods.Override)
			} else {
				am.Override = types.BoolNull()
			}
			corsModel.AllowMethods = am
		}
		if c.AllowCredentials != nil {
			corsModel.AllowCredentials = types.BoolValue(*c.AllowCredentials)
		} else {
			corsModel.AllowCredentials = types.BoolNull()
		}
		if c.MaxAge != nil || c.OverrideMaxAge != nil {
			ma := &CorsMaxAgeModelV2{}
			if c.MaxAge != nil {
				ma.Value = types.Int64Value(int64(*c.MaxAge))
			} else {
				ma.Value = types.Int64Null()
			}
			if c.OverrideMaxAge != nil {
				ma.Override = types.BoolValue(*c.OverrideMaxAge)
			} else {
				ma.Override = types.BoolNull()
			}
			corsModel.MaxAge = ma
		} else {
			corsModel.MaxAge = nil
		}
		model.Cors = corsModel
	}

	// Override Origin - obsolete, not returned by new service config API

	// Header modification actions — request_headers, response_headers, origin_response_headers
	deserializeHeaderActions := func(entries []ServiceConfigAPIHeaderAction) *[]HeaderActionModelV2 {
		if len(entries) == 0 {
			return nil
		}
		out := make([]HeaderActionModelV2, 0, len(entries))
		for _, h := range entries {
			entry := HeaderActionModelV2{Name: types.StringValue(h.Name)}
			if len(h.Values) > 0 {
				entry.Values = make([]types.String, 0, len(h.Values))
				for _, v := range h.Values {
					entry.Values = append(entry.Values, types.StringValue(v))
				}
			}
			// else: leave Values nil (null) — matches what the framework produces for
			// an absent optional list (e.g. delete action with no values in HCL)
			switch {
			case h.Delete != nil && *h.Delete:
				entry.Action = types.StringValue("delete")
			case h.Override != nil && !*h.Override:
				entry.Action = types.StringValue("add")
			default:
				entry.Action = types.StringValue("set")
			}
			out = append(out, entry)
		}
		return &out
	}
	model.RequestHeaders = deserializeHeaderActions(apiAction.RequestHeaders)
	model.ResponseHeaders = deserializeHeaderActions(apiAction.ResponseHeaders)
	model.OriginResponseHeaders = deserializeHeaderActions(apiAction.OriginResponseHeaders)

	// Follow Redirects
	if apiAction.FollowRedirects != nil {
		model.FollowRedirects = types.BoolValue(*apiAction.FollowRedirects)
	}

	// Stale TTL
	if apiAction.StaleTTL != nil {
		model.StaleTtl = types.Int64Value(int64(*apiAction.StaleTTL))
	}

	// Large Files Optimization
	if apiAction.LargeFilesOptimization != nil {
		model.LargeFilesOptimization = types.BoolValue(*apiAction.LargeFilesOptimization)
	}

	// URL Signing
	if apiAction.URLSigning != nil {
		model.UrlSigning = types.BoolValue(*apiAction.URLSigning)
	}

	// True Client IP
	if apiAction.TrueClientIP != nil {
		model.TrueClientIP = types.BoolValue(*apiAction.TrueClientIP)
	}

	// Deny Access
	if apiAction.DenyAccess != nil {
		model.DenyAccess = types.BoolValue(*apiAction.DenyAccess)
	}

	// Response Headers - not returned by new service config API in this format

	// Request Headers - not returned by new service config API in this format

	// Delete Response Headers - not returned by new service config API in this format

	// Delete Request Headers - not returned by new service config API in this format

	// Allow Access Only From IP
	if len(apiAction.AllowAccessOnlyFromIP) > 0 {
		ipList := []IPModelV2{}
		for _, ip := range apiAction.AllowAccessOnlyFromIP {
			ipList = append(ipList, IPModelV2{IP: types.StringValue(ip.IP)})
		}
		model.AllowAccessOnlyFromIP = &ipList
	}

	// Deny Access By IP
	if len(apiAction.DenyAccessByIP) > 0 {
		ipList := []IPModelV2{}
		for _, ip := range apiAction.DenyAccessByIP {
			ipList = append(ipList, IPModelV2{IP: types.StringValue(ip)})
		}
		model.DenyAccessByIP = &ipList
	}

	// Deny Access By Time
	if len(apiAction.DenyAccessByTime) > 0 {
		constraints := []TimeConstraintModelV2{}
		for _, tc := range apiAction.DenyAccessByTime {
			tcModel := TimeConstraintModelV2{}
			if tc.DateTimeWindow != nil {
				tcModel.DateTimeWindow = &DateTimeWindowModelV2{
					StartDate: types.Int64Value(tc.DateTimeWindow.StartDate),
					EndDate:   types.Int64Value(tc.DateTimeWindow.EndDate),
				}
			}
			if tc.TimePeriodic != nil {
				tcModel.TimePeriodic = &TimePeriodicModelV2{
					StartDate:         types.Int64Value(tc.TimePeriodic.StartDate),
					Duration:          types.Int64Value(tc.TimePeriodic.Duration),
					DurationUnits:     types.StringValue(tc.TimePeriodic.DurationUnits),
					RepeatPeriod:      types.Int64Value(tc.TimePeriodic.RepeatPeriod),
					RepeatPeriodUnits: types.StringValue(tc.TimePeriodic.RepeatPeriodUnits),
				}
			}
			constraints = append(constraints, tcModel)
		}
		model.DenyAccessByTime = &constraints
	}

	// URL Rewrites — only set when the API returned at least one entry.
	// The schema is Optional-only (not Computed), so returning nil correctly maps to null in state.
	if len(apiAction.UrlRewrites) > 0 {
		rewrites := []UrlRewriteModelV2{}
		for _, r := range apiAction.UrlRewrites {
			rewrites = append(rewrites, UrlRewriteModelV2{
				Source:      types.StringValue(r.Source),
				Destination: types.StringValue(r.Destination),
			})
		}
		model.UrlRewrites = &rewrites
	}

	// Generate Preflight Response
	if apiAction.GeneratePreflightResponse != nil {
		methods := []MethodModelV2{}
		for _, method := range apiAction.GeneratePreflightResponse.AllowedMethods {
			methods = append(methods, MethodModelV2{Method: types.StringValue(method)})
		}
		headers := []types.String{}
		for _, h := range apiAction.GeneratePreflightResponse.AllowedHeaders {
			headers = append(headers, types.StringValue(h))
		}
		maxAge := types.Int64Null()
		if apiAction.GeneratePreflightResponse.MaxTTL != nil {
			maxAge = types.Int64Value(int64(*apiAction.GeneratePreflightResponse.MaxTTL))
		}
		model.GeneratePreflightResponse = &GeneratePreflightResponseModelV2{
			AllowedMethods: &methods,
			AllowedHeaders: headers,
			MaxAge:         maxAge,
		}
	}

	// Status Code Browser Cache
	if len(apiAction.StatusCodeBrowserCache) > 0 {
		items := []StatusCodeBrowserCacheModelV2{}
		for _, item := range apiAction.StatusCodeBrowserCache {
			items = append(items, StatusCodeBrowserCacheModelV2{
				StatusCode: types.StringValue(item.Code),
				CacheTtl:   types.Int64Value(int64(item.TTL)),
			})
		}
		model.StatusCodeBrowserCache = items
	}

	// Stream Logs
	if apiAction.StreamLogs != nil {
		model.StreamLogs = &StreamLogsModelV2{
			UnifiedLogDestination:  types.StringValue(apiAction.StreamLogs.UnifiedLogDestination),
			UnifiedLogSamplingRate: types.Int64Value(int64(apiAction.StreamLogs.UnifiedLogSamplingRate)),
		}
	}

	// Status Code Cache
	if len(apiAction.StatusCodeCache) > 0 {
		items := []StatusCodeCacheModelV2{}
		for _, item := range apiAction.StatusCodeCache {
			cacheBehavior := "CACHE"
			if item.BypassCache {
				cacheBehavior = "BYPASS"
			}
			items = append(items, StatusCodeCacheModelV2{
				StatusCode:    types.StringValue(item.Code),
				CacheBehavior: types.StringValue(cacheBehavior),
				CacheTTL:      types.Int64Value(int64(item.TTL)),
			})
		}
		model.StatusCodeCache = items
	}

	// Status Code Custom Response
	if len(apiAction.StatusCodeCustomResponse) > 0 {
		items := []StatusCodeCustomResponseModelV2{}
		for _, item := range apiAction.StatusCodeCustomResponse {
			items = append(items, StatusCodeCustomResponseModelV2{
				StatusCode:  types.StringValue(item.Code),
				ResponseURL: types.StringValue(item.ResponseURL),
			})
		}
		model.StatusCodeCustomResponse = items
	}

	if len(apiAction.ProviderSpecific) > 0 {
		items := []ProviderSpecificModel{}
		for _, item := range apiAction.ProviderSpecific {
			providerName := item.Name
			name, ok := ProviderNamesMapBackendToHCL[providerName]
			if !ok {
				return nil, fmt.Errorf("unknown provider name returned by backend: %q", providerName)
			}
			items = append(items, ProviderSpecificModel{
				Provider: types.StringValue(name),
				Code:     jsontypes.NewNormalizedValue(item.Value),
			})
		}
		model.ProviderSpecific = items
	}

	return model, nil
}

func propagateMissingDefaultActions(dest *BehaviorActionV2ResourceModel, src *BehaviorActionV2ResourceModel) {
	// Pointer fields — nil means the API did not return a value; fill from defaults.
	if dest.CacheKey == nil {
		dest.CacheKey = src.CacheKey
	}
	if dest.AllowedMethods == nil {
		dest.AllowedMethods = src.AllowedMethods
	}
	if dest.CachedMethods == nil {
		dest.CachedMethods = src.CachedMethods
	}
	// types.Int64 / types.String are value types; IsNull() means unset.
	if dest.CacheTTL.IsNull() {
		dest.CacheTTL = src.CacheTTL
	}
	// status_codes_ttl is a slice — empty means unset.
	if len(dest.StatusCodeCache) == 0 {
		dest.StatusCodeCache = src.StatusCodeCache
	}
	// compression — backend always returns true for default_behavior
	if dest.Compression.IsNull() {
		dest.Compression = src.Compression
	}
	// cache_behavior — backend always returns "CACHE" for default_behavior
	if dest.CacheBehavior.IsNull() {
		dest.CacheBehavior = src.CacheBehavior
	}
}

func extractCustomBehaviors(ctx context.Context, planConfig *ServiceConfigModel) (*[]BehaviorModel, error) {
	planAllMatch := []BehaviorModel{}
	if planConfig != nil {
		var allMatchPlan types.List
		if !planConfig.Behaviors.IsNull() && !planConfig.Behaviors.IsUnknown() {
			var b BehaviorsBlockModel
			diags := planConfig.Behaviors.As(ctx, &b, basetypes.ObjectAsOptions{})
			if diags.HasError() {
				return nil, fmt.Errorf("failed to unmarshal plan config behaviors: %v", diags)
			}
			allMatchPlan = b.Custom
		}
		diags := allMatchPlan.ElementsAs(ctx, &planAllMatch, false)
		if diags.HasError() {
			return nil, fmt.Errorf("failed to extract behaviors from plan: %v", diags.Errors()[0])
		}
	}
	return &planAllMatch, nil
}

func findBehaviorByName(ctx context.Context, behaviors *[]BehaviorModel, name string) *BehaviorModel {
	if behaviors == nil {
		tflog.Debug(ctx, "[findBehaviorByName] behaviors is nil")
		return nil
	}
	for _, behavior := range *behaviors {
		if behavior.Name.IsNull() || behavior.Name.IsUnknown() {
			tflog.Debug(ctx, "[findBehaviorByName] behavior name is nil or unknown")
			continue
		}
		if name == behavior.Name.ValueString() {
			tflog.Debug(ctx, "[findBehaviorByName] found behavior: "+name)
			return &behavior
		}
	}
	tflog.Debug(ctx, "[findBehaviorByName] behavior not found: "+name)
	return nil
}
