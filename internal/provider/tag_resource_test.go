package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	ioriver "github.com/ioriver/ioriver-go"
)

const (
	tagTestResource   = "ioriver_tag.test"
	tagTestNamePrefix = "tf-test-tag-"
	testTagUUID       = "826c5d31-03e1-4ad0-9e7a-19d6b12a7301"
	testTagServiceA   = "00000000-0000-4000-8000-000000000001"
	testTagServiceB   = "00000000-0000-4000-8000-000000000002"
	testTagServiceC   = "00000000-0000-4000-8000-000000000003"
)

func init() {
	resource.AddTestSweepers("ioriver_tag", &resource.Sweeper{
		Name: "ioriver_tag",
		F: func(region string) error {
			client, err := sharedClient()
			if err != nil {
				return err
			}
			tags, err := client.ListTags()
			if err != nil {
				return err
			}
			for _, tag := range tags {
				if strings.HasPrefix(tag.Name, tagTestNamePrefix) {
					if err := client.DeleteTag(tag.Id); err != nil {
						return fmt.Errorf("delete test tag %s: %w", tag.Id, err)
					}
				}
			}
			return nil
		},
	})
}

func TestAccIORiverTag_Basic(t *testing.T) {
	name := tagTestNamePrefix + generateRandomResourceName()
	serviceID := os.Getenv("IORIVER_TEST_SERVICE_ID")
	config := func(name, attributes string) string {
		return fmt.Sprintf("resource \"ioriver_tag\" \"test\" {\nname = %q\n%s\n}", name, attributes)
	}
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreEnvVariable(t, "IORIVER_API_TOKEN")
			testAccPreEnvVariable(t, "IORIVER_TEST_SERVICE_ID")
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(state *terraform.State) error {
			for _, rs := range state.RootModule().Resources {
				if rs.Type != "ioriver_tag" {
					continue
				}
				_, err := testAccClient.GetTag(rs.Primary.ID)
				var apiErr *ioriver.APIError
				if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusNotFound {
					return fmt.Errorf("tag %s was not confirmed deleted: %v", rs.Primary.ID, err)
				}
			}
			return nil
		},
		Steps: []resource.TestStep{
			{Config: config(name, fmt.Sprintf("services = [%q]", serviceID)), Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr(tagTestResource, "name", name),
				resource.TestCheckResourceAttr(tagTestResource, "color", "gray"),
				resource.TestCheckTypeSetElemAttr(tagTestResource, "services.*", serviceID),
			)},
			{ResourceName: tagTestResource, ImportState: true, ImportStateVerify: true},
			{Config: config(name+"-renamed", "color = \"purple\"\nservices = []"), Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr(tagTestResource, "name", name+"-renamed"),
				resource.TestCheckResourceAttr(tagTestResource, "color", "purple"),
				resource.TestCheckResourceAttr(tagTestResource, "services.#", "0"),
			)},
			{Config: config(name, fmt.Sprintf("services = [%q]", serviceID))},
			{Config: config(name, ""), Check: resource.TestCheckResourceAttr(tagTestResource, "services.#", "0")},
		},
	})
}

func TestTagColorTranslation(t *testing.T) {
	colors := []struct{ name, hex string }{
		{"green", "#0AA636"}, {"blue", "#0065FF"}, {"purple", "#8232D6"},
		{"orange", "#D6781A"}, {"red", "#D63838"}, {"turquoise", "#1AA0C8"}, {"gray", "#5A697A"},
	}
	for _, color := range colors {
		t.Run(color.name, func(t *testing.T) {
			got, err := tagColorToAPI(color.name)
			if err != nil || got != color.hex {
				t.Fatalf("tagColorToAPI(%q) = %q, %v; want %q", color.name, got, err, color.hex)
			}
			for _, value := range []string{color.hex, strings.ToLower(color.hex)} {
				got, err := tagColorFromAPI(value)
				if err != nil || got != color.name {
					t.Fatalf("tagColorFromAPI(%q) = %q, %v; want %q", value, got, err, color.name)
				}
			}
		})
	}
	if _, err := tagColorToAPI("black"); err == nil {
		t.Fatal("unsupported Terraform color must fail")
	}
	if _, err := tagColorFromAPI("#000000"); err == nil {
		t.Fatal("unsupported backend color must fail")
	}
}

