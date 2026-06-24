package provider

import (
	"context"
	"fmt"
	"net"
	"regexp"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/float64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/float64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// ---------------------------------------------------------------------------
// Enums (mirroring backend WAFConditionField / WAFConditionOperator)
// ---------------------------------------------------------------------------

var wafConditionFields = []string{
	"http.request.header",
	"http.request.method",
	"http.request.uri_raw",
	"http.request.path",
	"http.request.json_param",
	"http.request.query_param",
	"http.request.cookie",
	"http.request.body",
	"client.ip.address",
	"client.ip.asn",
	"client.geo.country",
	"bot.advanced.score",
	"client.ja3",
	"client.ja4",
}

// Fields that require a field_key (collection fields)
var wafCollectionFields = map[string]bool{
	"http.request.header":      true,
	"http.request.json_param":  true,
	"http.request.query_param": true,
	"http.request.cookie":      true,
}

var wafConditionOperators = []string{
	"eq", "ne", "in", "not_in",
	"contains", "not_contains",
	"regex", "not_regex",
	"begins_with", "not_begins_with",
	"ends_with", "not_ends_with",
	"contains_word", "not_contains_word",
	"ip_match", "not_ip_match",
	"exists", "does_not_exist",
	"lt", "le", "gt", "ge",
}

var wafCustomRuleActions = []string{"block", "log", "allow", "bypass_managed", "ignore", "challenge", "interactive_challenge"}
var wafRateLimitActions = []string{"block", "log", "challenge", "interactive_challenge"}
var wafCheckpointModes = []string{"prevent", "disabled", "learn"}
var wafCheckpointConfidenceLevels = []string{"medium", "high", "critical"}
var wafCheckpointIPSPerformanceImpacts = []string{"low", "medium", "high"}
var wafCheckpointIPSSeverities = []string{"low", "medium", "high", "critical"}
var wafCheckpointIPSActions = []string{"block", "log"}
var wafIgnoreActionTypes = []string{"json_body_param", "indicators", "query_param", "header", "cookie", "urlencoded_form_field"}

var defaultLimitBodySize = false

var defaultSecurityEnabled = false

// defaultWafCheckpointValue is the backend default checkpoint config —
// defined once here and reused in both WafAttributes() and SecurityAttributes()
// (inside the waf static default), mirroring the cache_key pattern.
var defaultWafCheckpointValue = types.ObjectValueMust(
	WafCheckpointAttrTypes(),
	map[string]attr.Value{
		"web_attacks": types.ObjectValueMust(
			map[string]attr.Type{"mode": types.StringType, "confidence_level": types.StringType},
			map[string]attr.Value{"mode": types.StringValue("learn"), "confidence_level": types.StringValue("high")},
		),
		"ips": types.ObjectValueMust(
			map[string]attr.Type{
				"mode": types.StringType, "performance_impact": types.StringType,
				"severity": types.StringType, "high_confidence_action": types.StringType,
				"medium_confidence_action": types.StringType, "low_confidence_action": types.StringType,
			},
			map[string]attr.Value{
				"mode": types.StringValue("learn"), "performance_impact": types.StringValue("medium"),
				"severity": types.StringValue("medium"), "high_confidence_action": types.StringValue("block"),
				"medium_confidence_action": types.StringValue("block"), "low_confidence_action": types.StringValue("log"),
			},
		),
		"trusted_sources":     types.ListValueMust(types.StringType, []attr.Value{}),
		"minimal_num_sources": types.Int64Value(3),
	},
)

// defaultWafCheckpointWebAttacksValue / defaultWafCheckpointIpsValue mirror the
// values inside defaultWafCheckpointValue and are used as Default on their own
// sub-attributes so the framework never marks them (known after apply).
var defaultWafCheckpointWebAttacksValue = types.ObjectValueMust(
	map[string]attr.Type{"mode": types.StringType, "confidence_level": types.StringType},
	map[string]attr.Value{"mode": types.StringValue("learn"), "confidence_level": types.StringValue("high")},
)

var defaultWafCheckpointIpsValue = types.ObjectValueMust(
	map[string]attr.Type{
		"mode": types.StringType, "performance_impact": types.StringType,
		"severity": types.StringType, "high_confidence_action": types.StringType,
		"medium_confidence_action": types.StringType, "low_confidence_action": types.StringType,
	},
	map[string]attr.Value{
		"mode": types.StringValue("learn"), "performance_impact": types.StringValue("medium"),
		"severity": types.StringValue("medium"), "high_confidence_action": types.StringValue("block"),
		"medium_confidence_action": types.StringValue("block"), "low_confidence_action": types.StringValue("log"),
	},
)

// defaultWafValue is the full waf block default — used when the user omits the waf
// block entirely under security. It embeds defaultWafCheckpointValue so both vars
// stay in sync from a single source of truth.
var defaultWafValue = types.ObjectValueMust(
	WafAttrTypes(),
	map[string]attr.Value{
		"limit_body_size": types.BoolValue(defaultLimitBodySize),
		"checkpoint":      defaultWafCheckpointValue,
	},
)

// defaultBotManagementValue is the zero-value bot_management block — mirrors the
// Python config_field defaults: web_key="", secret_key="", thresholds=0.5.
var defaultBotManagementValue = types.ObjectValueMust(
	BotManagementAttrTypes(),
	map[string]attr.Value{
		"web_key":                types.StringUnknown(),
		"challenge_threshold":    types.Float64Value(0.5),
		"action_token_threshold": types.Float64Value(0.5),
	},
)

// defaultSecurityValue is the top-level security block default — used when the
// user omits the security block entirely, so defaults propagate without requiring
// the user to write the block explicitly.
var defaultSecurityValue = types.ObjectValueMust(
	SecurityAttrTypes(),
	map[string]attr.Value{
		"enabled":        types.BoolValue(defaultSecurityEnabled),
		"waf":            defaultWafValue,
		"custom_rules":   defaultEmptyCustomList,
		"rate_limit":     defaultEmptyRateLimitList,
		"bot_management": defaultBotManagementValue,
	},
)

// defaultEmptyCustomList / defaultEmptyRateLimitList are used as schema-level
// Default values so that omitting the block produces [] in state, not null.
var defaultEmptyCustomList = types.ListValueMust(
	types.ObjectType{AttrTypes: WafCustomRuleAttrTypes()},
	[]attr.Value{},
)

var defaultEmptyRateLimitList = types.ListValueMust(
	types.ObjectType{AttrTypes: WafRateLimitRuleAttrTypes()},
	[]attr.Value{},
)

// ---------------------------------------------------------------------------
// TF Model structs
// ---------------------------------------------------------------------------

// WafConditionModel is an alias for the shared ConditionModel.
// WAF-specific field/operator validation is applied in the schema only.
type WafConditionModel = ConditionModel

// WafConditionAndGroupModel is an alias for the shared ConditionAndGroupModel.
type WafConditionAndGroupModel = ConditionAndGroupModel

// WafConditionExpressionModel is an alias for the shared ConditionExpressionModel.
type WafConditionExpressionModel = ConditionExpressionModel

// WafIgnoreParamsModel for the "ignore" action params.
type WafIgnoreParamsModel struct {
	IgnoreType types.String `tfsdk:"ignore_type"`
	Value      types.String `tfsdk:"value"`
}

// WafCustomRuleModel represents a single WAF custom rule.
type WafCustomRuleModel struct {
	Name         types.String                 `tfsdk:"name"`
	Enabled      types.Bool                   `tfsdk:"enabled"`
	Action       types.String                 `tfsdk:"action"`
	Condition    *WafConditionExpressionModel `tfsdk:"condition"`
	IgnoreParams *WafIgnoreParamsModel        `tfsdk:"ignore_params"`
}

// WafRateLimitRuleModel represents a single WAF rate-limit rule.
type WafRateLimitRuleModel struct {
	Name                 types.String                 `tfsdk:"name"`
	Enabled              types.Bool                   `tfsdk:"enabled"`
	Action               types.String                 `tfsdk:"action"`
	NumOfRequests        types.Int64                  `tfsdk:"num_of_requests"`
	TimeWindowSeconds    types.Int64                  `tfsdk:"time_window_seconds"`
	BlockDurationSeconds types.Int64                  `tfsdk:"block_duration_seconds"`
	Condition            *WafConditionExpressionModel `tfsdk:"condition"`
}

// WafCheckpointWebAttacksModel maps to WAFCheckpointWebAttacks.
type WafCheckpointWebAttacksModel struct {
	Mode            types.String `tfsdk:"mode"`
	ConfidenceLevel types.String `tfsdk:"confidence_level"`
}

// WafCheckpointIPSModel maps to WAFCheckpointIPS.
type WafCheckpointIPSModel struct {
	Mode                   types.String `tfsdk:"mode"`
	PerformanceImpact      types.String `tfsdk:"performance_impact"`
	Severity               types.String `tfsdk:"severity"`
	HighConfidenceAction   types.String `tfsdk:"high_confidence_action"`
	MediumConfidenceAction types.String `tfsdk:"medium_confidence_action"`
	LowConfidenceAction    types.String `tfsdk:"low_confidence_action"`
}

// WafCheckpointModel maps to WAFCheckpointConfig.
type WafCheckpointModel struct {
	WebAttacks     *WafCheckpointWebAttacksModel `tfsdk:"web_attacks"`
	IPS            *WafCheckpointIPSModel        `tfsdk:"ips"`
	TrustedSources types.List                    `tfsdk:"trusted_sources"` // list(string)
	NumSources     types.Int64                   `tfsdk:"minimal_num_sources"`
}

// WafModel is the WAF config block embedded in SecurityModel.
type WafModel struct {
	LimitBodySize types.Bool          `tfsdk:"limit_body_size"`
	Checkpoint    *WafCheckpointModel `tfsdk:"checkpoint"`
}

// BotManagementModel maps to the Python BotManagement config object.
type BotManagementModel struct {
	WebKey               types.String  `tfsdk:"web_key"`
	ChallengeThreshold   types.Float64 `tfsdk:"challenge_threshold"`
	ActionTokenThreshold types.Float64 `tfsdk:"action_token_threshold"`
}

// SecurityModel is the top-level security config block embedded in ServiceConfigModel.
type SecurityModel struct {
	Enabled       types.Bool              `tfsdk:"enabled"`
	Waf           *WafModel               `tfsdk:"waf"`
	CustomRules   []WafCustomRuleModel    `tfsdk:"custom_rules"`
	RateLimit     []WafRateLimitRuleModel `tfsdk:"rate_limit"`
	BotManagement *BotManagementModel     `tfsdk:"bot_management"`
}

// ---------------------------------------------------------------------------
// Schema attribute builders
// ---------------------------------------------------------------------------

func wafConditionAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"field": schema.StringAttribute{
			MarkdownDescription: "Request field to evaluate.\n" +
				"  - Plain fields (no `field_key` needed):\n" +
				"    - `http.request.method` (HTTP verb)\n" +
				"    - `http.request.path` (URL path, e.g. `/api/users`)\n" +
				"    - `http.request.uri_raw` (full raw URI including query string)\n" +
				"    - `http.request.body` (request body text)\n" +
				"    - `client.ip.address` (client IPv4/IPv6 — use `ip_match`/`not_ip_match`)\n" +
				"    - `client.ip.asn` (autonomous system number)\n" +
				"    - `client.geo.country` (ISO 3166-1 alpha-2 country code)\n" +
				"    - `bot.advanced.score` (IO River bot score 0-100 — supports `lt`/`le`/`gt`/`ge`)\n" +
				"    - `client.ja3` (JA3 TLS fingerprint hash)\n" +
				"    - `client.ja4` (JA4 TLS fingerprint hash)\n" +
				"  - Collection fields (require `field_key`):\n" +
				"    - `http.request.header`\n" +
				"    - `http.request.cookie`\n" +
				"    - `http.request.query_param`\n" +
				"    - `http.request.json_param` \n  -",
			Required: true,
			Validators: []validator.String{
				stringvalidator.OneOf(wafConditionFields...),
			},
		},
		"operator": schema.StringAttribute{
			MarkdownDescription: "Match operator to apply.\n" +
				"  - **String:**\n " +
				"    - `eq` (exact match),\n" +
				"    - `ne` (not equal),\n" +
				"    - `begins_with` / `not_begins_with`,\n" +
				"    - `ends_with` / `not_ends_with`,\n" +
				"    - `contains` / `not_contains` (substring),\n" +
				"    - `contains_word` / `not_contains_word` (word-boundary match),\n" +
				"    - `regex` / `not_regex` (Python-compatible regular expression).\n" +
				"  - **List:**\n" +
				"    - `in` / `not_in` (value is in the supplied list).\n" +
				"  - **IP/CIDR:**\n" +
				"    - `ip_match` / `not_ip_match` (use with `client.ip.address`; supply one or more CIDRs/IPs in `value`).\n" +
				"  - **Existence** (set `value = []`):\n" +
				"    - `exists` / `does_not_exist` (field/header/cookie/param is present or absent).\n" +
				"  - **Numeric** (use with `bot.advanced.score`):\n" +
				"    - `lt`, `le`, `gt`, `ge`. \n  -",
			Required: true,
			Validators: []validator.String{
				stringvalidator.OneOf(wafConditionOperators...),
			},
		},
		"values": conditionValuesAttr(),
		"value":  conditionValueAttr(),
		"field_key": schema.StringAttribute{
			MarkdownDescription: "Name of the specific header, cookie, query parameter, or JSON body field to inspect.\n" +
				"  - **Required** when `field` is one of `http.request.header`, `http.request.cookie`, `http.request.query_param`, or `http.request.json_param`.\n" +
				"  - For example: `field_key = \"User-Agent\"` when `field = \"http.request.header\"`. \n  -",
			Optional: true,
		},
	}
}

func wafConditionExpressionAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"or": schema.ListNestedAttribute{
			MarkdownDescription: "OR-of-ANDs match expression.\n" +
				"  - The rule matches when **at least one** OR group matches.\n" +
				"  - Each OR group contains one or more `and` conditions that must **all** match simultaneously. \n  -",
			Required: true,
			NestedObject: schema.NestedAttributeObject{
				Attributes: map[string]schema.Attribute{
					"and": schema.ListNestedAttribute{
						MarkdownDescription: "List of conditions that must ALL match for this OR group to be satisfied.",
						Required:            true,
						NestedObject: schema.NestedAttributeObject{
							Attributes: wafConditionAttributes(),
						},
					},
				},
			},
		},
	}
}

func WafAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"limit_body_size": schema.BoolAttribute{
			MarkdownDescription: "Limit the request body size",
			Optional:            true,
			Computed:            true,
			Default:             booldefault.StaticBool(defaultLimitBodySize),
		},
		"checkpoint": schema.SingleNestedAttribute{
			MarkdownDescription: "Checkpoint WAF partial configuration. Some settings (e.g. learning mode results) are managed via the UI; the fields here control the active enforcement configuration.",
			Optional:            true,
			Computed:            true,
			Default:             objectdefault.StaticValue(defaultWafCheckpointValue),
			Attributes: map[string]schema.Attribute{
				"web_attacks": schema.SingleNestedAttribute{
					MarkdownDescription: "Web attacks (ML-based engine) detection settings",
					Optional:            true,
					Computed:            true,
					Default:             objectdefault.StaticValue(defaultWafCheckpointWebAttacksValue),
					Attributes: map[string]schema.Attribute{
						"mode": schema.StringAttribute{
							MarkdownDescription: "operation mode. Valid values: `" + strings.Join(wafCheckpointModes, "`, `") + "`",
							Optional:            true,
							Computed:            true,
							Default:             stringdefault.StaticString("learn"),
							Validators: []validator.String{
								stringvalidator.OneOf(wafCheckpointModes...),
							},
						},
						"confidence_level": schema.StringAttribute{
							MarkdownDescription: "Confidence threshold level. Valid values: `" + strings.Join(wafCheckpointConfidenceLevels, "`, `") + "`",
							Optional:            true,
							Computed:            true,
							Default:             stringdefault.StaticString("high"),
							Validators: []validator.String{
								stringvalidator.OneOf(wafCheckpointConfidenceLevels...),
							},
						},
					},
				},
				"ips": schema.SingleNestedAttribute{
					MarkdownDescription: "Intrusion Prevention System (IPS - rules-based engine) settings",
					Optional:            true,
					Computed:            true,
					Default:             objectdefault.StaticValue(defaultWafCheckpointIpsValue),
					Attributes: map[string]schema.Attribute{
						"mode": schema.StringAttribute{
							MarkdownDescription: "IPS mode. Valid values: `" + strings.Join(wafCheckpointModes, "`, `") + "`",
							Optional:            true,
							Computed:            true,
							Default:             stringdefault.StaticString("learn"),
							Validators: []validator.String{
								stringvalidator.OneOf(wafCheckpointModes...),
							},
						},
						"performance_impact": schema.StringAttribute{
							MarkdownDescription: "Include rules with performance impact at most: `" + strings.Join(wafCheckpointIPSPerformanceImpacts, "`, `") + "`",
							Optional:            true,
							Computed:            true,
							Default:             stringdefault.StaticString("medium"),
							Validators: []validator.String{
								stringvalidator.OneOf(wafCheckpointIPSPerformanceImpacts...),
							},
						},
						"severity": schema.StringAttribute{
							MarkdownDescription: "Include rules with severity at least: `" + strings.Join(wafCheckpointIPSSeverities, "`, `") + "`",
							Optional:            true,
							Computed:            true,
							Default:             stringdefault.StaticString("medium"),
							Validators: []validator.String{
								stringvalidator.OneOf(wafCheckpointIPSSeverities...),
							},
						},
						"high_confidence_action": schema.StringAttribute{
							MarkdownDescription: "Action for high-confidence matches. Valid values: `" + strings.Join(wafCheckpointIPSActions, "`, `") + "`",
							Optional:            true,
							Computed:            true,
							Default:             stringdefault.StaticString("block"),
							Validators: []validator.String{
								stringvalidator.OneOf(wafCheckpointIPSActions...),
							},
						},
						"medium_confidence_action": schema.StringAttribute{
							MarkdownDescription: "Action for medium-confidence matches. Valid values: `" + strings.Join(wafCheckpointIPSActions, "`, `") + "`",
							Optional:            true,
							Computed:            true,
							Default:             stringdefault.StaticString("block"),
							Validators: []validator.String{
								stringvalidator.OneOf(wafCheckpointIPSActions...),
							},
						},
						"low_confidence_action": schema.StringAttribute{
							MarkdownDescription: "Action for low-confidence matches. Valid values: `" + strings.Join(wafCheckpointIPSActions, "`, `") + "`",
							Optional:            true,
							Computed:            true,
							Default:             stringdefault.StaticString("log"),
							Validators: []validator.String{
								stringvalidator.OneOf(wafCheckpointIPSActions...),
							},
						},
					},
				},
				"trusted_sources": schema.ListAttribute{
					MarkdownDescription: "List of trusted source IP addresses used for learning mode",
					Optional:            true,
					Computed:            true, // backend always returns this field (empty list by default)
					PlanModifiers:       []planmodifier.List{ListNullClearsStateModifier()},
					ElementType:         types.StringType,
				},
				"minimal_num_sources": schema.Int64Attribute{
					MarkdownDescription: "Minimum number of trusted sources required for learning",
					Optional:            true,
					Computed:            true, // backend default: 3
					Default:             int64default.StaticInt64(3),
					PlanModifiers: []planmodifier.Int64{
						int64planmodifier.UseStateForUnknown(),
					},
				},
			},
		},
	}
}

