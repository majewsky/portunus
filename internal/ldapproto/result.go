// SPDX-FileCopyrightText: 2026 Stefan Majewsky <majewsky@gmx.net>
// SPDX-License-Identifier: GPL-3.0-only

package ldapproto

import (
	"fmt"

	"github.com/majewsky/portunus/internal/ber"
)

// Result represents the LDAPResult type defined in [RFC 4511, 4.1.9].
//
// This type is embedded in various choices for type [Operation], such as [BindResponse].
type Result struct {
	ResultCode        ResultCode
	MatchedDN         DN
	DiagnosticMessage String
	Referral          []URI `ber:"context-specific:3,optional"`
}

// ResultCode is an enum appearing in type [Result].
//
// The docstrings for each value are abridged from [RFC 4511, Appendix A].
type ResultCode int

var _ ber.Enum = ResultCodeSuccess

const (
	// ResultCodeSuccess indicates the successful completion of an operation (except for the Compare operation).
	ResultCodeSuccess ResultCode = 0
	// ResultCodeOperationsError indicates that the operation is not properly sequenced with relation to other operations (of same or different type).
	ResultCodeOperationsError ResultCode = 1
	// ResultCodeProtocolError indicates the server received data that is not well-formed.
	ResultCodeProtocolError ResultCode = 2
	// ResultCodeTimeLimitExceeded indicates that the time limit specified by the client was exceeded before the operation could be completed.
	ResultCodeTimeLimitExceeded ResultCode = 3
	// ResultCodeSizeLimitExceeded indicates that the size limit specified by the client was exceeded before the operation could be completed.
	ResultCodeSizeLimitExceeded ResultCode = 4
	// ResultCodeCompareFalse indicates that the Compare operation has successfully completed and the assertion has evaluated to FALSE or Undefined.
	ResultCodeCompareFalse ResultCode = 5
	// ResultCodeCompareTrue indicates that the Compare operation has successfully completed and the assertion has evaluated to TRUE.
	ResultCodeCompareTrue ResultCode = 6
	// ResultCodeAuthMethodNotSupported indicates that the authentication method or mechanism is not supported.
	ResultCodeAuthMethodNotSupported ResultCode = 7
	// ResultCodeStrongerAuthRequired indicates the server requires strong(er) authentication in order to complete the operation.
	ResultCodeStrongerAuthRequired ResultCode = 8
	// ResultCodeReferral indicates that a referral needs to be chased to complete the operation
	ResultCodeReferral ResultCode = 10
	// ResultCodeAdminLimitExceeded indicates that an administrative limit has been exceeded.
	ResultCodeAdminLimitExceeded ResultCode = 11
	// ResultCodeUnavailableCriticalExtension indicates a critical control is unrecognized.
	ResultCodeUnavailableCriticalExtension ResultCode = 12
	// ResultCodeConfidentialityRequired indicates that data confidentiality protections are required.
	ResultCodeConfidentialityRequired ResultCode = 13
	// ResultCodeSaslBindInProgress indicates the server requires the client to send a new bind request, with the same SASL mechanism, to continue the authentication process.
	ResultCodeSaslBindInProgress ResultCode = 14
	// ResultCodeNoSuchAttribute indicates that the named entry does not contain the specified attribute or attribute value.
	ResultCodeNoSuchAttribute ResultCode = 16
	// ResultCodeUndefinedAttributeType indicates that a request field contains an unrecognized attribute description.
	ResultCodeUndefinedAttributeType ResultCode = 17
	// ResultCodeInappropriateMatching indicates that an attempt was made (e.g., in an assertion) to use a matching rule not defined for the attribute type concerned.
	ResultCodeInappropriateMatching ResultCode = 18
	// ResultCodeConstraintViolation indicates that the client supplied an attribute value that does not conform to the constraints placed upon it by the data model.
	ResultCodeConstraintViolation ResultCode = 19
	// ResultCodeAttributeOrValueExists indicates that the client supplied an attribute or value to be added to an entry, but the attribute or value already exists.
	ResultCodeAttributeOrValueExists ResultCode = 20
	// ResultCodeInvalidAttributeSyntax indicates that a purported attribute value does not conform to the syntax of the attribute.
	ResultCodeInvalidAttributeSyntax ResultCode = 21
	// ResultCodeNoSuchObject indicates that the object does not exist in the DIT.
	ResultCodeNoSuchObject ResultCode = 32
	// ResultCodeAliasProblem indicates that an alias problem has occurred.
	ResultCodeAliasProblem ResultCode = 33
	// ResultCodeInvalidDNSyntax indicates that an LDAPDN or RelativeLDAPDN field of a request does not conform to the required syntax or contains attribute values that do not conform to the syntax of the attribute's type.
	ResultCodeInvalidDNSyntax ResultCode = 34
	// ResultCodeAliasDereferencingProblem indicates that a problem occurred while dereferencing an alias.
	ResultCodeAliasDereferencingProblem ResultCode = 36
	// ResultCodeInappropriateAuthentication indicates the server requires the client that had attempted to bind anonymously or without supplying credentials to provide some form of credentials.
	ResultCodeInappropriateAuthentication ResultCode = 48
	// ResultCodeInvalidCredentials indicates that the provided credentials (e.g., the user's name and password) are invalid.
	ResultCodeInvalidCredentials ResultCode = 49
	// ResultCodeInsufficientAccessRights indicates that the client does not have sufficient access rights to perform the operation.
	ResultCodeInsufficientAccessRights ResultCode = 50
	// ResultCodeBusy indicates that the server is too busy to service the operation.
	ResultCodeBusy ResultCode = 51
	// ResultCodeUnavailable indicates that the server is shutting down or a subsystem necessary to complete the operation is offline.
	ResultCodeUnavailable ResultCode = 52
	// ResultCodeUnwillingToPerform indicates that the server is unwilling to perform the operation.
	ResultCodeUnwillingToPerform ResultCode = 53
	// ResultCodeLoopDetect indicates that the server has detected an internal loop (e.g., while dereferencing aliases or chaining an operation).
	ResultCodeLoopDetect ResultCode = 54
	// ResultCodeNamingViolation indicates that the entry's name violates naming restrictions.
	ResultCodeNamingViolation ResultCode = 64
	// ResultCodeObjectClassViolation indicates that the entry violates object class restrictions.
	ResultCodeObjectClassViolation ResultCode = 65
	// ResultCodeNotAllowedOnNonLeaf indicates that the operation is inappropriately acting upon a non-leaf entry.
	ResultCodeNotAllowedOnNonLeaf ResultCode = 66
	// ResultCodeNotAllowedOnRDN indicates that the operation is inappropriately attempting to remove a value that forms the entry's relative distinguished name.
	ResultCodeNotAllowedOnRDN ResultCode = 67
	// ResultCodeEntryAlreadyExists indicates that the request cannot be fulfilled (added, moved, or renamed) as the target entry already exists.
	ResultCodeEntryAlreadyExists ResultCode = 68
	// ResultCodeObjectClassModsProhibited indicates that an attempt to modify the object class(es) of an entry's 'objectClass' attribute is prohibited.
	ResultCodeObjectClassModsProhibited ResultCode = 69
	// ResultCodeAffectsMultipleDSAs indicates that the operation cannot be performed as it would affect multiple servers (DSAs).
	ResultCodeAffectsMultipleDSAs ResultCode = 71
	// ResultCodeOther indicates the server has encountered an internal error.
	ResultCodeOther ResultCode = 80
)

