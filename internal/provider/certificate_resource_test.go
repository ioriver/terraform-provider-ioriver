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
	certificate       = "-----BEGIN CERTIFICATE-----\nMIIE6DCCA9CgAwIBAgISBMzwH2Es1AcRMxzzNItZXNG2MA0GCSqGSIb3DQEBCwUA\nMDIxCzAJBgNVBAYTAlVTMRYwFAYDVQQKEw1MZXQncyBFbmNyeXB0MQswCQYDVQQD\nEwJSMzAeFw0yNDAzMTIwNjQ4MDdaFw0yNDA2MTAwNjQ4MDZaMBkxFzAVBgNVBAMT\nDmlvcml2ZXItcWEuY29tMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA\n+ck1wBEcLfo10vHCZJMOTgTJTXa/jdx9/ZfTIM4sGRSTlkP6ne9cbMdDfD+c7iQM\ncSi/OAZ0maANE//npJjVzXVIMD/abtur+sW4IT7lh3+kVi9qk2dKvCqWyxEMVcc0\ninVILxRb/YfLSqATHXQl4Ec2qfy22oR4nLMBZpEW8Z6nqpS6gYcKhF25H/DvVVol\nEPMKRig4LbeJmN/ta+xR1czPNhQ1vh2aQZ5L7YaGCqt60g0SzvBla9mzc27v/Yjn\nq3H8GwuO2MYZqwpzv8Wf7IMsdYWXHD9s8QQNraLBLsyjt2KY+CLDT9uwn29uc6Zz\nSwqrDOu9hF3O58utMZqUUQIDAQABo4ICDzCCAgswDgYDVR0PAQH/BAQDAgWgMB0G\nA1UdJQQWMBQGCCsGAQUFBwMBBggrBgEFBQcDAjAMBgNVHRMBAf8EAjAAMB0GA1Ud\nDgQWBBSEWyQH12fDAg3l7B4di9jTl6WEBDAfBgNVHSMEGDAWgBQULrMXt1hWy65Q\nCUDmH6+dixTCxjBVBggrBgEFBQcBAQRJMEcwIQYIKwYBBQUHMAGGFWh0dHA6Ly9y\nMy5vLmxlbmNyLm9yZzAiBggrBgEFBQcwAoYWaHR0cDovL3IzLmkubGVuY3Iub3Jn\nLzAZBgNVHREEEjAQgg5pb3JpdmVyLXFhLmNvbTATBgNVHSAEDDAKMAgGBmeBDAEC\nATCCAQMGCisGAQQB1nkCBAIEgfQEgfEA7wB2AKLiv9Ye3i8vB6DWTm03p9xlQ7DG\ntS6i2reK+Jpt9RfYAAABjjGiDzUAAAQDAEcwRQIgeqHYHhNCh7aHuttZfkBcbGfi\nQ6r7q6vprKc4OVgevdYCIQCzAxD8bArztSFEKDDABd/xnsdiDGL6MR5fxPCpcM7y\nKgB1AHb/iD8KtvuVUcJhzPWHujS0pM27KdxoQgqf5mdMWjp0AAABjjGiD3IAAAQD\nAEYwRAIgXogdgzRBPv8v7/76SwNW1PbTx4HgD+2/V0uKrYcJgXUCICd0O+896koM\nGjtsaJIH2Kji2w+lkFXEXX5htaPdC0DVMA0GCSqGSIb3DQEBCwUAA4IBAQAjEuhh\nnnUz+YYdOxGtelvNvkANqt5R9PTdQYzTv/Lfxm+MfNKP6FYhSZaeRxqmB/I5jJRV\nKbxOOrM1dhgouebMBWLmwgOXzC/8iMr9FPC6xBqIEE0jBlyLOFNupTli9tDkNQZy\nr6gi1WRf9BdpnHrn3xftK8zWHfxaU8Evxx0UL2eIP/PN9nGJ9Irve/uTUakVDYLx\nwGAJGOG++n4WqU5VnHoNAaGZcQ8s7ig4Sh+vqO/b/pjKZoj95uspE8DtFP+Ksbo3\n/0nAfntAOrCxdQAPoZ9S2Zz+G4Tr1yig3wYuQVqRg6pCSiSJ8FnkpMoXtC91W/yk\naa3+a4eTEo7GkIM/\n-----END CERTIFICATE-----\n"
	private_key       = "-----BEGIN PRIVATE KEY-----\nMIIEvgIBADANBgkqhkiG9w0BAQEFAASCBKgwggSkAgEAAoIBAQD5yTXAERwt+jXS\n8cJkkw5OBMlNdr+N3H39l9MgziwZFJOWQ/qd71xsx0N8P5zuJAxxKL84BnSZoA0T\n/+ekmNXNdUgwP9pu26v6xbghPuWHf6RWL2qTZ0q8KpbLEQxVxzSKdUgvFFv9h8tK\noBMddCXgRzap/LbahHicswFmkRbxnqeqlLqBhwqEXbkf8O9VWiUQ8wpGKDgtt4mY\n3+1r7FHVzM82FDW+HZpBnkvthoYKq3rSDRLO8GVr2bNzbu/9iOercfwbC47Yxhmr\nCnO/xZ/sgyx1hZccP2zxBA2tosEuzKO3Ypj4IsNP27Cfb25zpnNLCqsM672EXc7n\ny60xmpRRAgMBAAECggEAYSN5JjjhMxYngWHoXbF8siWVXW0tQE97pbe58PuM1bIV\nVS4Zk/rmgB0b5wFcF5ZoSkd02aZVCPtfqqPt4ypWlyChkrX6Tftazdq/aRprK78d\nWzD0at+UBkovu0fleqROD/xdNoXd4mJBUkOfO2iUZDx9iyzOtXsn5pSMmmUZPZvX\nXJ5jqRE0x7FyL5t9+suk4XyrdGZ3mFVT7d5LsBc2hMZ3QngakBMAzFWv9iCMjcCX\nejX8QOG9BajrAVAVOpjAHtUkOwFAasi3iI1xiHw5lwioAI5JmVRVt8CwqSlCajSV\nqMKoNDHZhH99Mjq37Qx6zm9JLWDkFC8kJHHXZwB/qwKBgQD9ubm2dSKBlIyHQh9c\nzNMQ++4wSBOkkrA4e7hw5otoeNTY/20y73+3b+5eNevn7GLDrr9Crk8vbbAXT5aS\n2YL+8TbTXY5ywMsduvI9tELYJAMCGq3WVrkoLUy7uwVN+1o9m2VXWHDMO0/FUHv1\nK/hR+76GrCgqM4/Kb8zjJjowtwKBgQD8BnGYe2BIV2YuedVkujOfJbj17PccagRd\nu51SwxK1cW3xy2c7ngAjTTXDlQ5CMA0mJFvn5OevL6hXE5ZTXE2E+Ccrfa0MTaWh\n7l39WCNU2yfopsQ5kiwnUrdNBwLiSzow5mS8SLvtXRuzH0Ku8fx4CP1jBiqnI0VL\nArhxok/LNwKBgQCLhEX3a3+W610+vwBJ4iMpkq6OBAQxGawm0vk/s7Xys4au7/1W\n5dU/xB+51EKtHBHO8lcfToQiW1lZ6ByvEUXz9CWmoipDNXo7FeJARc//1AWHca4n\nTzavPgGUtSkckVs0xy85kVstImwh3rjavtvkEN7aZO4NDp9BvKpYOVwEDwKBgHJb\n0ivov/XTgtBQBF6ih04N5fHhxveju7t0qJynW9PtVoBDVeKdfV6HaIAJIOEzwKOw\nF+wP2HmL3I02nB3TYnGV0OBRNLbCfQgPi6Kr3cxhbaiKE7wz8ckeJYLUTaC1lgAe\na1Nshandd/Y9lxqfP3qQSbonUC9rN8QjxeH6Ts7dAoGBAPmVF+CLjbgk4+C4RhYJ\n44rIrw29aahb1YGo5CLv9jQecUQkQ5rkp8VmC5SMEVKy57lC6oi8jk1JeuzPjl+W\n4zUH9rjLrabVT6Lf5C0nUjGWJ25OB4sWekp6Y1n4UjVPzXkTuEbl1SBvT8SGnZzf\nXwQzRKe0+26Y0erfghC25kun\n-----END PRIVATE KEY-----\n"
	certificate_chain = "-----BEGIN CERTIFICATE-----\nMIIFFjCCAv6gAwIBAgIRAJErCErPDBinU/bWLiWnX1owDQYJKoZIhvcNAQELBQAw\nTzELMAkGA1UEBhMCVVMxKTAnBgNVBAoTIEludGVybmV0IFNlY3VyaXR5IFJlc2Vh\ncmNoIEdyb3VwMRUwEwYDVQQDEwxJU1JHIFJvb3QgWDEwHhcNMjAwOTA0MDAwMDAw\nWhcNMjUwOTE1MTYwMDAwWjAyMQswCQYDVQQGEwJVUzEWMBQGA1UEChMNTGV0J3Mg\nRW5jcnlwdDELMAkGA1UEAxMCUjMwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEK\nAoIBAQC7AhUozPaglNMPEuyNVZLD+ILxmaZ6QoinXSaqtSu5xUyxr45r+XXIo9cP\nR5QUVTVXjJ6oojkZ9YI8QqlObvU7wy7bjcCwXPNZOOftz2nwWgsbvsCUJCWH+jdx\nsxPnHKzhm+/b5DtFUkWWqcFTzjTIUu61ru2P3mBw4qVUq7ZtDpelQDRrK9O8Zutm\nNHz6a4uPVymZ+DAXXbpyb/uBxa3Shlg9F8fnCbvxK/eG3MHacV3URuPMrSXBiLxg\nZ3Vms/EY96Jc5lP/Ooi2R6X/ExjqmAl3P51T+c8B5fWmcBcUr2Ok/5mzk53cU6cG\n/kiFHaFpriV1uxPMUgP17VGhi9sVAgMBAAGjggEIMIIBBDAOBgNVHQ8BAf8EBAMC\nAYYwHQYDVR0lBBYwFAYIKwYBBQUHAwIGCCsGAQUFBwMBMBIGA1UdEwEB/wQIMAYB\nAf8CAQAwHQYDVR0OBBYEFBQusxe3WFbLrlAJQOYfr52LFMLGMB8GA1UdIwQYMBaA\nFHm0WeZ7tuXkAXOACIjIGlj26ZtuMDIGCCsGAQUFBwEBBCYwJDAiBggrBgEFBQcw\nAoYWaHR0cDovL3gxLmkubGVuY3Iub3JnLzAnBgNVHR8EIDAeMBygGqAYhhZodHRw\nOi8veDEuYy5sZW5jci5vcmcvMCIGA1UdIAQbMBkwCAYGZ4EMAQIBMA0GCysGAQQB\ngt8TAQEBMA0GCSqGSIb3DQEBCwUAA4ICAQCFyk5HPqP3hUSFvNVneLKYY611TR6W\nPTNlclQtgaDqw+34IL9fzLdwALduO/ZelN7kIJ+m74uyA+eitRY8kc607TkC53wl\nikfmZW4/RvTZ8M6UK+5UzhK8jCdLuMGYL6KvzXGRSgi3yLgjewQtCPkIVz6D2QQz\nCkcheAmCJ8MqyJu5zlzyZMjAvnnAT45tRAxekrsu94sQ4egdRCnbWSDtY7kh+BIm\nlJNXoB1lBMEKIq4QDUOXoRgffuDghje1WrG9ML+Hbisq/yFOGwXD9RiX8F6sw6W4\navAuvDszue5L3sz85K+EC4Y/wFVDNvZo4TYXao6Z0f+lQKc0t8DQYzk1OXVu8rp2\nyJMC6alLbBfODALZvYH7n7do1AZls4I9d1P4jnkDrQoxB3UqQ9hVl3LEKQ73xF1O\nyK5GhDDX8oVfGKF5u+decIsH4YaTw7mP3GFxJSqv3+0lUFJoi5Lc5da149p90Ids\nhCExroL1+7mryIkXPeFM5TgO9r0rvZaBFOvV2z0gp35Z0+L4WPlbuEjN/lxPFin+\nHlUjr8gRsI3qfJOQFy/9rKIJR0Y/8Omwt/8oTWgy1mdeHmmjk7j1nYsvC9JSQ6Zv\nMldlTTKB3zhThV1+XWYp6rjd5JW1zbVWEkLNxE7GJThEUG3szgBVGP7pSWTUTsqX\nnLRbwHOoq7hHwg==\n-----END CERTIFICATE-----\n"
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
