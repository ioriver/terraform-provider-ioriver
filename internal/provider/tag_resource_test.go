package provider

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	ioriver "github.com/ioriver/ioriver-go"
)

const (
	tagTestResource   = "ioriver_tag.test"
	tagTestNamePrefix = "tf-test-tag-"
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
