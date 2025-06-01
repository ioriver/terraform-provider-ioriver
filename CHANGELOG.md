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