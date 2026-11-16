package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	ioriver "github.com/ioriver/ioriver-go"
)

const (
	attachedFileResourceType  = "ioriver_attached_file"
	attachedFileTestPrefix    = "<!-- ioriver-terraform-acceptance-test:"
	attachedFileTestPrefixEnd = " -->"
)

func init() {
	resource.AddTestSweepers(attachedFileResourceType, &resource.Sweeper{
		Name: attachedFileResourceType,
		F: func(_ string) error {
			client, err := sharedClient()
			if err != nil {
				return fmt.Errorf("create attachment sweeper client: %w", err)
			}
			return sweepTestAttachedFiles(client)
		},
		Dependencies: []string{"ioriver_service"},
	})
}

func sweepTestAttachedFiles(client *ioriver.IORiverClient) error {
	files, err := ioriver.List[ioriver.AttachedFile](client, "attached-files/")
	if err != nil {
		return fmt.Errorf("list attachments for sweeping: %w", err)
	}
	for _, entry := range files {
		if entry.ID == "" {
			return errors.New("list attachments for sweeping: attachment has empty ID")
		}
		file, err := client.GetAttachedFile(entry.ID)
		if err != nil {
			return fmt.Errorf("read attachment %s for sweeping: %w", entry.ID, err)
		}
		if !strings.HasPrefix(file.Contents, attachedFileTestPrefix) {
			continue
		}
		if err := (AttachedFileResource{}).delete(context.Background(), client, entry.ID); err != nil {
			return fmt.Errorf("sweep test attachment %s: %w", entry.ID, err)
		}
	}
	return nil
}

func attachedFileAcceptanceContents(name string) string {
	return attachedFileTestPrefix + name + attachedFileTestPrefixEnd + "\n<html><body>Terraform acceptance test</body></html>"
}

func attachedFileAcceptanceConfig(contents string) string {
	return fmt.Sprintf(`
resource "ioriver_attached_file" "test" {
  file_type = %q
  contents = %q
}
`, attachedFileTypeCustomResponseBody, contents)
}

func TestAccIORiverAttachedFile(t *testing.T) {
	t.Parallel()
	name := generateRandomResourceName()
	initial := attachedFileAcceptanceContents(name)
	replacement := attachedFileAcceptanceContents(name + "-replacement")
	resourceName := attachedFileResourceType + ".test"
	check := func(contents string) resource.TestCheckFunc {
		return resource.ComposeTestCheckFunc(
			resource.TestCheckResourceAttr(resourceName, "file_type", attachedFileTypeCustomResponseBody),
			resource.TestCheckResourceAttr(resourceName, "contents", contents),
			func(state *terraform.State) error {
				rs, ok := state.RootModule().Resources[resourceName]
				if !ok || rs.Primary.ID == "" {
					return fmt.Errorf("%s missing from state or has empty ID", resourceName)
				}
				file, err := testAccClient.GetAttachedFile(rs.Primary.ID)
				if err != nil {
					return fmt.Errorf("read attachment %s: %w", rs.Primary.ID, err)
				}
				if file.ID != rs.Primary.ID ||
					(file.FileType != attachedFileBackendTypeHTML && file.FileType != attachedFileTypeCustomResponseBody) ||
					file.Contents != contents {
					return fmt.Errorf("attachment %s returned unexpected ID, type, or contents", rs.Primary.ID)
				}
				return nil
			},
		)
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreEnvVariable(t, "IORIVER_API_TOKEN") },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(state *terraform.State) error {
			for _, rs := range state.RootModule().Resources {
				if rs.Type != attachedFileResourceType || rs.Primary.ID == "" {
					continue
				}
				_, err := testAccClient.GetAttachedFile(rs.Primary.ID)
				var apiErr *ioriver.APIError
				if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusNotFound {
					return fmt.Errorf("attachment %s was not confirmed deleted: %v", rs.Primary.ID, err)
				}
			}
			return nil
		},
		Steps: []resource.TestStep{
			{Config: attachedFileAcceptanceConfig(initial), Check: check(initial)},
			{ResourceName: resourceName, ImportState: true, ImportStateVerify: true},
			{
				Config: attachedFileAcceptanceConfig(replacement),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionDestroyBeforeCreate),
				}},
				Check: check(replacement),
			},
			{Config: attachedFileAcceptanceConfig(replacement), PlanOnly: true},
		},
	})
}

