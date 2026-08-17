package provider

import (
	"testing"
)

func TestSplitString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		sep  string
		want []string
	}{
		{"empty string", "", "/", []string{""}},
		{"no separator", "abc", "/", []string{"abc"}},
		{"single separator", "a/b", "/", []string{"a", "b"}},
		{"three parts", "a/b/c", "/", []string{"a", "b", "c"}},
		{"trailing separator yields empty tail", "a/", "/", []string{"a", ""}},
		{"leading separator yields empty head", "/a", "/", []string{"", "a"}},
		{"consecutive separators yield empties", "a//b", "/", []string{"a", "", "b"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := splitString(tt.in, tt.sep)
			if len(got) != len(tt.want) {
				t.Fatalf("splitString(%q, %q) = %v (len %d), want %v (len %d)", tt.in, tt.sep, got, len(got), tt.want, len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("splitString(%q, %q)[%d] = %q, want %q", tt.in, tt.sep, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestSplitCompositeID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		id        string
		n         int
		wantParts []string
		wantErr   bool
	}{
		{"two parts ok", "proj/db", 2, []string{"proj", "db"}, false},
		{"three parts ok", "proj/db/hash", 3, []string{"proj", "db", "hash"}, false},
		{"too few parts", "proj", 2, nil, true},
		{"too many parts", "proj/db/extra", 2, nil, true},
		{"empty id expecting one part ok", "", 1, []string{""}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			parts, diags := splitCompositeID(tt.id, tt.n)
			if tt.wantErr {
				if !diags.HasError() {
					t.Fatalf("splitCompositeID(%q, %d) expected error diagnostics, got none", tt.id, tt.n)
				}
				return
			}
			if diags.HasError() {
				t.Fatalf("splitCompositeID(%q, %d) unexpected error: %v", tt.id, tt.n, diags)
			}
			if len(parts) != len(tt.wantParts) {
				t.Fatalf("splitCompositeID(%q, %d) = %v, want %v", tt.id, tt.n, parts, tt.wantParts)
			}
			for i := range parts {
				if parts[i] != tt.wantParts[i] {
					t.Errorf("part[%d] = %q, want %q", i, parts[i], tt.wantParts[i])
				}
			}
		})
	}
}

func TestFormatInt(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   int
		want string
	}{
		{2, "2"},
		{3, "3"},
		{4, "N"},
		{0, "N"},
	}
	for _, tt := range tests {
		if got := formatInt(tt.in); got != tt.want {
			t.Errorf("formatInt(%d) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestObjectAsOptions(t *testing.T) {
	t.Parallel()

	opts := objectAsOptions()
	if !opts.UnhandledNullAsEmpty {
		t.Error("expected UnhandledNullAsEmpty to be true")
	}
	if !opts.UnhandledUnknownAsEmpty {
		t.Error("expected UnhandledUnknownAsEmpty to be true")
	}
}
