This project follows [semantic versioning](https://semver.org/spec/v2.0.0.html). If you believe that
SemVer was not adhered to in one of our releases, please open an issue.

# v1.1.0-beta.1 (2022-07-31)

New features:

- Add "sshPublicKey" attribute. This attribute can also be maintained by users via self-service.
- Add seeding to support statically-configured users and groups.

Changes:

- Update all Go library dependencies.
- Modernize build system to fully use Go modules. The go-bindata dependency has been removed.

# v1.0.0 (2020-01-01)

New features:

- The README now describes how to connect applications to Portunus.

Changes:

- Use the [xyrillian.css](https://github.com/majewsky/xyrillian.css) framework.

# v1.0.0-beta.5 (2019-07-12)

New features:

- Add optional email address field to user accounts.
- Export email address to LDAP as `email` attribute.

# v1.0.0-beta.4 (2019-07-10)

New features:

- Add LDAPS support.

# v1.0.0-beta.3 (2019-07-07)

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
