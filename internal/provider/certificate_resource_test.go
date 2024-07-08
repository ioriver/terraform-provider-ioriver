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
				ImportStateVerify: true,
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
	certificate       = "-----BEGIN CERTIFICATE-----\nMIIE8zCCA9ugAwIBAgISAzu6qdWLfD8YbFCvxuT8cdiSMA0GCSqGSIb3DQEBCwUA\nMDMxCzAJBgNVBAYTAlVTMRYwFAYDVQQKEw1MZXQncyBFbmNyeXB0MQwwCgYDVQQD\nEwNSMTEwHhcNMjQwNjExMTMzNDA0WhcNMjQwOTA5MTMzNDAzWjAdMRswGQYDVQQD\nDBIqLmlvcml2ZXItcWEtNS5jb20wggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEK\nAoIBAQDBHKF0jwpNhdW+ycGz7DZo3TNuc4D/V3S9E5AIIGlONVxI+BUdlrigliy0\nbnmLH3DU5SjV1NYpcmS006gLlT0gW+7J7FEM6Z9E53qION19/Hd23lVddRvkipOR\nTDF9LnPm9yDOFhucm5a4Qxl0EQuu+5m/31KEXIHTRISYLCeDNNc7K90aX33/98aJ\nRAkGUnhzwsd1t8y5YveCZN1sRRHQTp8ZhLUz/fBO5Br9AmCLpYhSdka6Frl5dRli\nQMUPG7qq42nbrMBvkKWqiMMvVpJtT8QjuCVFDgMgUAI2PdL6TgMHk7nMHqbbTpfA\nnnX2OSG2o+6byibh3NeIwe25GD4BAgMBAAGjggIVMIICETAOBgNVHQ8BAf8EBAMC\nBaAwHQYDVR0lBBYwFAYIKwYBBQUHAwEGCCsGAQUFBwMCMAwGA1UdEwEB/wQCMAAw\nHQYDVR0OBBYEFKm6w5L0T1EMmxYWYwdHMFwNmkEvMB8GA1UdIwQYMBaAFMXPRqTq\n9MPAemyVxC2wXpIvJuO5MFcGCCsGAQUFBwEBBEswSTAiBggrBgEFBQcwAYYWaHR0\ncDovL3IxMS5vLmxlbmNyLm9yZzAjBggrBgEFBQcwAoYXaHR0cDovL3IxMS5pLmxl\nbmNyLm9yZy8wHQYDVR0RBBYwFIISKi5pb3JpdmVyLXFhLTUuY29tMBMGA1UdIAQM\nMAowCAYGZ4EMAQIBMIIBAwYKKwYBBAHWeQIEAgSB9ASB8QDvAHYAdv+IPwq2+5VR\nwmHM9Ye6NLSkzbsp3GhCCp/mZ0xaOnQAAAGQB7hrGgAABAMARzBFAiEAtviQ8b8R\n+GICqoN5l7GIHW5uC6agbZV6g3VtzhOUysICIG6J8KImn6kBSPMetBJz2qyp80Gt\ne9ocoujDN2L1tY6wAHUA3+FW66oFr7WcD4ZxjajAMk6uVtlup/WlagHRwTu+UlwA\nAAGQB7hrlAAABAMARjBEAiAyBIrsXNaPohK9tea/xbup470epeKi6WYDhomQDuaz\nvwIgDqkHprwoh4STswZrkPQFH//FWmNJa5O3Q9RNkOLdOmUwDQYJKoZIhvcNAQEL\nBQADggEBAJkM8GDQiVojQnhRPLFDFMLmxMeRRk5Or5nzQrnl4PuSP6GPWF0JW7rr\nNRSkk69N15UieGsDn+M+SSjc9EZDeDkUJZtBQVR92Uy5NU65qzl/I/iqvo1Zyxts\nV+AAToGzrEw+z+6x6cXOKVIkNxexFy0dSWFRrgOFDf1YCMByUfKZh9XaSYYRZ1j6\nqAKVy30YB5WatKOTlFqHpmg/cYtjQG4CqJPXwKLLfbvdkmv/CmIMu7ZorO6NX8gG\n2mokVgk3Hcps/dR3lKFQmYq//YdIup+GHPPGl1f7iSspRNruJWH/hB6OnTXiaU8v\nGpF4B9aUVwxhB0QKPneRRE9RmMveai4=\n-----END CERTIFICATE-----\n"
    private_key       = "-----BEGIN PRIVATE KEY-----\nMIIEuwIBADANBgkqhkiG9w0BAQEFAASCBKUwggShAgEAAoIBAQDBHKF0jwpNhdW+\nycGz7DZo3TNuc4D/V3S9E5AIIGlONVxI+BUdlrigliy0bnmLH3DU5SjV1NYpcmS0\n06gLlT0gW+7J7FEM6Z9E53qION19/Hd23lVddRvkipORTDF9LnPm9yDOFhucm5a4\nQxl0EQuu+5m/31KEXIHTRISYLCeDNNc7K90aX33/98aJRAkGUnhzwsd1t8y5YveC\nZN1sRRHQTp8ZhLUz/fBO5Br9AmCLpYhSdka6Frl5dRliQMUPG7qq42nbrMBvkKWq\niMMvVpJtT8QjuCVFDgMgUAI2PdL6TgMHk7nMHqbbTpfAnnX2OSG2o+6byibh3NeI\nwe25GD4BAgMBAAECgf8YMs+GipbcXPk7E7PAYjaf+YnJnq+r+jEszXuUTCorDFwW\nXxA2DqDK9JC1ymc/3xZQq4sxaQRpGNxQ6O/Qg8J2aAqsyPxKvmFLMNsRhA6vDCSv\nPoJnwd9vs8UbX7HgEi8OTFMRDkZvnoVpC4F+a19QgdUn8OhqP/BeK4D8MyaoAQBi\nwS+8PxYd30Cyoy/8y0V3q67kscQmqOhO9e5yDKzRxc4naia19oU72/YBfQ8HvVrt\nHSb+JTey4jVQWaLWIKdE7JUyfUpzFz2xeFoK7dtAVADU99LywmhVZIUPqGzaCtq+\nJW1p0wSjYcGHX0kv8xNRKVqghatjMjNrBUfZKRECgYEA3rAqB9Z48j4VodNbxv/w\nBq0vzdnKPUiNCMji7hyQRomvNZzVIGZzqfqcDzaiFMmKPJ4SAE6p5JeTn7ZCGG5G\nd6k/GWY+eKf47hAjCl02pPurSZ6tinDefcjBU29oy6dlMNQMH+JnE0VA3RLeN/uO\nLw0U/xbO83J3XT+8ooTpjy0CgYEA3f/XxbOWxeodpMlHCTrY147sFq0OK9hYcVJP\ngYphPjK1iLoDJ6Aq8mxb5hdHNP+A+eHL5c5iqo03uDDZU3vU5kjIE3LT5tt5gXcU\n7XoATgv5N1/GqzQbEvomNdvVtRn5/HM9SA1UXt/AGgOfpZBJV4SUWPMISdEeR+ji\nVJw0jqUCgYAwEfEQvhBfol1DEH/4RhlHJ61xDzlj3zxqkArEga/3OhNzTEaJdvQy\n1iFH+3Ajcpn/mdOi81gjO0Ensc00vuFBRWyUjAiiPQg0Q5F81EBOfvErtDAb+V9d\n4a8x1pPVmm3yi2OOom3DsChfUIpdpVS8/WLx6beEv8nafD65Cl3shQKBgQCrqM07\n/mYzm5hYd5sKNArAg69iyWYP2TZqSe9Yh5wx8BwXIV6XIW6UDd3xyUYyYO9mSYbU\nqAX7Qz82me8ycqppdxXelAfulv8ZcO8pwhDCSNfoTZLdh2j3/53UP6y8YN3Aq4tT\nT5tR0UpF0097Qlqz5bygGyzph14W1KlOK4soUQKBgFLXZtl7V35Dvpb1VPx5Xe4v\ne9jlRx3njlT9xXqbevP495t0yxGc75BJyD9dwR8J+u300f9aEe8+91vJTFcAyJXQ\n6zZL0Pim1w1kFPyHmdaimVw5bafcpqiRdGZFLb4A9w0B5Qywy/p/fk4tD0o75m3g\nhUrcRK1XzgoImhxoWlUg\n-----END PRIVATE KEY-----\n"
    certificate_chain = "-----BEGIN CERTIFICATE-----\nMIIFBjCCAu6gAwIBAgIRAIp9PhPWLzDvI4a9KQdrNPgwDQYJKoZIhvcNAQELBQAw\nTzELMAkGA1UEBhMCVVMxKTAnBgNVBAoTIEludGVybmV0IFNlY3VyaXR5IFJlc2Vh\ncmNoIEdyb3VwMRUwEwYDVQQDEwxJU1JHIFJvb3QgWDEwHhcNMjQwMzEzMDAwMDAw\nWhcNMjcwMzEyMjM1OTU5WjAzMQswCQYDVQQGEwJVUzEWMBQGA1UEChMNTGV0J3Mg\nRW5jcnlwdDEMMAoGA1UEAxMDUjExMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIB\nCgKCAQEAuoe8XBsAOcvKCs3UZxD5ATylTqVhyybKUvsVAbe5KPUoHu0nsyQYOWcJ\nDAjs4DqwO3cOvfPlOVRBDE6uQdaZdN5R2+97/1i9qLcT9t4x1fJyyXJqC4N0lZxG\nAGQUmfOx2SLZzaiSqhwmej/+71gFewiVgdtxD4774zEJuwm+UE1fj5F2PVqdnoPy\n6cRms+EGZkNIGIBloDcYmpuEMpexsr3E+BUAnSeI++JjF5ZsmydnS8TbKF5pwnnw\nSVzgJFDhxLyhBax7QG0AtMJBP6dYuC/FXJuluwme8f7rsIU5/agK70XEeOtlKsLP\nXzze41xNG/cLJyuqC0J3U095ah2H2QIDAQABo4H4MIH1MA4GA1UdDwEB/wQEAwIB\nhjAdBgNVHSUEFjAUBggrBgEFBQcDAgYIKwYBBQUHAwEwEgYDVR0TAQH/BAgwBgEB\n/wIBADAdBgNVHQ4EFgQUxc9GpOr0w8B6bJXELbBeki8m47kwHwYDVR0jBBgwFoAU\nebRZ5nu25eQBc4AIiMgaWPbpm24wMgYIKwYBBQUHAQEEJjAkMCIGCCsGAQUFBzAC\nhhZodHRwOi8veDEuaS5sZW5jci5vcmcvMBMGA1UdIAQMMAowCAYGZ4EMAQIBMCcG\nA1UdHwQgMB4wHKAaoBiGFmh0dHA6Ly94MS5jLmxlbmNyLm9yZy8wDQYJKoZIhvcN\nAQELBQADggIBAE7iiV0KAxyQOND1H/lxXPjDj7I3iHpvsCUf7b632IYGjukJhM1y\nv4Hz/MrPU0jtvfZpQtSlET41yBOykh0FX+ou1Nj4ScOt9ZmWnO8m2OG0JAtIIE38\n01S0qcYhyOE2G/93ZCkXufBL713qzXnQv5C/viOykNpKqUgxdKlEC+Hi9i2DcaR1\ne9KUwQUZRhy5j/PEdEglKg3l9dtD4tuTm7kZtB8v32oOjzHTYw+7KdzdZiw/sBtn\nUfhBPORNuay4pJxmY/WrhSMdzFO2q3Gu3MUBcdo27goYKjL9CTF8j/Zz55yctUoV\naneCWs/ajUX+HypkBTA+c8LGDLnWO2NKq0YD/pnARkAnYGPfUDoHR9gVSp/qRx+Z\nWghiDLZsMwhN1zjtSC0uBWiugF3vTNzYIEFfaPG7Ws3jDrAMMYebQ95JQ+HIBD/R\nPBuHRTBpqKlyDnkSHDHYPiNX3adPoPAcgdF3H2/W0rmoswMWgTlLn1Wu0mrks7/q\npdWfS6PJ1jty80r2VKsM/Dj3YIDfbjXKdaFU5C+8bhfJGqU3taKauuz0wHVGT3eo\n6FlWkWYtbt4pgdamlwVeZEW+LM7qZEJEsMNPrfC03APKmZsJgpWCDWOKZvkZcvjV\nuYkQ4omYCTX5ohy+knMjdOmdH9c7SpqEWBDC86fiNex+O0XOMEZSa8DA\n-----END CERTIFICATE-----\n"
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
