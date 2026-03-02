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
	certificate       = "-----BEGIN CERTIFICATE-----\nMIIEnjCCAoagAwIBAgIUZW3ObAMV6FjrbcJAG6liOdTxGdIwDQYJKoZIhvcNAQEL\nBQAwYzELMAkGA1UEBhMCVVMxCzAJBgNVBAgMAkNBMRUwEwYDVQQHDAxTYW5GcmFu\nY2lzY28xFjAUBgNVBAoMDUV4YW1wbGVSb290Q0ExGDAWBgNVBAMMD0V4YW1wbGUg\nUm9vdCBDQTAeFw0yNTEyMDgxNjQ5MDlaFw0yODEyMDcxNjQ5MDlaMB0xGzAZBgNV\nBAMMEiouaW9yaXZlci1xYS01LmNvbTCCASIwDQYJKoZIhvcNAQEBBQADggEPADCC\nAQoCggEBAMES4UzKqhiGPb1WBaIHCwcjLXm1OB+it/b8wZ5L8FTjoGVSeKUp79M0\n4CsqhWOrJxm5B8BGX55jnpGNlZkY9DUjhubMgioU9Kvkv4LbEDrVK9+ezGmnumhL\nNKIAlNfppE2MsEBubB4rG2cVsiUdple/Aj7sMPxW0nDVMuezH7N6mHgGPyJ6kEyH\n5Qnqw/XKyEVpQuEjhJtBsNX/qsOZB1A+FUdAFAHp833COpdeN839L2DDf6VPaEFB\nb7sgNVTRIOaijt/MqPvp+BL367Yju7hsjeW5nFpYUSyTmbV/nTDokVWIVY/hGpdy\n6J+Fpi7oGfUiBj864KrCq3OmEenXTLkCAwEAAaOBjzCBjDAfBgNVHSMEGDAWgBT7\nVi6yldLXX99S7T8h5UeKN9022DAJBgNVHRMEAjAAMAsGA1UdDwQEAwIFoDATBgNV\nHSUEDDAKBggrBgEFBQcDATAdBgNVHREEFjAUghIqLmlvcml2ZXItcWEtNS5jb20w\nHQYDVR0OBBYEFKVUPgakKQThuG1SvKJah9AutuJ9MA0GCSqGSIb3DQEBCwUAA4IC\nAQAyRcdilMAk53HBoe3NRghx5F/G5NNbUK6udXsvmELHqxq0bzxheJn20i0B28I1\nQ8VoXEmGa6tJOHQDUreCQO6i0Z7XxSSiCr4hdE4jyqL+rysRpN4kOAJB/5iAl5N/\nguExbMU+mntOdadgCfwfnd4Sf5gilqcaTNHL627UuZ6Omct5VZ4C8uB2EfUdFgxH\ntG46SAeOnRlai2U8ww4vVd9tgXGJoDQsl+s8kYXaix5se2aASWTuoWJMtflWhqQI\nMabL9+sx75end9TiLzVmPQHiYT0S2DUKL1rdceqcoQDM20816u8W5jyuWJCKJ+nc\nQ0ZoZHk4Z35E5aXKnecZlDSXNTzyTBhOmZbMjddUvFnKxpVo6ae0BoRa6pBjSTiy\nGhQTttoR7WrFpidoamtApDo55ofIBPWw1zJ2Sn23ohkj/1QgD7AQZHLoFx8qpGbV\nphAyebUpfqlUAIQYSZxGeE0qK3lnWsvp31fuKzEwxGpDn7Jtpry1lw4csQOku88n\nESagO3SuQ9tRimoFJEngrL8wmxZL0N/WYsyAlJAXLVK4ee8RMNq0hiJDXWRN14Ec\nX1P1H8b5g/zvGXVSixkwoq6TdTRlTdqF98vwW4fuKxzWqu7XEw3UUICAN5/TTwk0\n6XTSBI7h7L2BoJBVXHz2EGo/3IuidjE80umyf15pw9EecA==\n-----END CERTIFICATE-----\n"
	private_key       = "-----BEGIN PRIVATE KEY-----\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQDBEuFMyqoYhj29\nVgWiBwsHIy15tTgforf2/MGeS/BU46BlUnilKe/TNOArKoVjqycZuQfARl+eY56R\njZWZGPQ1I4bmzIIqFPSr5L+C2xA61Svfnsxpp7poSzSiAJTX6aRNjLBAbmweKxtn\nFbIlHaZXvwI+7DD8VtJw1TLnsx+zeph4Bj8iepBMh+UJ6sP1yshFaULhI4SbQbDV\n/6rDmQdQPhVHQBQB6fN9wjqXXjfN/S9gw3+lT2hBQW+7IDVU0SDmoo7fzKj76fgS\n9+u2I7u4bI3luZxaWFEsk5m1f50w6JFViFWP4RqXcuifhaYu6Bn1IgY/OuCqwqtz\nphHp10y5AgMBAAECggEAMdjGAiVwtNQzrGZBHgvjdPxICVwYGVLRXBr1ggDpE4GR\nL5eTPlENceH83igkOA9AEQwMTD/e/+2IStva+6PNqMp7UasLEAJJCPgN2aLlFctj\ngGBnNf/vyG1iMVElHHldygfAmWHo2AEZGgwn6h01jQHreoNQQlXIDwl8EwXT8WoJ\n9MnEvo6X63UUZN7phJifP4Im+JNp5FLuYwLA5lYsB/9TIuOXMbxmRJM15BLhTTCe\nXCoyZYmtAfcfVLTNLitpMeYTwS+nD4E171lMM+jyI4yaWdRU7XnW7/zwgndISO++\ndaP+iGUdsMHf/7VrivL63OK+JsHZSU8Xc+XSDqBS4QKBgQD28fGc1M3dtE20kSbe\nT/seAWjwNTwCCT37qqJAAJIN7NmB+Z6ZYq/iVsga7+Cyya4BHYfP1FBoe3ah6Znf\n9jUS8yfuMeJ6CvNFbCBXNXFrYJl8v+CH+q/jcEf113Oby3IaIZzK3e1Tu9NiRDSH\nYezSHZ1gwqECN6WVaSoMri6aJwKBgQDIJz/xdXJCHgWo/43RBI2j2Tj+JOiP44XI\n7hnwQK5WQPd4hRYhVbN4YoGmvvXfjcgqzRbpjnvAxtGvBC2VHOA1sWbvAOWUgbk/\niLb3b09VHd2CGiCDGRwyhOvjyRwu53Wgo1xovMpo9oAp+qUSsOwvB9lzHH7BWixy\nWXGkVWeOHwKBgQCqwHkUvIDtADOK26NIrX0yLj9leSnZLpLRZhdysfJL9q4flX75\nCKgdlWwgVCXG+nV7B/RU3LYMyPIq2uAvYIsqY0AFEDFNuiykoDNsmeOnH9CB1htn\nawwb9BOOBkBGRdLMBtnn3LSx5XowxICd7DRYxWmA8pNqeRfhzCnrQrWumQKBgBsn\nUAJ294BGyGfL+7ZeksSmxJed9DsJF+5Rdw1kCQLEn44nKABvuwBbBNHVWE/y0TQV\nTMV0wg8+KdY/j9uJ5lUCcz97dKn4C2S2LHRXEoEuow1yc/S1JGEqLUJi10L5vbiE\nURYYfrFMt8h6K4jknbYnr3VxaTTcAemlfshXmcvrAoGAQQ1OIFfAW0cN1DCA59qf\n8DP5qb+3ZdbsnfSbLTLxwUTt6FBo2uArCIMMPF1lFiKyiLzgFv3+z1/PLrUY62cH\nXyBkuYRtSb/eXweqLZtvqcQ83G6Q3ZOhHrVsde8qSGESInCwKJK4OsQ83F5j5hyJ\ncsg58ljgDh2WdPQufPZDnrY=\n-----END PRIVATE KEY-----\n"
	certificate_chain = "-----BEGIN CERTIFICATE-----\nMIIFpzCCA4+gAwIBAgIUebjrtaxvGsS3DTl0ieroDIdMRrMwDQYJKoZIhvcNAQEL\nBQAwYzELMAkGA1UEBhMCVVMxCzAJBgNVBAgMAkNBMRUwEwYDVQQHDAxTYW5GcmFu\nY2lzY28xFjAUBgNVBAoMDUV4YW1wbGVSb290Q0ExGDAWBgNVBAMMD0V4YW1wbGUg\nUm9vdCBDQTAeFw0yNTEyMDgxNjQ1MDJaFw0zNTEyMDYxNjQ1MDJaMGMxCzAJBgNV\nBAYTAlVTMQswCQYDVQQIDAJDQTEVMBMGA1UEBwwMU2FuRnJhbmNpc2NvMRYwFAYD\nVQQKDA1FeGFtcGxlUm9vdENBMRgwFgYDVQQDDA9FeGFtcGxlIFJvb3QgQ0EwggIi\nMA0GCSqGSIb3DQEBAQUAA4ICDwAwggIKAoICAQDL5IEr44GqXxigX9xKATWyN/cz\n6IK3FNgWIlylB+Jp9UVRj+pKo3+iuWICFNdxJtYqNRbQr3HBQzK6wzJBBIv+W7/W\nDH6TxrBbYju5J22F1dzPXBCivziAeytfHxiIdo3N+IIIWlTaihiJVpHHn4VYnR3J\nVhXrfx3dMBPq8ygR1D99BbzBAlyieu7et2NIReljZ6H3dNxepUg63ga4nma34EI1\nbPmqXFNGspMnfnlGqfkWhUWhh6clAr7HbLrkpfLZwHfAGvuDY7eysq2B0dTp6E4E\n81e/pBL/TYCUW7OyfeO6fAxFFcuUTKU0cfGSM3w1i1C25jK7xPaXsj8PM+HArLAu\nlReqPWDEu728ztXoKhvN0x21LqaaVu8A6d1c8VOHs7C5J8sUYW7c6ij/jNOAMbUb\nMeppiDnKxPvUZOs8CnJwOf6p6c7zaNPZEOxSPtzT3J5Oi5grf60cJxIcE/O5Lf+n\nwCDlPFiZ5Mi6wyGncoKfk62BFT6OqNYQbStK3/TsK1FzO8gaUHTm/dRP8MXI7gOQ\n9KaP2cV/w/b0wlfbVJ0OF0RXzhFbcJEVn/FhoQM8s5ymxbQN2UtsWtmsfi7oS8DF\nPdRpfE62bNpn4a/I8Affr7iNouyHr+Pu9lVK0l030MaG7ieaHCfPb6GisN11jwI8\nccNZtgfCSKTMko0AowIDAQABo1MwUTAdBgNVHQ4EFgQU+1YuspXS11/fUu0/IeVH\nijfdNtgwHwYDVR0jBBgwFoAU+1YuspXS11/fUu0/IeVHijfdNtgwDwYDVR0TAQH/\nBAUwAwEB/zANBgkqhkiG9w0BAQsFAAOCAgEAFeClhLmSjXyhdWB7EeVjfTKpGG3b\nkayKuJ87hXigHSLapetNNYP1jr4wVfRc3JZOkepnFtyXU21SCazBRPWQtEypBKiR\ndFQABE+ykk9FR6ewaMnU3/n5Q0z7Rp3ugVs258EBM9ahLMw8vWS0129pbzm3RMyj\nRo50coQzb6yjjCXPlwxuU9GXGhUJYS0SdzzpwIAW51fJNPSYGHzhwlN1qwhWJ7kz\n5K//QdBrgQok9NIJE9CBxhC0ZDEUfvPUkjUuKSL556OsPBfsNVyzffZhnAAt6Zzf\n8vFEYlsEIkxBNNQ8Mcsp3HZfls+HCDjVyuDltvabXn47sZzIwUtDRJqisgFKOApQ\nD5leGUctu7rGi8AOPEUx8/ECZ/QzX+hBBq3XXdJA+/jyVNTXlVovjYutwZ+ADyK9\ns2CSVA0fn/q/f/pLqCEt/LwT8XZ/4RePRymJzBDBAw+BnY8ZZCtnzoXKVpS3/mlM\nTA9h7PccRSr6KqvJVlUuRitQ89416qleLW0j5HUGRY7ZHR8G61ZC8CISjHsBRfxO\n5/3Op28fyAhwZZnUBXOmSzQr7fK7ZBvSSYNq8+CkWxBHE/JgtMhX0rD4BAOI1o2A\nNl846TTr8E2rf4TlGLWEArpiJyYC3IaiPpU6X2g5mG8xT/rHTZbP36nmlAaYTAjp\n3Wq/f7jvzmlEW9M=\n-----END CERTIFICATE-----\n"
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
