package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type ComputeModel struct {
	Name         types.String        `tfsdk:"name"`
	RequestCode  types.String        `tfsdk:"request_code"`
	ResponseCode types.String        `tfsdk:"response_code"`
	Routes       []ComputeRouteModel `tfsdk:"routes"`
}

func ComputeRouteAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"domain": types.StringType,
		"path":   types.StringType,
	}
}

func ComputeAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"name":          types.StringType,
		"request_code":  types.StringType,
		"response_code": types.StringType,
		"routes":        types.ListType{ElemType: types.ObjectType{AttrTypes: ComputeRouteAttrTypes()}},
	}
}

func ComputeAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"name": schema.StringAttribute{
			MarkdownDescription: "Compute function name",
			Required:            true,
		},
		"request_code": schema.StringAttribute{
			MarkdownDescription: "Compute code for request phase",
			Optional:            true,
			Computed:            true,
		},
		"response_code": schema.StringAttribute{
			MarkdownDescription: "Compute code for response phase",
			Optional:            true,
			Computed:            true,
		},
		"routes": schema.ListNestedAttribute{
			MarkdownDescription: "List of routes to apply the compute",
			Required:            true,
			NestedObject: schema.NestedAttributeObject{
				Attributes: map[string]schema.Attribute{
					"domain": schema.StringAttribute{
						MarkdownDescription: "Route domain name",
						Required:            true,
					},
					"path": schema.StringAttribute{
						MarkdownDescription: "Route path pattern",
						Required:            true,
					},
				},
			},
		},
	}
}