// customRuleAttributes returns the schema for the custom WAF rules list (used under security).
func customRuleAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"name": schema.StringAttribute{
			MarkdownDescription: "Rule name (unique within the service)",
			Required:            true,
		},
		"enabled": schema.BoolAttribute{
			MarkdownDescription: "Whether this rule is active",
			Optional:            true,
			Computed:            true,
			Default:             booldefault.StaticBool(true),
		},
		"condition": schema.SingleNestedAttribute{
			MarkdownDescription: "Match condition (OR-of-ANDs expression)",
			Required:            true,
			Attributes:          wafConditionExpressionAttributes(),
		},
		"action": schema.StringAttribute{
			MarkdownDescription: "Action when rule matches. Valid values: `" + strings.Join(wafCustomRuleActions, "`, `") + "`",
			Required:            true,
			Validators: []validator.String{
				stringvalidator.OneOf(wafCustomRuleActions...),
			},
		},
		"ignore_params": schema.SingleNestedAttribute{
			MarkdownDescription: "Parameters for the 'ignore' action (required when action = ignore)",
			Optional:            true,
			Attributes: map[string]schema.Attribute{
				"ignore_type": schema.StringAttribute{
					MarkdownDescription: "Type of ignore. Valid values: `" + strings.Join(wafIgnoreActionTypes, "`, `") + "`",
					Required:            true,
					Validators: []validator.String{
						stringvalidator.OneOf(wafIgnoreActionTypes...),
					},
				},
				"value": schema.StringAttribute{
					MarkdownDescription: "Value associated with the ignore type",
					Required:            true,
				},
			},
		},
	}
}

// rateLimitAttributes returns the shared schema for a rate-limit rule (used under security).
func rateLimitAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"name": schema.StringAttribute{
			MarkdownDescription: "Rule name (unique within the service)",
			Required:            true,
		},
		"enabled": schema.BoolAttribute{
			MarkdownDescription: "Whether this rule is active",
			Optional:            true,
			Computed:            true, // backend default: true
			Default:             booldefault.StaticBool(true),
		},
		"condition": schema.SingleNestedAttribute{
			MarkdownDescription: "Match condition (OR-of-ANDs expression)",
			Required:            true,
			Attributes:          wafConditionExpressionAttributes(),
		},
		"action": schema.StringAttribute{
			MarkdownDescription: "Action when rule matches. Valid values: `" + strings.Join(wafRateLimitActions, "`, `") + "`",
			Optional:            true,
			Computed:            true,
			Default:             stringdefault.StaticString("block"),
			Validators: []validator.String{
				stringvalidator.OneOf(wafRateLimitActions...),
			},
		},
		"num_of_requests": schema.Int64Attribute{
			MarkdownDescription: "Number of requests that trigger the rate limit",
			Required:            true,
			Validators: []validator.Int64{
				int64validator.AtLeast(1),
			},
		},
		"time_window_seconds": schema.Int64Attribute{
			MarkdownDescription: "Time window in seconds for counting requests",
			Required:            true,
			Validators: []validator.Int64{
				int64validator.AtLeast(1),
			},
		},
		"block_duration_seconds": schema.Int64Attribute{
			MarkdownDescription: "Duration in seconds to block the client after rate limit is exceeded",
			Required:            true,
			Validators: []validator.Int64{
				int64validator.AtLeast(1),
			},
		},
	}
}

