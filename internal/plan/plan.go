// Package plan holds the pure backup decisions: which folder rule applies, which account a file goes to,
// and what the folder's album is called. No I/O, so every edge case is unit-testable.
package plan

import (
	"path"
	"strings"
)

// Rule is a folder rule (see folder_rule table).
type Rule struct {
	ID              int64
	PathGlob        string // e.g. "Documents", "Pictures/Documents/*", "WhatsApp/**"
	TargetAccountID *int64 // nil = whatever account is active
	Backup          string // include | exclude
	Cleanup         string // allowed | protected
	Priority        int
}

// Account is the part of an account the planner needs.
type Account struct {
	ID        int64
	Role      string // primary | overflow
	Active    bool   // user has not disabled it
	Full      bool
	IsCurrent bool // has the open account_period (receives new backups)
}

// NormalizeFolder turns Android RELATIVE_PATH values like "DCIM/Camera/" or "/DCIM//Camera" into "DCIM/Camera".
func NormalizeFolder(p string) string {
	parts := strings.FieldsFunc(p, func(r rune) bool { return r == '/' || r == '\\' })
	return strings.Join(parts, "/")
}

// AlbumTitle is the Google Photos album name for a phone folder: "DCIM/Camera" -> "DCIM › Camera".
func AlbumTitle(folder string) string {
	f := NormalizeFolder(folder)
	if f == "" {
		return "Phone"
	}
	return strings.ReplaceAll(f, "/", " › ")
}

// Matches reports whether a rule glob applies to a folder. A glob matches the folder itself and,
// when it has no trailing "/*" pattern, every sub-folder: "Documents" covers "Documents/Bills".
// "X/**" explicitly means X and everything below it. Matching is case-insensitive (Android folders vary).
func Matches(glob, folder string) bool {
	g := strings.ToLower(NormalizeFolder(glob))
	f := strings.ToLower(NormalizeFolder(folder))
	if g == "" {
		return false
	}
	if strings.HasSuffix(g, "/**") || g == "**" {
		base := strings.TrimSuffix(strings.TrimSuffix(g, "**"), "/")
		return base == "" || f == base || strings.HasPrefix(f, base+"/")
	}
	if ok, _ := path.Match(g, f); ok {
		return true
	}
	if !strings.ContainsAny(g, "*?[") && strings.HasPrefix(f, g+"/") {
		return true
	}
	return false
}

// RuleFor returns the matching rule with the highest priority; on a tie the most specific
// (longest) glob wins, then the lowest id. nil means no rule: back up to the current account, cleanup allowed.
func RuleFor(rules []Rule, folder string) *Rule {
	var best *Rule
	for i := range rules {
		r := &rules[i]
		if !Matches(r.PathGlob, folder) {
			continue
		}
		if best == nil ||
			r.Priority > best.Priority ||
			(r.Priority == best.Priority && len(r.PathGlob) > len(best.PathGlob)) ||
			(r.Priority == best.Priority && len(r.PathGlob) == len(best.PathGlob) && r.ID < best.ID) {
			best = r
		}
	}
	return best
}

// Reasons a phone file may not be deleted by cleanup.
const (
	KeepProtected   = "protected"     // its folder rule says cleanup: protected
	KeepNotBackedUp = "not_backed_up" // skipped, excluded or still pending: deleting would lose it
	KeepUnverified  = "not_verified"  // uploaded but Google hasn't confirmed it yet
	KeepNotOnDevice = "not_on_device" // already removed
)

// CleanupBlocker returns "" if a phone file may be deleted, otherwise why not.
// backupStatus is the media's live backup status: "verified", "uploaded" or "" (none).
// Only verified backups qualify: a photo is deleted from the phone only once Google has confirmed it.
func CleanupBlocker(rule *Rule, onDevice bool, backupStatus string) string {
	switch {
	case !onDevice:
		return KeepNotOnDevice
	case rule != nil && rule.Cleanup == "protected":
		return KeepProtected
	case backupStatus == "":
		return KeepNotBackedUp
	case backupStatus != "verified":
		return KeepUnverified
	}
	return ""
}

// Reason values explain why a file is not planned for upload.
const (
	ReasonNoAccount      = "no_account_available" // nothing usable: all full, disabled or none connected
	ReasonTargetFull     = "target_account_full"  // the folder's fixed account is full
	ReasonTargetDisabled = "target_account_disabled"
	ReasonTargetMissing  = "target_account_missing"
)

// ChooseAccount picks the destination for a file in a folder governed by rule (may be nil).
// A rule with a fixed target never falls back to another account: Documents must stay in the primary,
// so if the primary is full those files wait and the reason is reported instead.
// Otherwise the current account is used if usable, else the first usable overflow account by id;
// switched is true when that means the current account must change (the caller records the period change).
func ChooseAccount(accts []Account, rule *Rule) (id int64, switched bool, reason string) {
	if rule != nil && rule.TargetAccountID != nil {
		for _, a := range accts {
			if a.ID != *rule.TargetAccountID {
				continue
			}
			switch {
			case !a.Active:
				return 0, false, ReasonTargetDisabled
			case a.Full:
				return 0, false, ReasonTargetFull
			}
			return a.ID, false, ""
		}
		return 0, false, ReasonTargetMissing
	}
	for _, a := range accts {
		if a.IsCurrent && a.Active && !a.Full {
			return a.ID, false, ""
		}
	}
	// Current account unusable: next is the first usable overflow, then the primary as a last resort.
	for _, role := range []string{"overflow", "primary"} {
		for _, a := range accts {
			if a.Role == role && a.Active && !a.Full && !a.IsCurrent {
				return a.ID, true, ""
			}
		}
	}
	return 0, false, ReasonNoAccount
}
