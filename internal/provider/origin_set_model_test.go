package provider

import (
	"fmt"
)

// ---------------------------------------------------------------------------
// HCL fixtures
// ---------------------------------------------------------------------------

func testAccServiceConfigOriginSetsSteps(idx int, resourceName string, certId string, domainHost string) string {
	var steps = []string{
		// Step 0: one origin set with default failover codes, domain mapped to it.
		`
resource "ioriver_service" "%s" {
	name        = "%s"
	certificate = "%s"
	description = "origin-set acceptance test"
	config = {
		origin_sets = [
			{
				name = "my-failover-set"
				origins = [
					{
						custom_origin = {
							host     = "primary.example.com"
							protocol = "https"
						}
					},
					{
						custom_origin = {
							host     = "failover.example.com"
							protocol = "https"
						}
					},
				]
			}
		]
		domains = [
			{
				domain = "%s"
				mappings = [
					{
						target_type    = "origin_set"
						target_mapping = "my-failover-set"
					}
				]
			}
		]
	}
}`,

		// Step 1: update failover_response_codes.
		`
resource "ioriver_service" "%s" {
	name        = "%s"
	certificate = "%s"
	description = "origin-set acceptance test"
	config = {
		origin_sets = [
			{
				name                    = "my-failover-set"
				failover_response_codes = [503, 504]
				origins = [
					{
						custom_origin = {
							host     = "primary.example.com"
							protocol = "https"
						}
					},
					{
						custom_origin = {
							host     = "failover.example.com"
							protocol = "https"
						}
					},
				]
			}
		]
		domains = [
			{
				domain = "%s"
				mappings = [
					{
						target_type    = "origin_set"
						target_mapping = "my-failover-set"
					}
				]
			}
		]
	}
}`,

		// Step 2: swap primary host.
		`
resource "ioriver_service" "%s" {
	name        = "%s"
	certificate = "%s"
	description = "origin-set acceptance test"
	config = {
		origin_sets = [
			{
				name                    = "my-failover-set"
				failover_response_codes = [503, 504]
				origins = [
					{
						custom_origin = {
							host     = "primary2.example.com"
							protocol = "https"
						}
					},
					{
						custom_origin = {
							host     = "failover.example.com"
							protocol = "https"
						}
					},
				]
			}
		]
		domains = [
			{
				domain = "%s"
				mappings = [
					{
						target_type    = "origin_set"
						target_mapping = "my-failover-set"
					}
				]
			}
		]
	}
}`,

		// Step 3: re-point domain to a standalone origin.
		`
locals {
	standalone_id = "standalone-origin"
}

resource "ioriver_service" "%s" {
	name        = "%s"
	certificate = "%s"
	description = "origin-set acceptance test"
	config = {
		origins = [
			{
				name = local.standalone_id
				custom_origin = {
					host     = "standalone.example.com"
					protocol = "https"
				}
			}
		]
		origin_sets = [
			{
				name = "my-failover-set"
				origins = [
					{
						custom_origin = {
							host     = "primary2.example.com"
							protocol = "https"
						}
					},
					{
						custom_origin = {
							host     = "failover.example.com"
							protocol = "https"
						}
					},
				]
			}
		]
		domains = [
			{
				domain = "%s"
				mappings = [
					{
						target_mapping = local.standalone_id
					}
				]
			}
		]
	}
}`,

		// Step 4: remove origin set entirely.
		`
locals {
	standalone_id = "standalone-origin"
}

resource "ioriver_service" "%s" {
	name        = "%s"
	certificate = "%s"
	description = "origin-set acceptance test"
	config = {
		origins = [
			{
				name = local.standalone_id
				custom_origin = {
					host     = "standalone.example.com"
					protocol = "https"
				}
			}
		]
		origin_sets = []
		domains = [
			{
				domain = "%s"
				mappings = [
					{
						target_mapping = local.standalone_id
					}
				]
			}
		]
	}
}`,

		// Step 5: add a second origin set (set1 at [0], set2 at [1]).
		`
resource "ioriver_service" "%s" {
	name        = "%s"
	certificate = "%s"
	description = "origin-set acceptance test"
	config = {
		origin_sets = [
			{
				name = "my-failover-set"
				origins = [
					{
						custom_origin = {
							host     = "primary.example.com"
							protocol = "https"
						}
					},
					{
						custom_origin = {
							host     = "failover.example.com"
							protocol = "https"
						}
					}
				]
			},
			{
				name = "my-failover-set-2"
				origins = [
					{
						custom_origin = {
							host     = "primary-b.example.com"
							protocol = "https"
						}
					},
					{
						custom_origin = {
							host     = "failover-b.example.com"
							protocol = "https"
						}
					}
				]
			}
		]
		domains = [
			{
				domain = "%s"
				mappings = [
					{
						target_type    = "origin_set"
						target_mapping = "my-failover-set"
					}
				]
			}
		]
	}
}`,

		// Step 6: swap origin set order in HCL — pure reorder, no value changes.
		// Used as a plan-only step to verify NamedListPlanModifier suppresses the diff.
		`
resource "ioriver_service" "%s" {
	name        = "%s"
	certificate = "%s"
	description = "origin-set acceptance test"
	config = {
		origin_sets = [
			{
				name = "my-failover-set-2"
				origins = [
					{
						custom_origin = {
							host     = "primary-b.example.com"
							protocol = "https"
						}
					},
					{
						custom_origin = {
							host     = "failover-b.example.com"
							protocol = "https"
						}
					}
				]
			},
			{
				name = "my-failover-set"
				origins = [
					{
						custom_origin = {
							host     = "primary.example.com"
							protocol = "https"
						}
					},
					{
						custom_origin = {
							host     = "failover.example.com"
							protocol = "https"
						}
					}
				]
			}
		]
		domains = [
			{
				domain = "%s"
				mappings = [
					{
						target_type    = "origin_set"
						target_mapping = "my-failover-set"
					}
				]
			}
		]
	}
}`,

		// Step 7: swap order AND add failover_response_codes to set 2 simultaneously.
		// set2 at [0] (with new failover codes), set1 at [1] — reorder+update in one step.
		`
resource "ioriver_service" "%s" {
	name        = "%s"
	certificate = "%s"
	description = "origin-set acceptance test"
	config = {
		origin_sets = [
			{
				name                    = "my-failover-set-2"
				failover_response_codes = [503, 504]
				origins = [
					{
						custom_origin = {
							host     = "primary-b.example.com"
							protocol = "https"
						}
					},
					{
						custom_origin = {
							host     = "failover-b.example.com"
							protocol = "https"
						}
					}
				]
			},
			{
				name = "my-failover-set"
				origins = [
					{
						custom_origin = {
							host     = "primary.example.com"
							protocol = "https"
						}
					},
					{
						custom_origin = {
							host     = "failover.example.com"
							protocol = "https"
						}
					}
				]
			}
		]
		domains = [
			{
				domain = "%s"
				mappings = [
					{
						target_type    = "origin_set"
						target_mapping = "my-failover-set"
					}
				]
			}
		]
	}
}`,
	}

	return fmt.Sprintf(steps[idx], resourceName, resourceName, certId, domainHost)
}