func TestDiffTagServices(t *testing.T) {
	cases := []struct {
		name             string
		desired, current []string
		add, remove      []string
	}{
		{"empty", nil, nil, nil, nil},
		{"unchanged", []string{"b", "a"}, []string{"a", "b"}, nil, nil},
		{"replace", []string{"c", "a"}, []string{"b", "a"}, []string{"c"}, []string{"b"}},
		{"clear", nil, []string{"b", "a"}, nil, []string{"a", "b"}},
		{"add sorted", []string{"c", "a", "b"}, nil, []string{"a", "b", "c"}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			add, remove := diffTagServices(tc.desired, tc.current)
			if !reflect.DeepEqual(add, tc.add) || !reflect.DeepEqual(remove, tc.remove) {
				t.Fatalf("got add=%v remove=%v; want add=%v remove=%v", add, remove, tc.add, tc.remove)
			}
		})
	}
}

func TestUpdateTagServicesReconcilesMembership(t *testing.T) {
	var serviceReads int
	var added, removed bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/services/":
			if got := req.URL.Query().Get("tags"); got != "tag-id" {
				t.Errorf("services filter = %q; want tag ID", got)
			}
			serviceReads++
			if serviceReads == 1 {
				_, _ = w.Write([]byte(`[{"id":"service-a"},{"id":"service-b"}]`))
			} else {
				_, _ = w.Write([]byte(`[{"id":"service-b"},{"id":"service-c"}]`))
			}
		case req.Method == http.MethodPost && req.URL.Path == "/v1/services/service-c/add_tags/":
			var payload struct {
				Tags []struct {
					Name string `json:"name"`
				} `json:"tags"`
			}
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Errorf("decode add-tag request: %v", err)
			}
			if len(payload.Tags) != 1 || payload.Tags[0].Name != "production" {
				t.Errorf("add-tag payload = %+v", payload)
			}
			added = true
			_, _ = w.Write([]byte(`[{"tag_name":"production","created":false}]`))
		case req.Method == http.MethodPost && req.URL.Path == "/v1/services/service-a/remove_tag/":
			var payload map[string]string
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Errorf("decode remove-tag request: %v", err)
			}
			if payload["tag_name"] != "production" {
				t.Errorf("remove-tag payload = %+v", payload)
			}
			removed = true
			_, _ = w.Write([]byte(`{"tag_name":"production","removed":true}`))
		default:
			t.Errorf("unexpected request: %s %s", req.Method, req.URL)
			http.Error(w, "unexpected request", http.StatusBadRequest)
		}
	}))
	defer server.Close()

	client := ioriver.NewClient("test-token")
	client.EndpointUrl = server.URL + "/"
	result := updateTagServices(client, ioriver.Tag{Id: "tag-id", Name: "production"}, []string{"service-b", "service-c"})
	if result.MembershipError != nil {
		t.Fatalf("update memberships: %v", result.MembershipError)
	}
	if serviceReads != 2 || !added || !removed {
		t.Fatalf("reads=%d added=%v removed=%v", serviceReads, added, removed)
	}
	if !reflect.DeepEqual(result.Services, []string{"service-b", "service-c"}) {
		t.Fatalf("services = %v; want [service-b service-c]", result.Services)
	}
}

type tagResourceTestAPI struct {
	mu             sync.Mutex
	tag            *ioriver.Tag
	members        map[string]bool
	otherMembers   map[string]bool
	failAdd        string
	failRemove     string
	missingRemove  string
	failList       bool
	filterNotFound bool
	calls          []string
}

func newTagResourceTestAPI(t *testing.T) (*tagResourceTestAPI, *TagResource) {
	t.Helper()
	api := &tagResourceTestAPI{
		members:      make(map[string]bool),
		otherMembers: map[string]bool{testTagServiceA: true},
	}
	server := httptest.NewServer(http.HandlerFunc(api.serveHTTP))
	t.Cleanup(server.Close)
	client := ioriver.NewClient("test-token")
	client.EndpointUrl = server.URL + "/"
	return api, &TagResource{client: client}
}

