package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type GeoRestrictionModel struct {
	Countries []string `tfsdk:"countries" json:"countries,omitempty"`
	Mode      string   `tfsdk:"mode" json:"mode,omitempty"`
}

func GeoRestrictionAttributes() map[string]schema.Attribute {
	attributes := map[string]schema.Attribute{
		"countries": schema.ListAttribute{
			MarkdownDescription: "List of countries for geo restriction",
			Optional:            true,
			ElementType:         types.StringType,
		},
		"mode": schema.StringAttribute{
			MarkdownDescription: "Mode of geo restriction",
			Optional:            true,
		},
	}
	return attributes
}

func (g GeoRestrictionModel) ModelToMap() map[string]interface{} {
	return map[string]interface{}{
		"countries": g.Countries,
		"mode":      g.Mode,
	}
}

func GeoRestrictionMapToModel(data map[string]interface{}) (GeoRestrictionModel, error) {
	var g GeoRestrictionModel
	if countries, ok := data["countries"].([]interface{}); ok {
		for _, country := range countries {
			if countryStr, ok := country.(string); ok {
				g.Countries = append(g.Countries, countryStr)
			}
		}
	}
	if mode, ok := data["mode"].(string); ok {
		g.Mode = mode
	}
	return g, nil
}
