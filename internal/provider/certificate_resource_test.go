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
	certificate       = "-----BEGIN CERTIFICATE-----\nMIIE9TCCA92gAwIBAgISBG1KNqeiiepdIgab163Hjn3kMA0GCSqGSIb3DQEBCwUA\nMDMxCzAJBgNVBAYTAlVTMRYwFAYDVQQKEw1MZXQncyBFbmNyeXB0MQwwCgYDVQQD\nEwNSMTAwHhcNMjUwMzExMTE0NzM5WhcNMjUwNjA5MTE0NzM4WjAdMRswGQYDVQQD\nDBIqLmlvcml2ZXItcWEtNS5jb20wggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEK\nAoIBAQC8yd5wnpslITA7OK48lbzOelZrmXU3GmY65ti4fiTsRCUMbkW51YY/sya7\nOv/iuDCtU/IOP/1MDNA4a/tsXCozVj7Eo3E7tSDpMy+dWQsUumZ+Dp+WTDHsZImq\nbh41l7m6mZQhpuVOZc+8A8CVcdx5+SyHHWBA5J+yjvdnWvc3MWSLlSY60xrCkISD\nvuAWjNWc2QzU5dlp6YjSs5IgFjdkvNGINj2vDTCyUxbQiF49GQPujnDCNlgy6LcT\nUYw5d99+py9lAdpk35SkHbQX7StQr9z7vAo2BWzWYP5Htx3/wSMKJ97WfLs2FyBm\njsg1cfpZvvnshnP0yXBox8rvXBvXAgMBAAGjggIXMIICEzAOBgNVHQ8BAf8EBAMC\nBaAwHQYDVR0lBBYwFAYIKwYBBQUHAwEGCCsGAQUFBwMCMAwGA1UdEwEB/wQCMAAw\nHQYDVR0OBBYEFJ6mA9KHhtL1vmyDvQZCOaszl601MB8GA1UdIwQYMBaAFLu8w0el\n5LypxsOkcgwQjaI14cjoMFcGCCsGAQUFBwEBBEswSTAiBggrBgEFBQcwAYYWaHR0\ncDovL3IxMC5vLmxlbmNyLm9yZzAjBggrBgEFBQcwAoYXaHR0cDovL3IxMC5pLmxl\nbmNyLm9yZy8wHQYDVR0RBBYwFIISKi5pb3JpdmVyLXFhLTUuY29tMBMGA1UdIAQM\nMAowCAYGZ4EMAQIBMIIBBQYKKwYBBAHWeQIEAgSB9gSB8wDxAHcAouMK5EXvva2b\nfjjtR2d3U9eCW4SU1yteGyzEuVCkR+cAAAGVhT27GQAABAMASDBGAiEA7qro7qsB\n69jvvnI2Q40T5ot0KTRk13OQ0Q0m0b6MZlsCIQCN+KfmIJjXvD+hc0dwTmNd7NOT\nWfTdlZ5t6zzxdKMm3wB2AE51oydcmhDDOFts1N8/Uusd8OCOG41pwLH6ZLFimjnf\nAAABlYU9uxcAAAQDAEcwRQIgF4oaw5ZEWWmsYnnjTkpTwplD6Egg5C42+U6VDdh2\nmLgCIQCbaMKoslt6GcgWGJFfx5g9y3DFgBoXvHgBUoPniDJ6PjANBgkqhkiG9w0B\nAQsFAAOCAQEAeC14waxe1AppCNA/JqecmvaUVbJ9vfTwM1i0qNcbtjGE1raPfq28\n5OCOTb8/iZ2QV/JuO7vQzRmm5afWe4EW/cde09yF1bq5fjKFX7kDAA0oUXZIUzI2\npVPoYRpZxiCPMC0jAxb56F5PIcLnwzTKnqgF6St51oj8YJ9SQckEPRDX1XosJ01R\nm4QFVtAPpnXJKBPp00C6Vbpfex1OdlaREolmTZkahj0e04qxFUZbDLUMODHxCYwk\n3PkRXFyE2bm94DfVpKvRvDNfwbrKqKQdHO103HHNEnKHymg8U1hRKwza+m0CHzSw\nYhZMCFJxiXKE6htQJc351OcMXEoeVwg1mA==\n-----END CERTIFICATE-----\n"
	private_key       = "-----BEGIN PRIVATE KEY-----\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQC8yd5wnpslITA7\nOK48lbzOelZrmXU3GmY65ti4fiTsRCUMbkW51YY/sya7Ov/iuDCtU/IOP/1MDNA4\na/tsXCozVj7Eo3E7tSDpMy+dWQsUumZ+Dp+WTDHsZImqbh41l7m6mZQhpuVOZc+8\nA8CVcdx5+SyHHWBA5J+yjvdnWvc3MWSLlSY60xrCkISDvuAWjNWc2QzU5dlp6YjS\ns5IgFjdkvNGINj2vDTCyUxbQiF49GQPujnDCNlgy6LcTUYw5d99+py9lAdpk35Sk\nHbQX7StQr9z7vAo2BWzWYP5Htx3/wSMKJ97WfLs2FyBmjsg1cfpZvvnshnP0yXBo\nx8rvXBvXAgMBAAECggEAXgkfZ3FZThFN+PGuuDbNqPt++HGj1SKtMUGzSZJrydPX\nsG1tBbe5+xi9fh1RQBkHBg7+TuLIxIzNWo1O2xa9XnzjHwdaa6c5EW+RlAq6XkTK\nsJeQHkktxNX/TIk1OvSsaqn9AxYiuf40jy4/SzE/5PGcoGCdhTVb5pEX4r+IzFBN\nQUEHtaO5dr59AU3QWn1JCFtfQtfwaHJRnGLBpAl2KBDPIjXmqixecj/JEJDTKzGQ\n693X0tlZzVEmS9oMAdUNLeh96N4mWtzl3mqlDGJcXMI/QASp1VREvgF8LYxdJ3lT\n2FFymfSR8LwRyEXiTEzbm1WeEffInUVL9iAdDceI4QKBgQD5oW4GR5ndK7a+pPpe\n3ASjcugTlMl/teRyIRg3i/dgjat1ohAezdgShUkjXYsnQktwhs6kTuHgRydA/3pn\nILLBOjwC7bPZ0fsd8HB+Nf80GxUJR2giJdgu7JRpaudx6VxsQSrFOT47oqECzndK\nP8OLZknVB8aT0WkCRTC2mN84nQKBgQDBmwXl1ZWo6eJOo62fv324+UD/icVO4ML6\niOsz4QsnYgoObQ6+e6ywxdF+IKGmzxnE/OrGtGEi5AwXJd/t44dsekP+4Gu26YHd\nuNkxj695LUWJD3xjk9iXxEvLu+a0/1ltNmyHuRsliaCRr98yR//eBq1Qf9Jk6eTs\nBrLw+HCaAwKBgBR6KYxaU0TRUSxSXDdr1PWTd3YjvmO7iAHUtSfZU3GYLXh40tm0\nCQV76YP9KG0QAyA37ruLvPuo2o96ZZAQHpm7LTEQTrCPiQnrr06rH0Qm9JLOSLyE\nXjd7MLF1E4dEnVBECD4lc/VwYcTZKu/sSx4kReozuRZnFzYYduaDo8wBAoGAaiXw\nyd6cu4vgRHWBUEDRUYV3maOTxnd875f6POt6DhG2qcopd06flBwhjCGf/7E008hH\ngMKNL3ARIO/nIqrJKTSv6yJobFUCmuoqSv4YmzuzED6pWH9LFYrOc9mF2F7YTQS1\n5IQc2ivnGXlvykWnh7fpdmVemW2T0cSqf2v3cLkCgYEA3pMmZLqwjANcWNR+Dy2o\nXmMFHYVUAtbvtdmy9Bj3uX3Px6384S2cG5bsCoz79mscw/BN500/wqKW5wxotlM6\nFLKF+U6mAX35dT9nWp7f3JtRu3QtGtJMVF5QJ58/09lRR6HOyAtWZouwzXStejsh\nHIby0PnVkqWE3U/2R7L7qpE=\n-----END PRIVATE KEY-----\n"
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
