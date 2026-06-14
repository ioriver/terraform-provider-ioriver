package provider

import "fmt"

func testAccServiceConfigDomainsSteps(idx int, params ...any) string {
	var testSteps = []string{
		// Step 0: One domain.
		// Params: resourceName, rndName, certId, origin1host, origin2host, domain0
		`
locals {
	origin_1_id = "origin-1"
	origin_2_id = "origin-2"
}

resource "ioriver_service" "%s" {
	name        = "%s"
	certificate = "%s"
	description = "desc"
	config = {
		origins = [
			{
				name = local.origin_1_id
				custom_origin = {
					host     = "%s"
					protocol = "https"
				}
			},
			{
				name = local.origin_2_id
				custom_origin = {
					host     = "%s"
					protocol = "https"
				}
			}
		]
		domains = [
			{
				domain = "%s"
				mappings = [{ target_mapping = local.origin_1_id }]
			},
		]
	}
}`,

		// Step 1: Two domains, both mapped to origin_1.
		// Params: resourceName, rndName, certId, origin1host, origin2host, domain0, domain1
		`
locals {
	origin_1_id = "origin-1"
	origin_2_id = "origin-2"
}

resource "ioriver_service" "%s" {
	name        = "%s"
	certificate = "%s"
	description = "desc"
	config = {
		origins = [
			{
				name = local.origin_1_id
				custom_origin = {
					host     = "%s"
					protocol = "https"
				}
			},
			{
				name = local.origin_2_id
				custom_origin = {
					host     = "%s"
					protocol = "https"
				}
			}
		]
		domains = [
			{
				domain = "%s"
				mappings = [{ target_mapping = local.origin_2_id }]
			},
			{
				domain = "%s"
				mappings = [{ target_mapping = local.origin_1_id }]
			}
		]
	}
}`,

		// Step 2: Three domains, each mapped to origin_1.
		// Params: resourceName, rndName, certId, origin1host, origin2host, domain0, domain1, domain2
		`
locals {
	origin_1_id = "origin-1"
	origin_2_id = "origin-2"
}

resource "ioriver_service" "%s" {
	name        = "%s"
	certificate = "%s"
	description = "desc"
	config = {
		origins = [
			{
				name = local.origin_1_id
				custom_origin = {
					host     = "%s"
					protocol = "https"
				}
			},
			{
				name = local.origin_2_id
				custom_origin = {
					host     = "%s"
					protocol = "https"
				}
			}
		]
		domains = [
			{
				domain = "%s"
				mappings = [{ target_mapping = local.origin_1_id }]
			},
			{
				domain = "%s"
				mappings = [{ target_mapping = local.origin_1_id }]
			},
			{
				domain = "%s"
				mappings = [{ target_mapping = local.origin_1_id }]
			}
		]
	}
}`,
	}

	if idx < 0 || idx >= len(testSteps) {
		panic("invalid config step index")
	}
	return fmt.Sprintf(testSteps[idx], params...)
}

func testAccServiceConfigMultiMappingSteps(idx int, resourceName string, certId string, domainHost string) string {
	var steps = []string{
		// Step 0: 2 origin sets + 2 standalone origins, domain maps to all 4.
		`
resource "ioriver_service" "%s" {
	name        = "%s"
	certificate = "%s"
	description = "multi-mapping acceptance test"
	config = {

		origins = [
			{
				name = "origin-a"
				custom_origin = {
					host     = "origin-a.example.com"
					protocol = "https"
				}
			},
			{
				name = "origin-b"
				custom_origin = {
					host     = "origin-b.example.com"
					protocol = "https"
				}
			},
		]
		origin_sets = [
			{
				name = "set-alpha"
				origins = [
					{
						custom_origin = {
							host     = "alpha-primary.example.com"
							protocol = "https"
						}
					},
					{
						custom_origin = {
							host     = "alpha-failover.example.com"
							protocol = "https"
						}
					},
				]
			},
			{
				name = "set-beta"
				origins = [
					{
						custom_origin = {
							host     = "beta-primary.example.com"
							protocol = "https"
						}
					},
					{
						custom_origin = {
							host     = "beta-failover.example.com"
							protocol = "https"
						}
					},
				]
			},
		]
		domains = [
			{
				domain = "%s"
				mappings = [
					{
						target_type    = "origin_set"
						target_mapping = "set-alpha"
						path_pattern   = "/api/*"
					},
					{
						target_type    = "origin_set"
						target_mapping = "set-beta"
						path_pattern   = "/static/*"
					},
					{
						target_mapping = "origin-a"
						path_pattern   = "/images/*"
					},
					{
						target_mapping = "origin-b"
						path_pattern   = "/*"
					},
				]
			}
		]
	}
}`,

		// Step 1: trim to 1 origin set + 1 standalone origin (2 mappings total).
		`
resource "ioriver_service" "%s" {
	name        = "%s"
	certificate = "%s"
	description = "multi-mapping acceptance test"
	config = {

		origins = [
			{
				name = "origin-a"
				custom_origin = {
					host     = "origin-a.example.com"
					protocol = "https"
				}
			},
		]
		origin_sets = [
			{
				name = "set-alpha"
				origins = [
					{
						custom_origin = {
							host     = "alpha-primary.example.com"
							protocol = "https"
						}
					},
					{
						custom_origin = {
							host     = "alpha-failover.example.com"
							protocol = "https"
						}
					},
				]
			},
		]
		domains = [
			{
				domain = "%s"
				mappings = [
					{
						target_type    = "origin_set"
						target_mapping = "set-alpha"
						path_pattern   = "/api/*"
					},
					{
						target_mapping = "origin-a"
						path_pattern   = "/*"
					},
				]
			}
		]
	}
}`,
	}

	return fmt.Sprintf(steps[idx], resourceName, resourceName, certId, domainHost)
}
