package cmd

import (
	"fmt"
)

type ErrorUnauthorized struct {
	message string
}

func (e ErrorUnauthorized) Error() string {
	return fmt.Sprintf("Auth error: %s\nFor more information please see https://docs.xata.io/cli/getting-started", e.message)
}
