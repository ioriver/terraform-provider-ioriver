package provider

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

// ─── HCL config generators (called from service_resource_test.go) ─────────────

func testAccCheckServiceConfigWithCompute(resourceName string, certId string) string {
	return fmt.Sprintf(`
resource "%s" "%s" {
	name        = "%s"
	certificate = "%s"
	description = "A generic service"

	config = {
		compute = {
			user_compute = [
				{
					function_name = "edge-fn"
					routes = ["example.com/some/api", "static.example.com/*"]
					viewer_request = "return request"
					origin_request = "return origin_request"
					origin_response = "return origin_response"
					viewer_response = "return response"
				}
			]
			report = {
				sending_reports_threshold = 100
				trigger_send_interval = 60
			}
		}
	}
}`, serviceResourceType, resourceName, resourceName, certId)
}

func testAccCheckServiceConfigWithoutCompute(resourceName string, certId string) string {
	return fmt.Sprintf(`
resource "%s" "%s" {
	name        = "%s"
	certificate = "%s"
	description = "A generic service"

	config = {
	}
}`, serviceResourceType, resourceName, resourceName, certId)
}

func TestComputeModel_MapRoundTrip(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	routesOne, diags := types.SetValueFrom(ctx, types.StringType, []string{"example.com/some/api", "static.example.com/*"})
	if diags.HasError() {
		t.Fatalf("failed to build routes set: %v", diags)
	}

	userCompute, err := ListObjectValueFrom(ctx, UserComputeAttrTypes(), []UserComputeModel{
		{
			FunctionName:   types.StringValue("edge-fn"),
			Routes:         routesOne,
			ViewerRequest:  types.StringValue("return request"),
			OriginRequest:  types.StringValue("return origin_request"),
			OriginResponse: types.StringValue("return origin_response"),
			ViewerResponse: types.StringValue("return response"),
		},
	})
	if err != nil {
		t.Fatalf("failed to build user_compute list: %v", err)
	}

	reportObj, diags := types.ObjectValueFrom(ctx, ComputeReportAttrTypes(), ComputeReportModel{
		SendingReportsThreshold: types.Int64Value(100),
		TriggerSendInterval:     types.Int64Value(60),
	})
	if diags.HasError() {
		t.Fatalf("failed to build report object: %v", diags)
	}

	in := &ComputeModel{
		UserCompute: userCompute,
		Report:      reportObj,
	}

	wire, err := in.ModelToMap(ctx)
	if err != nil {
		t.Fatalf("ModelToMap returned error: %v", err)
	}
	if wire == nil {
		t.Fatal("ModelToMap returned nil for non-nil model")
	}

	out := ComputeMapToModel(ctx, wire)
	if out == nil {
		t.Fatal("ComputeMapToModel returned nil for non-nil map")
	}

	decodedUserCompute, err := ListElementsAs[UserComputeModel](ctx, out.UserCompute)
	if err != nil {
		t.Fatalf("failed to decode user_compute: %v", err)
	}
	if len(decodedUserCompute) != 1 {
		t.Fatalf("expected 1 user_compute, got %d", len(decodedUserCompute))
	}
	uc := decodedUserCompute[0]
	if got := uc.FunctionName.ValueString(); got != "edge-fn" {
		t.Fatalf("unexpected user_compute name: got %q", got)
	}
	if got := uc.ViewerRequest.ValueString(); got != "return request" {
		t.Fatalf("unexpected viewer_request: got %q", got)
	}
	if got := uc.OriginRequest.ValueString(); got != "return origin_request" {
		t.Fatalf("unexpected origin_request: got %q", got)
	}
	if got := uc.OriginResponse.ValueString(); got != "return origin_response" {
		t.Fatalf("unexpected origin_response: got %q", got)
	}
	if got := uc.ViewerResponse.ValueString(); got != "return response" {
		t.Fatalf("unexpected viewer_response: got %q", got)
	}

	var decodedRoutes []string
	if d := uc.Routes.ElementsAs(ctx, &decodedRoutes, false); d.HasError() {
		t.Fatalf("failed to decode routes set: %v", d)
	}
	if len(decodedRoutes) != 2 {
		t.Fatalf("expected 2 routes, got %d", len(decodedRoutes))
	}

	var report ComputeReportModel
	if diags := out.Report.As(ctx, &report, basetypes.ObjectAsOptions{}); diags.HasError() {
		t.Fatalf("failed to decode report: %v", diags)
	}
	if report.SendingReportsThreshold.IsNull() || report.SendingReportsThreshold.ValueInt64() != 100 {
		t.Fatalf("unexpected sending_reports_threshold: %v", report.SendingReportsThreshold)
	}
	if report.TriggerSendInterval.IsNull() || report.TriggerSendInterval.ValueInt64() != 60 {
		t.Fatalf("unexpected trigger_send_interval: %v", report.TriggerSendInterval)
	}
}

func TestComputeModel_NilAndUnknownHandling(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	var nilModel *ComputeModel
	wire, err := nilModel.ModelToMap(ctx)
	if err != nil {
		t.Fatalf("nil model ModelToMap returned error: %v", err)
	}
	if wire != nil {
		t.Fatalf("nil model should map to nil, got: %+v", wire)
	}

	unknownCompute := &ComputeModel{
		UserCompute: types.ListUnknown(types.ObjectType{AttrTypes: UserComputeAttrTypes()}),
		Report:      types.ObjectUnknown(ComputeReportAttrTypes()),
	}
	wire, err = unknownCompute.ModelToMap(ctx)
	if err != nil {
		t.Fatalf("ModelToMap with unknown values returned error: %v", err)
	}

	rawUserCompute, ok := wire["user_compute"].([]interface{})
	if !ok {
		t.Fatalf("user_compute should be []interface{}, got %T", wire["user_compute"])
	}
	if len(rawUserCompute) != 0 {
		t.Fatalf("expected empty user_compute for unknown list, got %d", len(rawUserCompute))
	}

	rawReport, ok := wire["report"].(map[string]interface{})
	if !ok {
		t.Fatalf("report should be map[string]interface{}, got %T", wire["report"])
	}
	if len(rawReport) != 0 {
		t.Fatalf("expected empty report for unknown object, got %+v", rawReport)
	}

	if got := ComputeMapToModel(ctx, nil); got != nil {
		t.Fatalf("ComputeMapToModel(nil) should return nil, got %+v", got)
	}
}
