package cmd

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseBranchUrl(t *testing.T) {
	tests := map[string]struct {
		url        string
		worksapce  string
		dbname     string
		branch     string
		errMessage string
	}{
		"normal": {
			url:       "https://demo-21321.xata.sh/db/test:main",
			worksapce: "demo-21321",
			dbname:    "test",
			branch:    "main",
		},
		"special chars": {
			url:       "https://demo-21321.xata.sh/db/test-as_dsa!sada:main_12321-1231231",
			worksapce: "demo-21321",
			dbname:    "test-as_dsa!sada",
			branch:    "main_12321-1231231",
		},
		"missing workspace": {
			url:        "https://xata.sh/db/test:main",
			errMessage: "Expected URL hostname to be a single subdomain under xata.sh (Example demo-1234.xata.sh). Got: xata.sh",
		},
		"too many subdomains": {
			url:        "https://more.demo-21321.xata.sh/db/test:main",
			errMessage: "Expected URL hostname to be a single subdomain under xata.sh (Example demo-1234.xata.sh). Got: more.demo-21321.xata.sh",
		},
		"missing branch": {
			url:        "https://demo-21321.xata.sh/db/test",
			errMessage: "Expected URL path to be of the form /db/{database}:{branch}. Got: /db/test",
		},
		"too many colons": {
			url:        "https://demo-21321.xata.sh/db/test:main:main",
			errMessage: "Expected URL path to be of the form /db/{database}:{branch}. Got: /db/test:main:main",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			workspace, dbname, branch, err := parseBranchUrl(test.url)

			if test.errMessage == "" {
				require.NoError(t, err)
				require.Equal(t, test.worksapce, workspace)
				require.Equal(t, test.dbname, dbname)
				require.Equal(t, test.branch, branch)
			} else {
				require.EqualError(t, err, test.errMessage)
			}
		})
	}
}