// ---------------------------------------------------------------------------
// ModelToMap  (TF model → API map sent to backend)
// ---------------------------------------------------------------------------

func (w *WafModel) ModelToMap(ctx context.Context) map[string]interface{} {
	if w == nil {
		return nil
	}

	wafMap := make(map[string]interface{})

	// Always send limit_body_size; default to false when user omits it.
	if !w.LimitBodySize.IsNull() && !w.LimitBodySize.IsUnknown() {
		wafMap["limit_body_size"] = w.LimitBodySize.ValueBool()
	}

	// Checkpoint
	if w.Checkpoint != nil {
		checkpointMap := map[string]interface{}{}

		if w.Checkpoint.WebAttacks != nil {
			waMap := map[string]interface{}{}
			if !w.Checkpoint.WebAttacks.Mode.IsNull() && !w.Checkpoint.WebAttacks.Mode.IsUnknown() {
				waMap["mode"] = w.Checkpoint.WebAttacks.Mode.ValueString()
			}
			if !w.Checkpoint.WebAttacks.ConfidenceLevel.IsNull() && !w.Checkpoint.WebAttacks.ConfidenceLevel.IsUnknown() {
				waMap["confidence_level"] = w.Checkpoint.WebAttacks.ConfidenceLevel.ValueString()
			}
			checkpointMap["web_attacks"] = waMap
		}

		if w.Checkpoint.IPS != nil {
			ipsMap := map[string]interface{}{}
			if !w.Checkpoint.IPS.Mode.IsNull() && !w.Checkpoint.IPS.Mode.IsUnknown() {
				ipsMap["mode"] = w.Checkpoint.IPS.Mode.ValueString()
			}
			if !w.Checkpoint.IPS.PerformanceImpact.IsNull() && !w.Checkpoint.IPS.PerformanceImpact.IsUnknown() {
				ipsMap["performance_impact"] = w.Checkpoint.IPS.PerformanceImpact.ValueString()
			}
			if !w.Checkpoint.IPS.Severity.IsNull() && !w.Checkpoint.IPS.Severity.IsUnknown() {
				ipsMap["severity"] = w.Checkpoint.IPS.Severity.ValueString()
			}
			if !w.Checkpoint.IPS.HighConfidenceAction.IsNull() && !w.Checkpoint.IPS.HighConfidenceAction.IsUnknown() {
				ipsMap["high_confidence_action"] = w.Checkpoint.IPS.HighConfidenceAction.ValueString()
			}
			if !w.Checkpoint.IPS.MediumConfidenceAction.IsNull() && !w.Checkpoint.IPS.MediumConfidenceAction.IsUnknown() {
				ipsMap["medium_confidence_action"] = w.Checkpoint.IPS.MediumConfidenceAction.ValueString()
			}
			if !w.Checkpoint.IPS.LowConfidenceAction.IsNull() && !w.Checkpoint.IPS.LowConfidenceAction.IsUnknown() {
				ipsMap["low_confidence_action"] = w.Checkpoint.IPS.LowConfidenceAction.ValueString()
			}
			checkpointMap["ips"] = ipsMap
		}

		if !w.Checkpoint.TrustedSources.IsNull() && !w.Checkpoint.TrustedSources.IsUnknown() {
			var sources []string
			_ = w.Checkpoint.TrustedSources.ElementsAs(ctx, &sources, false)
			checkpointMap["trusted_sources"] = sources
		}
		if !w.Checkpoint.NumSources.IsNull() && !w.Checkpoint.NumSources.IsUnknown() {
			checkpointMap["num_sources"] = int(w.Checkpoint.NumSources.ValueInt64())
		}

		wafMap["checkpoint"] = checkpointMap
	}

	tflog.Debug(ctx, fmt.Sprintf("[WafModel.ModelToMap] WAF map: %+v", wafMap))
	return wafMap
}

// wafConditionExpressionToMap serialises the OR-of-ANDs expression.
// client.ip.address is a WAF-only field that the backend expects as a comma-separated
// string rather than a list; all other fields use the default list form.
func wafConditionExpressionToMap(ctx context.Context, expr *WafConditionExpressionModel) map[string]interface{} {
	return ConditionExpressionToMap(ctx, expr, func(field string, vals []string) interface{} {
		if field == "client.ip.address" {
			return strings.Join(vals, ",")
		}
		return vals
	})
}

// toFloat64 coerces int, float64 or json.Number to float64 for safe map reads.
func toFloat64(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case int32:
		return float64(n), true
	}
	return 0, false
}

// ---------------------------------------------------------------------------
// WafMapToModel  (API map → TF model)
// ---------------------------------------------------------------------------

func WafMapToModel(ctx context.Context, wafRaw interface{}) *WafModel {
	return wafMapToModelWithCtx(ctx, wafRaw, nil)
}

func wafMapToModelWithCtx(ctx context.Context, wafRaw interface{}, transformCtx *ServiceTransformContext) *WafModel {
	if wafRaw == nil {
		return nil
	}
	wafMap, ok := wafRaw.(map[string]interface{})
	if !ok {
		tflog.Warn(ctx, fmt.Sprintf("[WafMapToModel] unexpected waf type: %T", wafRaw))
		return nil
	}

	model := &WafModel{
		LimitBodySize: types.BoolNull(),
	}

	if v, ok := wafMap["limit_body_size"].(bool); ok {
		model.LimitBodySize = types.BoolValue(v)
	}
	tflog.Debug(ctx, fmt.Sprintf("[wafMapToModel] limit_body_size_raw=%v limit_body_size_model=%v",
		wafMap["limit_body_size"], model.LimitBodySize))

	// Always populate checkpoint from the backend response.
	// The SecurityWafDefaultsModifier ensures the plan matches these values at plan time.
	if checkpointRaw, ok := wafMap["checkpoint"].(map[string]interface{}); ok {
		cp := &WafCheckpointModel{
			TrustedSources: types.ListNull(types.StringType),
			NumSources:     types.Int64Null(),
		}

		if waRaw, ok := checkpointRaw["web_attacks"].(map[string]interface{}); ok {
			wa := &WafCheckpointWebAttacksModel{
				Mode:            types.StringNull(),
				ConfidenceLevel: types.StringNull(),
			}
			if v, ok := waRaw["mode"].(string); ok {
				wa.Mode = types.StringValue(v)
			}
			if v, ok := waRaw["confidence_level"].(string); ok {
				wa.ConfidenceLevel = types.StringValue(v)
			}
			cp.WebAttacks = wa
		}

		if ipsRaw, ok := checkpointRaw["ips"].(map[string]interface{}); ok {
			ips := &WafCheckpointIPSModel{
				Mode:                   types.StringNull(),
				PerformanceImpact:      types.StringNull(),
				Severity:               types.StringNull(),
				HighConfidenceAction:   types.StringNull(),
				MediumConfidenceAction: types.StringNull(),
				LowConfidenceAction:    types.StringNull(),
			}
			if v, ok := ipsRaw["mode"].(string); ok {
				ips.Mode = types.StringValue(v)
			}
			if v, ok := ipsRaw["performance_impact"].(string); ok {
				ips.PerformanceImpact = types.StringValue(v)
			}
			if v, ok := ipsRaw["severity"].(string); ok {
				ips.Severity = types.StringValue(v)
			}
			if v, ok := ipsRaw["high_confidence_action"].(string); ok {
				ips.HighConfidenceAction = types.StringValue(v)
			}
			if v, ok := ipsRaw["medium_confidence_action"].(string); ok {
				ips.MediumConfidenceAction = types.StringValue(v)
			}
			if v, ok := ipsRaw["low_confidence_action"].(string); ok {
				ips.LowConfidenceAction = types.StringValue(v)
			}
			cp.IPS = ips
		}

		if sourcesRaw, ok := checkpointRaw["trusted_sources"].([]interface{}); ok {
			sources := make([]string, 0, len(sourcesRaw))
			for _, s := range sourcesRaw {
				if sv, ok := s.(string); ok {
					sources = append(sources, sv)
				}
			}
			listVal, _ := types.ListValueFrom(ctx, types.StringType, sources)
			cp.TrustedSources = listVal
		}
		// else: leave cp.TrustedSources as types.ListNull(types.StringType)
		if v, ok := checkpointRaw["num_sources"]; ok {
			if fv, ok := toFloat64(v); ok {
				cp.NumSources = types.Int64Value(int64(fv))
			}
		}

		model.Checkpoint = cp
	} // end checkpointRaw

	return model
}

