package modelpolicy

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, s string) string {
	p := filepath.Join(t.TempDir(), "p.json")
	if err := os.WriteFile(p, []byte(s), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoad(t *testing.T) {
	p, err := Load(write(t, `{"models":[{"id":"a","provider":"x","enabled":true},{"id":"b","provider":"y","external":true}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Enabled()) != 1 || !p.Allowed("a") || p.Allowed("b") || p.Allowed("zz") {
		t.Fatal("bad policy")
	}
}

func TestInvalid(t *testing.T) {
	var many []string
	for i := 0; i < 11; i++ {
		many = append(many, fmt.Sprintf(`{"id":"m%d"}`, i))
	}
	for _, s := range []string{
		`{"models":[{"id":""}]}`,
		`{"models":[{"id":"a"},{"id":"a"}]}`,
		`{"models":[` + strings.Join(many, ",") + `]}`,
		`nope`,
	} {
		if _, err := Load(write(t, s)); err == nil {
			t.Fatalf("expected error: %s", s)
		}
	}
	if _, err := Load("/nonexistent"); err == nil {
		t.Fatal("expected error")
	}
}
