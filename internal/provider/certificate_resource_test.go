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
	certificate       = "-----BEGIN CERTIFICATE-----\nMIIE9DCCA9ygAwIBAgISA+BXfMyh5nHs/Ro8TwCX7nQhMA0GCSqGSIb3DQEBCwUA\nMDMxCzAJBgNVBAYTAlVTMRYwFAYDVQQKEw1MZXQncyBFbmNyeXB0MQwwCgYDVQQD\nEwNSMTEwHhcNMjQwOTEwMDgwMjQ1WhcNMjQxMjA5MDgwMjQ0WjAdMRswGQYDVQQD\nDBIqLmlvcml2ZXItcWEtNS5jb20wggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEK\nAoIBAQCnwL+wfQf4S4Fbfl9X8JpUwOST9T/DaJhx01Y/TZN1J6n+mcP+uTMiFnmd\n8FBAIntwBtmkVfdMIqDI/e8I/+iTA3DmdB0fqMR17EB9/r/oAdB4yeUYZgOVEbI/\nbA2WNaQB0W/BwVW1PJBkIW22Lw2g59InZmLlJCUdc2U7muAtGwgGI6xvTxDYV9+P\nnQownCQ7HFmdl2L8ecx06qepXRArUwz0eGj5ifjbntubI4awNJ7MQOjLV8bb1Yw0\n/y2jOoSIwZRAF+rVEAK2GMHC3TFFJTei1x38bv1XZMm4togoisOtVxnLHb9sn5/y\nVHhD0DsGVdtxOg0080RDk1kJji5XAgMBAAGjggIWMIICEjAOBgNVHQ8BAf8EBAMC\nBaAwHQYDVR0lBBYwFAYIKwYBBQUHAwEGCCsGAQUFBwMCMAwGA1UdEwEB/wQCMAAw\nHQYDVR0OBBYEFM6oTQpFUVpQg6ar9IjFi6aUInehMB8GA1UdIwQYMBaAFMXPRqTq\n9MPAemyVxC2wXpIvJuO5MFcGCCsGAQUFBwEBBEswSTAiBggrBgEFBQcwAYYWaHR0\ncDovL3IxMS5vLmxlbmNyLm9yZzAjBggrBgEFBQcwAoYXaHR0cDovL3IxMS5pLmxl\nbmNyLm9yZy8wHQYDVR0RBBYwFIISKi5pb3JpdmVyLXFhLTUuY29tMBMGA1UdIAQM\nMAowCAYGZ4EMAQIBMIIBBAYKKwYBBAHWeQIEAgSB9QSB8gDwAHYASLDja9qmRzQP\n5WoC+p0w6xxSActW3SyB2bu/qznYhHMAAAGR2ypn7gAABAMARzBFAiEAg2d8FpYj\n0+Cyo9dLRIxz62hf5+R1ZQgCvkKfnU0aFM8CIEMIFkLMsyOTrvHXLIhG0FUxPQIg\n+tlXwE7/uisuF97DAHYA3+FW66oFr7WcD4ZxjajAMk6uVtlup/WlagHRwTu+UlwA\nAAGR2ypoiAAABAMARzBFAiB1SVwMp4fnohYARPFRNbOu54wdweXEsfXa88whZp33\npAIhAPgO6b6EcgG+ZSqKsASRc7qYTBUlnDrTF7iC/mLedWZ4MA0GCSqGSIb3DQEB\nCwUAA4IBAQAJ+YOy7M/yd4McwiAQlQXj3ZfXCSRINt2FWmpr7X1Zv9N9o77iqfWv\n6BPXMeAPK+HRnErd1UCyMzIwVs410eZ19wjrQb4nfkbzEQIxl6SuLOw5weANwRxy\ncdnuDDtZMgu81uUgDkDI/u/WR2FiFBkMM+37GZtAymstPSC2MDmM/UTpr3jT0FHM\nzw/fWkMF5im7B3b9uzj6SXJEf4FDlqjr5Y13cSTcAs6Nqk/hupTh3rR+U29fGEnY\nvhgBf6kmZzYDULDAKvtT3OSorW00vMP57FwHL9OoUE4pzUxa5IWY2SIPl1x2iv3b\n8d5hHCGg9roLrz1N+5VDgn6R5ves/85V\n-----END CERTIFICATE-----\n"
	private_key       = "-----BEGIN PRIVATE KEY-----\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCnwL+wfQf4S4Fb\nfl9X8JpUwOST9T/DaJhx01Y/TZN1J6n+mcP+uTMiFnmd8FBAIntwBtmkVfdMIqDI\n/e8I/+iTA3DmdB0fqMR17EB9/r/oAdB4yeUYZgOVEbI/bA2WNaQB0W/BwVW1PJBk\nIW22Lw2g59InZmLlJCUdc2U7muAtGwgGI6xvTxDYV9+PnQownCQ7HFmdl2L8ecx0\n6qepXRArUwz0eGj5ifjbntubI4awNJ7MQOjLV8bb1Yw0/y2jOoSIwZRAF+rVEAK2\nGMHC3TFFJTei1x38bv1XZMm4togoisOtVxnLHb9sn5/yVHhD0DsGVdtxOg0080RD\nk1kJji5XAgMBAAECggEABHkOcaEJKUFbB4VkF2tnevELEeLigZU7mnVdVwWnEFS9\n276W9rU2H7XHtCZEjYTIhZLyEuQCIbMCOdYuK5Ltd9YJ/EeGTtPoZK3gTJESO1Ea\n2HhRp87/pTtWj+2v+GN23wptkE7juEKdev4BolMqotx5wpS8298Gvd/0XWO35WMP\nhS9Mt797b4Ewjc2jmj75Kbn8Q4uyKHCdTF7USIIQETjltzBOIMVulh+zoRahEBAu\nGEQh4gi8lFiQXfVuWXuNfKhVyzGb1qDNclSGKMIyZUyTSwjLuCY+av+3SpYsRqpp\nnqkHnMp5PFcHewOvPPi2k6VK58aZlXgT/7Mrc7+/oQKBgQDRujL18W/AFB1CHSiT\nTiG5uF3+54E9YdPmWoUdrT79dAdoQ6uBL6WiRqJGPfwsqe0wDVFQXMjxgsLES2g8\nEDroKQkjm990IEfSjaabnfdROYELa2vQWgPKPpoKJAiHXYjaF74vmPxZVDNTKdNR\nqR7ZryeMdkxLLvz6Yfm0/OqmkQKBgQDMw8Ep4KpJzTQ96gDAaWrswqT+urccriZJ\ng3NkrPkm/SYAhFvwuu6bHHkQHM1xn5zA7pBZTHaFsFmQ5gWWTSHpmOXOGIwkkQoz\nHgx4vqugidjLFEuIWzWDjgEv+6qB2aulvo38LStfh72A2Wzpwnduu79khZstVRKT\nbI4HXyuKZwKBgQC6mZaZ6LTrC3p0xojBd3TeT0GieMwulwn4HHXvz6MJ0uB8Tikc\nCg6u8XWUVbY27wHQDSlZ/RP0fclY6VbWigI/abNt8VPBeK8ukUW5k7TmmelNBcip\nWk2g9k4L07+a4QfQM+vNYaq0uAvqZH5WW8jNGeBwQxjik+4VwHJyK82DYQKBgGSf\n9jbsLwLhksCyU/g6Vc9Pv+FmREIV2r7ZmEVrM21Tje9HHB9q3YLgNSYT4Wnq9A9j\nrRAVIVGFXh50y14XPYkcGCJ1sbjuhcKlC5/yo0jbNOxnZs71c5DYogDAfgQdwdpL\nkF7Sm7PwctH1By7A4AxMuztc5OscGWrVN96riCwVAoGAYNLeL+45OS9CNahQ7tAA\nTiMDgx+hRcy65bvZ9DmrN6UcyW2kvcTS77EQh00X/RI2mmigh8wh/mRA3U5JI5OP\n57EwEhM5DtTzR2lOzl25MSPX9KMQT/nmSIPUVVpNi6DQFCD+kmkxSq0e1kTIQUFr\ncJ05xfb2JSakjAiNl0IZLq8=\n-----END PRIVATE KEY-----\n"
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
