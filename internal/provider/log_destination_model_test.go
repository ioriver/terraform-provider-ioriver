package provider

import (
	"fmt"
)

// ─── Config generators ────────────────────────────────────────────────────────

// testAccServiceConfigLogDestSteps returns step HCL for the log destination
// acceptance test. Parameters: resourceName, certId, logDestName1, logDestName2,
// behaviorName.
//
//	Step 0:  one log dest, NO credentials (import-friendly baseline)
//	Step 1:  same log dest, WITH credentials
//	Step 1b: omit credentials — verifies API keeps existing ones when not sent
//	Step 2:  add a behavior that streams to it
//	Step 3:  add a second log dest, behavior still streams to first
//	Step 4:  swap order of the two log dests in HCL (regression for list ordering)
func testAccServiceConfigLogDestSteps(idx int, params ...any) string {

	steps := []string{
		// ── Step 0: one log destination, NO credentials ───────────────────────
		`
locals {
	log_dest_name_1 = "%[4]s"
}

resource "ioriver_service" "%[1]s" {
	name        = "%[1]s"
	certificate = "%[2]s"
	description = "log dest flow test"

	config = {
		log_destinations = [
			{
				name = local.log_dest_name_1
				aws_s3 = {
					name   = "test-log-bucket"
					path   = "/"
					region = "us-east-1"
				}
			}
		]
	}
}`,

		// ── Step 1: same log dest, WITH credentials ───────────────────────────
		`
locals {
	log_dest_name_1 = "%[4]s"
}

resource "ioriver_service" "%[1]s" {
	name        = "%[1]s"
	certificate = "%[2]s"
	description = "log dest flow test"

	config = {
		log_destinations = [
			{
				name = local.log_dest_name_1
				aws_s3 = {
					name   = "test-log-bucket"
					path   = "/"
					region = "us-east-1"
					credentials_version = 1
					credentials = {
						access_key = {
							access_key = "AKIAIOSFODNN7EXAMPLE"
							secret_key = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
						}
					}
				}
			}
		]
	}
}`,

		// ── Step 1b: omit credentials — tests optional-on-update semantics ──────
		// If the API accepts this, it keeps existing credentials (only need them
		// on create/rotate).
		`
locals {
	log_dest_name_1 = "%[4]s"
}

resource "ioriver_service" "%[1]s" {
	name        = "%[1]s"
	certificate = "%[2]s"
	description = "log dest flow test"

	config = {
		log_destinations = [
			{
				name = local.log_dest_name_1
				aws_s3 = {
					name   = "test-log-bucket"
					path   = "/"
					region = "us-east-1"
				}
			}
		]
	}
}`,

		// ── Step 2: add a behavior that streams to log_dest_name_1 ────────────
		`
locals {
	log_dest_name_1 = "%[4]s"
}

resource "ioriver_service" "%[1]s" {
	name        = "%[1]s"
	certificate = "%[2]s"
	description = "log dest flow test"

	config = {
		log_destinations = [
			{
				name = local.log_dest_name_1
				aws_s3 = {
					name   = "test-log-bucket"
					path   = "/"
					region = "us-east-1"
					credentials_version = 1
					credentials = {
						access_key = {
							access_key = "AKIAIOSFODNN7EXAMPLE"
							secret_key = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
						}
					}
				}
			}
		]

		behaviors = {
			custom = [
				{
					name         = "%[6]s"
					path_pattern = "//*"
					actions = {
						cache_behavior = "BYPASS"
						cache_ttl      = 0
						stream_logs = {
							log_destination   = local.log_dest_name_1
							log_sampling_rate = 100
						}
					}
				}
			]
		}
	}
}`,

		// ── Step 3: add second log destination ───────────────────────────────
		`
locals {
	log_dest_name_1 = "%[4]s"
	log_dest_name_2 = "%[5]s"
}

resource "ioriver_service" "%[1]s" {
	name        = "%[1]s"
	certificate = "%[2]s"
	description = "log dest flow test"

	config = {



		log_destinations = [
			{
				name = local.log_dest_name_1
				aws_s3 = {
					name   = "test-log-bucket"
					path   = "/"
					region = "us-east-1"
					credentials_version = 1
					credentials = {
						access_key = {
							access_key = "AKIAIOSFODNN7EXAMPLE"
							secret_key = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
						}
					}
				}
			},
			{
				name = local.log_dest_name_2
				aws_s3 = {
					name   = "test-log-bucket-2"
					path   = "/2/"
					region = "eu-west-1"
					credentials_version = 1
					credentials = {
						access_key = {
							access_key = "AKIAIOSFODNN7EXAMPLE"
							secret_key = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
						}
					}
				}
			}
		]

		behaviors = {
			custom = [
				{
					name         = "%[6]s"
					path_pattern = "//*"
					actions = {
						cache_behavior = "BYPASS"
						cache_ttl      = 0
						stream_logs = {
							log_destination   = local.log_dest_name_1
							log_sampling_rate = 100
						}
					}
				}
			]
		}
	}
}`,

		// ── Step 4: swap order of log destinations in HCL ────────────────────
		`
locals {
	log_dest_name_1 = "%[4]s"
	log_dest_name_2 = "%[5]s"
}

resource "ioriver_service" "%[1]s" {
	name        = "%[1]s"
	certificate = "%[2]s"
	description = "log dest flow test"

	config = {



		log_destinations = [
			{
				name = local.log_dest_name_2
				aws_s3 = {
					name   = "test-log-bucket-2"
					path   = "/2/"
					region = "eu-west-1"
					credentials_version = 1
					credentials = {
						access_key = {
							access_key = "AKIAIOSFODNN7EXAMPLE"
							secret_key = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
						}
					}
				}
			},
			{
				name = local.log_dest_name_1
				aws_s3 = {
					name   = "test-log-bucket"
					path   = "/"
					region = "us-east-1"
					credentials_version = 1
					credentials = {
						access_key = {
							access_key = "AKIAIOSFODNN7EXAMPLE"
							secret_key = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
						}
					}
				}
			}
		]

		behaviors = {
			custom = [
				{
					name         = "%[6]s"
					path_pattern = "//*"
					actions = {
						cache_behavior = "BYPASS"
						cache_ttl      = 0
						stream_logs = {
							log_destination   = local.log_dest_name_1
							log_sampling_rate = 100
						}
					}
				}
			]
		}
	}
}`,
		// step 5 - swap order AND update alpha's bucket name simultaneously.
		// This proves the plan modifier correctly handles reorder+update in one step.
		`
locals {
	log_dest_name_1 = "%[4]s"
	log_dest_name_2 = "%[5]s"
}

resource "ioriver_service" "%[1]s" {
	name        = "%[1]s"
	certificate = "%[2]s"
	description = "log dest flow test"

	config = {
		log_destinations = [
			{
				name = local.log_dest_name_2
				aws_s3 = {
					name   = "test-log-bucket-2"
					path   = "/2/"
					region = "eu-west-1"
					credentials_version = 1
					credentials = {
						access_key = {
							access_key = "AKIAIOSFODNN7EXAMPLE"
							secret_key = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
						}
					}
				}
			},
			{
				name = local.log_dest_name_1
				aws_s3 = {
					name   = "test-log-bucket-UPDATED"
					path   = "/"
					region = "us-east-1"
					credentials_version = 1
					credentials = {
						access_key = {
							access_key = "AKIAIOSFODNN7EXAMPLE"
							secret_key = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
						}
					}
				}
			}
		]

		behaviors = {
			custom = [
				{
					name         = "%[6]s"
					path_pattern = "/*"
					actions = {
						cache_behavior = "BYPASS"
						cache_ttl      = 0
						stream_logs = {
							log_destination   = local.log_dest_name_1
							log_sampling_rate = 100
						}
					}
				}
			]
		}
	}
}`,

		// ── Step 6: replace log dest 1 with compatible_s3 type ───────────────
		`
locals {
	log_dest_name_1 = "%[4]s"
	log_dest_name_2 = "%[5]s"
}

resource "ioriver_service" "%[1]s" {
	name        = "%[1]s"
	certificate = "%[2]s"
	description = "log dest flow test"

	config = {
		log_destinations = [
			{
				name = local.log_dest_name_2
				aws_s3 = {
					name   = "test-log-bucket-2"
					path   = "/2/"
					region = "eu-west-1"
					credentials_version = 1
					credentials = {
						access_key = {
							access_key = "AKIAIOSFODNN7EXAMPLE"
							secret_key = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
						}
					}
				}
			},
			{
				name = local.log_dest_name_1
				compatible_s3 = {
					name   = "compat-bucket"
					path   = "/"
					region = "us-east-1"
					domain = "https://s3.example.com"
					credentials_version = 1
					credentials = {
						access_key = {
							access_key = "AKIAIOSFODNN7EXAMPLE"
							secret_key = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
						}
					}
				}
			}
		]

		behaviors = {
			custom = [
				{
					name         = "%[6]s"
					path_pattern = "//*"
					actions = {
						cache_behavior = "BYPASS"
						cache_ttl      = 0
						stream_logs = {
							log_destination   = local.log_dest_name_1
							log_sampling_rate = 100
						}
					}
				}
			]
		}
	}
}`,

		// ── Step 7: remove log dest 1, only log dest 2 remains ───────────────
		`
locals {
	log_dest_name_2 = "%[5]s"
}

resource "ioriver_service" "%[1]s" {
	name        = "%[1]s"
	certificate = "%[2]s"
	description = "log dest flow test"

	config = {


		log_destinations = [
			{
				name = local.log_dest_name_2
				aws_s3 = {
					name   = "test-log-bucket-2"
					path   = "/2/"
					region = "eu-west-1"
					credentials_version = 1
					credentials = {
						access_key = {
							access_key = "AKIAIOSFODNN7EXAMPLE"
							secret_key = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
						}
					}
				}
			}
		]
	}
}`,
	}

	if idx < 0 || idx >= len(steps) {
		panic("invalid log dest step index")
	}
	// params order: resourceName, certId, (unused %[3]s placeholder), logDestName1, logDestName2, behaviorName
	// We use named indices (%[1]s…%[6]s), so pass all six positional args.
	// params received: resourceName, certId, logDestName1, logDestName2, behaviorName
	// Map to: %[1]s=resourceName, %[2]s=certId, %[3]s="" (unused), %[4]s=logDestName1, %[5]s=logDestName2, %[6]s=behaviorName
	rn := fmt.Sprintf("%v", params[0])
	ci := fmt.Sprintf("%v", params[1])
	d1 := fmt.Sprintf("%v", params[2])
	d2 := fmt.Sprintf("%v", params[3])
	bn := fmt.Sprintf("%v", params[4])
	return fmt.Sprintf(steps[idx], rn, ci, "", d1, d2, bn)
}
