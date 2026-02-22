package backend

import "testing"

func TestChooseCanonicalUIDRecord(t *testing.T) {
	records := []*uidRecord{
		{
			dn:         "uid=dup1,ou=users,dc=example,dc=com",
			uid:        "carol",
			rdnAttr:    "uid",
			rdnValue:   "dup1",
			isUsersOU:  true,
			isUserObj:  true,
			isGroupsOU: false,
		},
		{
			dn:         "uid=carol,ou=users,dc=example,dc=com",
			uid:        "carol",
			rdnAttr:    "uid",
			rdnValue:   "carol",
			isUsersOU:  true,
			isUserObj:  true,
			isGroupsOU: false,
		},
		{
			dn:         "cn=engineering,ou=groups,dc=example,dc=com",
			uid:        "carol",
			rdnAttr:    "cn",
			rdnValue:   "engineering",
			isUsersOU:  false,
			isUserObj:  false,
			isGroupsOU: true,
			isGroupObj: true,
		},
	}

	keep := chooseCanonicalUIDRecord(records)
	if keep == nil {
		t.Fatal("expected canonical record")
	}
	if keep.dn != "uid=carol,ou=users,dc=example,dc=com" {
		t.Fatalf("unexpected canonical dn: %s", keep.dn)
	}
}

func TestParseDNFirstRDN(t *testing.T) {
	attr, val := parseDNFirstRDN("UID=Alice,OU=Users,DC=example,DC=com")
	if attr != "uid" || val != "alice" {
		t.Fatalf("unexpected parse result: attr=%s val=%s", attr, val)
	}
}
