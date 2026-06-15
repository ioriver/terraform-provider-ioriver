## [1.0.0] - 2026-06-15

### Breaking Changes

- Resources `domain`, `origin`, `origin_shield`, `behavior`, `log_destination`, `compute`, `protocol_config`, and `url_signing_key` have been removed and are no longer supported.

### Changed

- Service configuration is now provided through the `config` field under the `service` resource.
- The new service configuration includes security configuration such as WAF, custom rules, and rate limiting rules.
- If you need assistance migrating to the new schema, please contact support@ioriver.io.

## [0.42.0] - 2025-08-03

### Changed

- new CHANGELOG.MD

## [0.41.0] - 2025-07-27

## [0.40.0] - 2025-07-27

## [0.39.0] - 2025-07-27

### Added

- add gcp & akamai account providers

### Changed

- bump several go packages versions
- Testing new changelog flow

## [0.38.0] - 2025-06-01

### Added

- url signing

### Changed

- bump several go packages versions

### Fixed

- last open issue
- renew certificate for tests

## [0.37.0] - 2025-03-14

### Added

- new domains interface with aliases

### Fixed

- domain test fixed

### Changed

- bump golang.org/x/crypto from 0.31.0 to 0.35.0

## [0.36.0] - 2025-03-03

### Fixed

- Cloudfront credentials for logs destination

## [0.35.0] - 2025-02-23

### Fixed

- block creation of service provider until it is fully deployed

## [0.34.0] - 2025-02-16

### Breaking Changes **\***

- Resource `example_resource`: The attribute `old_name` has been renamed to `new_name` to align with the upstream API.
- Existing configurations using `old_name` will need to be updated. See the upgrade guide for detailed instructions.

### Added

- origin-shield documentation

### Fixed

- Fix operations which were not done under global lock to prevent 'other operation in progress' error
