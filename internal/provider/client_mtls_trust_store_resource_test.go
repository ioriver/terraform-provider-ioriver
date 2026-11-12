package provider

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	ioriver "github.com/ioriver/ioriver-go"
)

const clientMTLSTrustStoreResourceType = "ioriver_client_mtls_trust_store"
const mtlsTestNamePrefix = "tf-mtls-"

func init() {
	var testedObj TestedClientMTLSTrustStore
	resource.AddTestSweepers(clientMTLSTrustStoreResourceType, &resource.Sweeper{
		Name: clientMTLSTrustStoreResourceType,
		F: func(r string) error {
			return testSweepResources[ioriver.ViewerMTLSTrustStore](r, testedObj, []string{})
		},
		Dependencies: []string{"ioriver_service"},
	})
}

type TestedClientMTLSTrustStore struct {
	TestedObj[ioriver.ViewerMTLSTrustStore]
}

func (TestedClientMTLSTrustStore) Get(client *ioriver.IORiverClient, id string) (*ioriver.ViewerMTLSTrustStore, error) {
	return client.GetViewerMTLSTrustStore(id)
}

func (TestedClientMTLSTrustStore) List(client *ioriver.IORiverClient) ([]ioriver.ViewerMTLSTrustStore, error) {
	return client.ListViewerMTLSTrustStores()
}

func (TestedClientMTLSTrustStore) Delete(client *ioriver.IORiverClient, object ioriver.ViewerMTLSTrustStore, excludeIDs []string) error {
	if !strings.HasPrefix(object.Name, mtlsTestNamePrefix) {
		return nil
	}
	for _, id := range excludeIDs {
		if object.Id == id {
			return nil
		}
	}
	return client.DeleteViewerMTLSTrustStore(object.Id)
}

func mtlsTrustStoreResourceConfig(name, bundle string) string {
	return fmt.Sprintf(`
resource "ioriver_client_mtls_trust_store" "test" {
  name   = %q
  bundle = %q
  lifecycle { create_before_destroy = true }
}
`, name, bundle)
}

func mtlsAcceptanceBundle(t *testing.T) string {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	cert := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "IO River Terraform acceptance CA"},
		NotBefore:    time.Now().Add(-time.Hour), NotAfter: time.Now().AddDate(1, 0, 0),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, cert, cert, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})))
}

func TestAccIORiverClientMTLSTrustStore(t *testing.T) {
	t.Parallel()
	name := mtlsTestNamePrefix + generateRandomResourceName()
	bundle, replacement := mtlsAcceptanceBundle(t), mtlsAcceptanceBundle(t)
	var store ioriver.ViewerMTLSTrustStore
	var testedObj TestedClientMTLSTrustStore
	resourceName := clientMTLSTrustStoreResourceType + ".test"
	check := func(name, bundle string) resource.TestCheckFunc {
		return resource.ComposeTestCheckFunc(
			testAccCheckObjectExists[ioriver.ViewerMTLSTrustStore](resourceName, &store, testedObj),
			resource.TestCheckResourceAttr(resourceName, "name", name),
			resource.TestCheckResourceAttr(resourceName, "bundle", bundle),
			func(_ *terraform.State) error {
				if store.Name != name || store.Bundle != bundle {
					return fmt.Errorf("trust store did not persist name and bundle")
				}
				return nil
			},
		)
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreEnvVariable(t, "IORIVER_API_TOKEN") },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(state *terraform.State) error {
			return testAccCheckResourceDestroy[ioriver.ViewerMTLSTrustStore](state, testedObj, clientMTLSTrustStoreResourceType)
		},
		Steps: []resource.TestStep{
			{Config: mtlsTrustStoreResourceConfig(name, bundle), Check: check(name, bundle)},
			{ResourceName: resourceName, ImportState: true, ImportStateVerify: true},
			{
				Config: mtlsTrustStoreResourceConfig(name+"-renamed", bundle),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionCreateBeforeDestroy),
				}},
				Check: check(name+"-renamed", bundle),
			},
			{
				Config: mtlsTrustStoreResourceConfig(name+"-renamed", replacement),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionCreateBeforeDestroy),
				}},
				Check: check(name+"-renamed", replacement),
			},
			{Config: mtlsTrustStoreResourceConfig(name+"-renamed", replacement), PlanOnly: true},
		},
	})
}

