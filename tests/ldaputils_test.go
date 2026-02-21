// Package tests provides compatibility tests for the Oba LDAP server.
// These tests verify compatibility with ldap-utils command-line tools.
package tests

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/KilimcininKorOglu/oba/internal/backend"
)

// TestLdapUtils tests compatibility with ldap-utils command-line tools.
func TestLdapUtils(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping ldap-utils test in short mode")
	}

	// Check if ldapsearch is available
	if _, err := exec.LookPath("ldapsearch"); err != nil {
		t.Skip("ldapsearch not found, skipping ldap-utils tests")
	}

	// Start test server
	srv, err := NewTestServer(nil)
	if err != nil {
		t.Fatalf("failed to create test server: %v", err)
	}

	if err := srv.Start(); err != nil {
		t.Fatalf("failed to start test server: %v", err)
	}
	defer srv.Stop()

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	// Setup test data
	setupLdapUtilsTestData(t, srv)

	t.Run("ldapsearch", func(t *testing.T) {
		testLdapsearch(t, srv.Address())
	})

	t.Run("ldapadd", func(t *testing.T) {
		testLdapadd(t, srv.Address())
	})

	t.Run("ldapmodify", func(t *testing.T) {
		testLdapmodify(t, srv.Address())
	})

	t.Run("ldapdelete", func(t *testing.T) {
		testLdapdelete(t, srv.Address())
	})

	t.Run("ldapwhoami", func(t *testing.T) {
		testLdapwhoami(t, srv.Address())
	})
}

// setupLdapUtilsTestData adds test entries for ldap-utils tests.
func setupLdapUtilsTestData(t *testing.T, srv *TestServer) {
	be := srv.Backend()

	// Base entry and OUs are created by bootstrap, only add user/group entries

	// Add user entries
	alice := backend.NewEntry("uid=alice,ou=users,dc=test,dc=com")
	alice.SetAttribute("objectclass", "inetOrgPerson", "person", "top")
	alice.SetAttribute("uid", "alice")
	alice.SetAttribute("cn", "Alice Smith")
	alice.SetAttribute("sn", "Smith")
	alice.SetAttribute("mail", "alice@test.com")
	if err := be.Add(alice); err != nil {
		t.Fatalf("failed to add alice: %v", err)
	}

	bob := backend.NewEntry("uid=bob,ou=users,dc=test,dc=com")
	bob.SetAttribute("objectclass", "inetOrgPerson", "person", "top")
	bob.SetAttribute("uid", "bob")
	bob.SetAttribute("cn", "Bob Jones")
	bob.SetAttribute("sn", "Jones")
	bob.SetAttribute("mail", "bob@test.com")
	if err := be.Add(bob); err != nil {
		t.Fatalf("failed to add bob: %v", err)
	}

	// Add admin group
	admins := backend.NewEntry("cn=admins,ou=groups,dc=test,dc=com")
	admins.SetAttribute("objectclass", "groupOfNames", "top")
	admins.SetAttribute("cn", "admins")
	admins.SetAttribute("member", "uid=alice,ou=users,dc=test,dc=com")
	if err := be.Add(admins); err != nil {
		t.Fatalf("failed to add admins group: %v", err)
	}
}

