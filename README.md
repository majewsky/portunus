![Logo](./static/img/logo.png)

Portunus was the ancient Roman god of keys and doors. However, this repo does not
contain the god. It contains Portunus, a small and self-contained user/group
management and authentication service.

In this document:

* [Overview](#overview)
* [Running](#running)
  * [HTTP access](#http-access)
  * [LDAP directory structure](#ldap-directory-structure)
* [Connecting services to Portunus](#connecting-services-to-portunus)

## Overview

Portunus is aimed at individuals and small organizations that want to manage users and permissions
across different services, and don't want to deal with the minutiae of LDAP administration. This
product includes:

- a simple and clean web GUI for managing user accounts and group memberships (no JavaScript
  required!),
- a fully-functional OpenLDAP server that services can use to authenticate users.
- SAML or OAuth support will be added as soon as someone writes the code.

The OpenLDAP server is completely managed by Portunus. No LDAP experience is required to run
Portunus beyond what this README explains.

## Running

Once installed, run `portunus-orchestrator` with root privileges. Config is passed to it via the
following environment variables:

| Variable | Default | Explanation |
| -------- | ------- | ----------- |
| `PORTUNUS_DEBUG` | `false` | When true, log debug messages to standard error. May cause passwords to be logged. **Do not use in production.** |
| `PORTUNUS_LDAP_SUFFIX` | *(required)* | The DN of the topmost entry in your LDAP directory. Must currently be a sequence of `dc=xxx` RDNs. (This requirement may be lifted in future versions.) See [*LDAP directory structure*](#ldap-directory-structure) for details and a guide-level explanation. |
| `PORTUNUS_SERVER_BINARY` | `portunus-server` | Where to find the portunus-server binary. Semantics match those of `execvp(3)`: If the supplied value is not a path containing slashes, `$PATH` will be searched for it. |
| `PORTUNUS_SERVER_GROUP`<br>`PORTUNUS_SERVER_USER` | `portunus` each | The Unix user/group that Portunus' own server will be run as. |
| `PORTUNUS_SERVER_HTTP_LISTEN` | `127.0.0.1:8080` | Listen address where Portunus' HTTP server shall be running. |
| `PORTUNUS_SERVER_HTTP_SECURE` | `true` | **Do not unset this flag in productive deployments.** In test deployments, this can be set to `false` so that the web GUI works without TLS. |
| `PORTUNUS_SERVER_STATE_DIR` | `/var/lib/portunus` | The path where Portunus stores its database. **Set up a backup for this directory.** |
| `PORTUNUS_SLAPD_BINARY` | `slapd` | Where to find the binary of slapd (the OpenLDAP server). Semantics match those of `execvp(3)`: If the supplied value is not a path containing slashes, `$PATH` will be searched for it. |
| `PORTUNUS_SLAPD_GROUP`<br>`PORTUNUS_SLAPD_USER` | `ldap` each | The Unix user/group that slapd will be run as. |
| `PORTUNUS_SLAPD_SCHEMA_DIR` | `/etc/openldap/schema` | Where to find OpenLDAP's schema definitions. |
| `PORTUNUS_SLAPD_STATE_DIR` | `/var/run/portunus-slapd` | The path where slapd stores its database. The contents of this directory are ephemeral and will be wiped when Portunus restarts, so you do not need to back this up. Place this on a tmpfs for optimal performance. |
| `PORTUNUS_SLAPD_TLS_CERTIFICATE` | *(optional)* | **Recommended for productive deployments.** The path to the TLS certificate of the LDAP server. When given, LDAPS (on port 636) is served instead of LDAP (on port 389). |
| `PORTUNUS_SLAPD_TLS_CA_CERTIFICATE` | *(optional)* | *Required* when a TLS certificate is given. The full chain of CA certificates which has signed the TLS certificate, *including the root CA*. |
| `PORTUNUS_SLAPD_TLS_DOMAIN_NAME` | *(optional)* | *Required* when a TLS certificate is given. The domain name for which the certificate is valid. `portunus-server` will use this domain name when connecting to the LDAP server. |
| `PORTUNUS_SLAPD_TLS_PRIVATE_KEY` | *(optional)* | *Required* when a TLS certificate is given. The path to the private key belonging to the TLS certificate. |

Root privileges are required for the orchestrator because it needs to setup runtime directories and
bind the LDAP port which is a privileged port (389 without TLS, 636 with TLS). No process managed by
Portunus will offer a network service while running as root:

- LDAP and LDAPS are offered by slapd which is running as `ldap:ldap` by default.
- HTTP is offered by `portunus-server` which is running as `portunus:portunus` by default.

When Portunus first starts up, it will create an empty database with the initial user account
`admin`, and show that user's initial password on stdout **once**. It is highly recommended to
change this initial password after the first login.

### HTTP access

In a productive environment, the HTTP frontend offered by `portunus-server` MUST be secured with TLS
by putting it behind a TLS-capable reverse proxy such as httpd, nginx or haproxy.

### LDAP directory structure

*If you know LDAP, you can skip ahead to the table at the end of this section.*

Okay, you need just a tiny tiny bit of LDAP knowledge to understand this, so here we go. Objects in
an LDAP directory are identified by *Distinguished Names* (DNs), which have a structure sort of
similar to domain names. A domain name like

```
example.org
|-----| |-|
 word   word
```

is a dot-separated list of words where the most-specific word is on the left and the least-specific
one is on the right. Similarly, a distinguished name like

```
uid=john,ou=users,dc=example,dc=org
|------| |------| |--------| |----|
  RDN      RDN       RDN      RDN
```

is a comma-separated list of *Relative Distinguished Names* (RDNs), which in 99.9% of cases just
look like `attributename=value`, again with the most-specific RDN on the left. The attribute name
says something about the type of the object. In this example, starting from the right, we have
domain components (dc) describing the example.org domain. Below those domain components is an
organizational unit (ou) containing the users of example.org, and below that is the user "john".

Portunus defines the whole directory structure below the domain component objects in a way that
matches conventional LDAP design, but it's up to you to specify the domain component objects in the
`PORTUNUS_LDAP_SUFFIX` variable. If your services are below some domain, e.g. `foo.bar.tld`, your
LDAP suffix should match that domain, e.g. `dc=foo,dc=bar,dc=tld`. If you are on a private network
and don't have any domains registered, you can pick one under the `.home` or `.corp` TLDs for
your purposes and derive the suffix from that like above.

In the end, it doesn't matter much which suffix you pick, but this procedure ensures that Portunus
generates a nice standards-conformant LDAP directory. That way, if you ever need to switch to a
different LDAP setup, you can migrate your existing directory more easily.

With that out of the way, the following table shows all the objects that Portunus puts in the LDAP
directory. This just serves as a reference. If you just want to find out how to connect services to
Portunus, skip ahead to [the next section](#connecting-services-to-portunus).

For illustrative purposes, `dc=example,dc=org` is used as the `PORTUNUS_LDAP_SUFFIX`. The last column only lists those attributes that are not implied by the object's RDN.

| DN | Object classes | Explanation |
| -- | -------------- | ----------- |
| `dc=example,dc=org` | dcObject | |
| `cn=portunus,dc=example,dc=org` | organizationalRole | The service user used by `portunus-server`. This is the only LDAP user with full write privileges. |
| `cn=nobody,dc=example,dc=org` | organizationalRole | Since groups must have at least one `member` attribute, this dummy user is a member of all groups that have no actual members. |
| `ou=users,dc=example,dc=org` | organizationalUnit | Contains all user accounts. |
| `uid=xxx,ou=users,dc=example,dc=org` | posixAccount&nbsp;(maybe)<br>inetOrgPerson<br>organizationalPerson<br>person | A user account. The `uid` attribute is the login name.<br>*Attributes:* cn, sn, givenName, email (maybe), userPassword, isMemberOf&nbsp;(maybe; list of DNs).<br>*Attributes for POSIX users:* uidNumber, gidNumber, homeDirectory, loginShell&nbsp;(maybe), gecos. |
| `ou=groups,dc=example,dc=org` | organizationalUnit | Contains all groups. |
| `cn=xxx,ou=groups,dc=example,dc=org` | groupOfNames | A group. The `cn` attribute is the group name. *Attributes:* member (list of DNs). |
| `ou=posix-groups,dc=example,dc=org` | organizationalUnit | Contains duplicates of all groups that are POSIX groups, because the `groupOfNames` and `posixGroup` object classes are mutually exclusive. |
| `cn=xxx,ou=posix-groups,dc=example,dc=org` | posixGroup | A POSIX group. The `cn` attribute is the group name. *Attributes:* gidNumber, memberUid (list of login names). |

## Connecting services to Portunus

TODO describe how to consume Portunus' LDAP service from applications