func TestSweepTestAttachedFiles(t *testing.T) {
	for _, tc := range []struct {
		name        string
		failurePath string
		wantError   bool
		wantDeleted bool
	}{
		{name: "only test attachments", wantDeleted: true},
		{name: "list failure", failurePath: "/v1/attached-files/", wantError: true},
		{name: "read failure", failurePath: "/v1/attached-files/test-id/", wantError: true},
		{name: "delete failure", failurePath: "DELETE /v1/attached-files/test-id/", wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			deleted := []string{}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				if tc.failurePath == req.URL.Path || tc.failurePath == req.Method+" "+req.URL.Path {
					http.Error(w, "injected failure", http.StatusInternalServerError)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				switch {
				case req.Method == http.MethodGet && req.URL.Path == "/v1/attached-files/":
					_ = json.NewEncoder(w).Encode([]ioriver.AttachedFile{{ID: "test-id"}, {ID: "other-id"}})
				case req.Method == http.MethodGet && req.URL.Path == "/v1/attached-files/test-id/":
					_ = json.NewEncoder(w).Encode(ioriver.AttachedFile{ID: "test-id", Contents: attachedFileAcceptanceContents("random")})
				case req.Method == http.MethodGet && req.URL.Path == "/v1/attached-files/other-id/":
					_ = json.NewEncoder(w).Encode(ioriver.AttachedFile{ID: "other-id", Contents: "<html>customer file</html>"})
				case req.Method == http.MethodDelete:
					deleted = append(deleted, req.URL.Path)
					w.WriteHeader(http.StatusNoContent)
				default:
					t.Errorf("unexpected request %s %s", req.Method, req.URL.Path)
					http.Error(w, "unexpected request", http.StatusNotFound)
				}
			}))
			defer server.Close()
			client := ioriver.NewClient("test-token")
			client.EndpointUrl = server.URL + "/"
			err := sweepTestAttachedFiles(client)
			if (err != nil) != tc.wantError {
				t.Fatalf("sweep error = %v, wantError = %t", err, tc.wantError)
			}
			if tc.wantDeleted {
				if len(deleted) != 1 || deleted[0] != "/v1/attached-files/test-id/" {
					t.Fatalf("deleted paths = %v, want only test attachment", deleted)
				}
			} else if len(deleted) != 0 {
				t.Fatalf("unexpected deleted paths: %v", deleted)
			}
		})
	}
}

