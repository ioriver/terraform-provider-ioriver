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
	certificate       = "-----BEGIN CERTIFICATE-----\nMIIE/zCCA+egAwIBAgISBd13q7xy+i9jQdd9j0NGBjmOMA0GCSqGSIb3DQEBCwUA\nMDMxCzAJBgNVBAYTAlVTMRYwFAYDVQQKEw1MZXQncyBFbmNyeXB0MQwwCgYDVQQD\nEwNSMTMwHhcNMjUwOTA3MTQzOTA5WhcNMjUxMjA2MTQzOTA4WjAdMRswGQYDVQQD\nDBIqLmlvcml2ZXItcWEtNS5jb20wggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEK\nAoIBAQCUhGvg5w+NJHGsFUAOKfhWjV9gORBURMJSAVI4hVoRJ4umsY7B06B1tGy6\nrH49ElqJNaQ0Xi0IunLz0isDx1JSE5M1Jz4q0MFhaS3faleWj/nR5D0kFjZIz2lV\nLYDBFFseOu/1YA7k4OzutTWqXcqyhRvIyk/pBnQB/m4lGRtQHH5QFFm9jtbA/LcQ\nk3IYUSle16CmqZd4GkdJ1JleQP9EYqQIBgaKrxSvZsYlMc82DSXutAS/PDIYyGF2\nDa5mit765fC2FLGV657AponravegLNzVi615MPA9C2QjKUTQ7E0ozYHpBN8qSP3z\n6Xyn60q5+l8mIr4cYykcNpDprKv9AgMBAAGjggIhMIICHTAOBgNVHQ8BAf8EBAMC\nBaAwHQYDVR0lBBYwFAYIKwYBBQUHAwEGCCsGAQUFBwMCMAwGA1UdEwEB/wQCMAAw\nHQYDVR0OBBYEFBFtIAjuA5ImfwH4vem8uwSYlQRpMB8GA1UdIwQYMBaAFOernw8s\nM6BT015PeMiyhA471pIzMDMGCCsGAQUFBwEBBCcwJTAjBggrBgEFBQcwAoYXaHR0\ncDovL3IxMy5pLmxlbmNyLm9yZy8wHQYDVR0RBBYwFIISKi5pb3JpdmVyLXFhLTUu\nY29tMBMGA1UdIAQMMAowCAYGZ4EMAQIBMC4GA1UdHwQnMCUwI6AhoB+GHWh0dHA6\nLy9yMTMuYy5sZW5jci5vcmcvMTUuY3JsMIIBAwYKKwYBBAHWeQIEAgSB9ASB8QDv\nAHYA3dzKNJXX4RYF55Uy+sef+D0cUN/bADoUEnYKLKy7yCoAAAGZJNNrcQAABAMA\nRzBFAiEAtUtcSRowGGdWh/vUATI2NaQjkP5K2lfFz+4oRSE4fysCICMcGh/OxVmB\nIXr/WggXSxumxVpM+rTjra5PFOXBJEUNAHUAGgT/SdBUHUCv9qDDv/HYxGcvTuzu\nI0BomGsXQC7ciX0AAAGZJNNrdgAABAMARjBEAiBU/rQdVEN9TalqTUpcOlUM2F6S\nsf0ek+gMM/KxHNUuEQIgU7pzz3Br4IaSkaqM846sb1QYyUClPtMoeEImrtzlh9Uw\nDQYJKoZIhvcNAQELBQADggEBAGtmI9Ls9GSkWfzqPfFpzahTmGNcIgRBPeZQIjbx\n0oNaIsImGsEQnhIJ1t8iYno+lSAbG9g3HB/thhxthIFch/DJ3w7W92x987yFL3LK\nAU0iuGzspD/X6xJuoCxPBJ7gRZ/4oZANPhjww6Jdz9eEo3cZuJEDMsmqOnD1o99u\nUXvDz4ipjeU4haT+bApihI4re3gkTES3ihPrxk24/ep8f5akoIY6/st4eKvqS8lh\nKRXJoLFqSXpdRHgQiZ7eVRdAP2blHMSPLvDXuPyONezR9cBFOT5okI/kQ28+UZec\nzDXOVb6jaHPsmQe+d27BHngaNoMDLCA+TImIsIu72Shf7I4=\n-----END CERTIFICATE-----\n"
	private_key       = "-----BEGIN PRIVATE KEY-----\nMIIEvwIBADANBgkqhkiG9w0BAQEFAASCBKkwggSlAgEAAoIBAQCUhGvg5w+NJHGs\nFUAOKfhWjV9gORBURMJSAVI4hVoRJ4umsY7B06B1tGy6rH49ElqJNaQ0Xi0IunLz\n0isDx1JSE5M1Jz4q0MFhaS3faleWj/nR5D0kFjZIz2lVLYDBFFseOu/1YA7k4Ozu\ntTWqXcqyhRvIyk/pBnQB/m4lGRtQHH5QFFm9jtbA/LcQk3IYUSle16CmqZd4GkdJ\n1JleQP9EYqQIBgaKrxSvZsYlMc82DSXutAS/PDIYyGF2Da5mit765fC2FLGV657A\nponravegLNzVi615MPA9C2QjKUTQ7E0ozYHpBN8qSP3z6Xyn60q5+l8mIr4cYykc\nNpDprKv9AgMBAAECggEADD8PFjB3W9ARf3sRGRnk3F4z6x9JdWlmDJUHTpnQX3G/\nVjN17g2hQZNrE8l9b1PAG2hM7fSGCh41tF00Js+cvh/XF57wxV8JUxr1KWL/be7Z\nTrFfSUZ7m21e5iMmZsVi7g82EimlkMHrR5OxpSauvCG1tMzZ4gEiN7ffJTf4xJY0\nrbZZjNStRFpb0cscQq2+tQWChmrsM59m02j6fjQUh1JdSGKd9ii4WgFR8vy+OqRZ\nSBaDPYmNEhQYdTjJ8EAYMOTACdZizndzJFfYlaIVbNgGoqDmqig7VtzYpZpr5zFT\nx5Li43jPNgqkY9gK32bAD2czYT/dCGhuyALGyxgNgQKBgQDO09UjN3vxy1fABHOU\n7lHNMj8apAzGrW5jz2z1UvEQbC/jgmkp1RneIHDZ13OU5AVtmtc8sPikENieNIK8\nmKGLPIXNIP7duHEz2eXaaznWk1XziP4BDIOKXSV8q2HKzbOVPx/FqdP3AMjpapvj\n/ewJwOSrNUn+vdJKdSigm5M1FQKBgQC306XC+V4FsUGqG4YXQKQmau1vp2ZO3vcw\nd5JgDmlidhhiZ62dkpb9NB23zPPLsBWObgWNsM7lNq9n3BtgCbGYq8LRmbUoVkP1\nPQSrEopsVATK2uG+2X8XeAyVTZE2GVVhRgXrsmox7YiueS0Qoxk9RUti0W1jdEmp\nAY54FIylSQKBgQCu6+NP5IYT6jktsdYa+DAAzmUmX+ZaRaWeDnkFRn+Qtx8NWGce\ntRcqkN9ArgIXw31/xDwTHU08XO8HZjvHy4KcorQ615QV6v76rmfCgXsqKfPAg3Tn\naDD73Wlt9fhAMBaYvAlgABC/z08cckij20Y8vYHn9qq9Isdup4WTx+AJPQKBgQCl\nyoHOph7xXVvOssIuCIPDjl6Ue9LewWMJWF4wue67+aymW8GOwt3ggXdoBLXAeBAJ\nBBuIHfWLbtWmAzLBXBzLh+XOKiXjumHSNXUXYUJszx3/YoeFHB3uqbwXj/yuYQzL\nDV9bou76FrRWPz2wqpih1PRXrHBO6VthzOCUVlw+2QKBgQCLd1FDbiMziopZse0E\nmvLLUNHcIFogTaW8c8EYIp5ODZBVf6gHaW4UGFOAFvl28B2hfzS03W8v0FlJTaKX\nQ7CNxkf9BQ0BB7nGtpui14GC71+GmH3pjPc3HV0QQ4jzsIagVJLDrMFpOT3tniGR\n6F0op2SA5mwSE9c+rOq6Iqrnhw==\n-----END PRIVATE KEY-----\n"
	certificate_chain = "\n\n-----BEGIN CERTIFICATE-----\nMIIFBTCCAu2gAwIBAgIQWgDyEtjUtIDzkkFX6imDBTANBgkqhkiG9w0BAQsFADBP\nMQswCQYDVQQGEwJVUzEpMCcGA1UEChMgSW50ZXJuZXQgU2VjdXJpdHkgUmVzZWFy\nY2ggR3JvdXAxFTATBgNVBAMTDElTUkcgUm9vdCBYMTAeFw0yNDAzMTMwMDAwMDBa\nFw0yNzAzMTIyMzU5NTlaMDMxCzAJBgNVBAYTAlVTMRYwFAYDVQQKEw1MZXQncyBF\nbmNyeXB0MQwwCgYDVQQDEwNSMTMwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEK\nAoIBAQClZ3CN0FaBZBUXYc25BtStGZCMJlA3mBZjklTb2cyEBZPs0+wIG6BgUUNI\nfSvHSJaetC3ancgnO1ehn6vw1g7UDjDKb5ux0daknTI+WE41b0VYaHEX/D7YXYKg\nL7JRbLAaXbhZzjVlyIuhrxA3/+OcXcJJFzT/jCuLjfC8cSyTDB0FxLrHzarJXnzR\nyQH3nAP2/Apd9Np75tt2QnDr9E0i2gB3b9bJXxf92nUupVcM9upctuBzpWjPoXTi\ndYJ+EJ/B9aLrAek4sQpEzNPCifVJNYIKNLMc6YjCR06CDgo28EdPivEpBHXazeGa\nXP9enZiVuppD0EqiFwUBBDDTMrOPAgMBAAGjgfgwgfUwDgYDVR0PAQH/BAQDAgGG\nMB0GA1UdJQQWMBQGCCsGAQUFBwMCBggrBgEFBQcDATASBgNVHRMBAf8ECDAGAQH/\nAgEAMB0GA1UdDgQWBBTnq58PLDOgU9NeT3jIsoQOO9aSMzAfBgNVHSMEGDAWgBR5\ntFnme7bl5AFzgAiIyBpY9umbbjAyBggrBgEFBQcBAQQmMCQwIgYIKwYBBQUHMAKG\nFmh0dHA6Ly94MS5pLmxlbmNyLm9yZy8wEwYDVR0gBAwwCjAIBgZngQwBAgEwJwYD\nVR0fBCAwHjAcoBqgGIYWaHR0cDovL3gxLmMubGVuY3Iub3JnLzANBgkqhkiG9w0B\nAQsFAAOCAgEAUTdYUqEimzW7TbrOypLqCfL7VOwYf/Q79OH5cHLCZeggfQhDconl\nk7Kgh8b0vi+/XuWu7CN8n/UPeg1vo3G+taXirrytthQinAHGwc/UdbOygJa9zuBc\nVyqoH3CXTXDInT+8a+c3aEVMJ2St+pSn4ed+WkDp8ijsijvEyFwE47hulW0Ltzjg\n9fOV5Pmrg/zxWbRuL+k0DBDHEJennCsAen7c35Pmx7jpmJ/HtgRhcnz0yjSBvyIw\n6L1QIupkCv2SBODT/xDD3gfQQyKv6roV4G2EhfEyAsWpmojxjCUCGiyg97FvDtm/\nNK2LSc9lybKxB73I2+P2G3CaWpvvpAiHCVu30jW8GCxKdfhsXtnIy2imskQqVZ2m\n0Pmxobb28Tucr7xBK7CtwvPrb79os7u2XP3O5f9b/H66GNyRrglRXlrYjI1oGYL/\nf4I1n/Sgusda6WvA6C190kxjU15Y12mHU4+BxyR9cx2hhGS9fAjMZKJss28qxvz6\nAxu4CaDmRNZpK/pQrXF17yXCXkmEWgvSOEZy6Z9pcbLIVEGckV/iVeq0AOo2pkg9\np4QRIy0tK2diRENLSF2KysFwbY6B26BFeFs3v1sYVRhFW9nLkOrQVporCS0KyZmf\nwVD89qSTlnctLcZnIavjKsKUu1nA1iU0yYMdYepKR7lWbnwhdx3ewok=\n-----END CERTIFICATE-----\n"
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
