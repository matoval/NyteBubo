package types

import (
	"fmt"
	"strings"
)

type Config struct {
	ProjectName string
	GitRemote   string
	Lang        string
	Frameworks  []string
}

func (c Config) Display() string {
	var b strings.Builder
	b.WriteString("\nConfiguration Summary:\n")
	b.WriteString(fmt.Sprintf("  Project Name:  %s\n", c.ProjectName))
	b.WriteString(fmt.Sprintf("  Git Remote:    %s\n", c.GitRemote))
	b.WriteString(fmt.Sprintf("  Language:      %s\n", c.Lang))
	b.WriteString(fmt.Sprintf("  Frameworks:    %s\n\n", strings.Join(c.Frameworks, ", ")))
	return b.String()
}
