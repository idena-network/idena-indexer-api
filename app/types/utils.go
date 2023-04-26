package types

import (
	"fmt"
	"strings"
)

func DefaultContractVerifiedCodeFile(address string) string {
	return fmt.Sprintf("%s.zip", strings.ToLower(address))
}
