package huker

import (
	"bytes"
	"text/template"
)

// The global variables for configuration files.
type EnvVariables struct {
	ConfRootDir  string
	PkgRootDir   string
	PkgConfDir   string
	PkgDataDir   string
	PkgLogDir    string
	PkgStdoutDir string
}

const TML_NAME = "service.yaml"

// Render those global variables into configuration file.
func (e *EnvVariables) RenderTemplate(s string) (string, error) {
	var b bytes.Buffer
	t, err := template.New(TML_NAME).Parse(s)
	if err != nil {
		return "", err
	}
	err = t.Execute(&b, e)
	if err != nil {
		return "", err
	}
	return b.String(), err
}
