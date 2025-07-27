## 0.38.0 (2025-06-02)

#### 🎁 Feature

* **ioriver-go:** New release (295a2f16)
* **ioriver-go:** New release (41be236b)

#### 📦 Build

* **deps:** bump golang.org/x/net in the go_modules group (8163a72d)
* **deps:** bump the go_modules group with 2 updates (0c9f36c4)

## What's Changed
* build(deps): bump the go_modules group with 2 updates by @dependabot[bot] in https://github.com/ioriver/terraform-provider-ioriver/pull/61
* build(deps): bump golang.org/x/net from 0.36.0 to 0.38.0 in the go_modules group by @dependabot[bot] in https://github.com/ioriver/terraform-provider-ioriver/pull/67
* feat(ioriver-go): New release by @maayan-ioriver in https://github.com/ioriver/terraform-provider-ioriver/pull/68

## New Contributors
* @dependabot[bot] made their first contribution in https://github.com/ioriver/terraform-provider-ioriver/pull/61

**Full Changelog**: https://github.com/ioriver/terraform-provider-ioriver/compare/v0.37.0...v0.38.0

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
### Breaking Changes *****
- Resource `example_resource`: The attribute `old_name` has been renamed to `new_name` to align with the upstream API. 
- Existing configurations using `old_name` will need to be updated. See the upgrade guide for detailed instructions.


### Added
- origin-shield documentation

### Fixed
- Fix operations which were not done under global lock to prevent 'other operation in progress' error