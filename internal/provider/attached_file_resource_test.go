package provider

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	ioriver "github.com/ioriver/ioriver-go"
)

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
	created, err := r.create(ctx, client, ioriver.AttachedFile{FileType: "HTML", Contents: "<html>blocked</html>"})
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