// wafConditionExpressionFromMap deserialises the OR-of-ANDs API map for WAF conditions.
// Delegates to the shared ConditionExpressionFromMap with WafValueDeserializer.
func wafConditionExpressionFromMap(ctx context.Context, raw map[string]interface{}, prior *WafConditionExpressionModel) *WafConditionExpressionModel {
	return ConditionExpressionFromMap(ctx, raw, prior, WafValueDeserializer)
}

// strOrEmpty is a small helper to safely read a string from a map.
func strOrEmpty(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

// ---------------------------------------------------------------------------
// Validation helpers (called from service_resource or plan modifiers)
// ---------------------------------------------------------------------------

// ValidateWafModel validates cross-field rules that cannot be expressed
// purely with schema validators (e.g. field_key required for collection fields,
// ignore_params required when action=ignore).
func ValidateWafModel(ctx context.Context, waf *WafModel) []string {
	if waf == nil {
		return nil
	}
	return nil
}

// ValidateCustomRules validates cross-field rules on custom WAF rules.
// Called from ValidateSecurityModel since custom now lives under security.
func ValidateCustomRules(rules []WafCustomRuleModel) []string {
	var errs []string

	// Duplicate rule names (backend rejects duplicates).
	names := make([]string, len(rules))
	for i, r := range rules {
		names[i] = r.Name.ValueString()
	}
	errs = append(errs, validateNoDuplicateNames(names, "security.custom_rules")...)

	for i, rule := range rules {
		prefix := fmt.Sprintf("security.custom_rules[%d] (%s)", i, rule.Name.ValueString())

		// ignore_params required when action = "ignore"
		if rule.Action.ValueString() == "ignore" && rule.IgnoreParams == nil {
			errs = append(errs, fmt.Sprintf("%s: action 'ignore' requires ignore_params to be set", prefix))
		}
		// ignore_params must NOT be set for any other action
		if rule.Action.ValueString() != "ignore" && rule.IgnoreParams != nil {
			errs = append(errs, fmt.Sprintf("%s: ignore_params is only valid when action = 'ignore', got '%s'", prefix, rule.Action.ValueString()))
		}

		if rule.Condition != nil {
			for j, andGroup := range rule.Condition.Or {
				for k, cond := range andGroup.And {
					loc := fmt.Sprintf("%s.condition.or[%d].and[%d]", prefix, j, k)
					errs = append(errs, validateCondition(loc, cond)...)
				}
			}
		}
	}

	return errs
}

// ValidateRateLimitRules validates rate-limit rule conditions.
// Called from ValidateSecurityModel since rate_limit now lives under security.
func ValidateRateLimitRules(rules []WafRateLimitRuleModel) []string {
	var errs []string

	// Duplicate rule names (backend rejects duplicates).
	names := make([]string, len(rules))
	for i, r := range rules {
		names[i] = r.Name.ValueString()
	}
	errs = append(errs, validateNoDuplicateNames(names, "security.rate_limit")...)

	for i, rule := range rules {
		prefix := fmt.Sprintf("security.rate_limit[%d] (%s)", i, rule.Name.ValueString())
		if rule.Condition != nil {
			for j, andGroup := range rule.Condition.Or {
				for k, cond := range andGroup.And {
					loc := fmt.Sprintf("%s.condition.or[%d].and[%d]", prefix, j, k)
					errs = append(errs, validateCondition(loc, cond)...)
				}
			}
		}
	}
	return errs
}

// ---------------------------------------------------------------------------
// validateCondition — mirrors backend validate_waf_condition in config_validator.py
//
// Rules grouped to match exactly what the backend enforces:
//
//  A. field_key:
//     - collection fields (header, cookie, query_param, json_param) → required
//     - all other fields → must NOT be set
//
//  B. uri_raw operator restrictions (backend raises ValidationError):
//     - ip_match, not_ip_match, exists, does_not_exist, lt, le, gt, ge → forbidden
//     - eq, ne         → value must be a valid full URL (http:// or https://)
//     - in, not_in     → each value must be a valid full URL
//     - begins_with, not_begins_with → value must be a valid URL prefix
//     - regex, not_regex             → value must compile as a regexp
//     - contains, not_contains, ends_with, not_ends_with,
//       contains_word, not_contains_word → free-form, no value restriction
//
//  C. path operator restrictions:
//     - eq, ne          → every value must start with '/'
//     - regex, not_regex → every value must compile as a regexp
//
//  D. client.ip.address:
//     - every value must be a valid IP address or CIDR block
//
//  E. General value-presence:
//     - exists / does_not_exist → value must be empty []
//     - all other operators     → value must have at least one element
// ---------------------------------------------------------------------------

// wafURIRawForbiddenOps are operators the backend explicitly rejects for uri_raw.
var wafURIRawForbiddenOps = map[string]bool{
	"ip_match": true, "not_ip_match": true,
	"exists": true, "does_not_exist": true,
	"lt": true, "le": true, "gt": true, "ge": true,
}

// wafURIRawURLRequiredOps are operators that require a valid full URL for uri_raw.
var wafURIRawURLRequiredOps = map[string]bool{"eq": true, "ne": true}

// wafURIRawURLListOps are operators that require every value to be a valid full URL.
var wafURIRawURLListOps = map[string]bool{"in": true, "not_in": true}

// wafURIRawURLPrefixOps require the value to look like a URL prefix.
var wafURIRawURLPrefixOps = map[string]bool{"begins_with": true, "not_begins_with": true}

// wafURIRawRegexOps require the value to compile as a regexp.
var wafURIRawRegexOps = map[string]bool{"regex": true, "not_regex": true}

// wafPathMustStartSlashOps require each path value to start with '/'.
var wafPathMustStartSlashOps = map[string]bool{"eq": true, "ne": true}

// wafPathRegexOps require path values to compile as a regexp.
var wafPathRegexOps = map[string]bool{"regex": true, "not_regex": true}

// wafExistenceOps are operators that must have an empty value list.
var wafExistenceOps = map[string]bool{"exists": true, "does_not_exist": true}

// isValidURLPrefix returns true if s looks like a valid URL or URL prefix
// (mirrors the backend's begins_with check: "https://".startswith(value) || is_valid_url(value)).
func isValidURLPrefix(s string) bool {
	if strings.HasPrefix("https://", s) || strings.HasPrefix("http://", s) {
		return true
	}
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}

// isValidFullURL returns true when s has an http/https scheme and a host.
func isValidFullURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}

