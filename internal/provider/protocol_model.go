package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type ProtocolConfigModel struct {
	Http2Enabled           types.Bool   `tfsdk:"http2_enabled"`
	Http3Enabled           types.Bool   `tfsdk:"http3_enabled"`
	Ipv6Enabled            types.Bool   `tfsdk:"ipv6_enabled"`
	EnableViewerMTLS       types.Bool   `tfsdk:"enable_viewer_mtls"`
	ViewerMTLSTrustStoreID types.String `tfsdk:"viewer_mtls_truststore_id"`
}

func ProtocolAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"http2_enabled":             types.BoolType,
		"http3_enabled":             types.BoolType,
		"ipv6_enabled":              types.BoolType,
		"enable_viewer_mtls":        types.BoolType,
		"viewer_mtls_truststore_id": types.StringType,
	}
}

var defaultHttp2Enabled = true
var defaultHttp3Enabled = false
var defaultIpv6Enabled = true

var defaultProtocolValue = types.ObjectValueMust(
	ProtocolAttrTypes(),
	map[string]attr.Value{
		"http2_enabled":             types.BoolValue(defaultHttp2Enabled),
		"http3_enabled":             types.BoolValue(defaultHttp3Enabled),
		"ipv6_enabled":              types.BoolValue(defaultIpv6Enabled),
		"enable_viewer_mtls":        types.BoolNull(),
		"viewer_mtls_truststore_id": types.StringNull(),
	},
)

func ProtocolAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{

		"http2_enabled": schema.BoolAttribute{
			MarkdownDescription: "Enable HTTP/2",
			Optional:            true,
			Computed:            true,
			Default:             booldefault.StaticBool(defaultHttp2Enabled),
		},
		"http3_enabled": schema.BoolAttribute{
			MarkdownDescription: "Enable HTTP/3",
			Optional:            true,
			Computed:            true,
			Default:             booldefault.StaticBool(defaultHttp3Enabled),
		},
		"ipv6_enabled": schema.BoolAttribute{
			MarkdownDescription: "Enable IPv6",
			Optional:            true,
			Computed:            true,
			Default:             booldefault.StaticBool(defaultIpv6Enabled),
		},
		"enable_viewer_mtls": schema.BoolAttribute{
			MarkdownDescription: "Enable client mTLS. When true, viewer_mtls_truststore_id must identify a trust store in the same account. Omitted values remain unset.",
			Optional:            true,
		},
		"viewer_mtls_truststore_id": schema.StringAttribute{
			MarkdownDescription: "ID of the client mTLS trust store. Reference ioriver_client_mtls_trust_store.<name>.id to establish dependency ordering.",
			Optional:            true,
		},
	}
}

// ToMap converts ProtocolConfigModel to map for API
func (p *ProtocolConfigModel) ModelToMap() map[string]interface{} {
	if p == nil {
		return nil
	}

	protocolMap := make(map[string]interface{})

	if !p.Http2Enabled.IsNull() && !p.Http2Enabled.IsUnknown() {
		protocolMap["http2_enabled"] = p.Http2Enabled.ValueBool()
	}

	if !p.Http3Enabled.IsNull() && !p.Http3Enabled.IsUnknown() {
		protocolMap["http3_enabled"] = p.Http3Enabled.ValueBool()
	}

	if !p.Ipv6Enabled.IsNull() && !p.Ipv6Enabled.IsUnknown() {
		protocolMap["ipv6_enabled"] = p.Ipv6Enabled.ValueBool()
	}
	if !p.EnableViewerMTLS.IsNull() && !p.EnableViewerMTLS.IsUnknown() {
		protocolMap["enable_viewer_mtls"] = p.EnableViewerMTLS.ValueBool()
	}
	if !p.ViewerMTLSTrustStoreID.IsNull() && !p.ViewerMTLSTrustStoreID.IsUnknown() {
		protocolMap["viewer_mtls_truststore_id"] = p.ViewerMTLSTrustStoreID.ValueString()
	}

	if len(protocolMap) == 0 {
		return nil
	}

	return protocolMap
}

// ProtocolConfigFromMap converts API map to ProtocolConfigModel
func ProtocolConfigMapToModel(ctx context.Context, protocolMap map[string]interface{}) *ProtocolConfigModel {
	protocol := &ProtocolConfigModel{
		Http2Enabled:           types.BoolNull(),
		Http3Enabled:           types.BoolNull(),
		Ipv6Enabled:            types.BoolNull(),
		EnableViewerMTLS:       types.BoolNull(),
		ViewerMTLSTrustStoreID: types.StringNull(),
	}

	if http2, ok := protocolMap["http2_enabled"].(bool); ok {
		protocol.Http2Enabled = types.BoolValue(http2)
	}

	if http3, ok := protocolMap["http3_enabled"].(bool); ok {
		protocol.Http3Enabled = types.BoolValue(http3)
	}

	if ipv6, ok := protocolMap["ipv6_enabled"].(bool); ok {
		protocol.Ipv6Enabled = types.BoolValue(ipv6)
	}
	if enabled, ok := protocolMap["enable_viewer_mtls"].(bool); ok {
		protocol.EnableViewerMTLS = types.BoolValue(enabled)
	}
	if id, ok := protocolMap["viewer_mtls_truststore_id"].(string); ok {
		protocol.ViewerMTLSTrustStoreID = types.StringValue(id)
	}

	return protocol
}
