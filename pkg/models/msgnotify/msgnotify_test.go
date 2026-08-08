package msgnotify

import (
	"reflect"
	"testing"
)

func TestAssociationsList_Get(t *testing.T) {
	l := AssociationsList{"foo=bar", "baz"}

	if val, hit := l.Get(0); !hit || val != "foo=bar" {
		t.Errorf("Get(0) = (%q, %v), want (%q, true)", val, hit, "foo=bar")
	}
	if val, hit := l.Get(1); !hit || val != "baz" {
		t.Errorf("Get(1) = (%q, %v), want (%q, true)", val, hit, "baz")
	}
	for _, i := range []int{-1, 2} {
		if val, hit := l.Get(i); hit || val != "" {
			t.Errorf("Get(%d) = (%q, %v), want (%q, false)", i, val, hit, "")
		}
	}
}

func TestAssociationsList_GetAll(t *testing.T) {
	l := AssociationsList{"a", "b"}
	if got := l.GetAll(); !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Errorf("GetAll() = %v, want %v", got, []string{"a", "b"})
	}
}

func TestMakeLabelKey(t *testing.T) {
	if got := MakeLabelKey("foo", "bar"); got != "foo=bar" {
		t.Errorf("MakeLabelKey(%q, %q) = %q, want %q", "foo", "bar", got, "foo=bar")
	}
	if got := MakeLabelKey("empty", ""); got != "empty=" {
		t.Errorf("MakeLabelKey(%q, %q) = %q, want %q", "empty", "", got, "empty=")
	}
}

func TestAssociationsList_GetByLabelKey(t *testing.T) {
	l := AssociationsList{
		MakeLabelKey("foo", "bar"),
		MakeLabelKey("baz", "1"),
		MakeLabelKey("foo", "qux"),
		MakeLabelKey("foobar", "nope"),  // must not match key "foo": prefix is "foo="
		MakeLabelKey("foo", " spaced "), // values are not trimmed
		MakeLabelKey("foo", ""),         // empty value is a match
	}

	want := []string{"bar", "qux", " spaced ", ""}
	if got := l.GetByLabelKey("foo"); !reflect.DeepEqual(got, want) {
		t.Errorf("GetByLabelKey(%q) = %q, want %q", "foo", got, want)
	}

	if got := l.GetByLabelKey("baz"); !reflect.DeepEqual(got, []string{"1"}) {
		t.Errorf("GetByLabelKey(%q) = %q, want %q", "baz", got, []string{"1"})
	}

	if got := l.GetByLabelKey("missing"); got != nil {
		t.Errorf("GetByLabelKey(%q) = %q, want nil", "missing", got)
	}
}
