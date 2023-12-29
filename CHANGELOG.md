This project follows [semantic versioning](https://semver.org/spec/v2.0.0.html). If you believe that
SemVer was not adhered to in one of our releases, please open an issue.

# v2.1.0 (TBD)

Changes:

- The size of the orchestrator binary that runs with root privileges has been reduced by about 10-15% by replacing
  usages of a regex engine with explicit string parsers.
- Binaries can now be installed with `go install` if `make` is not available for some reason.

# v2.0.0 (2023-12-27)

For relevant changes including backwards-incompatible changes, please refer to v2.0.0-beta.1 below.

# v2.0.0-beta.2 (2023-11-09)

Changes since beta.1:

- A bug was fixed where the LDAP server initialization could deadlock on databases with more than 64 users and groups.
- Interactive changes to the database will not fail anymore if there is an unrelated user with a seeded password.

# v2.0.0-beta.1 (2023-10-29)

**Backwards-incompatible changes:**

- Portunus now links libcrypt and requires several features that are specific to [libxcrypt][libxcrypt]. Most Linux
  distributions already use libxcrypt as their libcrypt in order to support non-ancient password hashes, so this
  requirement should hopefully not be too painful for Linux users. Note that Portunus must use the same libcrypt as its
  `slapd`, otherwise both parties might disagree on how password hashes work.

New features:

- With the move to libxcrypt, Portunus supports all the same strong password hashes that libxcrypt supports (such as
  bcrypt and yescrypt).
- Existing user accounts with weak password hashes in your Portunus database will continue to work. After the upgrade,
  instruct all your users to log into the Portunus UI once. Upon successful login, Portunus will transparently upgrade
  their stored password hashes to a stronger hash method. To enumerate users that have not been upgraded to a stronger
  hash method yet, use this command:
  ```sh
  jq -r '.users[] | select(.password | match("^\\{CRYPT\\}\\$5\\$")) | "\(.login_name) <\(.email)>"' < /var/lib/portunus/database.json
  ```
- While creating or updating a group, memberships can be adjusted (without needing to edit the individual users).

Changes:

- The core business logic was completely rewritten into a more modular design suitable for unit tests. Tests have been
  added to cover the logic core, including seeding and validation, the LDAP handling as well as the disk store handling.
  The only major gap in the automated test coverage is the UI, which is still being tested manually for the time being.
  At least one bug was discovered and fixed by the new test suite, and more bugs may have been fixed by accident during
  the rewrite. :)

[libxcrypt]: https://github.com/besser82/libxcrypt

# v1.1.0 (2022-08-19)

No changes since the last beta.

# v1.1.0-beta.2 (2022-08-07)

New features:

- The login form now also accepts the user's e-mail address instead of their login name.

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