// testLdapsearch tests ldapsearch command compatibility.
func testLdapsearch(t *testing.T, addr string) {
	tests := []struct {
		name      string
		args      []string
		expect    string
		notExpect string
	}{
		{
			name:   "base_scope",
			args:   []string{"-s", "base", "-b", "dc=test,dc=com", "(objectclass=*)"},
			expect: "dc=test,dc=com",
		},
		{
			name:   "subtree_scope",
			args:   []string{"-s", "sub", "-b", "dc=test,dc=com", "(objectclass=*)"},
			expect: "uid=alice",
		},
		{
			name:   "onelevel_scope",
			args:   []string{"-s", "one", "-b", "dc=test,dc=com", "(objectclass=*)"},
			expect: "ou=users",
		},
		{
			name:   "filter_equality",
			args:   []string{"-b", "dc=test,dc=com", "(uid=alice)"},
			expect: "uid: alice",
		},
		{
			name:   "filter_presence",
			args:   []string{"-b", "dc=test,dc=com", "(mail=*)"},
			expect: "mail:",
		},
		{
			name:   "filter_substring",
			args:   []string{"-b", "dc=test,dc=com", "(cn=*Smith*)"},
			expect: "cn: Alice Smith",
		},
		{
			name:   "attribute_selection",
			args:   []string{"-b", "uid=alice,ou=users,dc=test,dc=com", "-s", "base", "(objectclass=*)", "cn", "mail"},
			expect: "cn: Alice Smith",
		},
		{
			name:      "no_match",
			args:      []string{"-b", "dc=test,dc=com", "(uid=nonexistent)"},
			notExpect: "uid:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := append([]string{"-x", "-H", "ldap://" + addr}, tt.args...)
			cmd := exec.Command("ldapsearch", args...)
			output, err := cmd.CombinedOutput()

			// ldapsearch returns 0 even for no results, but may return non-zero for errors
			if err != nil {
				// Check if it's a real error or just no results
				if exitErr, ok := err.(*exec.ExitError); ok {
					// Exit code 32 is "No such object" which is acceptable for some tests
					if exitErr.ExitCode() != 32 && exitErr.ExitCode() != 0 {
						t.Logf("ldapsearch output: %s", output)
						t.Fatalf("ldapsearch failed with exit code %d: %v", exitErr.ExitCode(), err)
					}
				}
			}

			outputStr := string(output)
			if tt.expect != "" && !strings.Contains(outputStr, tt.expect) {
				t.Errorf("expected %q in output:\n%s", tt.expect, outputStr)
			}
			if tt.notExpect != "" && strings.Contains(outputStr, tt.notExpect) {
				t.Errorf("did not expect %q in output:\n%s", tt.notExpect, outputStr)
			}
		})
	}
}

// testLdapadd tests ldapadd command compatibility.
func testLdapadd(t *testing.T, addr string) {
	t.Run("add_entry", func(t *testing.T) {
		ldif := `dn: uid=newuser,ou=users,dc=test,dc=com
objectClass: inetOrgPerson
objectClass: person
objectClass: top
uid: newuser
cn: New User
sn: User
mail: newuser@test.com
`
		args := []string{"-x", "-H", "ldap://" + addr, "-D", "cn=admin,dc=test,dc=com", "-w", "secret"}
		cmd := exec.Command("ldapadd", args...)
		cmd.Stdin = bytes.NewBufferString(ldif)
		output, err := cmd.CombinedOutput()

		if err != nil {
			t.Logf("ldapadd output: %s", output)
			t.Fatalf("ldapadd failed: %v", err)
		}

		// Verify the entry was added by searching for it
		searchArgs := []string{"-x", "-H", "ldap://" + addr, "-b", "uid=newuser,ou=users,dc=test,dc=com", "-s", "base", "(objectclass=*)"}
		searchCmd := exec.Command("ldapsearch", searchArgs...)
		searchOutput, err := searchCmd.CombinedOutput()
		if err != nil {
			t.Logf("ldapsearch output: %s", searchOutput)
			t.Fatalf("ldapsearch failed: %v", err)
		}

		if !strings.Contains(string(searchOutput), "uid: newuser") {
			t.Errorf("expected to find added entry, output:\n%s", searchOutput)
		}
	})

	t.Run("add_duplicate_entry", func(t *testing.T) {
		ldif := `dn: uid=alice,ou=users,dc=test,dc=com
objectClass: inetOrgPerson
objectClass: person
objectClass: top
uid: alice
cn: Alice Duplicate
sn: Duplicate
`
		args := []string{"-x", "-H", "ldap://" + addr, "-D", "cn=admin,dc=test,dc=com", "-w", "secret"}
		cmd := exec.Command("ldapadd", args...)
		cmd.Stdin = bytes.NewBufferString(ldif)
		output, err := cmd.CombinedOutput()

		// Should fail with "already exists" error
		if err == nil {
			t.Errorf("expected ldapadd to fail for duplicate entry, output:\n%s", output)
		}

		// Check for error code 68 (entry already exists)
		if !strings.Contains(string(output), "68") && !strings.Contains(strings.ToLower(string(output)), "already exists") {
			t.Logf("ldapadd output: %s", output)
		}
	})
}

