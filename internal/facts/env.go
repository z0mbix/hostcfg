package facts

import (
	"os"
	"strings"
)

// gatherEnvFacts collects environment variables as a map
func gatherEnvFacts() map[string]string {
	env := make(map[string]string)

	for _, e := range os.Environ() {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 {
			env[parts[0]] = parts[1]
		}
	}

	return env
}
