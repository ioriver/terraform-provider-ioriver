package provider

import (
	"fmt"
	"os"
	"testing"

	"golang.org/x/exp/slices"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	ioriver "github.com/ioriver/ioriver-go"
)

var certResourceType string = "ioriver_certificate"

func init() {
	var testedObj TestedCertificate
	excludeId := os.Getenv("IORIVER_TEST_CERT_ID")
	resource.AddTestSweepers(certResourceType, &resource.Sweeper{
		Name: certResourceType,
		F: func(r string) error {
			return testSweepResources[ioriver.Certificate](r, testedObj, []string{excludeId})
		},
	})
}

type TestedCertificate struct {
	TestedObj[ioriver.Certificate]
}

func (TestedCertificate) Get(client *ioriver.IORiverClient, id string) (*ioriver.Certificate, error) {
	return client.GetCertificate(id)
}

func (TestedCertificate) List(client *ioriver.IORiverClient) ([]ioriver.Certificate, error) {
	return client.ListCertificates()
}

func (TestedCertificate) Delete(client *ioriver.IORiverClient, object ioriver.Certificate, excludeIds []string) error {
	idx := slices.IndexFunc(excludeIds, func(id string) bool { return id == object.Id })
	if idx < 0 {
		return client.DeleteCertificate(object.Id)
	} else {
		return nil
	}
}

func TestAccIORiverCertificate_Basic(t *testing.T) {
	var certificate ioriver.Certificate
	var testedObj TestedCertificate

	rndName := generateRandomResourceName()
	resourceName := certResourceType + "." + rndName

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(s *terraform.State) error {
			return testAccCheckResourceDestroy[ioriver.Certificate](s, testedObj, certResourceType)
		},
		Steps: []resource.TestStep{
			{
				Config: testAccCheckCertificateConfig(rndName, rndName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ioriver.Certificate](resourceName, &certificate, testedObj),
					resource.TestCheckResourceAttr(resourceName, "name", rndName),
				),
			},
			{
				ResourceName:      "ioriver_certificate." + rndName,
				ImportState:       true,
				ImportStateVerify: true,
				// ignore since these fields cannot be read
				ImportStateVerifyIgnore: []string{"certificate", "private_key", "certificate_chain"},
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ioriver.Certificate](resourceName, &certificate, testedObj),
				),
			},
		},
	})
}

func TestAccIORiverCertificate_Update(t *testing.T) {
	var certificate ioriver.Certificate
	var testedObj TestedCertificate

	rndName := generateRandomResourceName()
	resourceName := certResourceType + "." + rndName
	updatedCertName := rndName + "_updated"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(s *terraform.State) error {
			return testAccCheckResourceDestroy[ioriver.Certificate](s, testedObj, certResourceType)
		},
		Steps: []resource.TestStep{
			{
				Config: testAccCheckCertificateConfig(rndName, rndName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ioriver.Certificate](resourceName, &certificate, testedObj),
					resource.TestCheckResourceAttr(resourceName, "name", rndName),
				),
			},
			{
				Config: testAccCheckCertificateConfig(rndName, updatedCertName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ioriver.Certificate](resourceName, &certificate, testedObj),
					resource.TestCheckResourceAttr(resourceName, "name", updatedCertName),
				),
			},
		},
	})
}

func testAccCheckCertificateConfig(rndName string, certName string) string {
	return fmt.Sprintf(`
resource "ioriver_certificate" "%[1]s" {
	name              = "%[2]s"
	type              = "SELF_MANAGED"
	certificate       = "-----BEGIN CERTIFICATE-----\n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\n-----END CERTIFICATE-----"
	private_key       = "-----BEGIN RSA PRIVATE KEY-----\nMIIEpAIBAAKCAQEAjD8fpCJW9FJwkGPFbzpGylMmdcauVbI5tNWEIls8traiJnUVz6uTpIA9kV7qqPlX3+o7kmhYu3mlbQIx8ZUnSAOTm0BrjYcUDMdevPhQERlEu8N6L812Dt8FYkkN7LDH40ep0eJtXCYftNGiJBZ2NxiYdk6p8NgQKTEqNEkBKemsdnkZie4hy9J2CaTNb7F+pNXLwJORhs+bTkegOJSbDbl/Qn5mScJTVO50H+gNNUwAFGie0tSexDxJ3yfiRU0OjpV8pU/Vfsw15LGbnMZJ+k6cflkCmA+uNvj6JRsVIE6xLR60pXzo9YJlBeduO4Zq6molXTtC9xscGfMvox6bfQIDAQABAoIBABUxKKfVpIwQtP+sg9X12WKTQ/mCBy/d2YhwxyGl5bu6RzBGewBBbfLqieMgk5bq7pNgQpYx/E5/6DZboY5eumvQVoqcJmRhZ+8yZSdq4jZjOhahSCJXCqLeomKipV8Bq4K1fny/mUTWYe4hyz1mw4A50Df4VQeWroJ68mSqL2nUsRDi2j/UGaTD7L956bJZLJsWp+/ti+R8y8UM+7D6hkXgh0GGDtNtg1nhNXfqfhnEY5GtHyXsIFB5jg+YAR4qPLW/vC6NKdRBjxrvTzrXb+/AqNY1q6T8UuUJeJF4/ze72qFkDaUOiVzm4+B0Tbc2fdOQx/ZGkBbkGmZTQN0JMwUCgYEA+Wyv17O3YFPwr66xfVcjaJbOZv26QhS7Nac6VTfimo8FZh0NpdAVnwMZ+P65OMyZMiAjuYNKdpJvBx69LQsJ1ixBJOljWEq2l66p6wHA8Nlr91i0Ww8q5D8F3SCYva1abm3nV9M73t9lZvSv0TE460wOiup8Vho3/MfJoa2fH28CgYEAj/GeKRR6tIVCB545DcGSEX+PAYflitmK3Us16UMTEKQqmDoqJPhwHa1aYP3EtkKiLH0a6y18tWA1EELiRm1IZMUr1hxqbJIRV+JdxMPmSAKmgELrQbERDGvoHHNKddSkbJfNUmaOHZEG6+/dd6wsvp2KbmWm2Ug27EfOHZQJ/dMCgYEA1F/g0a8qQpD2bQA4DFs3wQQ3NqZwA3gXdzWui4UMI0IH/MxcJIUrA7vmT4bEO0KqZm3LPVg2/QLuGofn2ASAGaaQyVcXycPD+R81eu6BVBIsxez3lFkz0ih/W6s3ormKOGDIDJXFcp2Qf7t0QJDCwEaAU3QY7k9gwJF0c3+b720CgYB89ea2Bv9XQ/BEqMki9g6WfkRpsc5GMgDph+dvbzlX0wzfRm9b1QmP2fSCCwwApewf7yO1UrHWy4SFb2r8dNbKFJmvsM97HXtM7kk1DlQV46cj5fRR/SOtwuen+zaDAG0VkNtAU6PAayy1GnEK+T+G40FQAZNNQfHcQaHf76qU3QKBgQDvmBlFVblVolSYvtoCxauhP9sQniHkk5SC4lHW6ZGDaSqz8cscybnjwaICQjV9t2xheqeqHfrx7EvJ2RtObriDX6GoqRffiV3wZTpbPS4Lj26Brsq4M8u9WMp7pyipzbuoxLJIQhM0Dw1PP7KyUbL3OorpQ+VkxRX8hnzQE6LW+g==\n-----END RSA PRIVATE KEY-----"
	certificate_chain = "-----BEGIN CERTIFICATE-----\n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\n-----END CERTIFICATE-----"
	}`, rndName, certName)
}
