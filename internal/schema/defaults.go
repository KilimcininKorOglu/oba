package schema

// Default LDAP schema definitions including core object classes and attribute types.
// These are based on RFC 4512, RFC 4519, and common LDAP implementations.

// defaultAttributeTypes contains the standard LDAP attribute type definitions.
var defaultAttributeTypes = []string{
	// Core attributes (RFC 4512)
	`( 2.5.4.0 NAME 'objectClass' DESC 'Object class membership' EQUALITY objectIdentifierMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.38 )`,
	`( 2.5.4.1 NAME ( 'aliasedObjectName' 'aliasedEntryName' ) DESC 'Aliased object name' EQUALITY distinguishedNameMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.12 SINGLE-VALUE )`,

	// Naming attributes (RFC 4519)
	`( 2.5.4.3 NAME ( 'cn' 'commonName' ) DESC 'Common name' SUP name )`,
	`( 2.5.4.4 NAME ( 'sn' 'surname' ) DESC 'Surname' SUP name )`,
	`( 2.5.4.5 NAME 'serialNumber' DESC 'Serial number' EQUALITY caseIgnoreMatch SUBSTR caseIgnoreSubstringsMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.44 )`,
	`( 2.5.4.6 NAME ( 'c' 'countryName' ) DESC 'Country name' SUP name SINGLE-VALUE )`,
	`( 2.5.4.7 NAME ( 'l' 'localityName' ) DESC 'Locality name' SUP name )`,
	`( 2.5.4.8 NAME ( 'st' 'stateOrProvinceName' ) DESC 'State or province name' SUP name )`,
	`( 2.5.4.9 NAME ( 'street' 'streetAddress' ) DESC 'Street address' EQUALITY caseIgnoreMatch SUBSTR caseIgnoreSubstringsMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.15 )`,
	`( 2.5.4.10 NAME ( 'o' 'organizationName' ) DESC 'Organization name' SUP name )`,
	`( 2.5.4.11 NAME ( 'ou' 'organizationalUnitName' ) DESC 'Organizational unit name' SUP name )`,
	`( 2.5.4.12 NAME 'title' DESC 'Title' SUP name )`,
	`( 2.5.4.13 NAME 'description' DESC 'Description' EQUALITY caseIgnoreMatch SUBSTR caseIgnoreSubstringsMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.15 )`,

	// User attributes
	`( 2.5.4.20 NAME 'telephoneNumber' DESC 'Telephone number' EQUALITY telephoneNumberMatch SUBSTR telephoneNumberSubstringsMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.50 )`,
	`( 2.5.4.35 NAME 'userPassword' DESC 'User password' EQUALITY octetStringMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.40 )`,
	`( 2.5.4.41 NAME 'name' DESC 'Name' EQUALITY caseIgnoreMatch SUBSTR caseIgnoreSubstringsMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.15 )`,
	`( 2.5.4.42 NAME ( 'givenName' 'gn' ) DESC 'Given name' SUP name )`,
	`( 2.5.4.43 NAME 'initials' DESC 'Initials' SUP name )`,
	`( 2.5.4.44 NAME 'generationQualifier' DESC 'Generation qualifier' SUP name )`,
	`( 2.5.4.45 NAME 'x500UniqueIdentifier' DESC 'X.500 unique identifier' EQUALITY bitStringMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.6 )`,
	`( 2.5.4.46 NAME 'dnQualifier' DESC 'DN qualifier' EQUALITY caseIgnoreMatch ORDERING caseIgnoreOrderingMatch SUBSTR caseIgnoreSubstringsMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.44 )`,
	`( 2.5.4.49 NAME 'distinguishedName' DESC 'Distinguished name' EQUALITY distinguishedNameMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.12 )`,

	// Domain component (RFC 4519)
	`( 0.9.2342.19200300.100.1.25 NAME ( 'dc' 'domainComponent' ) DESC 'Domain component' EQUALITY caseIgnoreIA5Match SUBSTR caseIgnoreIA5SubstringsMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.26 SINGLE-VALUE )`,

	// User ID (RFC 4519)
	`( 0.9.2342.19200300.100.1.1 NAME ( 'uid' 'userid' ) DESC 'User ID' EQUALITY caseIgnoreMatch SUBSTR caseIgnoreSubstringsMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.15 )`,

	// Mail (RFC 4524)
	`( 0.9.2342.19200300.100.1.3 NAME ( 'mail' 'rfc822Mailbox' ) DESC 'Email address' EQUALITY caseIgnoreIA5Match SUBSTR caseIgnoreIA5SubstringsMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.26 )`,

	// Member attributes
	`( 2.5.4.31 NAME 'member' DESC 'Member' SUP distinguishedName )`,
	`( 2.5.4.50 NAME 'uniqueMember' DESC 'Unique member' EQUALITY uniqueMemberMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.34 )`,
	`( 2.5.4.34 NAME 'seeAlso' DESC 'See also' SUP distinguishedName )`,

	// Operational attributes (RFC 4512)
	`( 2.5.18.1 NAME 'createTimestamp' DESC 'Creation timestamp' EQUALITY generalizedTimeMatch ORDERING generalizedTimeOrderingMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.24 SINGLE-VALUE NO-USER-MODIFICATION USAGE directoryOperation )`,
	`( 2.5.18.2 NAME 'modifyTimestamp' DESC 'Modification timestamp' EQUALITY generalizedTimeMatch ORDERING generalizedTimeOrderingMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.24 SINGLE-VALUE NO-USER-MODIFICATION USAGE directoryOperation )`,
	`( 2.5.18.3 NAME 'creatorsName' DESC 'Creators name' EQUALITY distinguishedNameMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.12 SINGLE-VALUE NO-USER-MODIFICATION USAGE directoryOperation )`,
	`( 2.5.18.4 NAME 'modifiersName' DESC 'Modifiers name' EQUALITY distinguishedNameMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.12 SINGLE-VALUE NO-USER-MODIFICATION USAGE directoryOperation )`,
	`( 2.5.18.10 NAME 'subschemaSubentry' DESC 'Subschema subentry' EQUALITY distinguishedNameMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.12 SINGLE-VALUE NO-USER-MODIFICATION USAGE directoryOperation )`,
	`( 2.5.21.9 NAME 'structuralObjectClass' DESC 'Structural object class' EQUALITY objectIdentifierMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.38 SINGLE-VALUE NO-USER-MODIFICATION USAGE directoryOperation )`,
	`( 1.3.6.1.1.20 NAME 'entryDN' DESC 'Entry DN' EQUALITY distinguishedNameMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.12 SINGLE-VALUE NO-USER-MODIFICATION USAGE directoryOperation )`,
	`( 1.3.6.1.1.16.4 NAME 'entryUUID' DESC 'Entry UUID' EQUALITY UUIDMatch ORDERING UUIDOrderingMatch SYNTAX 1.3.6.1.1.16.1 SINGLE-VALUE NO-USER-MODIFICATION USAGE directoryOperation )`,
	`( 2.5.21.8 NAME 'hasSubordinates' DESC 'Has subordinates' EQUALITY booleanMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.7 SINGLE-VALUE NO-USER-MODIFICATION USAGE directoryOperation )`,
	`( 2.16.840.1.113730.3.1.69 NAME 'numSubordinates' DESC 'Number of subordinates' EQUALITY integerMatch ORDERING integerOrderingMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.27 SINGLE-VALUE NO-USER-MODIFICATION USAGE directoryOperation )`,
}

