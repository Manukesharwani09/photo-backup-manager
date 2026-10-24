package planner

import "testing"

func ptr(i int64) *int64 { return &i }

func TestNormalizeFolder(t *testing.T) {
	cases := map[string]string{
		"DCIM/Camera/":    "DCIM/Camera",
		"/DCIM//Camera":   "DCIM/Camera",
		"Pictures\\Bills": "Pictures/Bills",
		"":                "",
		"/":               "",
		"Android/media/com.whatsapp/WhatsApp/Media/WhatsApp Images/": "Android/media/com.whatsapp/WhatsApp/Media/WhatsApp Images",
	}
	for in, want := range cases {
		if got := NormalizeFolder(in); got != want {
			t.Errorf("NormalizeFolder(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestAlbumTitle(t *testing.T) {
	cases := map[string]string{
		"DCIM/Camera/":             "DCIM › Camera",
		"Pictures/Documents/Bills": "Pictures › Documents › Bills",
		"Screenshots":              "Screenshots",
		"":                         "Phone",
	}
	for in, want := range cases {
		if got := AlbumTitle(in); got != want {
			t.Errorf("AlbumTitle(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMatches(t *testing.T) {
	cases := []struct {
		glob, folder string
		want         bool
	}{
		{"Documents", "Documents", true},
		{"Documents", "Documents/Bills", true},     // plain glob covers sub-folders
		{"Documents", "Documents2", false},         // not a prefix match on names
		{"Documents", "Pictures/Documents", false}, // anchored at the root
		{"documents", "Documents/Bills", true},     // case-insensitive
		{"Pictures/*", "Pictures/Bills", true},
		{"Pictures/*", "Pictures/Bills/2024", false}, // * is one level
		{"Pictures/*", "Pictures", false},
		{"WhatsApp/**", "WhatsApp", true},
		{"WhatsApp/**", "WhatsApp/Media/Images", true},
		{"**", "anything/at/all", true},
		{"DCIM/Camera/", "DCIM/Camera", true}, // trailing slash in the rule
		{"", "DCIM", false},
		{"*/Screenshots", "Pictures/Screenshots", true},
	}
	for _, c := range cases {
		if got := Matches(c.glob, c.folder); got != c.want {
			t.Errorf("Matches(%q, %q) = %v, want %v", c.glob, c.folder, got, c.want)
		}
	}
}

func TestRuleForPrecedence(t *testing.T) {
	rules := []Rule{
		{ID: 1, PathGlob: "Pictures", Backup: "include"},
		{ID: 2, PathGlob: "Pictures/Documents", Backup: "include", Cleanup: "protected"},
		{ID: 3, PathGlob: "Pictures/Private", Backup: "exclude"},
		{ID: 4, PathGlob: "Pictures/**", Backup: "exclude", Priority: 10},
	}
	// Higher priority beats specificity.
	if r := RuleFor(rules, "Pictures/Documents"); r == nil || r.ID != 4 {
		t.Fatalf("got %+v, want rule 4", r)
	}
	// Equal priority: most specific wins.
	if r := RuleFor(rules[:3], "Pictures/Documents/Bills"); r == nil || r.ID != 2 {
		t.Fatalf("got %+v, want rule 2", r)
	}
	if r := RuleFor(rules[:3], "DCIM/Camera"); r != nil {
		t.Fatalf("got %+v, want no rule", r)
	}
	// Exact tie (same priority and length): lowest id wins, deterministically.
	tie := []Rule{{ID: 9, PathGlob: "A"}, {ID: 5, PathGlob: "A"}}
	if r := RuleFor(tie, "A"); r.ID != 5 {
		t.Fatalf("tie: got %d", r.ID)
	}
}

func TestCleanupBlocker(t *testing.T) {
	protected := &Rule{Cleanup: "protected"}
	allowed := &Rule{Cleanup: "allowed"}
	cases := []struct {
		rule   *Rule
		on     bool
		status string
		want   string
	}{
		{nil, true, "verified", ""},
		{allowed, true, "verified", ""},
		{protected, true, "verified", KeepProtected}, // Documents: never cleaned
		{nil, true, "", KeepNotBackedUp},             // start-point / excluded / pending
		{nil, true, "uploaded", KeepUnverified},      // not yet confirmed by Google
		{nil, false, "verified", KeepNotOnDevice},
		{protected, false, "", KeepNotOnDevice},
	}
	for _, c := range cases {
		if got := CleanupBlocker(c.rule, c.on, c.status); got != c.want {
			t.Errorf("CleanupBlocker(%+v, %v, %q) = %q, want %q", c.rule, c.on, c.status, got, c.want)
		}
	}
}

func TestChooseAccount(t *testing.T) {
	primary := Account{ID: 1, Role: "primary", Active: true, IsCurrent: true}
	b := Account{ID: 2, Role: "overflow", Active: true}
	c := Account{ID: 3, Role: "overflow", Active: true}

	t.Run("current account used", func(t *testing.T) {
		id, sw, why := ChooseAccount([]Account{primary, b}, nil)
		if id != 1 || sw || why != "" {
			t.Fatalf("got %d %v %q", id, sw, why)
		}
	})
	t.Run("current full -> first overflow, switch", func(t *testing.T) {
		p := primary
		p.Full = true
		id, sw, _ := ChooseAccount([]Account{p, b, c}, nil)
		if id != 2 || !sw {
			t.Fatalf("got %d %v", id, sw)
		}
	})
	t.Run("skip full and disabled overflow", func(t *testing.T) {
		p, b2 := primary, b
		p.Full, b2.Full = true, true
		cc := c
		id, _, _ := ChooseAccount([]Account{p, b2, cc}, nil)
		if id != 3 {
			t.Fatalf("got %d", id)
		}
		cc.Active = false
		if _, _, why := ChooseAccount([]Account{p, b2, cc}, nil); why != ReasonNoAccount {
			t.Fatalf("got %q", why)
		}
	})
	t.Run("current overflow full -> back to primary if it has space", func(t *testing.T) {
		p := primary
		p.IsCurrent = false
		bb := b
		bb.IsCurrent, bb.Full = true, true
		id, sw, _ := ChooseAccount([]Account{p, bb}, nil)
		if id != 1 || !sw {
			t.Fatalf("got %d %v", id, sw)
		}
	})
	t.Run("no current period yet -> first usable overflow", func(t *testing.T) {
		p := primary
		p.IsCurrent = false
		id, sw, _ := ChooseAccount([]Account{p, b}, nil)
		if id != 2 || !sw {
			t.Fatalf("got %d %v", id, sw)
		}
	})
	t.Run("no accounts", func(t *testing.T) {
		if _, _, why := ChooseAccount(nil, nil); why != ReasonNoAccount {
			t.Fatalf("got %q", why)
		}
	})
	t.Run("fixed target never falls back", func(t *testing.T) {
		rule := &Rule{TargetAccountID: ptr(1)}
		if id, _, _ := ChooseAccount([]Account{primary, b}, rule); id != 1 {
			t.Fatalf("got %d", id)
		}
		p := primary
		p.Full = true
		if id, _, why := ChooseAccount([]Account{p, b}, rule); id != 0 || why != ReasonTargetFull {
			t.Fatalf("got %d %q", id, why)
		}
		p.Full, p.Active = false, false
		if _, _, why := ChooseAccount([]Account{p, b}, rule); why != ReasonTargetDisabled {
			t.Fatalf("got %q", why)
		}
		if _, _, why := ChooseAccount([]Account{b}, rule); why != ReasonTargetMissing {
			t.Fatalf("got %q", why)
		}
	})
	t.Run("fixed target to a non-current account does not switch", func(t *testing.T) {
		rule := &Rule{TargetAccountID: ptr(2)}
		id, sw, _ := ChooseAccount([]Account{primary, b}, rule)
		if id != 2 || sw {
			t.Fatalf("got %d %v", id, sw)
		}
	})
}