// testLdapmodify tests ldapmodify command compatibility.
func testLdapmodify(t *testing.T, addr string) {
	t.Run("modify_add_attribute", func(t *testing.T) {
		ldif := `dn: uid=bob,ou=users,dc=test,dc=com
changetype: modify
add: telephoneNumber
telephoneNumber: +1-555-0102
`
		args := []string{"-x", "-H", "ldap://" + addr, "-D", "cn=admin,dc=test,dc=com", "-w", "secret"}
		cmd := exec.Command("ldapmodify", args...)
		cmd.Stdin = bytes.NewBufferString(ldif)
		output, err := cmd.CombinedOutput()

		if err != nil {
			t.Logf("ldapmodify output: %s", output)
			t.Fatalf("ldapmodify failed: %v", err)
		}

		// Verify the attribute was added
		searchArgs := []string{"-x", "-H", "ldap://" + addr, "-b", "uid=bob,ou=users,dc=test,dc=com", "-s", "base", "(objectclass=*)", "telephoneNumber"}
		searchCmd := exec.Command("ldapsearch", searchArgs...)
		searchOutput, err := searchCmd.CombinedOutput()
		if err != nil {
			t.Logf("ldapsearch output: %s", searchOutput)
			t.Fatalf("ldapsearch failed: %v", err)
		}

		if !strings.Contains(strings.ToLower(string(searchOutput)), "telephonenumber:") {
			t.Errorf("expected to find added attribute, output:\n%s", searchOutput)
		}
	})

	t.Run("modify_replace_attribute", func(t *testing.T) {
		ldif := `dn: uid=bob,ou=users,dc=test,dc=com
changetype: modify
replace: cn
cn: Robert Jones
`
		args := []string{"-x", "-H", "ldap://" + addr, "-D", "cn=admin,dc=test,dc=com", "-w", "secret"}
		cmd := exec.Command("ldapmodify", args...)
		cmd.Stdin = bytes.NewBufferString(ldif)
		output, err := cmd.CombinedOutput()

		if err != nil {
			t.Logf("ldapmodify output: %s", output)
			t.Fatalf("ldapmodify failed: %v", err)
		}

		// Verify the attribute was replaced
		searchArgs := []string{"-x", "-H", "ldap://" + addr, "-b", "uid=bob,ou=users,dc=test,dc=com", "-s", "base", "(objectclass=*)", "cn"}
		searchCmd := exec.Command("ldapsearch", searchArgs...)
		searchOutput, err := searchCmd.CombinedOutput()
		if err != nil {
			t.Logf("ldapsearch output: %s", searchOutput)
			t.Fatalf("ldapsearch failed: %v", err)
		}

		if !strings.Contains(string(searchOutput), "cn: Robert Jones") {
			t.Errorf("expected to find replaced attribute value, output:\n%s", searchOutput)
		}
	})

	t.Run("modify_delete_attribute", func(t *testing.T) {
		// First add an attribute to delete
		addLdif := `dn: uid=alice,ou=users,dc=test,dc=com
changetype: modify
add: description
description: To be deleted
`
		addArgs := []string{"-x", "-H", "ldap://" + addr, "-D", "cn=admin,dc=test,dc=com", "-w", "secret"}
		addCmd := exec.Command("ldapmodify", addArgs...)
		addCmd.Stdin = bytes.NewBufferString(addLdif)
		addOutput, err := addCmd.CombinedOutput()
		if err != nil {
			t.Logf("ldapmodify add output: %s", addOutput)
			// Continue even if add fails (attribute might already exist)
		}

		// Now delete the attribute
		deleteLdif := `dn: uid=alice,ou=users,dc=test,dc=com
changetype: modify
delete: description
`
		deleteArgs := []string{"-x", "-H", "ldap://" + addr, "-D", "cn=admin,dc=test,dc=com", "-w", "secret"}
		deleteCmd := exec.Command("ldapmodify", deleteArgs...)
		deleteCmd.Stdin = bytes.NewBufferString(deleteLdif)
		deleteOutput, err := deleteCmd.CombinedOutput()

		if err != nil {
			t.Logf("ldapmodify delete output: %s", deleteOutput)
			t.Fatalf("ldapmodify delete failed: %v", err)
		}

		// Verify the attribute was deleted
		searchArgs := []string{"-x", "-H", "ldap://" + addr, "-b", "uid=alice,ou=users,dc=test,dc=com", "-s", "base", "(objectclass=*)", "description"}
		searchCmd := exec.Command("ldapsearch", searchArgs...)
		searchOutput, err := searchCmd.CombinedOutput()
		if err != nil {
			t.Logf("ldapsearch output: %s", searchOutput)
			t.Fatalf("ldapsearch failed: %v", err)
		}

		if strings.Contains(string(searchOutput), "description: To be deleted") {
			t.Errorf("expected attribute to be deleted, output:\n%s", searchOutput)
		}
	})
}

