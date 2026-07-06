package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectdefault"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

type ComputeModel struct {
	UserCompute types.List   `tfsdk:"user_compute"`
	Report      types.Object `tfsdk:"report"`
}

type UserComputeModel struct {
	FunctionName   types.String `tfsdk:"function_name"`
	Routes         types.Set    `tfsdk:"routes"`
	ViewerRequest  types.String `tfsdk:"viewer_request"`
	OriginRequest  types.String `tfsdk:"origin_request"`
	OriginResponse types.String `tfsdk:"origin_response"`
	ViewerResponse types.String `tfsdk:"viewer_response"`
}

type ComputeReportModel struct {
	SendingReportsThreshold types.Int64 `tfsdk:"sending_reports_threshold"`
	TriggerSendInterval     types.Int64 `tfsdk:"trigger_send_interval"`
}

func UserComputeAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"function_name":   types.StringType,
		"routes":          types.SetType{ElemType: types.StringType},
		"viewer_request":  types.StringType,
		"origin_request":  types.StringType,
		"origin_response": types.StringType,
		"viewer_response": types.StringType,
	}
}

func ComputeReportAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"sending_reports_threshold": types.Int64Type,
		"trigger_send_interval":     types.Int64Type,
	}
}

func ComputeAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"user_compute": types.ListType{ElemType: types.ObjectType{AttrTypes: UserComputeAttrTypes()}},
		"report":       types.ObjectType{AttrTypes: ComputeReportAttrTypes()},
	}
}

var defaultComputeReportObjectValue = types.ObjectValueMust(
	ComputeReportAttrTypes(),
	map[string]attr.Value{
		"sending_reports_threshold": types.Int64Null(),
		"trigger_send_interval":     types.Int64Null(),
	},
)

var defaultComputeValue = types.ObjectValueMust(
	ComputeAttrTypes(),
	map[string]attr.Value{
		"user_compute": types.ListValueMust(types.ObjectType{AttrTypes: UserComputeAttrTypes()}, []attr.Value{}),
		"report":       defaultComputeReportObjectValue,
	},
)

func ComputeAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"user_compute": schema.ListNestedAttribute{
			MarkdownDescription: "User compute functions to run at the edge. Each function can have one or more host/path routes, and optional handlers for viewer request, origin request, origin response, and viewer response.",
			Optional:            true,
			Computed:            true,
			Default:             listdefault.StaticValue(types.ListValueMust(types.ObjectType{AttrTypes: UserComputeAttrTypes()}, []attr.Value{})),
			NestedObject: schema.NestedAttributeObject{
				Attributes: map[string]schema.Attribute{
					"function_name": schema.StringAttribute{
						MarkdownDescription: "Compute function name",
						Required:            true,
					},
					"routes": schema.SetAttribute{
						MarkdownDescription: "Set of host/path routes, e.g. example.com/some/api",
						ElementType:         types.StringType,
						Required:            true,
					},
					"viewer_request": schema.StringAttribute{
						MarkdownDescription: "Viewer request handler code",
						Optional:            true,
					},
					"origin_request": schema.StringAttribute{
						MarkdownDescription: "Origin request handler code",
						Optional:            true,
					},
					"origin_response": schema.StringAttribute{
						MarkdownDescription: "Origin response handler code",
						Optional:            true,
					},
					"viewer_response": schema.StringAttribute{
						MarkdownDescription: "Viewer response handler code",
						Optional:            true,
					},
				},
			},
		},
		"report": schema.SingleNestedAttribute{
			MarkdownDescription: "Compute report settings",
			Optional:            true,
			Computed:            true,
			Default:             objectdefault.StaticValue(defaultComputeReportObjectValue),
			Attributes: map[string]schema.Attribute{
				"sending_reports_threshold": schema.Int64Attribute{
					MarkdownDescription: "Threshold for sending reports",
					Optional:            true,
				},
				"trigger_send_interval": schema.Int64Attribute{
					MarkdownDescription: "Report send trigger interval in seconds",
					Optional:            true,
				},
			},
		},
	}
}

