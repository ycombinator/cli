package envcfg

import (
	"fmt"
	"os"
	"sync"

	"github.com/joho/godotenv"
)

var loadOnce sync.Once
var loadErr error

// ReadEnv is a wrapper on godotenv to read the env variables from the
// given files and load them in the environment.
// TODO: do we want to use the server envcfg here? This is just a
// simplified version of that.
func ReadEnv(files []string) error {
	loadOnce.Do(func() {
		overwrite := map[string]string{}

		// do not use godotenv in order to not error out if
		// .env or .env.local do not exist. Ignore not exists errors
		// and contiue with loading/overwrites
		for _, filename := range files {
			f, err := os.Open(filename)
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				loadErr = fmt.Errorf("load env: file '%v': %w", filename, err)
				break
			}
			defer f.Close()

			m, err := godotenv.Parse(f)
			if err != nil {
				loadErr = fmt.Errorf("read env: file '%v': %w", filename, err)
				break
			}

			for k, v := range m {
				overwrite[k] = v
			}
		}

		if loadErr == nil {
			// Add environment varialbes that have not been available
			// during process startup.
			for k, v := range overwrite {
				if _, exists := os.LookupEnv(k); !exists {
					os.Setenv(k, v)
				}
			}
		}
	})
	return loadErr
}
