package canonize

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"sigs.k8s.io/yaml"

	"github.com/pmezard/go-difflib/difflib"
)

const (
	envDoCanonize          = "CANONIZE"
	canonDirName           = "canondata"
	canonFileName          = "test.canondata"
	defaultFilePermissions = 0o644
	defaultDirPermissions  = 0o755
)

type T interface {
	Name() string
	Errorf(format string, args ...any)
	FailNow()
}

func Assert(t T, data []byte) {
	canonFilePath := getCanonFilePath(t, canonFileName)

	if isCanonizeNeeded() {
		err := writeCanonData(canonFilePath, data)
		if err != nil {
			t.Errorf("can't write canon data with error: %q", err.Error())
			t.FailNow()
			return
		}
	}

	canonData, err := readCanonData(canonFilePath)
	if err != nil {
		t.Errorf("can't read canon data with error: %q", err.Error())
		t.FailNow()
		return
	}
	canonDataTrimmed := strings.TrimSpace(string(canonData))

	diff := difflib.UnifiedDiff{
		A:        difflib.SplitLines(canonDataTrimmed),
		B:        difflib.SplitLines(string(data)),
		FromFile: "old",
		ToFile:   "new",
		Context:  3,
	}
	text, err := difflib.GetUnifiedDiffString(diff)
	if err != nil {
		t.Errorf("cannot diff: %v", err)
	}
	if text != "" {
		t.Errorf("%s", addColorsToDiff(text))
	}
}

func AssertStruct(t T, name string, s any) {
	canonFilePath := getCanonFilePath(t, name+".yaml")

	data, err := yaml.Marshal(s)
	if err != nil {
		t.Errorf("can't encode data with error: %q", err.Error())
		t.FailNow()
		return
	}

	if isCanonizeNeeded() {
		err := writeCanonData(canonFilePath, data)
		if err != nil {
			t.Errorf("can't write canon data with error: %q", err.Error())
			t.FailNow()
			return
		}
	}

	canonData, err := readCanonData(canonFilePath)
	if err != nil {
		t.Errorf("can't read canon data with error: %q", err.Error())
		t.FailNow()
		return
	}

	diff := difflib.UnifiedDiff{
		A:        difflib.SplitLines(string(canonData)),
		B:        difflib.SplitLines(string(data)),
		FromFile: "old",
		ToFile:   "new",
		Context:  3,
	}
	text, err := difflib.GetUnifiedDiffString(diff)
	if err != nil {
		t.Errorf("cannot diff: %v", err)
	}

	if text != "" {
		t.Errorf("%s", addColorsToDiff(text))
	}
}

func AssertStructDiff(t T, base, name string, s any) {
	testData, err := yaml.Marshal(s)
	if err != nil {
		t.Errorf("can't encode data with error: %q", err.Error())
		t.FailNow()
		return
	}

	baseFilePath := filepath.Join(canonDirName, base, name+".yaml")
	baseData, err := readCanonData(baseFilePath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Errorf("can't read base data with error: %q", err.Error())
		t.FailNow()
		return
	}

	canonFilePath := getCanonFilePath(t, name+".diff")
	testDiff := difflib.UnifiedDiff{
		A:        difflib.SplitLines(string(baseData)),
		B:        difflib.SplitLines(string(testData)),
		FromFile: fmt.Sprintf("base/%s.yson", name),
		ToFile:   fmt.Sprintf("test/%s.yson", name),
		Context:  3,
	}
	data, err := difflib.GetUnifiedDiffString(testDiff)
	if err != nil {
		t.Errorf("cannot diff: %v", err)
	}

	// NOTE: Do not save empty diff.
	if isCanonizeNeeded() && data != "" {
		err = writeCanonData(canonFilePath, []byte(data))
		if err != nil {
			t.Errorf("can't write canon data with error: %q", err.Error())
			t.FailNow()
			return
		}
	}

	canonData, err := readCanonData(canonFilePath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Errorf("can't read canon data with error: %q", err.Error())
		t.FailNow()
		return
	}

	diff := difflib.UnifiedDiff{
		A:        difflib.SplitLines(string(canonData)),
		B:        difflib.SplitLines(data),
		FromFile: fmt.Sprintf("old/%s.diff", name),
		ToFile:   fmt.Sprintf("new/%s.diff", name),
		Context:  3,
	}
	text, err := difflib.GetUnifiedDiffString(diff)
	if err != nil {
		t.Errorf("cannot diff: %v", err)
	}

	if text != "" {
		t.Errorf("%s", addColorsToDiff(text))
	}
}

func readCanonData(canonFilePath string) ([]byte, error) {
	if _, err := os.Stat(canonFilePath); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf(
				"can't find canon data file %q, please run tests with %s=y environment variable: %w",
				canonFilePath,
				envDoCanonize,
				err,
			)
		}
		return nil, err
	}

	return os.ReadFile(canonFilePath)
}

func writeCanonData(canonFilePath string, data []byte) error {
	if err := createCanonDirsIfNeeded(canonFilePath); err != nil {
		return err
	}
	return os.WriteFile(canonFilePath, data, defaultFilePermissions)
}

func isCanonizeNeeded() bool {
	_, ok := os.LookupEnv(envDoCanonize)
	return ok
}

func createCanonDirsIfNeeded(canonFilePath string) error {
	canonDir := filepath.Dir(canonFilePath)
	_, err := os.Stat(canonDir)

	if err != nil && os.IsNotExist(err) {
		return os.MkdirAll(canonDir, defaultDirPermissions)
	}

	return err
}

func getCanonFilePath(t T, fileName string) string {
	return filepath.Join(canonDirName, t.Name(), fileName)
}