func TestAttachedFileResourceAPI(t *testing.T) {
	const id = "697dd97d-437e-46f8-915f-1e28bab9df2c"
	methods := make(chan string, 3)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		methods <- req.Method
		if req.Header.Get("Authorization") != "token test-token" {
			t.Error("missing SDK authorization header")
		}
		w.Header().Set("Content-Type", "application/json")
		switch req.Method {
		case http.MethodPost:
			if req.URL.Path != "/v1/attached-files/" {
				t.Errorf("unexpected create path %s", req.URL.Path)
			}
			var payload map[string]interface{}
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Error(err)
			}
			if payload["file_type"] != "HTML" || payload["contents"] != "<html>blocked</html>" {
				t.Errorf("unexpected upload payload: %#v", payload)
			}
			if _, ok := payload["service"]; ok {
				t.Error("account attachment must not require a service ID")
			}
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(ioriver.AttachedFile{ID: id, FileType: "HTML", Contents: "<html>blocked</html>"})
		case http.MethodGet:
			if req.URL.Path != "/v1/attached-files/"+id+"/" {
				t.Errorf("unexpected read path %s", req.URL.Path)
			}
			_ = json.NewEncoder(w).Encode(ioriver.AttachedFile{ID: id, FileType: "HTML", Contents: "<html>blocked</html>"})
		case http.MethodDelete:
			if req.URL.Path != "/v1/attached-files/"+id+"/" {
				t.Errorf("unexpected delete path %s", req.URL.Path)
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected method %s", req.Method)
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer server.Close()
	client := ioriver.NewClient("test-token")
	client.EndpointUrl = server.URL + "/"
	r := AttachedFileResource{}
	ctx := context.Background()
	input, err := r.resourceToObj(ctx, AttachedFileResourceModel{
		FileType: types.StringValue(attachedFileTypeCustomResponseBody),
		Contents: types.StringValue("<html>blocked</html>"),
	})
	if err != nil {
		t.Fatal(err)
	}
	created, err := r.create(ctx, client, input)
	if err != nil {
		t.Fatal(err)
	}
	if created.(*ioriver.AttachedFile).ID != id {
		t.Fatal("create did not return the attachment ID")
	}
	if _, err := r.read(ctx, client, id); err != nil {
		t.Fatal(err)
	}
	if err := r.delete(ctx, client, id); err != nil {
		t.Fatal(err)
	}
	sequence := []string{<-methods, <-methods, <-methods}
	if strings.Join(sequence, ",") != "POST,GET,DELETE" {
		t.Fatalf("unexpected lifecycle calls: %v", sequence)
	}
}

func TestAttachedFileTypeTranslation(t *testing.T) {
	r := AttachedFileResource{}
	ctx := context.Background()
	for _, fileType := range []string{attachedFileBackendTypeHTML, attachedFileTypeCustomResponseBody} {
		t.Run("read_"+fileType, func(t *testing.T) {
			model, err := r.objToResource(ctx, &ioriver.AttachedFile{
				ID: "file-id", FileType: fileType, Contents: "body",
			}, AttachedFileResourceModel{})
			if err != nil {
				t.Fatal(err)
			}
			if got := model.(AttachedFileResourceModel).FileType.ValueString(); got != attachedFileTypeCustomResponseBody {
				t.Fatalf("file type = %q, want %q", got, attachedFileTypeCustomResponseBody)
			}
		})
	}
	t.Run("unsupported_backend_type", func(t *testing.T) {
		_, err := r.objToResource(ctx, &ioriver.AttachedFile{ID: "file-id", FileType: "other"}, AttachedFileResourceModel{})
		if err == nil || !strings.Contains(err.Error(), "unsupported backend type") {
			t.Fatalf("expected unsupported backend type error, got %v", err)
		}
	})
	t.Run("unsupported_config_type", func(t *testing.T) {
		_, err := r.resourceToObj(ctx, AttachedFileResourceModel{FileType: types.StringValue("HTML")})
		if err == nil || !strings.Contains(err.Error(), "unsupported attachment type") {
			t.Fatalf("expected unsupported attachment type error, got %v", err)
		}
	})
}

func TestAttachedFileTerraformTypeTranslation(t *testing.T) {
	for _, backendType := range []string{attachedFileBackendTypeHTML, attachedFileTypeCustomResponseBody} {
		t.Run(backendType, func(t *testing.T) {
			const id = "697dd97d-437e-46f8-915f-1e28bab9df2c"
			const contents = "<html>blocked</html>"
			var exists atomic.Bool
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch {
				case req.Method == http.MethodPost && req.URL.Path == "/v1/attached-files/":
					var payload ioriver.AttachedFile
					if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
						t.Errorf("decode create payload: %v", err)
						http.Error(w, "invalid JSON", http.StatusBadRequest)
						return
					}
					if payload.FileType != attachedFileBackendTypeHTML || payload.Contents != contents {
						t.Errorf("unexpected create payload: %#v", payload)
						http.Error(w, "unexpected payload", http.StatusBadRequest)
						return
					}
					exists.Store(true)
					w.WriteHeader(http.StatusCreated)
					_ = json.NewEncoder(w).Encode(ioriver.AttachedFile{ID: id, FileType: backendType, Contents: contents})
				case req.Method == http.MethodGet && req.URL.Path == "/v1/attached-files/"+id+"/":
					if !exists.Load() {
						http.NotFound(w, req)
						return
					}
					_ = json.NewEncoder(w).Encode(ioriver.AttachedFile{ID: id, FileType: backendType, Contents: contents})
				case req.Method == http.MethodDelete && req.URL.Path == "/v1/attached-files/"+id+"/":
					exists.Store(false)
					w.WriteHeader(http.StatusNoContent)
				default:
					t.Errorf("unexpected request %s %s", req.Method, req.URL.Path)
					http.NotFound(w, req)
				}
			}))
			defer server.Close()
			config := fmt.Sprintf("provider \"ioriver\" { endpoint = %q }\n", server.URL+"/") +
				attachedFileAcceptanceConfig(contents)
			resource.UnitTest(t, resource.TestCase{
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Steps: []resource.TestStep{
					{Config: config, Check: resource.TestCheckResourceAttr("ioriver_attached_file.test", "file_type", attachedFileTypeCustomResponseBody)},
					{ResourceName: "ioriver_attached_file.test", ImportState: true, ImportStateVerify: true},
					{Config: config, PlanOnly: true},
				},
			})
		})
	}
}

