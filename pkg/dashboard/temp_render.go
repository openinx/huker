package pkg

import (
	"bytes"
	"github.com/qiniu/log"
	"html/template"
	"io/ioutil"
	"strings"
)

func RenderTemplate(tmplFile string, baseFile string, args map[string]interface{}, funcMap template.FuncMap) (string, error) {
	var err error
	var data []byte
	var buf bytes.Buffer

	t := template.New(tmplFile)
	if funcMap != nil {
		t.Funcs(funcMap)
	}

	data, err = ioutil.ReadFile(baseFile)
	if err != nil {
		log.Errorf("Read template file failed: " + err.Error())
		return "", err
	}

	t, err = t.Parse(string(data))
	if err != nil {
		log.Errorf("Parse template file failed: %s" + err.Error())
		return "", err
	}

	t, err = t.ParseFiles(tmplFile)

	if err != nil {
		log.Errorf("Parse base file failed: %s" + err.Error())
		return "", err
	}

	if err = t.Execute(&buf, args); err != nil {
		log.Errorf("Execute tmplate failed: " + err.Error())
		return "", err
	}

	body := strings.Replace(buf.String(), "&lt;", "<", -1)
	body = strings.Replace(body, "&gt;", ">", -1)
	return body, nil
}
