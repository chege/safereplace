package diff

import (
	"strings"
	"testing"
)

func TestHasChanges(t *testing.T) {
	if HasChanges("a", "a") {
		t.Fatalf("expected no changes")
	}
	if !HasChanges("a", "b") {
		t.Fatalf("expected changes")
	}
}

func TestDiff_NoChanges(t *testing.T) {
	out, changed, err := Diff("foo", "foo", Options{})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if changed {
		t.Fatalf("expected unchanged")
	}
	if out != "" {
		t.Fatalf("expected empty diff, got %q", out)
	}
}

func TestDiff_SimpleChange(t *testing.T) {
	before := "foo\nbar"
	after := "foo\nbaz"
	out, changed, err := Diff(before, after, Options{})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !changed {
		t.Fatalf("expected changes")
	}
	if !strings.Contains(out, "-bar") || !strings.Contains(out, "+baz") {
		t.Fatalf("diff missing expected lines:\n%s", out)
	}
}

func TestDiff_MultipleChanges(t *testing.T) {
	before := "one\ntwo\nthree"
	after := "ONE\ntwo\nTHREE"
	out, changed, err := Diff(before, after, Options{})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !changed {
		t.Fatalf("expected changes")
	}
	if !strings.Contains(out, "-one") || !strings.Contains(out, "+ONE") {
		t.Fatalf("diff missing one->ONE: %s", out)
	}
	if !strings.Contains(out, "-three") || !strings.Contains(out, "+THREE") {
		t.Fatalf("diff missing three->THREE: %s", out)
	}
}

func TestDiff_Colorized(t *testing.T) {
	before := "a"
	after := "b"
	out, changed, err := Diff(before, after, Options{Color: true})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !changed {
		t.Fatalf("expected changes")
	}
	if !strings.Contains(out, "\x1b[31m-") || !strings.Contains(out, "\x1b[32m+") {
		t.Fatalf("expected ANSI colors, got: %q", out)
	}
}

func TestDiff_IncludesHeadersWhenChanged(t *testing.T) {
	out, changed, err := Diff("x", "y", Options{})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !changed {
		t.Fatalf("expected changes")
	}
	if !strings.Contains(out, "--- before") || !strings.Contains(out, "+++ after") {
		t.Fatalf("missing diff headers:\n%s", out)
	}
}

func TestDiff_NoColor_HasNoANSI(t *testing.T) {
	out, changed, err := Diff("a", "b", Options{Color: false})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !changed {
		t.Fatalf("expected changes")
	}
	if strings.Contains(out, "\x1b[") {
		t.Fatalf("unexpected ANSI escapes: %q", out)
	}
}

func TestDiff_EmptyLineInsertion_ShowsOnlyAddedLine(t *testing.T) {
	before := "a\n\nc"
	after := "a\nb\nc"
	out, changed, err := Diff(before, after, Options{})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !changed {
		t.Fatalf("expected changes")
	}
	if !strings.Contains(out, "+b") {
		t.Fatalf("expected +b line, got:\n%s", out)
	}
	// Current implementation does not render a '-' for empty removed line; document behavior.
	if strings.Contains(out, "-\n") {
		t.Fatalf("did not expect explicit deletion of empty line")
	}
}

func TestDiff_TrailingNewlineDifference_Ignored(t *testing.T) {
	// Document current behavior: trailing newline-only changes are ignored.
	out, changed, err := Diff("a", "a\n", Options{})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if changed {
		t.Fatalf("trailing newline difference should be ignored; got diff:\n%s", out)
	}
}

func TestDiff_ContextOption_CurrentlyIgnored(t *testing.T) {
	// Setting Context should not change output for the minimal implementation.
	before := "foo\nbar\nbaz"
	after := "foo\nBAR\nbaz"
	out1, changed1, err1 := Diff(before, after, Options{Context: 0})
	if err1 != nil || !changed1 {
		t.Fatalf("baseline err=%v changed=%v", err1, changed1)
	}
	out2, changed2, err2 := Diff(before, after, Options{Context: 5})
	if err2 != nil || !changed2 {
		t.Fatalf("context err=%v changed=%v", err2, changed2)
	}
	if out1 != out2 {
		t.Fatalf("Context should be ignored in MVP; outputs differ\nA:\n%s\nB:\n%s", out1, out2)
	}
}