func TestAttachedFileTerraformRejectsLegacyType(t *testing.T) {
	config := `resource "ioriver_attached_file" "test" {
  file_type = "HTML"
  contents = "body"
}`
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config:      config,
			PlanOnly:    true,
			ExpectError: regexp.MustCompile(`custom_response_body`),
		}},
	})
}

func TestAttachedFileResourceDeletionErrors(t *testing.T) {
	for _, status := range []int{http.StatusNotFound, http.StatusBadRequest, http.StatusForbidden} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				http.Error(w, "attachment is referenced", status)
			}))
			defer server.Close()
			client := ioriver.NewClient("")
			client.EndpointUrl = server.URL + "/"
			err := (AttachedFileResource{}).delete(context.Background(), client, "file-id")
			if status == http.StatusNotFound {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			var apiErr *ioriver.APIError
			if !errors.As(err, &apiErr) || apiErr.StatusCode != status {
				t.Fatalf("expected backend error %d, got %v", status, err)
			}
		})
	}
}

func TestAttachedFileResourcePreservesConfiguredWhitespace(t *testing.T) {
	r := AttachedFileResource{}
	prior := AttachedFileResourceModel{
		Contents: types.StringValue("<html>blocked</html>\n"),
	}

	raw := &ioriver.AttachedFile{ID: "file-id", FileType: "HTML", Contents: "<html>blocked</html>"}
	model, err := r.objToResource(context.Background(), raw, prior)
	if err != nil {
		t.Fatal(err)
	}
	if !model.(AttachedFileResourceModel).Contents.Equal(prior.Contents) {
		t.Fatal("configured trailing newline was lost")
	}
	raw.Contents = "<html>changed externally</html>"
	model, err = r.objToResource(context.Background(), raw, prior)
	if err != nil {
		t.Fatal(err)
	}
	if model.(AttachedFileResourceModel).Contents.ValueString() != raw.Contents {
		t.Fatal("external content drift was hidden")
	}
}

func TestAttachedFileContentsRequireReplace(t *testing.T) {
	for _, tc := range []struct {
		name    string
		before  types.String
		after   types.String
		replace bool
	}{
		{"unchanged", types.StringValue("body"), types.StringValue("body"), false},
		{"outer whitespace", types.StringValue("body\n"), types.StringValue(" body "), false},
		{"content change", types.StringValue("body"), types.StringValue("new body"), true},
		{"unknown content", types.StringValue("body"), types.StringUnknown(), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var resp stringplanmodifier.RequiresReplaceIfFuncResponse
			attachedFileContentsRequireReplace(context.Background(), planmodifier.StringRequest{
				StateValue: tc.before, PlanValue: tc.after,
			}, &resp)
			if resp.RequiresReplace != tc.replace {
				t.Fatalf("expected replacement=%t, got %t", tc.replace, resp.RequiresReplace)
			}
		})
	}
}