// defaultObjectClasses contains the standard LDAP object class definitions.
var defaultObjectClasses = []string{
	// Core object classes (RFC 4512)
	`( 2.5.6.0 NAME 'top' DESC 'Top of the object class hierarchy' ABSTRACT MUST objectClass )`,
	`( 2.5.6.1 NAME 'alias' DESC 'Alias object class' SUP top STRUCTURAL MUST aliasedObjectName )`,

	// RFC 4519 object classes
	`( 2.5.6.2 NAME 'country' DESC 'Country' SUP top STRUCTURAL MUST c MAY ( searchGuide $ description ) )`,
	`( 2.5.6.3 NAME 'locality' DESC 'Locality' SUP top STRUCTURAL MAY ( street $ seeAlso $ searchGuide $ st $ l $ description ) )`,
	`( 2.5.6.4 NAME 'organization' DESC 'Organization' SUP top STRUCTURAL MUST o MAY ( userPassword $ searchGuide $ seeAlso $ businessCategory $ x121Address $ registeredAddress $ destinationIndicator $ preferredDeliveryMethod $ telexNumber $ teletexTerminalIdentifier $ telephoneNumber $ internationaliSDNNumber $ facsimileTelephoneNumber $ street $ postOfficeBox $ postalCode $ postalAddress $ physicalDeliveryOfficeName $ st $ l $ description ) )`,
	`( 2.5.6.5 NAME 'organizationalUnit' DESC 'Organizational unit' SUP top STRUCTURAL MUST ou MAY ( userPassword $ searchGuide $ seeAlso $ businessCategory $ x121Address $ registeredAddress $ destinationIndicator $ preferredDeliveryMethod $ telexNumber $ teletexTerminalIdentifier $ telephoneNumber $ internationaliSDNNumber $ facsimileTelephoneNumber $ street $ postOfficeBox $ postalCode $ postalAddress $ physicalDeliveryOfficeName $ st $ l $ description ) )`,
	`( 2.5.6.6 NAME 'person' DESC 'Person' SUP top STRUCTURAL MUST ( sn $ cn ) MAY ( userPassword $ telephoneNumber $ seeAlso $ description ) )`,
	`( 2.5.6.7 NAME 'organizationalPerson' DESC 'Organizational person' SUP person STRUCTURAL MAY ( title $ x121Address $ registeredAddress $ destinationIndicator $ preferredDeliveryMethod $ telexNumber $ teletexTerminalIdentifier $ telephoneNumber $ internationaliSDNNumber $ facsimileTelephoneNumber $ street $ postOfficeBox $ postalCode $ postalAddress $ physicalDeliveryOfficeName $ ou $ st $ l ) )`,
	`( 2.5.6.8 NAME 'organizationalRole' DESC 'Organizational role' SUP top STRUCTURAL MUST cn MAY ( x121Address $ registeredAddress $ destinationIndicator $ preferredDeliveryMethod $ telexNumber $ teletexTerminalIdentifier $ telephoneNumber $ internationaliSDNNumber $ facsimileTelephoneNumber $ seeAlso $ roleOccupant $ preferredDeliveryMethod $ street $ postOfficeBox $ postalCode $ postalAddress $ physicalDeliveryOfficeName $ ou $ st $ l $ description ) )`,
	`( 2.5.6.9 NAME 'groupOfNames' DESC 'Group of names' SUP top STRUCTURAL MUST ( member $ cn ) MAY ( businessCategory $ seeAlso $ owner $ ou $ o $ description ) )`,
	`( 2.5.6.17 NAME 'groupOfUniqueNames' DESC 'Group of unique names' SUP top STRUCTURAL MUST ( uniqueMember $ cn ) MAY ( businessCategory $ seeAlso $ owner $ ou $ o $ description ) )`,

	// inetOrgPerson (RFC 2798)
	`( 2.16.840.1.113730.3.2.2 NAME 'inetOrgPerson' DESC 'Internet organizational person' SUP organizationalPerson STRUCTURAL MAY ( audio $ businessCategory $ carLicense $ departmentNumber $ displayName $ employeeNumber $ employeeType $ givenName $ homePhone $ homePostalAddress $ initials $ jpegPhoto $ labeledURI $ mail $ manager $ mobile $ o $ pager $ photo $ roomNumber $ secretary $ uid $ userCertificate $ x500uniqueIdentifier $ preferredLanguage $ userSMIMECertificate $ userPKCS12 ) )`,

	// Domain component (RFC 4519)
	`( 0.9.2342.19200300.100.4.13 NAME 'domain' DESC 'Domain' SUP top STRUCTURAL MUST dc MAY ( userPassword $ searchGuide $ seeAlso $ businessCategory $ x121Address $ registeredAddress $ destinationIndicator $ preferredDeliveryMethod $ telexNumber $ teletexTerminalIdentifier $ telephoneNumber $ internationaliSDNNumber $ facsimileTelephoneNumber $ street $ postOfficeBox $ postalCode $ postalAddress $ physicalDeliveryOfficeName $ st $ l $ description $ o $ associatedName ) )`,
	`( 1.3.6.1.4.1.1466.344 NAME 'dcObject' DESC 'Domain component object' SUP top AUXILIARY MUST dc )`,

	// Subschema (RFC 4512)
	`( 2.5.20.1 NAME 'subschema' DESC 'Subschema' AUXILIARY MAY ( dITStructureRules $ nameForms $ ditContentRules $ objectClasses $ attributeTypes $ matchingRules $ matchingRuleUse ) )`,

	// LDAP subentry (RFC 3672)
	`( 2.16.840.1.113719.2.142.6.1.1 NAME 'ldapSubEntry' DESC 'LDAP subentry' SUP top STRUCTURAL MAY cn )`,

	// Extended person (common extension)
	`( 1.3.6.1.4.1.5923.1.2.2 NAME 'eduPerson' DESC 'Educational person' AUXILIARY MAY ( eduPersonAffiliation $ eduPersonNickname $ eduPersonOrgDN $ eduPersonOrgUnitDN $ eduPersonPrimaryAffiliation $ eduPersonPrincipalName $ eduPersonEntitlement $ eduPersonPrimaryOrgUnitDN $ eduPersonScopedAffiliation ) )`,

	// Simple security object
	`( 0.9.2342.19200300.100.4.19 NAME 'simpleSecurityObject' DESC 'Simple security object' SUP top AUXILIARY MUST userPassword )`,

	// Account (RFC 4524)
	`( 0.9.2342.19200300.100.4.5 NAME 'account' DESC 'Account' SUP top STRUCTURAL MUST uid MAY ( description $ seeAlso $ l $ o $ ou $ host ) )`,

	// POSIX account and group (RFC 2307)
	`( 1.3.6.1.1.1.2.0 NAME 'posixAccount' DESC 'POSIX account' SUP top AUXILIARY MUST ( cn $ uid $ uidNumber $ gidNumber $ homeDirectory ) MAY ( userPassword $ loginShell $ gecos $ description ) )`,
	`( 1.3.6.1.1.1.2.2 NAME 'posixGroup' DESC 'POSIX group' SUP top STRUCTURAL MUST ( cn $ gidNumber ) MAY ( userPassword $ memberUid $ description ) )`,
}

