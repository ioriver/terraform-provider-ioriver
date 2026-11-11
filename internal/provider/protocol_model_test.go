package provider

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// ─── HCL config generators (called from service_resource_test.go) ─────────────

func testAccCheckServiceConfigWithProtocol(resourceName string, certId string, http2 bool, http3 bool, ipv6 bool) string {
	return fmt.Sprintf(`
resource "%s" "%s" {
	name        = "%s"
	certificates = ["%s"]
	description = "A generic service"

	config = {
		protocol = {
			http2_enabled = %t
			http3_enabled = %t
			ipv6_enabled  = %t
		}
	}
}`, serviceResourceType, resourceName, resourceName, certId, http2, http3, ipv6)
}

func testAccCheckServiceConfigWithoutProtocol(resourceName string, certId string) string {
	return fmt.Sprintf(`
resource "%s" "%s" {
	name        = "%s"
	certificates = ["%s"]
	description = "A generic service"

	config = {
	}
}`, serviceResourceType, resourceName, resourceName, certId)
}

func TestProtocolConfig_RoundTrip(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	in := &ProtocolConfigModel{
		Http2Enabled: types.BoolValue(true),
		Http3Enabled: types.BoolValue(false),
		Ipv6Enabled:  types.BoolValue(true),
	}

	wire := in.ModelToMap()
	if wire == nil {
		t.Fatal("ModelToMap returned nil for populated protocol model")
	}
	if got, ok := wire["http2_enabled"].(bool); !ok || !got {
		t.Fatalf("expected http2_enabled=true, got %T %v", wire["http2_enabled"], wire["http2_enabled"])
	}
	if got, ok := wire["http3_enabled"].(bool); !ok || got {
		t.Fatalf("expected http3_enabled=false, got %T %v", wire["http3_enabled"], wire["http3_enabled"])
	}
	if got, ok := wire["ipv6_enabled"].(bool); !ok || !got {
		t.Fatalf("expected ipv6_enabled=true, got %T %v", wire["ipv6_enabled"], wire["ipv6_enabled"])
	}

	out := ProtocolConfigMapToModel(ctx, wire)
	if out == nil {
		t.Fatal("ProtocolConfigMapToModel returned nil for non-nil map")
	}
	if out.Http2Enabled.IsNull() || !out.Http2Enabled.ValueBool() {
		t.Fatalf("unexpected http2_enabled after round-trip: %v", out.Http2Enabled)
	}
	if out.Http3Enabled.IsNull() || out.Http3Enabled.ValueBool() {
		t.Fatalf("unexpected http3_enabled after round-trip: %v", out.Http3Enabled)
	}
	if out.Ipv6Enabled.IsNull() || !out.Ipv6Enabled.ValueBool() {
		t.Fatalf("unexpected ipv6_enabled after round-trip: %v", out.Ipv6Enabled)
	}
}

func TestProtocolConfig_NilContracts(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	var nilModel *ProtocolConfigModel
	if got := nilModel.ModelToMap(); got != nil {
		t.Fatalf("ModelToMap on nil receiver must return nil, got: %+v", got)
	}

	nullModel := ProtocolConfigMapToModel(ctx, nil)
	if nullModel == nil {
		t.Fatal("ProtocolConfigMapToModel(nil) must return a non-nil model with null fields")
	}
	if !nullModel.Http2Enabled.IsNull() || !nullModel.Http3Enabled.IsNull() || !nullModel.Ipv6Enabled.IsNull() ||
		!nullModel.EnableViewerMTLS.IsNull() || !nullModel.ViewerMTLSTrustStoreID.IsNull() {
		t.Fatalf("expected null fields for nil map, got: %+v", nullModel)
	}
}

func TestProtocolConfig_MTLSRoundTrip(t *testing.T) {
	for _, wire := range []map[string]interface{}{
		{"enable_viewer_mtls": true, "viewer_mtls_truststore_id": "56bef007-2d0b-4f10-974d-94ee714b44fd"},
		{"enable_viewer_mtls": false},
		{"viewer_mtls_truststore_id": "56bef007-2d0b-4f10-974d-94ee714b44fd"},
	} {
		model := ProtocolConfigMapToModel(context.Background(), wire)
		if got := model.ModelToMap(); !reflect.DeepEqual(got, wire) {
			t.Fatalf("round trip = %#v, want %#v", got, wire)
		}
	}
}

func TestProtocolConfig_MTLSNullAndUnknown(t *testing.T) {
	for _, wire := range []map[string]interface{}{
		nil, {}, {"enable_viewer_mtls": nil, "viewer_mtls_truststore_id": nil},
	} {
		model := ProtocolConfigMapToModel(context.Background(), wire)
		if !model.EnableViewerMTLS.IsNull() || !model.ViewerMTLSTrustStoreID.IsNull() {
			t.Fatalf("expected null mTLS attributes, got %+v", model)
		}
		if got := model.ModelToMap(); got != nil {
			t.Fatalf("null attributes must be omitted, got %#v", got)
		}
	}
	model := &ProtocolConfigModel{
		Http2Enabled: types.BoolNull(), Http3Enabled: types.BoolNull(), Ipv6Enabled: types.BoolNull(),
		EnableViewerMTLS: types.BoolUnknown(), ViewerMTLSTrustStoreID: types.StringUnknown(),
	}
	if got := model.ModelToMap(); got != nil {
		t.Fatalf("unknown attributes must not be sent, got %#v", got)
	}
	defaults := defaultProtocolValue.Attributes()
	if !defaults["enable_viewer_mtls"].IsNull() || !defaults["viewer_mtls_truststore_id"].IsNull() {
		t.Fatal("omitted protocol must default mTLS attributes to null")
	}
}
