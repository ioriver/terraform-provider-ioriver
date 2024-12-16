package provider

import (
	"fmt"
	"os"
	"testing"

	"golang.org/x/exp/slices"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
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

func TestAccIORiverCertificate_BasicManaged(t *testing.T) {
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
				Config: testAccCheckCertificateConfigManaged(rndName, rndName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckObjectExists[ioriver.Certificate](resourceName, &certificate, testedObj),
					resource.TestCheckResourceAttr(resourceName, "name", rndName),
				),
			},
			{
				ResourceName:      "ioriver_certificate." + rndName,
				ImportState:       true,
				ImportStateVerify: false, // should be disabled since challenges field is populated in a delay
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
	certificate       = "-----BEGIN CERTIFICATE-----\nMIIE9jCCA96gAwIBAgISBLB/tYKcOSI8XZ0bomEkV1FDMA0GCSqGSIb3DQEBCwUA\nMDMxCzAJBgNVBAYTAlVTMRYwFAYDVQQKEw1MZXQncyBFbmNyeXB0MQwwCgYDVQQD\nEwNSMTAwHhcNMjQxMjEwMTQwNTAwWhcNMjUwMzEwMTQwNDU5WjAdMRswGQYDVQQD\nDBIqLmlvcml2ZXItcWEtNS5jb20wggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEK\nAoIBAQDdQHfl2NkjJy//4h+vzA5lAwYusMGLkuPcJ+UIixt7imzfzUbVBFrAjSw2\nKY9Ech0WVVEGo8554DD9XRsndMCFVwhkM7U7IESjdGoZrGRvwU2FsL3d3MOJ9mn0\nxKnbNQDv63SWAVmu+VXUbn62WjRzu0qdjCgYUVTCYl3Di3cJrzivNtFBe2fMgZXH\n5bAu9vfqubbkSJd23frJ2yBF1CDe9EmBiZ1D95Jj+JNjkvlavrs4psOMqlaOoao/\nSlDFUd9+CxkpC55jMl2zC/APuLaPyaKQqVxeeqdFF5oFwznATDrBglFmOBj85Rg0\neJT8iZe3+b15dAoNSQNVORdkau+ZAgMBAAGjggIYMIICFDAOBgNVHQ8BAf8EBAMC\nBaAwHQYDVR0lBBYwFAYIKwYBBQUHAwEGCCsGAQUFBwMCMAwGA1UdEwEB/wQCMAAw\nHQYDVR0OBBYEFHvE0Tz47Zb75HO0ivIkGkAomMx8MB8GA1UdIwQYMBaAFLu8w0el\n5LypxsOkcgwQjaI14cjoMFcGCCsGAQUFBwEBBEswSTAiBggrBgEFBQcwAYYWaHR0\ncDovL3IxMC5vLmxlbmNyLm9yZzAjBggrBgEFBQcwAoYXaHR0cDovL3IxMC5pLmxl\nbmNyLm9yZy8wHQYDVR0RBBYwFIISKi5pb3JpdmVyLXFhLTUuY29tMBMGA1UdIAQM\nMAowCAYGZ4EMAQIBMIIBBgYKKwYBBAHWeQIEAgSB9wSB9ADyAHcAzPsPaoVxCWX+\nlZtTzumyfCLphVwNl422qX5UwP5MDbAAAAGTsRjGlAAABAMASDBGAiEApIQUSOBs\nyegSy0XkfkJGa5SFlTeN1m8GY10F0wTKgSoCIQCOWdKGhh4rV1esnF4n0I55T4cp\nVQ/Mn+nwr6HkFDBlGgB3AM8RVu7VLnyv84db2Wkum+kacWdKsBfsrAHSW3fOzDsI\nAAABk7EYxrEAAAQDAEgwRgIhAJmKZ2hgYskT1FP9fbB7RBkNYg+uthUhe187zVSa\nNqFGAiEA9jbgzfE4iKeVT4vtQ3V1UXJT9m+Dh+YTWt/GQHbbQEEwDQYJKoZIhvcN\nAQELBQADggEBAFckooyPT/CKQ/KyxUzTXP/pO4Xzkru2hkc73ya/Zv6kxXjDw/oR\nwP3oNj5g3b2uPMm8XAAVjoCln+IgLE0S3fuGKjN3hQL/I+in3H20ZP+gR60Mggu+\nwHnRozPHOG+djAKOIzjRxk4R6vfrXsmM0LbP89IdV/mWHzHBQhW+awHyTyhgz/Oo\n6WQp6FpNEpJeOpo0Ay5ankhxPGr9YBmeKG1RC8HO3Y1TS4jytEdJbbpPcWYwFs4u\n+RyyPCz6O/BO4oVtqeMmt+m4E1GVjQ6/U2HnDqgk0N0uroa2+Qcd7qZFZZxDqSlb\n+C7L6taXaf1cPAorvfHNoM1tWZbS5KhzJVY=\n-----END CERTIFICATE-----\n"
	private_key       = "-----BEGIN PRIVATE KEY-----\nMIIEvgIBADANBgkqhkiG9w0BAQEFAASCBKgwggSkAgEAAoIBAQDdQHfl2NkjJy//\n4h+vzA5lAwYusMGLkuPcJ+UIixt7imzfzUbVBFrAjSw2KY9Ech0WVVEGo8554DD9\nXRsndMCFVwhkM7U7IESjdGoZrGRvwU2FsL3d3MOJ9mn0xKnbNQDv63SWAVmu+VXU\nbn62WjRzu0qdjCgYUVTCYl3Di3cJrzivNtFBe2fMgZXH5bAu9vfqubbkSJd23frJ\n2yBF1CDe9EmBiZ1D95Jj+JNjkvlavrs4psOMqlaOoao/SlDFUd9+CxkpC55jMl2z\nC/APuLaPyaKQqVxeeqdFF5oFwznATDrBglFmOBj85Rg0eJT8iZe3+b15dAoNSQNV\nORdkau+ZAgMBAAECggEADokk32ZD3Lv/OqzO12A8/VypoDFnY820l+MMmGc4OOF3\nJexw9/97XV1g/1VzZsQm/k/AXSLvpqDoowqmBS111QTK5bdic5YbAFCehF2HszJk\nTxFVpgDyHUshuq2deav1qe2CiTThQR47KfPQ8hbCzawrOqbJvxA/1O38ukM8QLfh\nmZ6XjeUoAyfb9FVSNxLuxtsTpf3lagDLM2hEN1F6Ylf48CuVxD5zqWqGGQMajE10\nN8ngs+3ENwfaWBOlPqNSnw9VKiP2tt+s1qzyTdjZMATdtKPeBu7V4VIZV5Xd5kG/\nXkzf7ld9NwlwybjSn2+/6kTCkYX3BG/S9X5NHG5BsQKBgQD83mVOcy5EIxRIOo37\nZdyP4CRYCpINHcELc/1Cke+elAWmgArLq49xBYk/Jvx10T3l03AYi++fTEqUSvCs\nnf6E95kyK0xJ0jF+YlMSfwL79XB15FaYQBsW/u4xS28GG2RIVyZA2BHxL4p4ZWwP\nmWYnQgO+IRquz+bcXWvcKkPw6QKBgQDf/diCVqGiI2VfrNLYrny/+eJ+WehIKdjd\n32//L9d5zvFifmu6tQPTCsC5mfnENbDfqsH/sE7aJI45jRqAPzktVHOgyfAQZq0w\nTgWth121QnmYUeJ8fBX7TWyX502gDKZjVibtDc7mF9RjUP5GgsUPWfQiXQWZ6dVT\nqxRBDC9bMQKBgQCzbnwkdsbVwq6ZsjMduOIRldM0RgvtErfxEJUdr8CAnjiENUdz\nzoEyieMh1OBAGgH6G1bnlCSsvM4O/D5bvqDkaW1jlCXGHEjSjaK09TuA3mC2xxhL\nYPHYF32drRFTHAzE6FJUoP3aTwnK9O0BBLDgGo/dUlBEy3Hd3My0pakgQQKBgDgw\nPXe0s6cwqeVuPRYN701ZKe/4LcSserJtsnKZVbxApVzHyc50ShlOwTnN3lHSBiSV\neUTivWULdoIJW3SAZ59vZ/IVthrnJtrPN61oH6Gwo0AKin4fwoPf0DXq2BYmFjcl\nDW654AToRWcEkkSt2TEYp51XKGiLt+/dIp+OpvKBAoGBAImHgp0pdAiYdYYBIeM1\nOsZYVSfoZVVptrai7ohzh1duCVvemmqCe4H0rwWBf4T88W0t/JGhdr9Hom0L2/H9\nvO+XUuAP2TXoJRzl754L2g4VtXhclhDGnceeA+NLVS5w/XP/rYvsvXXK8o2+kxSG\nYKdKrICo2U4MKu6StOtbbMWh\n-----END PRIVATE KEY-----\n"
	certificate_chain = "-----BEGIN CERTIFICATE-----\nMIIFBTCCAu2gAwIBAgIQS6hSk/eaL6JzBkuoBI110DANBgkqhkiG9w0BAQsFADBP\nMQswCQYDVQQGEwJVUzEpMCcGA1UEChMgSW50ZXJuZXQgU2VjdXJpdHkgUmVzZWFy\nY2ggR3JvdXAxFTATBgNVBAMTDElTUkcgUm9vdCBYMTAeFw0yNDAzMTMwMDAwMDBa\nFw0yNzAzMTIyMzU5NTlaMDMxCzAJBgNVBAYTAlVTMRYwFAYDVQQKEw1MZXQncyBF\nbmNyeXB0MQwwCgYDVQQDEwNSMTAwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEK\nAoIBAQDPV+XmxFQS7bRH/sknWHZGUCiMHT6I3wWd1bUYKb3dtVq/+vbOo76vACFL\nYlpaPAEvxVgD9on/jhFD68G14BQHlo9vH9fnuoE5CXVlt8KvGFs3Jijno/QHK20a\n/6tYvJWuQP/py1fEtVt/eA0YYbwX51TGu0mRzW4Y0YCF7qZlNrx06rxQTOr8IfM4\nFpOUurDTazgGzRYSespSdcitdrLCnF2YRVxvYXvGLe48E1KGAdlX5jgc3421H5KR\nmudKHMxFqHJV8LDmowfs/acbZp4/SItxhHFYyTr6717yW0QrPHTnj7JHwQdqzZq3\nDZb3EoEmUVQK7GH29/Xi8orIlQ2NAgMBAAGjgfgwgfUwDgYDVR0PAQH/BAQDAgGG\nMB0GA1UdJQQWMBQGCCsGAQUFBwMCBggrBgEFBQcDATASBgNVHRMBAf8ECDAGAQH/\nAgEAMB0GA1UdDgQWBBS7vMNHpeS8qcbDpHIMEI2iNeHI6DAfBgNVHSMEGDAWgBR5\ntFnme7bl5AFzgAiIyBpY9umbbjAyBggrBgEFBQcBAQQmMCQwIgYIKwYBBQUHMAKG\nFmh0dHA6Ly94MS5pLmxlbmNyLm9yZy8wEwYDVR0gBAwwCjAIBgZngQwBAgEwJwYD\nVR0fBCAwHjAcoBqgGIYWaHR0cDovL3gxLmMubGVuY3Iub3JnLzANBgkqhkiG9w0B\nAQsFAAOCAgEAkrHnQTfreZ2B5s3iJeE6IOmQRJWjgVzPw139vaBw1bGWKCIL0vIo\nzwzn1OZDjCQiHcFCktEJr59L9MhwTyAWsVrdAfYf+B9haxQnsHKNY67u4s5Lzzfd\nu6PUzeetUK29v+PsPmI2cJkxp+iN3epi4hKu9ZzUPSwMqtCceb7qPVxEbpYxY1p9\n1n5PJKBLBX9eb9LU6l8zSxPWV7bK3lG4XaMJgnT9x3ies7msFtpKK5bDtotij/l0\nGaKeA97pb5uwD9KgWvaFXMIEt8jVTjLEvwRdvCn294GPDF08U8lAkIv7tghluaQh\n1QnlE4SEN4LOECj8dsIGJXpGUk3aU3KkJz9icKy+aUgA+2cP21uh6NcDIS3XyfaZ\nQjmDQ993ChII8SXWupQZVBiIpcWO4RqZk3lr7Bz5MUCwzDIA359e57SSq5CCkY0N\n4B6Vulk7LktfwrdGNVI5BsC9qqxSwSKgRJeZ9wygIaehbHFHFhcBaMDKpiZlBHyz\nrsnnlFXCb5s8HKn5LsUgGvB24L7sGNZP2CX7dhHov+YhD+jozLW2p9W4959Bz2Ei\nRmqDtmiXLnzqTpXbI+suyCsohKRg6Un0RC47+cpiVwHiXZAW+cn8eiNIjqbVgXLx\nKPpdzvvtTnOPlC7SQZSYmdunr3Bf9b77AiC/ZidstK36dRILKz7OA54=\n-----END CERTIFICATE-----\n"
	}`, rndName, certName)
}

func testAccCheckCertificateConfigManaged(rndName string, certName string) string {
	return fmt.Sprintf(`
resource "ioriver_certificate" "%[1]s" {
	name              = "%[2]s"
	type              = "MANAGED"
	cn                = "[\"test.example.com\"]"
	}`, rndName, certName)
}