func (api *tagResourceTestAPI) serveHTTP(w http.ResponseWriter, r *http.Request) {
	api.mu.Lock()
	defer api.mu.Unlock()
	api.calls = append(api.calls, r.Method+" "+r.URL.RequestURI())
	w.Header().Set("Content-Type", "application/json")
	fail := func(code int, message string) { http.Error(w, message, code) }
	respond := func(value interface{}) { _ = json.NewEncoder(w).Encode(value) }
	switch {
	case r.URL.Path == "/v1/tags/" && r.Method == http.MethodPost:
		if api.tag != nil {
			fail(http.StatusBadRequest, "tag already exists")
			return
		}
		var tag ioriver.Tag
		if err := json.NewDecoder(r.Body).Decode(&tag); err != nil {
			fail(http.StatusBadRequest, err.Error())
			return
		}
		tag.Id = testTagUUID
		api.tag = &tag
		w.WriteHeader(http.StatusCreated)
		respond(tag)
	case r.URL.Path == "/v1/tags/"+testTagUUID+"/":
		if api.tag == nil {
			fail(http.StatusNotFound, "tag not found")
			return
		}
		switch r.Method {
		case http.MethodGet:
			respond(api.tag)
		case http.MethodPut:
			var tag ioriver.Tag
			if err := json.NewDecoder(r.Body).Decode(&tag); err != nil {
				fail(http.StatusBadRequest, err.Error())
				return
			}
			api.tag.Name, api.tag.Color = tag.Name, tag.Color
			respond(api.tag)
		case http.MethodDelete:
			api.tag = nil
			api.members = make(map[string]bool)
			w.WriteHeader(http.StatusNoContent)
		default:
			fail(http.StatusMethodNotAllowed, "unexpected tag method")
		}
	case r.URL.Path == "/v1/services/" && r.Method == http.MethodGet:
		if api.filterNotFound {
			fail(http.StatusNotFound, "filter unavailable")
			return
		}
		if api.failList {
			fail(http.StatusInternalServerError, "membership read failed")
			return
		}
		if api.tag == nil || r.URL.Query().Get("tags") != api.tag.Id || len(r.URL.Query()) != 1 {
			fail(http.StatusBadRequest, "expected current tag UUID filter")
			return
		}
		services := []ioriver.Service{}
		for id := range api.members {
			services = append(services, ioriver.Service{Id: id})
		}
		respond(services)
	case strings.HasPrefix(r.URL.Path, "/v1/services/") && r.Method == http.MethodPost:
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) != 4 {
			fail(http.StatusNotFound, "invalid service action")
			return
		}
		serviceID, action := parts[2], parts[3]
		if action == "add_tags" {
			if serviceID == api.failAdd {
				fail(http.StatusNotFound, "service not found")
				return
			}
			var payload struct {
				Tags []struct {
					Name string `json:"name"`
				} `json:"tags"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				fail(http.StatusBadRequest, err.Error())
				return
			}
			if len(payload.Tags) != 1 || api.tag == nil || payload.Tags[0].Name != api.tag.Name {
				fail(http.StatusBadRequest, "attachment used stale tag name")
				return
			}
			api.members[serviceID] = true
			respond([]ioriver.ServiceTagResult{{TagName: api.tag.Name}})
		} else if action == "remove_tag" {
			if serviceID == api.missingRemove {
				delete(api.members, serviceID)
				fail(http.StatusNotFound, "service deleted concurrently")
				return
			}
			if serviceID == api.failRemove {
				fail(http.StatusForbidden, "membership removal forbidden")
				return
			}
			var payload map[string]string
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				fail(http.StatusBadRequest, err.Error())
				return
			}
			if api.tag == nil || payload["tag_name"] != api.tag.Name {
				fail(http.StatusBadRequest, "removal used stale tag name")
				return
			}
			delete(api.members, serviceID)
			respond(ioriver.ServiceTagRemoval{TagName: api.tag.Name, Removed: true})
		} else {
			fail(http.StatusNotFound, "unexpected service action")
		}
	default:
		fail(http.StatusNotFound, "unexpected endpoint")
	}
}

func tagResourceTestState(t *testing.T, data TagResourceModel) tfsdk.State {
	t.Helper()
	var schemaResp frameworkresource.SchemaResponse
	(&TagResource{}).Schema(context.Background(), frameworkresource.SchemaRequest{}, &schemaResp)
	state := tfsdk.State{Schema: schemaResp.Schema}
	if diags := state.Set(context.Background(), &data); diags.HasError() {
		t.Fatal(diags)
	}
	return state
}

func tagResourceTestModel(name string, ids ...string) TagResourceModel {
	values := make([]attr.Value, len(ids))
	for i, id := range ids {
		values[i] = types.StringValue(id)
	}
	return TagResourceModel{
		Id: types.StringValue(testTagUUID), Name: types.StringValue(name),
		Color: types.StringValue("gray"), Services: types.SetValueMust(types.StringType, values),
	}
}

func checkTagResourceTestState(t *testing.T, state tfsdk.State, name string, ids ...string) {
	t.Helper()
	var data TagResourceModel
	if diags := state.Get(context.Background(), &data); diags.HasError() {
		t.Fatal(diags)
	}
	if !data.Id.Equal(types.StringValue(testTagUUID)) || !data.Name.Equal(types.StringValue(name)) {
		t.Fatalf("unexpected tag state: %+v", data)
	}
	if !data.Services.Equal(tagResourceTestModel(name, ids...).Services) {
		t.Fatalf("membership got %v; want %v", data.Services, ids)
	}
}

func TestTagConversionRejectsUnsupportedColor(t *testing.T) {
	obj := &TagWithServices{Tag: ioriver.Tag{Id: testTagUUID, Name: "prod", Color: "#000000"}}
	if _, err := (TagResource{}).objToResource(context.Background(), obj, tagResourceTestModel("prod")); err == nil {
		t.Fatal("conversion must reject unsupported colors")
	}
}

func TestTagResourceToObjHandlesNullAndUnknownServices(t *testing.T) {
	for name, services := range map[string]types.Set{
		"null":    types.SetNull(types.StringType),
		"unknown": types.SetUnknown(types.StringType),
	} {
		t.Run(name, func(t *testing.T) {
			model := tagResourceTestModel("prod")
			model.Services = services
			obj, err := (TagResource{}).resourceToObj(context.Background(), model)
			if err != nil {
				t.Fatalf("convert tag with %s services: %v", name, err)
			}
			if got := obj.(TagWithServices).Services; len(got) != 0 {
				t.Fatalf("services = %v; want no memberships for %s set", got, name)
			}
		})
	}
}

func TestTagTerraformPalette(t *testing.T) {
	api, r := newTagResourceTestAPI(t)
	var steps []resource.TestStep
	for _, tc := range []struct{ name, hex string }{
		{"green", "#0AA636"}, {"blue", "#0065FF"}, {"purple", "#8232D6"},
		{"orange", "#D6781A"}, {"red", "#D63838"}, {"turquoise", "#1AA0C8"},
		{"gray", "#5A697A"},
	} {
		steps = append(steps, resource.TestStep{
			Config: fmt.Sprintf(`
provider "ioriver" {
  endpoint = %q
  token = "test-token"
}
resource "ioriver_tag" "test" {
  name = "palette"
  color = %q
}`, r.client.EndpointUrl, tc.name),
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr(tagTestResource, "color", tc.name),
				func(_ *terraform.State) error {
					api.mu.Lock()
					defer api.mu.Unlock()
					if api.tag == nil || api.tag.Color != tc.hex {
						return fmt.Errorf("API must receive %s for %s, got %+v", tc.hex, tc.name, api.tag)
					}
					return nil
				},
			),
		}, resource.TestStep{ResourceName: tagTestResource, ImportState: true, ImportStateVerify: true})
	}
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps:                    steps,
	})
}

func TestTagCreateDuplicateIsNotAdopted(t *testing.T) {
	api, r := newTagResourceTestAPI(t)
	api.tag = &ioriver.Tag{Id: testTagUUID, Name: "prod"}
	plan := tagResourceTestState(t, tagResourceTestModel("prod"))
	resp := frameworkresource.CreateResponse{State: plan}
	r.Create(context.Background(), frameworkresource.CreateRequest{
		Plan: tfsdk.Plan{Schema: plan.Schema, Raw: plan.Raw},
	}, &resp)
	if !resp.Diagnostics.HasError() {
		t.Fatal("existing tag must require import, not be adopted")
	}
	if len(api.calls) != 1 {
		t.Fatalf("duplicate create must not modify memberships: %v", api.calls)
	}
}

func TestTagCreatePartialFailurePreservesState(t *testing.T) {
	api, r := newTagResourceTestAPI(t)
	api.failAdd = testTagServiceB
	plan := tagResourceTestState(t, tagResourceTestModel("prod", testTagServiceA, testTagServiceB))
	resp := frameworkresource.CreateResponse{State: plan}
	r.Create(context.Background(), frameworkresource.CreateRequest{
		Plan: tfsdk.Plan{Schema: plan.Schema, Raw: plan.Raw},
	}, &resp)
	if !resp.Diagnostics.HasError() {
		t.Fatal("expected membership failure")
	}
	checkTagResourceTestState(t, resp.State, "prod", testTagServiceA)
	if api.tag == nil {
		t.Fatal("partially created tag must stay tracked")
	}
}

func TestTagCreateMembershipReadFailurePreservesID(t *testing.T) {
	api, r := newTagResourceTestAPI(t)
	api.failList = true
	plan := tagResourceTestState(t, tagResourceTestModel("prod", testTagServiceA))
	resp := frameworkresource.CreateResponse{State: plan}
	r.Create(context.Background(), frameworkresource.CreateRequest{
		Plan: tfsdk.Plan{Schema: plan.Schema, Raw: plan.Raw},
	}, &resp)
	if !resp.Diagnostics.HasError() {
		t.Fatal("expected membership read failure")
	}
	checkTagResourceTestState(t, resp.State, "prod")
	if api.tag == nil {
		t.Fatal("tag must remain tracked when membership cannot be read")
	}
}

func TestTagUpdatePartialFailurePreservesState(t *testing.T) {
	api, r := newTagResourceTestAPI(t)
	api.tag = &ioriver.Tag{Id: testTagUUID, Name: "old", Color: "#5A697A"}
	api.members[testTagServiceA] = true
	api.failRemove = testTagServiceA
	state := tagResourceTestState(t, tagResourceTestModel("old", testTagServiceA))
	plan := tagResourceTestState(t, tagResourceTestModel("new", testTagServiceB))
	resp := frameworkresource.UpdateResponse{State: state}
	r.Update(context.Background(), frameworkresource.UpdateRequest{
		State: state, Plan: tfsdk.Plan{Schema: plan.Schema, Raw: plan.Raw},
	}, &resp)
	if !resp.Diagnostics.HasError() {
		t.Fatal("expected membership removal failure")
	}
	checkTagResourceTestState(t, resp.State, "new", testTagServiceA, testTagServiceB)
	if len(api.calls) == 0 || api.calls[0] != "PUT /v1/tags/"+testTagUUID+"/" {
		t.Fatalf("update must PUT directly without a pre-update GET: %v", api.calls)
	}
}

func TestTagReadDistinguishesMissingTagFromMembershipFailure(t *testing.T) {
	for _, tc := range []struct {
		name      string
		missing   bool
		badFilter bool
	}{
		{"tag deleted", true, false},
		{"membership endpoint 404 is not tag deletion", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			api, r := newTagResourceTestAPI(t)
			if !tc.missing {
				api.tag = &ioriver.Tag{Id: testTagUUID, Name: "prod", Color: "#5A697A"}
			}
			api.filterNotFound = tc.badFilter
			state := tagResourceTestState(t, tagResourceTestModel("prod"))
			resp := frameworkresource.ReadResponse{State: state}
			r.Read(context.Background(), frameworkresource.ReadRequest{State: state}, &resp)
			if tc.missing {
				if resp.Diagnostics.HasError() || !resp.State.Raw.IsNull() {
					t.Fatalf("missing tag should be removed from state: %v", resp.Diagnostics)
				}
				return
			}
			if !resp.Diagnostics.HasError() || resp.State.Raw.IsNull() {
				t.Fatal("membership read failure must preserve state and report an error")
			}
			checkTagResourceTestState(t, resp.State, "prod")
		})
	}
}

func TestTagDeleteMissing(t *testing.T) {
	_, r := newTagResourceTestAPI(t)
	state := tagResourceTestState(t, tagResourceTestModel("prod"))
	resp := frameworkresource.DeleteResponse{State: state}
	r.Delete(context.Background(), frameworkresource.DeleteRequest{State: state}, &resp)
	if !resp.Diagnostics.HasError() {
		t.Fatal("delete must report the missing tag error")
	}
}

func TestTagImportPassthrough(t *testing.T) {
	for _, id := range []string{testTagUUID, "backend-validates-this-id"} {
		t.Run(id, func(t *testing.T) {
			state := tagResourceTestState(t, tagResourceTestModel(""))
			resp := frameworkresource.ImportStateResponse{State: state}
			(&TagResource{}).ImportState(context.Background(), frameworkresource.ImportStateRequest{ID: id}, &resp)
			if resp.Diagnostics.HasError() {
				t.Fatalf("unexpected import diagnostics: %v", resp.Diagnostics)
			}
			var actual string
			if diags := resp.State.GetAttribute(context.Background(), path.Root("id"), &actual); diags.HasError() {
				t.Fatal(diags)
			}
			if actual != id {
				t.Fatalf("import ID got %q; want %q", actual, id)
			}
		})
	}
}

func TestTagTerraformLifecycle(t *testing.T) {
	api, r := newTagResourceTestAPI(t)
	config := func(name, attributes string) string {
		return fmt.Sprintf(`
provider "ioriver" {
  endpoint = %q
  token = "test-token"
}
resource "ioriver_tag" "test" {
  name = %q
  %s
}`, r.client.EndpointUrl, name, attributes)
	}
	withA := config("prod", fmt.Sprintf(`services = [%q]`, testTagServiceA))
	withB := config("renamed", fmt.Sprintf("services = [%q]\ncolor = \"blue\"", testTagServiceB))
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(_ *terraform.State) error {
			api.mu.Lock()
			defer api.mu.Unlock()
			if api.tag != nil || len(api.members) != 0 || !api.otherMembers[testTagServiceA] {
				return fmt.Errorf("destroy must delete only the managed tag and its memberships")
			}
			return nil
		},
		Steps: []resource.TestStep{
			{Config: withA, Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr(tagTestResource, "id", testTagUUID),
				resource.TestCheckResourceAttr(tagTestResource, "color", "gray"),
				resource.TestCheckResourceAttr(tagTestResource, "services.#", "1"),
				resource.TestCheckTypeSetElemAttr(tagTestResource, "services.*", testTagServiceA),
			)},
			{ResourceName: tagTestResource, ImportState: true, ImportStateVerify: true},
			{Config: withB, Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr(tagTestResource, "name", "renamed"),
				resource.TestCheckResourceAttr(tagTestResource, "color", "blue"),
				resource.TestCheckTypeSetElemAttr(tagTestResource, "services.*", testTagServiceB),
			)},
			{ResourceName: tagTestResource, ImportState: true, ImportStateVerify: true},
			{Config: withB, PlanOnly: true, ExpectNonEmptyPlan: true, PreConfig: func() {
				api.mu.Lock()
				defer api.mu.Unlock()
				api.members[testTagServiceC] = true
			}},
			{Config: withB, Check: resource.TestCheckResourceAttr(tagTestResource, "services.#", "1")},
			{Config: config("renamed-and-cleared", `services = []`), Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr(tagTestResource, "name", "renamed-and-cleared"),
				resource.TestCheckResourceAttr(tagTestResource, "services.#", "0"),
			)},
			{Config: withA},
			{Config: config("prod", ""), Check: resource.TestCheckResourceAttr(tagTestResource, "services.#", "0")},
			{ResourceName: tagTestResource, ImportState: true, ImportStateVerify: true},
		},
	})
}

func TestTagTerraformMembershipSetOrdering(t *testing.T) {
	_, r := newTagResourceTestAPI(t)
	config := func(a, b string) string {
		return fmt.Sprintf(`
provider "ioriver" {
  endpoint = %q
  token = "test-token"
}
resource "ioriver_tag" "test" {
  name = "prod"
  services = [%q, %q]
}`, r.client.EndpointUrl, a, b)
	}
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: config(testTagServiceA, testTagServiceB)},
			{Config: config(testTagServiceB, testTagServiceA), PlanOnly: true},
			{ResourceName: tagTestResource, ImportState: true, ImportStateVerify: true},
		},
	})
}

func TestTagTerraformExternalDeletion(t *testing.T) {
	api, r := newTagResourceTestAPI(t)
	config := fmt.Sprintf(`
provider "ioriver" {
  endpoint = %q
  token = "test-token"
}
resource "ioriver_tag" "test" {
  name = "prod"
}`, r.client.EndpointUrl)
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: config},
			{Config: config, PlanOnly: true, ExpectNonEmptyPlan: true, PreConfig: func() {
				api.mu.Lock()
				defer api.mu.Unlock()
				api.tag = nil
			}},
			{Config: config, Check: resource.TestCheckResourceAttr(tagTestResource, "id", testTagUUID)},
		},
	})
}

func TestTagTerraformPartialFailureRecovery(t *testing.T) {
	for _, mode := range []string{"create", "update"} {
		t.Run(mode, func(t *testing.T) {
			api, r := newTagResourceTestAPI(t)
			config := func(name string, ids ...string) string {
				members, err := json.Marshal(ids)
				if err != nil {
					t.Fatal(err)
				}
				return fmt.Sprintf(`
provider "ioriver" {
  endpoint = %q
  token = "test-token"
}
resource "ioriver_tag" "test" {
  name = %q
  services = %s
}`, r.client.EndpointUrl, name, members)
			}
			var steps []resource.TestStep
			target := config("prod", testTagServiceA, testTagServiceB)
			if mode == "create" {
				api.failAdd = testTagServiceB
			} else {
				steps = append(steps, resource.TestStep{Config: config("old", testTagServiceA)})
				target = config("renamed", testTagServiceB)
			}
			steps = append(steps,
				resource.TestStep{
					Config: target, ExpectError: regexp.MustCompile("Error setting tag services"),
					PreConfig: func() {
						if mode == "update" {
							api.mu.Lock()
							defer api.mu.Unlock()
							api.failRemove = testTagServiceA
						}
					},
				},
				resource.TestStep{
					Config: target,
					PreConfig: func() {
						api.mu.Lock()
						defer api.mu.Unlock()
						api.failAdd, api.failRemove = "", ""
					},
					Check: resource.TestCheckTypeSetElemAttr(tagTestResource, "services.*", testTagServiceB),
				},
			)
			resource.UnitTest(t, resource.TestCase{
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Steps:                    steps,
			})
		})
	}
}

func TestTagTerraformValidation(t *testing.T) {
	for _, tc := range []struct {
		name, attributes, message string
	}{
		{"empty name", `name = ""`, "length must be"},
		{"long name", fmt.Sprintf(`name = %q`, strings.Repeat("x", 256)), "length must be"},
		{"invalid color case", `name = "prod"` + "\n" + `color = "Red"`, "must be one of"},
		{"raw hex color", `name = "prod"` + "\n" + `color = "#D63838"`, "must be one of"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resource.UnitTest(t, resource.TestCase{
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Steps: []resource.TestStep{{
					Config: `provider "ioriver" { endpoint = "http://127.0.0.1:1/" }` +
						"\nresource \"ioriver_tag\" \"test\" {\n" + tc.attributes + "\n}",
					PlanOnly: true, ExpectError: regexp.MustCompile(tc.message),
				}},
			})
		})
	}
}

func TestTagReconcileUsesServerMembership(t *testing.T) {
	api, r := newTagResourceTestAPI(t)
	api.tag = &ioriver.Tag{Id: testTagUUID, Name: "prod"}
	api.members[testTagServiceA] = true
	api.members[testTagServiceC] = true
	result := updateTagServices(r.client, *api.tag, []string{testTagServiceA, testTagServiceB})
	if result.MembershipError != nil {
		t.Fatal(result.MembershipError)
	}

	var actual []string
	for id := range api.members {
		actual = append(actual, id)
	}
	sort.Strings(actual)
	if !reflect.DeepEqual(actual, []string{testTagServiceA, testTagServiceB}) {
		t.Fatalf("wrong reconciled set: %v", actual)
	}
	for _, call := range api.calls {
		if strings.Contains(call, testTagServiceA+"/add_tags") {
			t.Fatal("unchanged membership must not be rewritten")
		}
	}
}

func TestTagReconcileDeletedServiceRemoval(t *testing.T) {
	api, r := newTagResourceTestAPI(t)
	api.tag = &ioriver.Tag{Id: testTagUUID, Name: "prod"}
	api.members[testTagServiceA] = true
	api.missingRemove = testTagServiceA
	result := updateTagServices(r.client, *api.tag, nil)
	var apiErr *ioriver.APIError
	if !errors.As(result.MembershipError, &apiErr) || apiErr.StatusCode != http.StatusNotFound {
		t.Fatalf("detach 404 must be reported, got: %v", result.MembershipError)
	}
	if len(api.members) != 0 {
		t.Fatal("deleted service should be absent from tag")
	}
}
