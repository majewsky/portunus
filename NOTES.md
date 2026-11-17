# Notes from reading core RFCs

RFC4511:

- allow unknown choices on: AuthenticationChoice, Filter
- 4.1.1: unparsable LDAPMessage (error from Unmarshal) -> NoD (4.4.1) and close connection
- 4.1.1: general response for problematic requests -> Result with protocolError
- 4.2: unknown AuthenticationChoice -> Result with authMethodNotSupported
- 4.3: any UnbindRequest -> terminate connection
- 4.5.1.7: malformed filters are not a protocol filter, evaluate to `None[bool]()`
- 4.5.1.7.7: need to research examples of Filter.extensibleMatch
- 6: test suite for protocol errors at <http://web.archive.org/*/www.ee.oulu.fi/research/ouspg/protos/testing/c06/ldapv3/>

RFC 4512:

- TODO
- model must include root DSE (TODO: where?)

RFC 4513:

- 5.1.1: accept SimpleAuthentication with empty username and password, set authstate to anonymous (same as before first BindRequest; i.e. forbid everything except bind)
- 5.1.2: reject SimpleAuthentication with empty username XOR empty password -> Result with unwillingToPerform (5.1.3 says undefined behavior for empty username, but non-empty password)
- 5.1.3: invalid password on SimpleAuthentication -> Result with invalidCredentials
- 5.2: all SASL requests -> Result with authMethodNotSupported

RFC 4514:

- TODO

RFC 4517:

- includes definitions for the standard syntaxes and matching rules

RFC 4519:

- includes definitions for the standard attribute types and object classes

# Required extensions/controls

- RFC 3673: feature 1.3.6.1.4.1.4203.1.5.1; attribute description "+" to return operational attributes)
- RFC 6171: control 1.3.6.1.1.22 (Don't Use Copy); just accept and do not do anything
- RFC 4526: feature 1.3.6.1.4.1.4203.1.5.3; allow empty and and or filters (may just implement this by accident)

# Nice-to-haves

- RFC 2891: control 1.2.840.113556.1.4.473 (Server Side Sorting of Search Results)
- RFC 3876: control 1.2.826.0.1.3344810.2.3 (Returning Matched Values)
- RFC 4528: control 1.3.6.1.1.12 (Assertion Control) for search requests
- RFC 4529: feature 1.3.6.1.4.1.4203.1.5.2 (Return of All Attributes of an Object Class)
- RFC 4532: extension 1.3.6.1.4.1.4203.1.11.3 (Who Am I?)
