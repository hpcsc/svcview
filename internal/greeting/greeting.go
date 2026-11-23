package greeting

import (
	"fmt"
	"strings"
)

func For(name string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "Hello!"
	}
	return fmt.Sprintf("Hello, %s!", trimmed)
}
