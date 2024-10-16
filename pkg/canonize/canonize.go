package canonize

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pmezard/go-difflib/difflib"
)

const (
	envDoCanonize          = "CANONIZE"
	canonDirName           = "canondata"
	canonFileName          = "test.canondata"
	defaultFilePermissions = 0644
	defaultDirPermissions  = 0755
)

func Assert(t *testing.T, data []byte) {
	canonFilePath := getCanonFilePath(t)

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
	text, _ := difflib.GetUnifiedDiffString(diff)
	if text != "" {
		t.Errorf("%s", addColorsToDiff(text))
	}
}

func readCanonData(canonFilePath string) ([]byte, error) {
	if _, err := os.Stat(canonFilePath); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf(
				"can't find canon data file %q, please run tests with %s=y environment variable",
				canonFilePath,
				envDoCanonize,
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

func getCanonFilePath(t *testing.T) string {
	return filepath.Join(canonDirName, t.Name(), canonFileName)
}
