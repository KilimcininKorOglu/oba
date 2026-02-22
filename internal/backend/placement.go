package backend

import (
	"fmt"
	"strings"
)

var userObjectClasses = map[string]struct{}{
	"person":               {},
	"organizationalperson": {},
	"inetorgperson":        {},
	"user":                 {},
	"posixaccount":         {},
}

var groupObjectClasses = map[string]struct{}{
	"group":              {},
	"groupofnames":       {},
	"groupofuniquenames": {},
	"posixgroup":         {},
}

func validateEntryPlacement(entry *Entry) error {
	if entry == nil {
		return ErrInvalidEntry
	}

	dn := normalizeDN(entry.DN)
	if dn == "" {
		return ErrInvalidDN
	}

	objectClasses := getObjectClasses(entry)
	if len(objectClasses) == 0 {
		return nil
	}

	var hasUserClass bool
	var hasGroupClass bool
	for _, oc := range objectClasses {
		normalizedOC := strings.ToLower(strings.TrimSpace(oc))
		if _, ok := userObjectClasses[normalizedOC]; ok {
			hasUserClass = true
		}
		if _, ok := groupObjectClasses[normalizedOC]; ok {
			hasGroupClass = true
		}
	}

	if hasUserClass && !dnUnderOU(dn, "users") {
		return fmt.Errorf("%w: user entries must be under ou=users", ErrInvalidPlacement)
	}
	if hasGroupClass && !dnUnderOU(dn, "groups") {
		return fmt.Errorf("%w: group entries must be under ou=groups", ErrInvalidPlacement)
	}

	return nil
}

func getObjectClasses(entry *Entry) []string {
	if entry == nil || entry.Attributes == nil {
		return nil
	}
	for name, values := range entry.Attributes {
		if strings.EqualFold(name, "objectclass") {
			return values
		}
	}
	return nil
}

func dnUnderOU(dn, ouName string) bool {
	needle := "ou=" + strings.ToLower(strings.TrimSpace(ouName))
	if dn == needle {
		return true
	}
	if strings.Contains(dn, ","+needle+",") {
		return true
	}
	return strings.HasSuffix(dn, ","+needle)
}
