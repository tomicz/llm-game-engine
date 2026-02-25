package env

import (
	"bufio"
	"os"
	"strings"
)

// Load reads the given file (e.g. ".env") and sets environment variables for each
// line of the form KEY=VALUE. Empty lines and lines starting with # are skipped.
// The file may be missing; that is not an error.
func Load(path string) error {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		i := strings.Index(line, "=")
		if i <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:i])
		value := strings.TrimSpace(line[i+1:])
		if key == "" {
			continue
		}
		// Remove surrounding quotes if present
		if len(value) >= 2 && (value[0] == '"' && value[len(value)-1] == '"' || value[0] == '\'' && value[len(value)-1] == '\'') {
			value = value[1 : len(value)-1]
		}
		_ = os.Setenv(key, value)
	}
	return scanner.Err()
}