// defaultMatchingRules contains the standard LDAP matching rule definitions.
var defaultMatchingRules = []string{
	`( 2.5.13.0 NAME 'objectIdentifierMatch' SYNTAX 1.3.6.1.4.1.1466.115.121.1.38 )`,
	`( 2.5.13.1 NAME 'distinguishedNameMatch' SYNTAX 1.3.6.1.4.1.1466.115.121.1.12 )`,
	`( 2.5.13.2 NAME 'caseIgnoreMatch' SYNTAX 1.3.6.1.4.1.1466.115.121.1.15 )`,
	`( 2.5.13.3 NAME 'caseIgnoreOrderingMatch' SYNTAX 1.3.6.1.4.1.1466.115.121.1.15 )`,
	`( 2.5.13.4 NAME 'caseIgnoreSubstringsMatch' SYNTAX 1.3.6.1.4.1.1466.115.121.1.58 )`,
	`( 2.5.13.5 NAME 'caseExactMatch' SYNTAX 1.3.6.1.4.1.1466.115.121.1.15 )`,
	`( 2.5.13.6 NAME 'caseExactOrderingMatch' SYNTAX 1.3.6.1.4.1.1466.115.121.1.15 )`,
	`( 2.5.13.7 NAME 'caseExactSubstringsMatch' SYNTAX 1.3.6.1.4.1.1466.115.121.1.58 )`,
	`( 2.5.13.8 NAME 'numericStringMatch' SYNTAX 1.3.6.1.4.1.1466.115.121.1.36 )`,
	`( 2.5.13.10 NAME 'numericStringSubstringsMatch' SYNTAX 1.3.6.1.4.1.1466.115.121.1.58 )`,
	`( 2.5.13.11 NAME 'caseIgnoreListMatch' SYNTAX 1.3.6.1.4.1.1466.115.121.1.41 )`,
	`( 2.5.13.13 NAME 'booleanMatch' SYNTAX 1.3.6.1.4.1.1466.115.121.1.7 )`,
	`( 2.5.13.14 NAME 'integerMatch' SYNTAX 1.3.6.1.4.1.1466.115.121.1.27 )`,
	`( 2.5.13.15 NAME 'integerOrderingMatch' SYNTAX 1.3.6.1.4.1.1466.115.121.1.27 )`,
	`( 2.5.13.16 NAME 'bitStringMatch' SYNTAX 1.3.6.1.4.1.1466.115.121.1.6 )`,
	`( 2.5.13.17 NAME 'octetStringMatch' SYNTAX 1.3.6.1.4.1.1466.115.121.1.40 )`,
	`( 2.5.13.18 NAME 'octetStringOrderingMatch' SYNTAX 1.3.6.1.4.1.1466.115.121.1.40 )`,
	`( 2.5.13.20 NAME 'telephoneNumberMatch' SYNTAX 1.3.6.1.4.1.1466.115.121.1.50 )`,
	`( 2.5.13.21 NAME 'telephoneNumberSubstringsMatch' SYNTAX 1.3.6.1.4.1.1466.115.121.1.58 )`,
	`( 2.5.13.22 NAME 'presentationAddressMatch' SYNTAX 1.3.6.1.4.1.1466.115.121.1.43 )`,
	`( 2.5.13.23 NAME 'uniqueMemberMatch' SYNTAX 1.3.6.1.4.1.1466.115.121.1.34 )`,
	`( 2.5.13.27 NAME 'generalizedTimeMatch' SYNTAX 1.3.6.1.4.1.1466.115.121.1.24 )`,
	`( 2.5.13.28 NAME 'generalizedTimeOrderingMatch' SYNTAX 1.3.6.1.4.1.1466.115.121.1.24 )`,
	`( 2.5.13.29 NAME 'integerFirstComponentMatch' SYNTAX 1.3.6.1.4.1.1466.115.121.1.27 )`,
	`( 2.5.13.30 NAME 'objectIdentifierFirstComponentMatch' SYNTAX 1.3.6.1.4.1.1466.115.121.1.38 )`,
	`( 1.3.6.1.4.1.1466.109.114.1 NAME 'caseExactIA5Match' SYNTAX 1.3.6.1.4.1.1466.115.121.1.26 )`,
	`( 1.3.6.1.4.1.1466.109.114.2 NAME 'caseIgnoreIA5Match' SYNTAX 1.3.6.1.4.1.1466.115.121.1.26 )`,
	`( 1.3.6.1.4.1.1466.109.114.3 NAME 'caseIgnoreIA5SubstringsMatch' SYNTAX 1.3.6.1.4.1.1466.115.121.1.58 )`,
	`( 1.3.6.1.1.16.2 NAME 'UUIDMatch' SYNTAX 1.3.6.1.1.16.1 )`,
	`( 1.3.6.1.1.16.3 NAME 'UUIDOrderingMatch' SYNTAX 1.3.6.1.1.16.1 )`,
}