// validateCondition checks all cross-field invariants for a single condition,
// matching the backend validate_waf_condition logic exactly.
func validateCondition(loc string, cond WafConditionModel) []string {
	var errs []string
	field := cond.Field.ValueString()
	op := cond.Operator.ValueString()

	// Collect the string values once for reuse below.
	// Coalesce: `value` (single-string shorthand, read from config) or `values` (set form).
	var vals []string
	if !cond.Value.IsNull() && !cond.Value.IsUnknown() {
		vals = []string{cond.Value.ValueString()}
	} else if !cond.Values.IsNull() && !cond.Values.IsUnknown() {
		_ = cond.Values.ElementsAs(context.Background(), &vals, false)
	}

	// ── A. field_key ─────────────────────────────────────────────────────────
	if wafCollectionFields[field] {
		if cond.FieldKey.IsNull() || cond.FieldKey.ValueString() == "" {
			errs = append(errs, fmt.Sprintf("%s: field '%s' requires field_key to be set", loc, field))
		}
	} else {
		if !cond.FieldKey.IsNull() && cond.FieldKey.ValueString() != "" {
			errs = append(errs, fmt.Sprintf(
				"%s: field_key must not be set for non-collection field '%s'", loc, field))
		}
	}

	// ── E. value-presence (checked first so B/C can assume vals is populated) ─
	if wafExistenceOps[op] {
		if len(vals) > 0 {
			errs = append(errs, fmt.Sprintf(
				"%s: operator '%s' requires an empty value list []", loc, op))
		}
	} else {
		if len(vals) == 0 {
			errs = append(errs, fmt.Sprintf(
				"%s: operator '%s' requires at least one value", loc, op))
		}
	}

	// ── B. uri_raw operator restrictions ─────────────────────────────────────
	if field == "http.request.uri_raw" {
		switch {
		case wafURIRawForbiddenOps[op]:
			errs = append(errs, fmt.Sprintf(
				"%s: operator '%s' is not supported for field 'http.request.uri_raw'", loc, op))

		case wafURIRawURLRequiredOps[op]:
			for _, v := range vals {
				if !isValidFullURL(v) {
					errs = append(errs, fmt.Sprintf(
						"%s: uri_raw + '%s' requires a full URL (http:// or https://), got %q", loc, op, v))
				}
			}

		case wafURIRawURLListOps[op]:
			for _, v := range vals {
				if !isValidFullURL(v) {
					errs = append(errs, fmt.Sprintf(
						"%s: uri_raw + '%s' requires every value to be a full URL, got %q", loc, op, v))
				}
			}

		case wafURIRawURLPrefixOps[op]:
			for _, v := range vals {
				if !isValidURLPrefix(v) {
					errs = append(errs, fmt.Sprintf(
						"%s: uri_raw + '%s' requires a valid URL prefix, got %q", loc, op, v))
				}
			}

		case wafURIRawRegexOps[op]:
			for _, v := range vals {
				if _, err := regexp.Compile(v); err != nil {
					errs = append(errs, fmt.Sprintf(
						"%s: uri_raw + '%s' value %q is not a valid regexp: %s", loc, op, v, err))
				}
			}
		}
	}

	// ── C. path operator restrictions ─────────────────────────────────────────
	if field == "http.request.path" {
		switch {
		case wafPathMustStartSlashOps[op]:
			for _, v := range vals {
				if !strings.HasPrefix(v, "/") {
					errs = append(errs, fmt.Sprintf(
						"%s: path + '%s' value %q must start with '/'", loc, op, v))
				}
			}
		case wafPathRegexOps[op]:
			for _, v := range vals {
				if _, err := regexp.Compile(v); err != nil {
					errs = append(errs, fmt.Sprintf(
						"%s: path + '%s' value %q is not a valid regexp: %s", loc, op, v, err))
				}
			}
		}
	}

	// ── D. client.ip.address IP/CIDR validation ───────────────────────────────
	if field == "client.ip.address" {
		for _, v := range vals {
			// backend splits on comma; we receive a list so each element is one entry
			if net.ParseIP(v) == nil {
				if _, _, err := net.ParseCIDR(v); err != nil {
					errs = append(errs, fmt.Sprintf(
						"%s: client.ip.address value %q is not a valid IP address or CIDR", loc, v))
				}
			}
		}
	}

	return errs
}

// validateNoDuplicateNames returns an error for each duplicate name in the slice.
func validateNoDuplicateNames(names []string, prefix string) []string {
	seen := make(map[string]int)
	for _, n := range names {
		seen[n]++
	}
	var errs []string
	for n, count := range seen {
		if count > 1 {
			errs = append(errs, fmt.Sprintf("%s: duplicate rule name %q", prefix, n))
		}
	}
	return errs
}

// ValidateSecurityModel is the single entry-point for security block validation.
// It delegates to ValidateWafModel, ValidateCustomRules (custom rules at security level)
// and ValidateRateLimitRules (rate_limit rules at security level).
func ValidateSecurityModel(ctx context.Context, sec *SecurityModel) []string {
	if sec == nil {
		return nil
	}
	var errs []string
	errs = append(errs, ValidateWafModel(ctx, sec.Waf)...)
	errs = append(errs, ValidateCustomRules(sec.CustomRules)...)
	errs = append(errs, ValidateRateLimitRules(sec.RateLimit)...)
	return errs
}

// ---------------------------------------------------------------------------
// SecurityModel helpers
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// Attr-type maps (needed for types.ObjectNull / types.ObjectValueFrom)
// ---------------------------------------------------------------------------

func wafIgnoreParamsAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"ignore_type": types.StringType,
		"value":       types.StringType,
	}
}

func WafCustomRuleAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"name":          types.StringType,
		"enabled":       types.BoolType,
		"action":        types.StringType,
		"condition":     types.ObjectType{AttrTypes: ConditionExpressionAttrTypes()},
		"ignore_params": types.ObjectType{AttrTypes: wafIgnoreParamsAttrTypes()},
	}
}

func WafRateLimitRuleAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"name":                   types.StringType,
		"enabled":                types.BoolType,
		"action":                 types.StringType,
		"num_of_requests":        types.Int64Type,
		"time_window_seconds":    types.Int64Type,
		"block_duration_seconds": types.Int64Type,
		"condition":              types.ObjectType{AttrTypes: ConditionExpressionAttrTypes()},
	}
}

func WafCheckpointAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"web_attacks": types.ObjectType{AttrTypes: map[string]attr.Type{
			"mode":             types.StringType,
			"confidence_level": types.StringType,
		}},
		"ips": types.ObjectType{AttrTypes: map[string]attr.Type{
			"mode":                     types.StringType,
			"performance_impact":       types.StringType,
			"severity":                 types.StringType,
			"high_confidence_action":   types.StringType,
			"medium_confidence_action": types.StringType,
			"low_confidence_action":    types.StringType,
		}},
		"trusted_sources":     types.ListType{ElemType: types.StringType},
		"minimal_num_sources": types.Int64Type,
	}
}

func WafAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"limit_body_size": types.BoolType,
		"checkpoint":      types.ObjectType{AttrTypes: WafCheckpointAttrTypes()},
	}
}

func BotManagementAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"web_key":                types.StringType,
		"challenge_threshold":    types.Float64Type,
		"action_token_threshold": types.Float64Type,
	}
}

func SecurityAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"enabled":        types.BoolType,
		"waf":            types.ObjectType{AttrTypes: WafAttrTypes()},
		"custom_rules":   types.ListType{ElemType: types.ObjectType{AttrTypes: WafCustomRuleAttrTypes()}},
		"rate_limit":     types.ListType{ElemType: types.ObjectType{AttrTypes: WafRateLimitRuleAttrTypes()}},
		"bot_management": types.ObjectType{AttrTypes: BotManagementAttrTypes()},
	}
}

// BotManagementAttributes returns the schema attributes for the bot_management block.
func BotManagementAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"web_key": schema.StringAttribute{
			MarkdownDescription: "reCAPTCHA Site Key. Add this key to your front-end code to enable action-level bot mitigation.\n  - This key is issued by IORiver and is required for bot management to function.\n  - If you do not have a key, contact IORiver support to obtain one.\n  -",
			Computed:            true,
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.UseStateForUnknown(),
			},
		},
		"challenge_threshold": schema.Float64Attribute{
			MarkdownDescription: "Bot score above which a challenge is issued. Range `0.0`\u20131.0`. Default `0.5`.",
			Optional:            true,
			Computed:            true,
			Default:             float64default.StaticFloat64(0.5),
			Validators: []validator.Float64{
				float64validator.Between(0.0, 1.0),
			},
		},
		"action_token_threshold": schema.Float64Attribute{
			MarkdownDescription: "Bot score above which an action token is required. Range `0.0`\u20131.0`. Default `0.5`.",
			Optional:            true,
			Computed:            true,
			Default:             float64default.StaticFloat64(0.5),
			Validators: []validator.Float64{
				float64validator.Between(0.0, 1.0),
			},
		},
	}
}

// SecurityAttributes returns the schema attributes for the top-level security block.
func SecurityAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"enabled": schema.BoolAttribute{
			MarkdownDescription: "Enable WAF/security for this service",
			Optional:            true,
			Computed:            true,
			Default:             booldefault.StaticBool(defaultSecurityEnabled),
		},
		"waf": schema.SingleNestedAttribute{
			MarkdownDescription: "WAF configuration",
			Optional:            true,
			Computed:            true,
			Default:             objectdefault.StaticValue(defaultWafValue),
			Attributes:          WafAttributes(),
		},
		"rate_limit": schema.ListNestedAttribute{
			MarkdownDescription: "Ordered list of rate limiting rules. \n" +
				"  - A rule fires when the client IP exceeds `num_of_requests` requests within the `time_window_seconds` sliding window.\n" +
				"  - The client is then subject to the rule's action for `block_duration_seconds`.\n" +
				"  - **Actions:**\n" +
				"    - `block` (drop excess requests),\n" +
				"    - `log` (observe without blocking),\n" +
				"    - `challenge` (automatic Captcha challenge),\n" +
				"    - `interactive_challenge` (always interactive Captcha challenge),\n" +
				"  - **Conditions:**\n" +
				"    - Each rule condition uses an OR-of-ANDs expression: the rule fires when at least one OR group matches, and all AND conditions in that group match. \n  -",
			Optional: true,
			Computed: true,
			Default:  listdefault.StaticValue(defaultEmptyRateLimitList),
			NestedObject: schema.NestedAttributeObject{
				Attributes: rateLimitAttributes(),
			},
		},
		"bot_management": schema.SingleNestedAttribute{
			MarkdownDescription: "Bot-management keys and challenge thresholds.\n" +
				"  - `web_key` is issued by IORiver.\n" +
				"  - Both thresholds are floats in `[0.0, 1.0]` and default to `0.5`.\n  -",
			Optional:   true,
			Computed:   true,
			Default:    objectdefault.StaticValue(defaultBotManagementValue),
			Attributes: BotManagementAttributes(),
		},
		"custom_rules": schema.ListNestedAttribute{
			MarkdownDescription: "Ordered list of custom rules, evaluated **before** the WAF engine.\n" +
				"  - Rules are evaluated first-to-last.\n" +
				"  - **Actions:**\n" +
				"    - `block` (return 403 immediately, do not evaluate further rules or WAF engine),\n" +
				"    - `log` (allow the request, and add to sampled logs),\n" +
				"    - `bypass_managed` (allow + skip managed ruleset),\n" +
				"    - `allow` (skip remaining rules and WAF engine),\n" +
				"    - `challenge` (automatic Captcha challenge),\n" +
				"    - `interactive_challenge` (always interactive Captcha challenge),\n" +
				"    - `ignore` (instruct WAF engine to ignore a specific parameter — requires `ignore_params`).\n" +
				"  - **Conditions:**\n" +
				"    - Each rule condition uses an OR-of-ANDs expression: the rule fires when at least one OR group matches, and all AND conditions in that group match. \n  -",
			Optional: true,
			Computed: true,
			Default:  listdefault.StaticValue(defaultEmptyCustomList),
			NestedObject: schema.NestedAttributeObject{
				Attributes: customRuleAttributes(),
			},
		},
	}
}

// SecurityModelToMap serialises SecurityModel → API map.
// The backend stores everything under the "waf" config key: custom rules,
// enabled, checkpoint, limit_body_size and rate_limit are all siblings inside
// that single object.  So we start from the flat WafModel map and add
// rate_limit directly into it — no extra nesting layer.
func (s *SecurityModel) SecurityModelToMap(ctx context.Context) map[string]interface{} {
	if s == nil {
		return nil
	}
	// Start from the waf fields (enabled, limit_body_size, checkpoint, custom …)
	wafMap := make(map[string]interface{})
	if s.Waf != nil {
		if m := s.Waf.ModelToMap(ctx); m != nil {
			wafMap = m
		}
	}
	// enabled lives on SecurityModel but backend expects it inside the waf object.
	if !s.Enabled.IsNull() && !s.Enabled.IsUnknown() {
		wafMap["enabled"] = s.Enabled.ValueBool()
	}
	// Rate limit rules live inside the same "waf" object on the backend.
	rateLimitArr := []interface{}{}
	for _, rule := range s.RateLimit {
		ruleMap := map[string]interface{}{
			"name":                   rule.Name.ValueString(),
			"num_of_requests":        int(rule.NumOfRequests.ValueInt64()),
			"time_window_seconds":    int(rule.TimeWindowSeconds.ValueInt64()),
			"block_duration_seconds": int(rule.BlockDurationSeconds.ValueInt64()),
		}
		if !rule.Action.IsNull() && !rule.Action.IsUnknown() {
			ruleMap["action"] = rule.Action.ValueString()
		}
		if !rule.Enabled.IsNull() && !rule.Enabled.IsUnknown() {
			ruleMap["enabled"] = rule.Enabled.ValueBool()
		}
		if rule.Condition != nil {
			ruleMap["condition"] = wafConditionExpressionToMap(ctx, rule.Condition)
		}
		rateLimitArr = append(rateLimitArr, ruleMap)
	}
	wafMap["rate_limit"] = rateLimitArr
	// Custom rules live inside the same "waf" object on the backend.
	customArr := []interface{}{}
	for _, rule := range s.CustomRules {
		ruleMap := map[string]interface{}{
			"name": rule.Name.ValueString(),
		}
		if !rule.Action.IsNull() && !rule.Action.IsUnknown() {
			ruleMap["action"] = rule.Action.ValueString()
		}
		if !rule.Enabled.IsNull() && !rule.Enabled.IsUnknown() {
			ruleMap["enabled"] = rule.Enabled.ValueBool()
		}
		if rule.Condition != nil {
			ruleMap["condition"] = wafConditionExpressionToMap(ctx, rule.Condition)
		}
		if rule.IgnoreParams != nil {
			ruleMap["ignore_params"] = map[string]interface{}{
				"ignore_type": rule.IgnoreParams.IgnoreType.ValueString(),
				"value":       rule.IgnoreParams.Value.ValueString(),
			}
		}
		customArr = append(customArr, ruleMap)
	}
	wafMap["custom"] = customArr
	tflog.Debug(ctx, fmt.Sprintf("[SecurityModelToMap] waf map: %+v", wafMap))
	return wafMap
}

