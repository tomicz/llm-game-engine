package env

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMissingFile(t *testing.T) {
	if err := Load(filepath.Join(t.TempDir(), "does-not-exist.env")); err != nil {
		t.Fatalf("Load(missing) = %v, want nil", err)
	}
}

func TestLoad(t *testing.T) {
	content := `# comment line

GE_TEST_ENV_A=bar
   GE_TEST_ENV_B = baz
GE_TEST_ENV_C="quoted value"
GE_TEST_ENV_D='single'
GE_TEST_ENV_E=a=b=c
GE_TEST_ENV_F=
GE_TEST_ENV_G="unterminated
=novalue
NOEQUALS_LINE
  # indented comment
`
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	keys := []string{"GE_TEST_ENV_A", "GE_TEST_ENV_B", "GE_TEST_ENV_C", "GE_TEST_ENV_D", "GE_TEST_ENV_E", "GE_TEST_ENV_F", "GE_TEST_ENV_G", "NOEQUALS_LINE"}
	for _, k := range keys {
		t.Setenv(k, "") // registers cleanup that restores the original value
		os.Unsetenv(k)
	}

	if err := Load(path); err != nil {
		t.Fatalf("Load = %v", err)
	}

	want := map[string]string{
		"GE_TEST_ENV_A": "bar",
		"GE_TEST_ENV_B": "baz",
		"GE_TEST_ENV_C": "quoted value",
		"GE_TEST_ENV_D": "single",
		"GE_TEST_ENV_E": "a=b=c",
		"GE_TEST_ENV_F": "",
		"GE_TEST_ENV_G": `"unterminated`,
	}
	for k, v := range want {
		got, ok := os.LookupEnv(k)
		if !ok {
			t.Errorf("%s not set", k)
			continue
		}
		if got != v {
			t.Errorf("%s = %q, want %q", k, got, v)
		}
	}
	if _, ok := os.LookupEnv("NOEQUALS_LINE"); ok {
		t.Error("line without '=' should not set a variable")
	}
}