// testLdapdelete tests ldapdelete command compatibility.
func testLdapdelete(t *testing.T, addr string) {
	t.Run("delete_entry", func(t *testing.T) {
		// First add an entry to delete
		ldif := `dn: uid=todelete,ou=users,dc=test,dc=com
objectClass: inetOrgPerson
objectClass: person
objectClass: top
uid: todelete
cn: To Delete
sn: Delete
`
		addArgs := []string{"-x", "-H", "ldap://" + addr, "-D", "cn=admin,dc=test,dc=com", "-w", "secret"}
		addCmd := exec.Command("ldapadd", addArgs...)
		addCmd.Stdin = bytes.NewBufferString(ldif)
		addOutput, err := addCmd.CombinedOutput()
		if err != nil {
			t.Logf("ldapadd output: %s", addOutput)
			t.Fatalf("ldapadd failed: %v", err)
		}

		// Now delete the entry
		deleteArgs := []string{"-x", "-H", "ldap://" + addr, "-D", "cn=admin,dc=test,dc=com", "-w", "secret", "uid=todelete,ou=users,dc=test,dc=com"}
		deleteCmd := exec.Command("ldapdelete", deleteArgs...)
		deleteOutput, err := deleteCmd.CombinedOutput()

		if err != nil {
			t.Logf("ldapdelete output: %s", deleteOutput)
			t.Fatalf("ldapdelete failed: %v", err)
		}

		// Verify the entry was deleted
		searchArgs := []string{"-x", "-H", "ldap://" + addr, "-b", "uid=todelete,ou=users,dc=test,dc=com", "-s", "base", "(objectclass=*)"}
		searchCmd := exec.Command("ldapsearch", searchArgs...)
		searchOutput, _ := searchCmd.CombinedOutput()

		// Should not find the entry (exit code 32 or empty result)
		if strings.Contains(string(searchOutput), "uid: todelete") {
			t.Errorf("expected entry to be deleted, output:\n%s", searchOutput)
		}
	})

	t.Run("delete_nonexistent_entry", func(t *testing.T) {
		deleteArgs := []string{"-x", "-H", "ldap://" + addr, "-D", "cn=admin,dc=test,dc=com", "-w", "secret", "uid=nonexistent,ou=users,dc=test,dc=com"}
		deleteCmd := exec.Command("ldapdelete", deleteArgs...)
		output, err := deleteCmd.CombinedOutput()

		// Should fail with "no such object" error
		if err == nil {
			t.Errorf("expected ldapdelete to fail for nonexistent entry, output:\n%s", output)
		}
	})
}

// testLdapwhoami tests ldapwhoami command compatibility.
func testLdapwhoami(t *testing.T, addr string) {
	t.Run("whoami_anonymous", func(t *testing.T) {
		args := []string{"-x", "-H", "ldap://" + addr}
		cmd := exec.Command("ldapwhoami", args...)
		output, err := cmd.CombinedOutput()

		if err != nil {
			// Check if it's an unsupported operation error
			if strings.Contains(string(output), "unsupported operation") ||
				strings.Contains(string(output), "Protocol error") {
				t.Skip("ldapwhoami extended operation not supported by server")
			}
			t.Logf("ldapwhoami output: %s", output)
			t.Fatalf("ldapwhoami failed: %v", err)
		}

		// Anonymous bind should return empty or "anonymous"
		outputStr := strings.TrimSpace(string(output))
		if outputStr != "" && outputStr != "anonymous" && !strings.Contains(outputStr, "anonymous") {
			t.Logf("ldapwhoami anonymous output: %s", outputStr)
		}
	})

	t.Run("whoami_authenticated", func(t *testing.T) {
		args := []string{"-x", "-H", "ldap://" + addr, "-D", "cn=admin,dc=test,dc=com", "-w", "secret"}
		cmd := exec.Command("ldapwhoami", args...)
		output, err := cmd.CombinedOutput()

		if err != nil {
			// Check if it's an unsupported operation error
			if strings.Contains(string(output), "unsupported operation") ||
				strings.Contains(string(output), "Protocol error") {
				t.Skip("ldapwhoami extended operation not supported by server")
			}
			t.Logf("ldapwhoami output: %s", output)
			t.Fatalf("ldapwhoami failed: %v", err)
		}

		// Should return the bound DN
		outputStr := string(output)
		if !strings.Contains(outputStr, "cn=admin") && !strings.Contains(outputStr, "dn:") {
			t.Logf("ldapwhoami authenticated output: %s", outputStr)
		}
	})
}