func TestAccIORiverClientMTLSTrustStore_ReadMissing(t *testing.T) {
	name := mtlsTestNamePrefix + generateRandomResourceName()
	config := mtlsTrustStoreResourceConfig(name, mtlsAcceptanceBundle(t))
	var store ioriver.ViewerMTLSTrustStore
	var testedObj TestedClientMTLSTrustStore
	resourceName := clientMTLSTrustStoreResourceType + ".test"
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreEnvVariable(t, "IORIVER_API_TOKEN") },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(state *terraform.State) error {
			return testAccCheckResourceDestroy[ioriver.ViewerMTLSTrustStore](state, testedObj, clientMTLSTrustStoreResourceType)
		},
		Steps: []resource.TestStep{
			{
				Config: config,
				Check:  testAccCheckObjectExists[ioriver.ViewerMTLSTrustStore](resourceName, &store, testedObj),
			},
			{
				PreConfig: func() {
					if err := testAccClient.DeleteViewerMTLSTrustStore(store.Id); err != nil {
						t.Fatal(err)
					}
				},
				Config: config,
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionCreate),
				}},
				Check: testAccCheckObjectExists[ioriver.ViewerMTLSTrustStore](resourceName, &store, testedObj),
			},
		},
	})
}

// Test-only resource reproduces the original raw API-to-state conversion.
type untrimmedStateTrustStoreResource struct {
	ClientMTLSTrustStoreResource
}

func (r *untrimmedStateTrustStoreResource) Create(ctx context.Context, req frameworkresource.CreateRequest, resp *frameworkresource.CreateResponse) {
	var data ClientMTLSTrustStoreResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if newData := resourceCreate(r.client, ctx, req, resp, r, data, false); newData != nil {
		resp.Diagnostics.Append(resp.State.Set(ctx, newData)...)
	}
}

func (r *untrimmedStateTrustStoreResource) objToResource(ctx context.Context, obj interface{}, data interface{}) (interface{}, error) {
	store := obj.(*ioriver.ViewerMTLSTrustStore)
	return ClientMTLSTrustStoreResourceModel{
		Id: types.StringValue(store.Id), Name: types.StringValue(store.Name), Bundle: types.StringValue(store.Bundle),
	}, nil
}

type whitespaceRegressionProvider struct {
	IORiverProvider
}

func (p *whitespaceRegressionProvider) Resources(ctx context.Context) []func() frameworkresource.Resource {
	return []func() frameworkresource.Resource{
		func() frameworkresource.Resource { return &untrimmedStateTrustStoreResource{} },
	}
}

func TestAccIORiverClientMTLSTrustStore_Whitespace(t *testing.T) {
	canonical := mtlsAcceptanceBundle(t)
	bundle := " \t\n" + canonical + "\n \t"
	config := mtlsTrustStoreResourceConfig(mtlsTestNamePrefix+"whitespace-"+generateRandomResourceName(), bundle)
	checkDestroy := func(state *terraform.State) error {
		for _, rs := range state.RootModule().Resources {
			if rs.Type != clientMTLSTrustStoreResourceType || rs.Primary.ID == "" {
				continue
			}
			_, err := testAccClient.GetViewerMTLSTrustStore(rs.Primary.ID)
			var apiErr *ioriver.APIError
			if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusNotFound {
				return fmt.Errorf("trust store %s was not confirmed deleted: %v", rs.Primary.ID, err)
			}
		}
		return nil
	}
	t.Run("without_fix_expected_failure", func(t *testing.T) {
		resource.Test(t, resource.TestCase{
			PreCheck: func() { testAccPreEnvVariable(t, "IORIVER_API_TOKEN") },
			ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
				"ioriver": providerserver.NewProtocol6WithError(&whitespaceRegressionProvider{}),
			},
			CheckDestroy: checkDestroy,
			Steps: []resource.TestStep{{
				Config:      config,
				ExpectError: regexp.MustCompile(`(?s)Provider produced inconsistent result after apply.*\.bundle:`),
			}},
		})
	})
	t.Run("with_fix", func(t *testing.T) {
		resource.Test(t, resource.TestCase{
			PreCheck:                 func() { testAccPreEnvVariable(t, "IORIVER_API_TOKEN") },
			ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
			CheckDestroy:             checkDestroy,
			Steps: []resource.TestStep{
				{
					Config: config,
					Check: resource.ComposeTestCheckFunc(
						resource.TestCheckResourceAttr("ioriver_client_mtls_trust_store.test", "bundle", bundle),
						func(state *terraform.State) error {
							id := state.RootModule().Resources["ioriver_client_mtls_trust_store.test"].Primary.ID
							store, err := testAccClient.GetViewerMTLSTrustStore(id)
							if err != nil {
								return err
							}
							if store.Bundle != canonical {
								return fmt.Errorf("expected API to trim outer whitespace; returned length %d, canonical length %d", len(store.Bundle), len(canonical))
							}
							return nil
						},
					),
				},
				{Config: config, PlanOnly: true},
				{
					ResourceName: "ioriver_client_mtls_trust_store.test", ImportState: true,
					ImportStateCheck: func(states []*terraform.InstanceState) error {
						if len(states) != 1 || states[0].Attributes["bundle"] != canonical {
							return fmt.Errorf("import must read the trimmed API bundle")
						}
						return nil
					},
				},
			},
		})
	})
}
