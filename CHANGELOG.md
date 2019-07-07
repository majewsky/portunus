This project follows [semantic versioning](https://semver.org/spec/v2.0.0.html). If you believe that
SemVer was not adhered to in one of our releases, please open an issue.

# v1.0.0-beta.3 (TBD)

Changes:

- Rename the `memberOf` attribute to `isMemberOf` to accommodate OpenLDAP
  versions that auto-define the `memberOf` attribute according to the
  slapo-memberof overlay.

# v1.0.0-beta.2 (2019-07-07)

Changes:

- Enable debug logging of slapd when `PORTUNUS_DEBUG` is set.

Bugfixes:

- Fix an error where the `portunus-viewers` virtual group could not be created
  in LDAP when it has no members.

# v1.0.0-beta.1 (2019-07-07)

Initial release.