// IsEnum implements the [ber.Enum] interface.
func (ResultCode) IsEnum() bool { return true }

// Validate implements the [ber.Enum] interface.
func (c ResultCode) Validate() error {
	switch c {
	case ResultCodeSuccess,
		ResultCodeOperationsError,
		ResultCodeProtocolError,
		ResultCodeTimeLimitExceeded,
		ResultCodeSizeLimitExceeded,
		ResultCodeCompareFalse,
		ResultCodeCompareTrue,
		ResultCodeAuthMethodNotSupported,
		ResultCodeStrongerAuthRequired,
		ResultCodeReferral,
		ResultCodeAdminLimitExceeded,
		ResultCodeUnavailableCriticalExtension,
		ResultCodeConfidentialityRequired,
		ResultCodeSaslBindInProgress,
		ResultCodeNoSuchAttribute,
		ResultCodeUndefinedAttributeType,
		ResultCodeInappropriateMatching,
		ResultCodeConstraintViolation,
		ResultCodeAttributeOrValueExists,
		ResultCodeInvalidAttributeSyntax,
		ResultCodeNoSuchObject,
		ResultCodeAliasProblem,
		ResultCodeInvalidDNSyntax,
		ResultCodeAliasDereferencingProblem,
		ResultCodeInappropriateAuthentication,
		ResultCodeInvalidCredentials,
		ResultCodeInsufficientAccessRights,
		ResultCodeBusy,
		ResultCodeUnavailable,
		ResultCodeUnwillingToPerform,
		ResultCodeLoopDetect,
		ResultCodeNamingViolation,
		ResultCodeObjectClassViolation,
		ResultCodeNotAllowedOnNonLeaf,
		ResultCodeNotAllowedOnRDN,
		ResultCodeEntryAlreadyExists,
		ResultCodeObjectClassModsProhibited,
		ResultCodeAffectsMultipleDSAs,
		ResultCodeOther:
		return nil
	default:
		return fmt.Errorf("unacceptable value for ResultCode: %d", c)
	}
}
