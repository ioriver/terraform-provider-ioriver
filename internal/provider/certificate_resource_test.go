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
	certificate       = "-----BEGIN CERTIFICATE-----\nMIIE7TCCA9WgAwIBAgISAzS++A+Qk+UajnceBiI9NkQeMA0GCSqGSIb3DQEBCwUA\nMDIxCzAJBgNVBAYTAlVTMRYwFAYDVQQKEw1MZXQncyBFbmNyeXB0MQswCQYDVQQD\nEwJSMzAeFw0yMzEyMTIxMjM1NDBaFw0yNDAzMTExMjM1MzlaMBsxGTAXBgNVBAMM\nECouaW9yaXZlci1xYS5jb20wggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIB\nAQDAyshqIwSQawJc5fIB4sR+p5bxV0HuC+a2O4o3SemcMEBSnvDelu/k7dml7E4S\nsLidKKDl5i0okgqBmR+K6wAaNTK0GcIi3MUkqk15V/XMlIyXbH9B72qJ+C8IuHdc\nJEOBgWq6asmB0RucH/477ZL4kb7WIxn0tDUlyKUNfaHy5vU+v1PrGwLzC5UJG6QH\ne4YbEmuCSoWmWto09kiwK/KVIiX85H6nE+Al8yo0Lvnl6IzCbHGyJOKvpcK9qfD2\npNqxUmqOUDgZlZb8FUTSN80zM+NNah1TFewzPxq2Jpt2xXPeWSTO+fJ5/vdW2gCT\nO6e8rrYVdMzl/ayzvgv4lysbAgMBAAGjggISMIICDjAOBgNVHQ8BAf8EBAMCBaAw\nHQYDVR0lBBYwFAYIKwYBBQUHAwEGCCsGAQUFBwMCMAwGA1UdEwEB/wQCMAAwHQYD\nVR0OBBYEFLTZKwOINeBx9GE+HTbgwi5bbGDRMB8GA1UdIwQYMBaAFBQusxe3WFbL\nrlAJQOYfr52LFMLGMFUGCCsGAQUFBwEBBEkwRzAhBggrBgEFBQcwAYYVaHR0cDov\nL3IzLm8ubGVuY3Iub3JnMCIGCCsGAQUFBzAChhZodHRwOi8vcjMuaS5sZW5jci5v\ncmcvMBsGA1UdEQQUMBKCECouaW9yaXZlci1xYS5jb20wEwYDVR0gBAwwCjAIBgZn\ngQwBAgEwggEEBgorBgEEAdZ5AgQCBIH1BIHyAPAAdgA7U3d1Pi25gE6LMFsG/kA7\nZ9hPw/THvQANLXJv4frUFwAAAYxePYkgAAAEAwBHMEUCIAVVYrbs9daeDy18TNG/\nx/GroUfIrdW8HemD99aM5DsEAiEA4Ezbnlqbi25ktMWYoutmLZ4uL1/NNDbIZoBv\nJi/j1AYAdgBIsONr2qZHNA/lagL6nTDrHFIBy1bdLIHZu7+rOdiEcwAAAYxePYlF\nAAAEAwBHMEUCIGTUwJUalHMgTZfkhKrAvRH9LkLIkQ+ubQZ8sbdlbxj/AiEA/DkN\niRY0stDhekQGhNo3tutFMOZ9eKAzlGDQSnwK+uQwDQYJKoZIhvcNAQELBQADggEB\nAGAGWQZt7t53bRTKe3auPc3kvAPYWGPZMPLZHnylkZ4KMLrd3m6SQYpd/LvQ+WT+\nqH6JnZHUOhlUA6uFlRO8vUhDvL7ItU5VzwRbQaBjChFuM8Efa0hW5JIQaa8x79Rt\nVmKsHr2+vVj6UymeaTggguk9Ep/OnDRs1xVz5whRH3W1pB063950+cR7Rcbmkwp6\nLE4bfVLw2XOLdS/QKLPpcIm/hyJJjg3Wy/hAphLPL0jxggsbljwbyYS+pjLr8Y0z\nHPn0HbY1q5pvpxddw4cX7msKa3TAPNP0dJXJFDD8Cv1bkJoxCZgZTC1yazwL1O8+\nSQbD99IOQK/kaXghawcoQKo=\n-----END CERTIFICATE-----\n"
	private_key       = "-----BEGIN PRIVATE KEY-----\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQDAyshqIwSQawJc\n5fIB4sR+p5bxV0HuC+a2O4o3SemcMEBSnvDelu/k7dml7E4SsLidKKDl5i0okgqB\nmR+K6wAaNTK0GcIi3MUkqk15V/XMlIyXbH9B72qJ+C8IuHdcJEOBgWq6asmB0Ruc\nH/477ZL4kb7WIxn0tDUlyKUNfaHy5vU+v1PrGwLzC5UJG6QHe4YbEmuCSoWmWto0\n9kiwK/KVIiX85H6nE+Al8yo0Lvnl6IzCbHGyJOKvpcK9qfD2pNqxUmqOUDgZlZb8\nFUTSN80zM+NNah1TFewzPxq2Jpt2xXPeWSTO+fJ5/vdW2gCTO6e8rrYVdMzl/ayz\nvgv4lysbAgMBAAECggEAJueE4+4nGKGPe4GngXvqXQiirBcIpene571CGgAfVTZj\ndIjAYJqi1FavCg2Sf7ELwCMXvOzjBgHafuaQd+8OOTus/K0KARD983UuZyM55cvA\nRzpMv9h5blgX3bGj3IMw4CxmhjHQchLpeGr4Wc2KUADROtMghMHsy7AepodIrARX\nmVzuYd94FJlv8gCjx2lF6YebY5gW/YWylDAlf6v2u6+u5MqYERQ++0Wrf5UFuUrR\nFKxFYGD5RF9i2t7V4kTHEd1pVmFWCpHkHP12okeHZocu0GXidjPHhN+nkezn9VSm\n7m/5MrbmpDVz3sQjPqZMxtfXntuiq8MApF3i1/dEEQKBgQDlf0tQGkRaRW+p0r3d\nEyPsyIpy/dNFsWMtB9TZOujJz6z71guQ8tUlCojQwz66Io+avlsUBe2G5EdZzPSL\n9pKiDDoE7vzvwhoYDNi5bn0KIJ0zIXoAS5rU6x+FJoMg+J1DMpOc5z6Wh47E9BRy\nWmvTLRhD0n3OobMRHHVzVsmFwwKBgQDXDlzPejynPpDRCGabuopT/QoSlnppwbwc\ntxNtQT2Jrj2AUje0WtPlmwtx8U5M1651Z1M83nEM1FGNYpL2cue0xUvnG3NUf/qo\n8S4wTJgtPwWKaegLS9iXjL3XOqaizzalGnBioQ548S2gKB52nDNvQvCzSdneoRi3\nXIsaj5r3yQKBgGzLN4y1pwmUOScsfE09MZ6iQt/YbDtxqC5EtCZ2wrxI8xw/kCQa\nuehhYhJ1PFtI3wLgkpSfasazmQ99FcD0FvczDJ4iHU4bmfsku+jL9ALFC0Cd8hQJ\nw1CEVeDtLOSIdyTP6SJMjWMWyBueCcNcEhl+Gy6rrnAyP40xDIys68O5AoGBAKUA\nPQ9nPyAmrd/j7S4wuq9kJxVJ5VQ9M8JoaPxboQaA7GkHK/wx8ABrrCVZOnVUymMD\nyuaZ2O05/fRXnGCAmuykr+76rcs4gi6bFZAzRFL61ppzVXlNUTo93u5C7tVd3RRi\nK7ZQ0hTTHumRvoXMpN4J4zn8QLBCs/8Dfyr64bCZAoGANN28fS5YfpOUixIdQa+v\n4+zGYOWJVysFjhr0EoC1rNy+1QBiUdCRSgrZRGIBViFxpEIquZrgdfIFeKcERjIR\nwRnsUlaiFdnxpZmM8p+OZlRyt/Gy5/fDyh9Gfy7skaqERH5S0Gm4nvmYO9KjGzdD\nm3KFLPMow/YKYc7j/vrDnDI=\n-----END PRIVATE KEY-----\n"
	certificate_chain = "-----BEGIN CERTIFICATE-----\nMIIFFjCCAv6gAwIBAgIRAJErCErPDBinU/bWLiWnX1owDQYJKoZIhvcNAQELBQAw\nTzELMAkGA1UEBhMCVVMxKTAnBgNVBAoTIEludGVybmV0IFNlY3VyaXR5IFJlc2Vh\ncmNoIEdyb3VwMRUwEwYDVQQDEwxJU1JHIFJvb3QgWDEwHhcNMjAwOTA0MDAwMDAw\nWhcNMjUwOTE1MTYwMDAwWjAyMQswCQYDVQQGEwJVUzEWMBQGA1UEChMNTGV0J3Mg\nRW5jcnlwdDELMAkGA1UEAxMCUjMwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEK\nAoIBAQC7AhUozPaglNMPEuyNVZLD+ILxmaZ6QoinXSaqtSu5xUyxr45r+XXIo9cP\nR5QUVTVXjJ6oojkZ9YI8QqlObvU7wy7bjcCwXPNZOOftz2nwWgsbvsCUJCWH+jdx\nsxPnHKzhm+/b5DtFUkWWqcFTzjTIUu61ru2P3mBw4qVUq7ZtDpelQDRrK9O8Zutm\nNHz6a4uPVymZ+DAXXbpyb/uBxa3Shlg9F8fnCbvxK/eG3MHacV3URuPMrSXBiLxg\nZ3Vms/EY96Jc5lP/Ooi2R6X/ExjqmAl3P51T+c8B5fWmcBcUr2Ok/5mzk53cU6cG\n/kiFHaFpriV1uxPMUgP17VGhi9sVAgMBAAGjggEIMIIBBDAOBgNVHQ8BAf8EBAMC\nAYYwHQYDVR0lBBYwFAYIKwYBBQUHAwIGCCsGAQUFBwMBMBIGA1UdEwEB/wQIMAYB\nAf8CAQAwHQYDVR0OBBYEFBQusxe3WFbLrlAJQOYfr52LFMLGMB8GA1UdIwQYMBaA\nFHm0WeZ7tuXkAXOACIjIGlj26ZtuMDIGCCsGAQUFBwEBBCYwJDAiBggrBgEFBQcw\nAoYWaHR0cDovL3gxLmkubGVuY3Iub3JnLzAnBgNVHR8EIDAeMBygGqAYhhZodHRw\nOi8veDEuYy5sZW5jci5vcmcvMCIGA1UdIAQbMBkwCAYGZ4EMAQIBMA0GCysGAQQB\ngt8TAQEBMA0GCSqGSIb3DQEBCwUAA4ICAQCFyk5HPqP3hUSFvNVneLKYY611TR6W\nPTNlclQtgaDqw+34IL9fzLdwALduO/ZelN7kIJ+m74uyA+eitRY8kc607TkC53wl\nikfmZW4/RvTZ8M6UK+5UzhK8jCdLuMGYL6KvzXGRSgi3yLgjewQtCPkIVz6D2QQz\nCkcheAmCJ8MqyJu5zlzyZMjAvnnAT45tRAxekrsu94sQ4egdRCnbWSDtY7kh+BIm\nlJNXoB1lBMEKIq4QDUOXoRgffuDghje1WrG9ML+Hbisq/yFOGwXD9RiX8F6sw6W4\navAuvDszue5L3sz85K+EC4Y/wFVDNvZo4TYXao6Z0f+lQKc0t8DQYzk1OXVu8rp2\nyJMC6alLbBfODALZvYH7n7do1AZls4I9d1P4jnkDrQoxB3UqQ9hVl3LEKQ73xF1O\nyK5GhDDX8oVfGKF5u+decIsH4YaTw7mP3GFxJSqv3+0lUFJoi5Lc5da149p90Ids\nhCExroL1+7mryIkXPeFM5TgO9r0rvZaBFOvV2z0gp35Z0+L4WPlbuEjN/lxPFin+\nHlUjr8gRsI3qfJOQFy/9rKIJR0Y/8Omwt/8oTWgy1mdeHmmjk7j1nYsvC9JSQ6Zv\nMldlTTKB3zhThV1+XWYp6rjd5JW1zbVWEkLNxE7GJThEUG3szgBVGP7pSWTUTsqX\nnLRbwHOoq7hHwg==\n-----END CERTIFICATE-----\n\n-----BEGIN CERTIFICATE-----\nMIIFYDCCBEigAwIBAgIQQAF3ITfU6UK47naqPGQKtzANBgkqhkiG9w0BAQsFADA/\nMSQwIgYDVQQKExtEaWdpdGFsIFNpZ25hdHVyZSBUcnVzdCBDby4xFzAVBgNVBAMT\nDkRTVCBSb290IENBIFgzMB4XDTIxMDEyMDE5MTQwM1oXDTI0MDkzMDE4MTQwM1ow\nTzELMAkGA1UEBhMCVVMxKTAnBgNVBAoTIEludGVybmV0IFNlY3VyaXR5IFJlc2Vh\ncmNoIEdyb3VwMRUwEwYDVQQDEwxJU1JHIFJvb3QgWDEwggIiMA0GCSqGSIb3DQEB\nAQUAA4ICDwAwggIKAoICAQCt6CRz9BQ385ueK1coHIe+3LffOJCMbjzmV6B493XC\nov71am72AE8o295ohmxEk7axY/0UEmu/H9LqMZshftEzPLpI9d1537O4/xLxIZpL\nwYqGcWlKZmZsj348cL+tKSIG8+TA5oCu4kuPt5l+lAOf00eXfJlII1PoOK5PCm+D\nLtFJV4yAdLbaL9A4jXsDcCEbdfIwPPqPrt3aY6vrFk/CjhFLfs8L6P+1dy70sntK\n4EwSJQxwjQMpoOFTJOwT2e4ZvxCzSow/iaNhUd6shweU9GNx7C7ib1uYgeGJXDR5\nbHbvO5BieebbpJovJsXQEOEO3tkQjhb7t/eo98flAgeYjzYIlefiN5YNNnWe+w5y\nsR2bvAP5SQXYgd0FtCrWQemsAXaVCg/Y39W9Eh81LygXbNKYwagJZHduRze6zqxZ\nXmidf3LWicUGQSk+WT7dJvUkyRGnWqNMQB9GoZm1pzpRboY7nn1ypxIFeFntPlF4\nFQsDj43QLwWyPntKHEtzBRL8xurgUBN8Q5N0s8p0544fAQjQMNRbcTa0B7rBMDBc\nSLeCO5imfWCKoqMpgsy6vYMEG6KDA0Gh1gXxG8K28Kh8hjtGqEgqiNx2mna/H2ql\nPRmP6zjzZN7IKw0KKP/32+IVQtQi0Cdd4Xn+GOdwiK1O5tmLOsbdJ1Fu/7xk9TND\nTwIDAQABo4IBRjCCAUIwDwYDVR0TAQH/BAUwAwEB/zAOBgNVHQ8BAf8EBAMCAQYw\nSwYIKwYBBQUHAQEEPzA9MDsGCCsGAQUFBzAChi9odHRwOi8vYXBwcy5pZGVudHJ1\nc3QuY29tL3Jvb3RzL2RzdHJvb3RjYXgzLnA3YzAfBgNVHSMEGDAWgBTEp7Gkeyxx\n+tvhS5B1/8QVYIWJEDBUBgNVHSAETTBLMAgGBmeBDAECATA/BgsrBgEEAYLfEwEB\nATAwMC4GCCsGAQUFBwIBFiJodHRwOi8vY3BzLnJvb3QteDEubGV0c2VuY3J5cHQu\nb3JnMDwGA1UdHwQ1MDMwMaAvoC2GK2h0dHA6Ly9jcmwuaWRlbnRydXN0LmNvbS9E\nU1RST09UQ0FYM0NSTC5jcmwwHQYDVR0OBBYEFHm0WeZ7tuXkAXOACIjIGlj26Ztu\nMA0GCSqGSIb3DQEBCwUAA4IBAQAKcwBslm7/DlLQrt2M51oGrS+o44+/yQoDFVDC\n5WxCu2+b9LRPwkSICHXM6webFGJueN7sJ7o5XPWioW5WlHAQU7G75K/QosMrAdSW\n9MUgNTP52GE24HGNtLi1qoJFlcDyqSMo59ahy2cI2qBDLKobkx/J3vWraV0T9VuG\nWCLKTVXkcGdtwlfFRjlBz4pYg1htmf5X6DYO8A4jqv2Il9DjXA6USbW1FzXSLr9O\nhe8Y4IWS6wY7bCkjCWDcRQJMEhg76fsO3txE+FiYruq9RUWhiF1myv4Q6W+CyBFC\nDfvp7OOGAN6dEOM4+qR9sdjoSYKEBpsr6GtPAQw4dy753ec5\n-----END CERTIFICATE-----\n"
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
