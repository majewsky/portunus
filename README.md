# Portunus

Portunus was the ancient Roman god of keys and doors. However, this repo does not
contain the god. It contains Portunus, a small and self-contained user/group
management and authentication service.

TODO explain more

Notes:

- `slapd` and `slappasswd` must be in `$PATH`
- schema files are read from `/etc/openldap/schema/*.schema`
- required env variables: `$PORTUNUS_LDAP_SUFFIX`, `$XDG_RUNTIME_DIR`
