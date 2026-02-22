package backend

import (
	"errors"
	"sort"
	"strings"

	"github.com/KilimcininKorOglu/oba/internal/storage"
)

// UIDRepairAction describes one repair operation.
type UIDRepairAction struct {
	DN      string `json:"dn"`
	UID     string `json:"uid"`
	Action  string `json:"action"`
	Reason  string `json:"reason"`
	Applied bool   `json:"applied"`
	Error   string `json:"error,omitempty"`
}

// UIDRepairReport summarizes a UID uniqueness repair run.
type UIDRepairReport struct {
	DryRun         bool               `json:"dryRun"`
	ScannedEntries int                `json:"scannedEntries"`
	UIDEntries     int                `json:"uidEntries"`
	DuplicateUIDs  int                `json:"duplicateUIDs"`
	Fixed          int                `json:"fixed"`
	Failed         int                `json:"failed"`
	Actions        []*UIDRepairAction `json:"actions"`
}

type uidRecord struct {
	dn         string
	uid        string
	rdnAttr    string
	rdnValue   string
	isUsersOU  bool
	isGroupsOU bool
	isUserObj  bool
	isGroupObj bool
}

// RepairUIDUniqueness repairs inconsistent uid data and duplicate uid values.
// Rules:
// 1. Entries outside ou=users do not keep uid (attribute removed).
// 2. For users where RDN is uid=... but uid attribute mismatches, uid is replaced with RDN value.
// 3. Remaining duplicates are resolved deterministically by keeping one canonical entry and deleting others.
func (b *ObaBackend) RepairUIDUniqueness(dryRun bool, bindDN string) (*UIDRepairReport, error) {
	entries, err := b.Search("", int(storage.ScopeSubtree), nil)
	if err != nil {
		return nil, err
	}

	report := &UIDRepairReport{
		DryRun:         dryRun,
		ScannedEntries: len(entries),
		Actions:        make([]*UIDRepairAction, 0),
	}

	records := make([]*uidRecord, 0)
	for _, e := range entries {
		if e == nil {
			continue
		}
		uid := strings.TrimSpace(e.GetFirstAttribute("uid"))
		if uid == "" {
			continue
		}
		report.UIDEntries++

		rdnAttr, rdnValue := parseDNFirstRDN(e.DN)
		records = append(records, &uidRecord{
			dn:         normalizeDN(e.DN),
			uid:        uid,
			rdnAttr:    rdnAttr,
			rdnValue:   rdnValue,
			isUsersOU:  isUnderOU(e.DN, "users"),
			isGroupsOU: isUnderOU(e.DN, "groups"),
			isUserObj:  hasAnyObjectClass(e, "inetorgperson", "person", "organizationalperson", "posixaccount"),
			isGroupObj: hasAnyObjectClass(e, "groupofnames", "groupofuniquenames", "posixgroup"),
		})
	}

	// Phase 1: uid attribute is only valid for entries under ou=users.
	kept := make([]*uidRecord, 0, len(records))
	for _, rec := range records {
		if rec.isUsersOU {
			kept = append(kept, rec)
			continue
		}
		reason := "uid attribute outside ou=users"
		action := &UIDRepairAction{
			DN:     rec.dn,
			UID:    rec.uid,
			Action: "remove_uid",
			Reason: reason,
		}
		if dryRun {
			action.Applied = false
			report.Actions = append(report.Actions, action)
			continue
		}
		err := b.ModifyWithBindDN(rec.dn, []Modification{
			{Type: ModDelete, Attribute: "uid"},
		}, bindDN)
		if err != nil {
			action.Error = err.Error()
			report.Failed++
		} else {
			action.Applied = true
			report.Fixed++
		}
		report.Actions = append(report.Actions, action)
	}

	// Build current uid usage map from kept records.
	uidToRecords := make(map[string][]*uidRecord)
	for _, rec := range kept {
		uidToRecords[rec.uid] = append(uidToRecords[rec.uid], rec)
	}

	// Phase 2: normalize users where RDN is uid=... but attribute mismatches.
	for _, rec := range kept {
		if rec.rdnAttr != "uid" || rec.rdnValue == "" || rec.rdnValue == rec.uid {
			continue
		}
		targetUID := rec.rdnValue
		conflict := false
		for _, other := range uidToRecords[targetUID] {
			if other.dn != rec.dn {
				conflict = true
				break
			}
		}

		if conflict {
			action := &UIDRepairAction{
				DN:     rec.dn,
				UID:    rec.uid,
				Action: "delete_entry",
				Reason: "uid attribute conflicts with RDN and target uid already exists",
			}
			if dryRun {
				report.Actions = append(report.Actions, action)
				continue
			}
			err := b.Delete(rec.dn)
			if err != nil && !errors.Is(err, ErrEntryNotFound) {
				action.Error = err.Error()
				report.Failed++
			} else {
				action.Applied = true
				report.Fixed++
			}
			report.Actions = append(report.Actions, action)
			continue
		}

		action := &UIDRepairAction{
			DN:     rec.dn,
			UID:    rec.uid,
			Action: "replace_uid",
			Reason: "uid attribute mismatched with RDN; replaced with uid RDN value",
		}
		if dryRun {
			report.Actions = append(report.Actions, action)
			continue
		}
		err := b.ModifyWithBindDN(rec.dn, []Modification{
			{Type: ModReplace, Attribute: "uid", Values: []string{targetUID}},
		}, bindDN)
		if err != nil {
			action.Error = err.Error()
			report.Failed++
		} else {
			action.Applied = true
			report.Fixed++
			// Update record view for duplicate phase.
			if prev := uidToRecords[rec.uid]; len(prev) > 0 {
				filtered := make([]*uidRecord, 0, len(prev))
				for _, p := range prev {
					if p.dn != rec.dn {
						filtered = append(filtered, p)
					}
				}
				if len(filtered) == 0 {
					delete(uidToRecords, rec.uid)
				} else {
					uidToRecords[rec.uid] = filtered
				}
			}
			rec.uid = targetUID
			uidToRecords[targetUID] = append(uidToRecords[targetUID], rec)
		}
		report.Actions = append(report.Actions, action)
	}

	// Phase 3: resolve remaining duplicates by canonical keep + delete.
	for uid, group := range uidToRecords {
		if len(group) <= 1 {
			continue
		}
		report.DuplicateUIDs++

		keep := chooseCanonicalUIDRecord(group)
		for _, rec := range group {
			if rec.dn == keep.dn {
				continue
			}
			action := &UIDRepairAction{
				DN:     rec.dn,
				UID:    uid,
				Action: "delete_entry",
				Reason: "duplicate uid; deterministic canonical entry kept: " + keep.dn,
			}
			if dryRun {
				report.Actions = append(report.Actions, action)
				continue
			}
			err := b.Delete(rec.dn)
			if err != nil && !errors.Is(err, ErrEntryNotFound) {
				action.Error = err.Error()
				report.Failed++
			} else {
				action.Applied = true
				report.Fixed++
			}
			report.Actions = append(report.Actions, action)
		}
	}

	return report, nil
}

