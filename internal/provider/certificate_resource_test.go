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
	certificate       = "-----BEGIN CERTIFICATE-----\nMIIFADCCA+igAwIBAgISBjxwS3CAqoSay4WAnlD1T5lnMA0GCSqGSIb3DQEBCwUA\nMDMxCzAJBgNVBAYTAlVTMRYwFAYDVQQKEw1MZXQncyBFbmNyeXB0MQwwCgYDVQQD\nEwNSMTAwHhcNMjUwNjA5MTEzMDQxWhcNMjUwOTA3MTEzMDQwWjAdMRswGQYDVQQD\nDBIqLmlvcml2ZXItcWEtNS5jb20wggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEK\nAoIBAQC4qoTyGEkQo7LXeuKblfyQMIJII0eZBdka0RIwSWAxqkZJfU+GRYYQZEp5\n10FCw0WSLt2h6TUP1JbWu17hZum+PApRTnLv76OYSjQBCampdp5IYfgdV8x006F1\nsE3DYjQuh/4f8gY7lDL297OhbwS5bTtJVZsiRj7ftcenF1uCD611x8s9l0aslBmT\nbO7TQay0uqeXKKP1GBnh/Zq+HmTBToymz9XBjcrCXoCW9KMjSvgBMdiQhpJgZ3FC\nqlZb8rNzC2032pDK1ywiUW/giNKUxE+RsRerhDa8OPaKse6Btewtb6Ot0tZXVifr\nLjHgAOdafPyTI9sJ7fim4fN3KnidAgMBAAGjggIiMIICHjAOBgNVHQ8BAf8EBAMC\nBaAwHQYDVR0lBBYwFAYIKwYBBQUHAwEGCCsGAQUFBwMCMAwGA1UdEwEB/wQCMAAw\nHQYDVR0OBBYEFKZpXjAv7reNzqHV1oLFDRhCd8MxMB8GA1UdIwQYMBaAFLu8w0el\n5LypxsOkcgwQjaI14cjoMDMGCCsGAQUFBwEBBCcwJTAjBggrBgEFBQcwAoYXaHR0\ncDovL3IxMC5pLmxlbmNyLm9yZy8wHQYDVR0RBBYwFIISKi5pb3JpdmVyLXFhLTUu\nY29tMBMGA1UdIAQMMAowCAYGZ4EMAQIBMC8GA1UdHwQoMCYwJKAioCCGHmh0dHA6\nLy9yMTAuYy5sZW5jci5vcmcvMTA0LmNybDCCAQMGCisGAQQB1nkCBAIEgfQEgfEA\n7wB2AKRCxQZJYGFUjw/U6pz7ei0mRU2HqX8v30VZ9idPOoRUAAABl1SqiZYAAAQD\nAEcwRQIgTXOrXF/nll/ZbF4acQhQ6DkoMbI7Q+pbR31+HlJz/scCIQC7Ygesduwf\nRblP4tCn1Bps6H+L8L/OzSsp46zAwypsggB1ABLxTjS9U3JMhAYZw48/ehP457Vi\nh4icbTAFhOvlhiY6AAABl1SqiZ4AAAQDAEYwRAIgYkGX/Po+BLjBeWViiqIDGvig\nwF13BcbRHzU/ZfHZeFoCIBzUR122qIbXVu7Og9I9gqedQzjL4IhGqr2OxGmbHEgA\nMA0GCSqGSIb3DQEBCwUAA4IBAQDNGBqneUkEH/vzipvyzJ8q3Sb40fdVCRVY+Rjz\nlZaOmiDOtRSRJHR/Mo0qGQWUjJlIt7IGiisLWFrd9D+36Abukg3UtcEQrnX9IOA5\nlkiUJ83fxHFM4/Sc8zEvzKim8JFMc8d0uvkSeYxD6csJJcCo8dYgZnjVs8C53lnT\nXlBhk1B9MeDpaMKgVuaIxOLDUBWu2RINcWyWAQ+ZU/fzvlBS6gxRYQRpV+YAGM6p\nLPHnPBYnBItxKNBrD3BGCMWdWs+Sf95e50BJ/igcthsPgGEJBG/h1qD5WGAjRjwM\n/Cu79Of6V/DWb3OETCVztzjPGjWWkEt2EUZdLTQ6kkGXIDpg\n-----END CERTIFICATE-----\n"
	private_key       = "-----BEGIN PRIVATE KEY-----\nMIIEvgIBADANBgkqhkiG9w0BAQEFAASCBKgwggSkAgEAAoIBAQC4qoTyGEkQo7LX\neuKblfyQMIJII0eZBdka0RIwSWAxqkZJfU+GRYYQZEp510FCw0WSLt2h6TUP1JbW\nu17hZum+PApRTnLv76OYSjQBCampdp5IYfgdV8x006F1sE3DYjQuh/4f8gY7lDL2\n97OhbwS5bTtJVZsiRj7ftcenF1uCD611x8s9l0aslBmTbO7TQay0uqeXKKP1GBnh\n/Zq+HmTBToymz9XBjcrCXoCW9KMjSvgBMdiQhpJgZ3FCqlZb8rNzC2032pDK1ywi\nUW/giNKUxE+RsRerhDa8OPaKse6Btewtb6Ot0tZXVifrLjHgAOdafPyTI9sJ7fim\n4fN3KnidAgMBAAECggEABv4gsKYuG3IyHdS/wKfU+4TKuik+gdM7Mw0YWjm9FG8K\niv0yxgvbQBhRdacAB/jVqUEbEBGA+ot7TBWmScoYLyWeN7v3wDwxKV14oKgZWPBu\nnUu7FdQISvg56fdLdTWXx5dLY9xJ/iGQ2HyYEb80IkK6TFRF3pi4XXJIZXxjRlvP\nHRPoSv71qG6x5hRfIS7NR7yei0NjcanNtiDCUpFtKXZ0gMnuc+XppiEY5z19Ov8v\nyEYNFoAyGVB5dHGtv2w8k95QbVfwGtqBU979z/xEFQTz1db2OSzgxy5gVoR4Loc/\ndl4M6TKYPcGLteqNpiKB8VybVryIJ3aQMaYX7kgnQQKBgQDpDaxZzX6drJKgHRV0\niZO+w7TJ+u+HIK5JGwseG8VZDL8HDCkafINOmUpcjk7MNVG3UIogWJ65PiSTwbCE\ntG/4T7SNJN+MgTkK6nKYP7PrjZBvt8FsfsQNbEJHiaS4KnEZAzYiaxL1Zf22vPpd\nZIYtIGCMP+mcg9ozilpbuKduTQKBgQDK2TLsr3ok9CQIOD99SnozRp5ntunH9gRO\nJ1mYy+lSGpZnC2zexeobMGUDF5Ck2TsFQkKxLmLRT2J0duvoTczvUwsPJhh5JqdK\nwGHiOKt+z4dpait+LzdHDLla6HxgkQbLXV8pp8GfF73dlazyOTyehpi0Vq3tpkqM\nC0XbTq57kQKBgQCSCzR/TixTSKrV1YP1dKV2fRPVIwBpcIxnWaAc7RA9nqQzGWbE\np1Rts9gKqk8s3xjnRHxais5kjVHEmjMw5hXoyKH/dST12qDRe1v2lqz8JslliQSY\nJdRcCQR76gCkPEyFfSK2bN0DlTdqBYDrd6wxqUF3gjG2GFZrx/6Zzdx2XQKBgQCq\npXAG11R9E/ngBFm88FO/ITCPdbxUIO3cRZRFS328OWu/wkfTXVIlj1/a6w8e7zSM\npwJuBeTRyuO7sHOjWRgHWagbFWRPPypLY261HhF/u9xh3RQ7skLhfZ3NEXnYzwiV\nOrac12i2iwWKDKmSmH4bqoV6aNUm8NcT20PoS9fTUQKBgCdrnQz0w1ibsYA7savQ\nRVgvS0t9hszhh4B/1vnwbEeAC/TRMeHRqkFzG2HkBkWBm58fQ5/YPsWM6mmOC/Lx\nKJrNs0pSSCII2Cdv/MdM75J6Foa4kSnQQVl2Ac6mFmEml+pYsJAc8DCYz0SvBO9E\npspwebPpqEKPFs6DI5KBmmuO\n-----END PRIVATE KEY-----\n"
	certificate_chain = "\n\n-----BEGIN CERTIFICATE-----\nMIIFBTCCAu2gAwIBAgIQS6hSk/eaL6JzBkuoBI110DANBgkqhkiG9w0BAQsFADBP\nMQswCQYDVQQGEwJVUzEpMCcGA1UEChMgSW50ZXJuZXQgU2VjdXJpdHkgUmVzZWFy\nY2ggR3JvdXAxFTATBgNVBAMTDElTUkcgUm9vdCBYMTAeFw0yNDAzMTMwMDAwMDBa\nFw0yNzAzMTIyMzU5NTlaMDMxCzAJBgNVBAYTAlVTMRYwFAYDVQQKEw1MZXQncyBF\nbmNyeXB0MQwwCgYDVQQDEwNSMTAwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEK\nAoIBAQDPV+XmxFQS7bRH/sknWHZGUCiMHT6I3wWd1bUYKb3dtVq/+vbOo76vACFL\nYlpaPAEvxVgD9on/jhFD68G14BQHlo9vH9fnuoE5CXVlt8KvGFs3Jijno/QHK20a\n/6tYvJWuQP/py1fEtVt/eA0YYbwX51TGu0mRzW4Y0YCF7qZlNrx06rxQTOr8IfM4\nFpOUurDTazgGzRYSespSdcitdrLCnF2YRVxvYXvGLe48E1KGAdlX5jgc3421H5KR\nmudKHMxFqHJV8LDmowfs/acbZp4/SItxhHFYyTr6717yW0QrPHTnj7JHwQdqzZq3\nDZb3EoEmUVQK7GH29/Xi8orIlQ2NAgMBAAGjgfgwgfUwDgYDVR0PAQH/BAQDAgGG\nMB0GA1UdJQQWMBQGCCsGAQUFBwMCBggrBgEFBQcDATASBgNVHRMBAf8ECDAGAQH/\nAgEAMB0GA1UdDgQWBBS7vMNHpeS8qcbDpHIMEI2iNeHI6DAfBgNVHSMEGDAWgBR5\ntFnme7bl5AFzgAiIyBpY9umbbjAyBggrBgEFBQcBAQQmMCQwIgYIKwYBBQUHMAKG\nFmh0dHA6Ly94MS5pLmxlbmNyLm9yZy8wEwYDVR0gBAwwCjAIBgZngQwBAgEwJwYD\nVR0fBCAwHjAcoBqgGIYWaHR0cDovL3gxLmMubGVuY3Iub3JnLzANBgkqhkiG9w0B\nAQsFAAOCAgEAkrHnQTfreZ2B5s3iJeE6IOmQRJWjgVzPw139vaBw1bGWKCIL0vIo\nzwzn1OZDjCQiHcFCktEJr59L9MhwTyAWsVrdAfYf+B9haxQnsHKNY67u4s5Lzzfd\nu6PUzeetUK29v+PsPmI2cJkxp+iN3epi4hKu9ZzUPSwMqtCceb7qPVxEbpYxY1p9\n1n5PJKBLBX9eb9LU6l8zSxPWV7bK3lG4XaMJgnT9x3ies7msFtpKK5bDtotij/l0\nGaKeA97pb5uwD9KgWvaFXMIEt8jVTjLEvwRdvCn294GPDF08U8lAkIv7tghluaQh\n1QnlE4SEN4LOECj8dsIGJXpGUk3aU3KkJz9icKy+aUgA+2cP21uh6NcDIS3XyfaZ\nQjmDQ993ChII8SXWupQZVBiIpcWO4RqZk3lr7Bz5MUCwzDIA359e57SSq5CCkY0N\n4B6Vulk7LktfwrdGNVI5BsC9qqxSwSKgRJeZ9wygIaehbHFHFhcBaMDKpiZlBHyz\nrsnnlFXCb5s8HKn5LsUgGvB24L7sGNZP2CX7dhHov+YhD+jozLW2p9W4959Bz2Ei\nRmqDtmiXLnzqTpXbI+suyCsohKRg6Un0RC47+cpiVwHiXZAW+cn8eiNIjqbVgXLx\nKPpdzvvtTnOPlC7SQZSYmdunr3Bf9b77AiC/ZidstK36dRILKz7OA54=\n-----END CERTIFICATE-----\n"
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