// defaultSyntaxes contains the standard LDAP syntax definitions.
var defaultSyntaxes = []string{
	`( 1.3.6.1.4.1.1466.115.121.1.3 DESC 'Attribute Type Description' )`,
	`( 1.3.6.1.4.1.1466.115.121.1.6 DESC 'Bit String' )`,
	`( 1.3.6.1.4.1.1466.115.121.1.7 DESC 'Boolean' )`,
	`( 1.3.6.1.4.1.1466.115.121.1.11 DESC 'Country String' )`,
	`( 1.3.6.1.4.1.1466.115.121.1.12 DESC 'DN' )`,
	`( 1.3.6.1.4.1.1466.115.121.1.14 DESC 'Delivery Method' )`,
	`( 1.3.6.1.4.1.1466.115.121.1.15 DESC 'Directory String' )`,
	`( 1.3.6.1.4.1.1466.115.121.1.22 DESC 'Facsimile Telephone Number' )`,
	`( 1.3.6.1.4.1.1466.115.121.1.24 DESC 'Generalized Time' )`,
	`( 1.3.6.1.4.1.1466.115.121.1.26 DESC 'IA5 String' )`,
	`( 1.3.6.1.4.1.1466.115.121.1.27 DESC 'INTEGER' )`,
	`( 1.3.6.1.4.1.1466.115.121.1.34 DESC 'Name And Optional UID' )`,
	`( 1.3.6.1.4.1.1466.115.121.1.36 DESC 'Numeric String' )`,
	`( 1.3.6.1.4.1.1466.115.121.1.37 DESC 'Object Class Description' )`,
	`( 1.3.6.1.4.1.1466.115.121.1.38 DESC 'OID' )`,
	`( 1.3.6.1.4.1.1466.115.121.1.39 DESC 'Other Mailbox' )`,
	`( 1.3.6.1.4.1.1466.115.121.1.40 DESC 'Octet String' )`,
	`( 1.3.6.1.4.1.1466.115.121.1.41 DESC 'Postal Address' )`,
	`( 1.3.6.1.4.1.1466.115.121.1.43 DESC 'Presentation Address' )`,
	`( 1.3.6.1.4.1.1466.115.121.1.44 DESC 'Printable String' )`,
	`( 1.3.6.1.4.1.1466.115.121.1.50 DESC 'Telephone Number' )`,
	`( 1.3.6.1.4.1.1466.115.121.1.51 DESC 'Teletex Terminal Identifier' )`,
	`( 1.3.6.1.4.1.1466.115.121.1.52 DESC 'Telex Number' )`,
	`( 1.3.6.1.4.1.1466.115.121.1.53 DESC 'UTC Time' )`,
	`( 1.3.6.1.4.1.1466.115.121.1.58 DESC 'Substring Assertion' )`,
	`( 1.3.6.1.1.16.1 DESC 'UUID' )`,
}

// loadDefaultAttributeTypes loads the default attribute types into the schema.
func loadDefaultAttributeTypes(s *Schema) error {
	for _, def := range defaultAttributeTypes {
		at, err := parseAttributeType(def)
		if err != nil {
			return err
		}
		s.AddAttributeType(at)
	}
	return nil
}

// loadDefaultObjectClasses loads the default object classes into the schema.
func loadDefaultObjectClasses(s *Schema) error {
	for _, def := range defaultObjectClasses {
		oc, err := parseObjectClass(def)
		if err != nil {
			return err
		}
		s.AddObjectClass(oc)
	}
	return nil
}

// loadDefaultMatchingRules loads the default matching rules into the schema.
func loadDefaultMatchingRules(s *Schema) error {
	for _, def := range defaultMatchingRules {
		mr, err := parseMatchingRule(def)
		if err != nil {
			return err
		}
		s.AddMatchingRule(mr)
	}
	return nil
}

// loadDefaultSyntaxes loads the default syntaxes into the schema.
func loadDefaultSyntaxes(s *Schema) error {
	for _, def := range defaultSyntaxes {
		syn, err := parseSyntaxDef(def)
		if err != nil {
			return err
		}
		s.AddSyntax(syn)
	}
	return nil
}