// SecurityMapToModel deserialises an API waf map → SecurityModel.
// The backend places all WAF fields (custom, enabled, checkpoint,
// limit_body_size, rate_limit) directly inside the "waf" config key, so the
// argument here IS the waf object — not a "security" envelope.
func SecurityMapToModel(ctx context.Context, secRaw interface{}) *SecurityModel {
	return securityMapToModelWithCtx(ctx, secRaw, nil, nil)
}

func securityMapToModelWithCtx(ctx context.Context, secRaw interface{}, transformCtx *ServiceTransformContext, priorSec *SecurityModel) *SecurityModel {
	if secRaw == nil {
		return nil
	}
	wafMap, ok := secRaw.(map[string]interface{})
	if !ok {
		tflog.Warn(ctx, fmt.Sprintf("[SecurityMapToModel] unexpected waf type: %T", secRaw))
		return nil
	}
	model := &SecurityModel{}
	// enabled comes from the waf object on the backend.
	if v, ok := wafMap["enabled"].(bool); ok {
		model.Enabled = types.BoolValue(v)
	} else {
		model.Enabled = types.BoolNull()
	}
	// Parse the waf block directly from the flat map.
	model.Waf = wafMapToModelWithCtx(ctx, wafMap, transformCtx)
	// Always initialize to empty slice so state never flips null↔[] on empty backend response.
	model.RateLimit = []WafRateLimitRuleModel{}
	model.CustomRules = []WafCustomRuleModel{}
	// Rate limit rules — only populate if backend returned actual rules.
	if rlRaw, ok := wafMap["rate_limit"].([]interface{}); ok && len(rlRaw) > 0 {
		for rlIdx, rRaw := range rlRaw {
			rMap, ok := rRaw.(map[string]interface{})
			if !ok {
				continue
			}
			rule := WafRateLimitRuleModel{
				Name:                 types.StringValue(strOrEmpty(rMap, "name")),
				Action:               types.StringValue(strOrEmpty(rMap, "action")),
				Enabled:              types.BoolNull(),
				NumOfRequests:        types.Int64Null(),
				TimeWindowSeconds:    types.Int64Null(),
				BlockDurationSeconds: types.Int64Null(),
			}
			if v, ok := rMap["enabled"].(bool); ok {
				rule.Enabled = types.BoolValue(v)
			}
			if v, ok := rMap["num_of_requests"]; ok {
				if fv, ok := toFloat64(v); ok {
					rule.NumOfRequests = types.Int64Value(int64(fv))
				}
			}
			if v, ok := rMap["time_window_seconds"]; ok {
				if fv, ok := toFloat64(v); ok {
					rule.TimeWindowSeconds = types.Int64Value(int64(fv))
				}
			}
			if v, ok := rMap["block_duration_seconds"]; ok {
				if fv, ok := toFloat64(v); ok {
					rule.BlockDurationSeconds = types.Int64Value(int64(fv))
				}
			}
			if condRaw, ok := rMap["condition"].(map[string]interface{}); ok {
				var priorCond *WafConditionExpressionModel
				if priorSec != nil && rlIdx < len(priorSec.RateLimit) {
					priorCond = priorSec.RateLimit[rlIdx].Condition
				}
				rule.Condition = wafConditionExpressionFromMap(ctx, condRaw, priorCond)
			}
			model.RateLimit = append(model.RateLimit, rule)
		}
	}
	// Custom rules — only populate if backend returned actual rules.
	if customRaw, ok := wafMap["custom"].([]interface{}); ok && len(customRaw) > 0 {
		for rIdx, rRaw := range customRaw {
			rMap, ok := rRaw.(map[string]interface{})
			if !ok {
				continue
			}
			rule := WafCustomRuleModel{
				Name:    types.StringValue(strOrEmpty(rMap, "name")),
				Action:  types.StringValue(strOrEmpty(rMap, "action")),
				Enabled: types.BoolNull(),
			}
			if v, ok := rMap["enabled"].(bool); ok {
				rule.Enabled = types.BoolValue(v)
			}
			if condRaw, ok := rMap["condition"].(map[string]interface{}); ok {
				var priorCond *WafConditionExpressionModel
				if priorSec != nil && rIdx < len(priorSec.CustomRules) {
					priorCond = priorSec.CustomRules[rIdx].Condition
				}
				rule.Condition = wafConditionExpressionFromMap(ctx, condRaw, priorCond)
			}
			if ipRaw, ok := rMap["ignore_params"].(map[string]interface{}); ok {
				rule.IgnoreParams = &WafIgnoreParamsModel{
					IgnoreType: types.StringValue(strOrEmpty(ipRaw, "ignore_type")),
					Value:      types.StringValue(strOrEmpty(ipRaw, "value")),
				}
			}
			model.CustomRules = append(model.CustomRules, rule)
		}
	}
	return model
}

// ---------------------------------------------------------------------------
// BotManagement serialization / deserialization
// ---------------------------------------------------------------------------

// BotManagementToMap serialises the bot_management block into a wire map.
// Returns nil when BotManagement is nil so the caller can omit the key entirely.
// Note: the UUID is never sent — the backend generates / preserves it automatically.
func (s *SecurityModel) BotManagementToMap(ctx context.Context) map[string]interface{} {
	if s == nil || s.BotManagement == nil {
		return nil
	}
	bm := s.BotManagement
	out := map[string]interface{}{}
	if !bm.WebKey.IsNull() && !bm.WebKey.IsUnknown() {
		out["web_key"] = bm.WebKey.ValueString()
	}
	if !bm.ChallengeThreshold.IsNull() && !bm.ChallengeThreshold.IsUnknown() {
		out["challenge_threshold"] = bm.ChallengeThreshold.ValueFloat64()
	}
	if !bm.ActionTokenThreshold.IsNull() && !bm.ActionTokenThreshold.IsUnknown() {
		out["action_token_threshold"] = bm.ActionTokenThreshold.ValueFloat64()
	}
	tflog.Debug(ctx, fmt.Sprintf("[BotManagementToMap] map: %+v", out))
	return out
}

// BotManagementMapToModel deserialises a wire bot_management map into a TF model.
// Unknown keys (e.g. uuid) are silently dropped — they are server-managed.
func BotManagementMapToModel(ctx context.Context, raw interface{}) *BotManagementModel {
	m, ok := raw.(map[string]interface{})
	if !ok {
		return nil
	}
	bm := &BotManagementModel{
		WebKey:               types.StringNull(),
		ChallengeThreshold:   types.Float64Null(),
		ActionTokenThreshold: types.Float64Null(),
	}
	if v, ok := m["web_key"].(string); ok {
		bm.WebKey = types.StringValue(v)
	}
	if v, ok := m["challenge_threshold"]; ok {
		if fv, ok := toFloat64(v); ok {
			bm.ChallengeThreshold = types.Float64Value(fv)
		}
	}
	if v, ok := m["action_token_threshold"]; ok {
		if fv, ok := toFloat64(v); ok {
			bm.ActionTokenThreshold = types.Float64Value(fv)
		}
	}
	tflog.Debug(ctx, fmt.Sprintf("[BotManagementMapToModel] model: %+v", bm))
	return bm
}
