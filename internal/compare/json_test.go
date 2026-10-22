package compare

import (
	"github.com/yusufalper/reqdiff/internal/jsondoc"
	"testing"
)

func document(t *testing.T, s string) any {
	t.Helper()
	v, err := jsondoc.Decode([]byte(s), 128)
	if err != nil {
		t.Fatal(err)
	}
	return v
}
func TestJSONStructure(t *testing.T) {
	l := document(t, `{"user":{"username":"a","id":1}}`)
	r := document(t, `{"user":{"avatar":"x","id":"1"}}`)
	diff := JSON(l, r, "structure", nil, nil)
	if len(diff) != 3 || diff[0].Path != "/user/avatar" || diff[0].Kind != "added" || diff[1].Path != "/user/id" || diff[1].Kind != "type_changed" || diff[2].Kind != "removed" {
		t.Fatalf("%+v", diff)
	}
	if d := JSON(document(t, `{"a":1,"b":true}`), document(t, `{"b":false,"a":2}`), "structure", nil, nil); len(d) != 0 {
		t.Fatal(d)
	}
}
func TestJSONValues(t *testing.T) {
	for _, tc := range []struct {
		l, r string
		n    int
		kind string
	}{
		{`1`, `1.0`, 0, ""}, {`1e0`, `1`, 0, ""},
		{`9007199254740992`, `9007199254740993`, 1, "value_changed"},
		{`{"x":null}`, `{}`, 1, "removed"}, {`null`, `0`, 1, "type_changed"},
		{`{}`, `{"a":{"b":1}}`, 1, "added"},
	} {
		d := JSON(document(t, tc.l), document(t, tc.r), "values", nil, nil)
		if len(d) != tc.n || tc.n > 0 && d[0].Kind != tc.kind {
			t.Errorf("%s vs %s: %+v", tc.l, tc.r, d)
		}
	}
}
func TestArraySemantics(t *testing.T) {
	for _, tc := range []struct {
		l, r, mode string
		n          int
	}{
		{`[1,2]`, `[2,1]`, "structure", 0}, {`[1,2]`, `[2,1]`, "values", 2},
		{`[]`, `[1]`, "structure", 1}, {`[[1]]`, `[["x"]]`, "structure", 1},
		{`[{"a":1},{"b":1}]`, `[{"b":1},{"a":1}]`, "structure", 4},
	} {
		d := JSON(document(t, tc.l), document(t, tc.r), tc.mode, nil, nil)
		if len(d) != tc.n {
			t.Errorf("%s: %+v", tc.mode, d)
		}
	}
}
func TestIgnoreAndIgnoreValues(t *testing.T) {
	paths, err := CompilePaths([]string{"/users/*/id"})
	if err != nil {
		t.Fatal(err)
	}
	l := document(t, `{"users":[{"id":1}]}`)
	if d := JSON(l, document(t, `{"users":[{"id":2}]}`), "values", nil, paths); len(d) != 0 {
		t.Fatal(d)
	}
	if d := JSON(l, document(t, `{"users":[{"id":"2"}]}`), "values", nil, paths); len(d) != 1 {
		t.Fatal(d)
	}
	if d := JSON(l, document(t, `{"users":[{}]}`), "values", nil, paths); len(d) != 1 {
		t.Fatal(d)
	}
	if d := JSON(l, document(t, `{"users":[{}]}`), "values", paths, nil); len(d) != 0 {
		t.Fatal(d)
	}
	root, _ := CompilePaths([]string{""})
	if d := JSON(l, document(t, `null`), "values", root, nil); len(d) != 0 {
		t.Fatal(d)
	}
	container, _ := CompilePaths([]string{"/users"})
	if d := JSON(l, document(t, `{"users":[{"id":2,"new":true}]}`), "values", nil, container); len(d) != 1 || d[0].Kind != "added" {
		t.Fatal(d)
	}
}
func TestPointerPatterns(t *testing.T) {
	p, err := CompilePaths([]string{"/a~1b/~0x/*"})
	if err != nil || !p.Match([]string{"a/b", "~x", "3"}) || !p.Covers([]string{"a/b", "~x", "3", "child"}) {
		t.Fatalf("%v", err)
	}
	for _, s := range []string{"a", "/~2", "/~"} {
		if _, err := CompilePaths([]string{s}); err == nil {
			t.Errorf("accepted %s", s)
		}
	}
	p, _ = CompilePaths([]string{"/0"})
	d := JSON(document(t, `[0,1]`), document(t, `[9,2]`), "values", p, nil)
	if len(d) != 1 || d[0].Path != "/1" {
		t.Fatal(d)
	}
}
func TestDiffOrder(t *testing.T) {
	l, r := document(t, `{"z":1,"a":1}`), document(t, `{"z":2,"a":2}`)
	for i := 0; i < 50; i++ {
		d := JSON(l, r, "values", nil, nil)
		if len(d) != 2 || d[0].Path != "/a" || d[1].Path != "/z" {
			t.Fatal(d)
		}
	}
}