func parseDNFirstRDN(dn string) (string, string) {
	parts := strings.SplitN(strings.TrimSpace(dn), ",", 2)
	if len(parts) == 0 {
		return "", ""
	}
	kv := strings.SplitN(strings.TrimSpace(parts[0]), "=", 2)
	if len(kv) != 2 {
		return "", ""
	}
	return strings.ToLower(strings.TrimSpace(kv[0])), strings.ToLower(strings.TrimSpace(kv[1]))
}

func isUnderOU(dn, ou string) bool {
	parts := strings.Split(strings.ToLower(dn), ",")
	target := "ou=" + strings.ToLower(strings.TrimSpace(ou))
	for _, p := range parts {
		if strings.TrimSpace(p) == target {
			return true
		}
	}
	return false
}

func hasAnyObjectClass(entry *Entry, names ...string) bool {
	if entry == nil {
		return false
	}
	values := entry.GetAttribute("objectClass")
	if len(values) == 0 {
		return false
	}
	set := make(map[string]struct{}, len(values))
	for _, v := range values {
		set[strings.ToLower(strings.TrimSpace(v))] = struct{}{}
	}
	for _, n := range names {
		if _, ok := set[strings.ToLower(strings.TrimSpace(n))]; ok {
			return true
		}
	}
	return false
}

func chooseCanonicalUIDRecord(records []*uidRecord) *uidRecord {
	if len(records) == 0 {
		return nil
	}
	sorted := make([]*uidRecord, len(records))
	copy(sorted, records)
	sort.Slice(sorted, func(i, j int) bool {
		si := uidRecordScore(sorted[i])
		sj := uidRecordScore(sorted[j])
		if si != sj {
			return si > sj
		}
		return sorted[i].dn < sorted[j].dn
	})
	return sorted[0]
}

func uidRecordScore(rec *uidRecord) int {
	if rec == nil {
		return -1000
	}
	score := 0
	if rec.isUsersOU {
		score += 100
	} else {
		score -= 25
	}
	if rec.isGroupsOU {
		score -= 50
	}
	if rec.isUserObj {
		score += 30
	}
	if rec.isGroupObj {
		score -= 40
	}
	if rec.rdnAttr == "uid" && rec.rdnValue == strings.ToLower(rec.uid) {
		score += 40
	}
	return score
}