// ModelToMap converts compute to the API wire format.
func (c *ComputeModel) ModelToMap(ctx context.Context) (map[string]interface{}, error) {
	if c == nil {
		return nil, nil
	}

	out := map[string]interface{}{}

	userComputeArray := []interface{}{}
	if !c.UserCompute.IsNull() && !c.UserCompute.IsUnknown() {
		computeModels, err := ListElementsAs[UserComputeModel](ctx, c.UserCompute)
		if err != nil {
			return nil, err
		}
		for _, uc := range computeModels {
			entry := map[string]interface{}{}
			if !uc.FunctionName.IsNull() && !uc.FunctionName.IsUnknown() {
				entry["name"] = uc.FunctionName.ValueString()
			}

			routes := []string{}
			if !uc.Routes.IsNull() && !uc.Routes.IsUnknown() {
				_ = uc.Routes.ElementsAs(ctx, &routes, false)
			}
			entry["routes"] = routes

			if !uc.ViewerRequest.IsNull() && !uc.ViewerRequest.IsUnknown() {
				entry["viewer_request"] = uc.ViewerRequest.ValueString()
			}
			if !uc.OriginRequest.IsNull() && !uc.OriginRequest.IsUnknown() {
				entry["origin_request"] = uc.OriginRequest.ValueString()
			}
			if !uc.OriginResponse.IsNull() && !uc.OriginResponse.IsUnknown() {
				entry["origin_response"] = uc.OriginResponse.ValueString()
			}
			if !uc.ViewerResponse.IsNull() && !uc.ViewerResponse.IsUnknown() {
				entry["viewer_response"] = uc.ViewerResponse.ValueString()
			}

			userComputeArray = append(userComputeArray, entry)
		}
	}
	out["user_compute"] = userComputeArray

	reportMap := map[string]interface{}{}
	if !c.Report.IsNull() && !c.Report.IsUnknown() {
		var report ComputeReportModel
		if diags := c.Report.As(ctx, &report, basetypes.ObjectAsOptions{}); !diags.HasError() {
			if !report.SendingReportsThreshold.IsNull() && !report.SendingReportsThreshold.IsUnknown() {
				reportMap["sending_reports_threshold"] = report.SendingReportsThreshold.ValueInt64()
			}
			if !report.TriggerSendInterval.IsNull() && !report.TriggerSendInterval.IsUnknown() {
				reportMap["trigger_send_interval"] = report.TriggerSendInterval.ValueInt64()
			}
		}
	}
	out["report"] = reportMap

	if len(out) == 0 {
		return nil, nil
	}

	return out, nil
}

// ComputeMapToModel converts API compute payload into Terraform model.
func ComputeMapToModel(ctx context.Context, computeMap map[string]interface{}) *ComputeModel {
	if computeMap == nil {
		return nil
	}

	model := &ComputeModel{
		UserCompute: types.ListValueMust(types.ObjectType{AttrTypes: UserComputeAttrTypes()}, []attr.Value{}),
		Report:      defaultComputeReportObjectValue,
	}

	userComputeModels := []UserComputeModel{}
	if rawUserCompute, ok := computeMap["user_compute"].([]interface{}); ok {
		for _, raw := range rawUserCompute {
			ucMap, ok := raw.(map[string]interface{})
			if !ok {
				continue
			}

			uc := UserComputeModel{
				FunctionName:   types.StringNull(),
				Routes:         types.SetNull(types.StringType),
				ViewerRequest:  types.StringNull(),
				OriginRequest:  types.StringNull(),
				OriginResponse: types.StringNull(),
				ViewerResponse: types.StringNull(),
			}

			if name, ok := ucMap["name"].(string); ok {
				uc.FunctionName = types.StringValue(name)
			}

			routes := []string{}
			switch rawRoutes := ucMap["routes"].(type) {
			case []interface{}:
				for _, r := range rawRoutes {
					if s, ok := r.(string); ok {
						routes = append(routes, s)
					}
				}
			case []string:
				routes = append(routes, rawRoutes...)
			}
			if setVal, diags := types.SetValueFrom(ctx, types.StringType, routes); !diags.HasError() {
				uc.Routes = setVal
			}

			if v, ok := ucMap["viewer_request"].(string); ok {
				uc.ViewerRequest = types.StringValue(v)
			}
			if v, ok := ucMap["origin_request"].(string); ok {
				uc.OriginRequest = types.StringValue(v)
			}
			if v, ok := ucMap["origin_response"].(string); ok {
				uc.OriginResponse = types.StringValue(v)
			}
			if v, ok := ucMap["viewer_response"].(string); ok {
				uc.ViewerResponse = types.StringValue(v)
			}

			userComputeModels = append(userComputeModels, uc)
		}
	}
	if listVal, err := ListObjectValueFrom(ctx, UserComputeAttrTypes(), userComputeModels); err == nil {
		model.UserCompute = listVal
	}

	if rawReport, ok := computeMap["report"].(map[string]interface{}); ok {
		report := ComputeReportModel{
			SendingReportsThreshold: types.Int64Null(),
			TriggerSendInterval:     types.Int64Null(),
		}
		if v, ok := asInt64(rawReport["sending_reports_threshold"]); ok {
			report.SendingReportsThreshold = types.Int64Value(v)
		}
		if v, ok := asInt64(rawReport["trigger_send_interval"]); ok {
			report.TriggerSendInterval = types.Int64Value(v)
		}
		if objVal, diags := types.ObjectValueFrom(ctx, ComputeReportAttrTypes(), report); !diags.HasError() {
			model.Report = objVal
		}
	}

	return model
}

func asInt64(v interface{}) (int64, bool) {
	switch n := v.(type) {
	case int:
		return int64(n), true
	case int64:
		return n, true
	case float64:
		return int64(n), true
	default:
		return 0, false
	}
}
