package pkg

import (
	"bytes"
	"fmt"
	"html/template"
	"io/ioutil"
	"strings"
)

func RenderTemplate(tmplFile string, baseFile string, args map[string]interface{}, funcMap template.FuncMap) (string, error) {
	var err error
	var fbytes []byte
	var buf bytes.Buffer

	t := template.New(tmplFile)
	if funcMap != nil {
		t.Funcs(funcMap)
	}

	fbytes, err = ioutil.ReadFile(baseFile)
	if err != nil {
		fmt.Println("read template file failed: " + err.Error())
		return "", err
	}

	t, err = t.Parse(string(fbytes))
	if err != nil {
		fmt.Println("parse template file failed: %s" + err.Error())
		return "", err
	}

	t, err = t.ParseFiles(tmplFile)

	if err != nil {
		fmt.Println("parse base file failed: %s" + err.Error())
		return "", err
	}

	if err = t.Execute(&buf, args); err != nil {
		fmt.Println("Execute tmplate failed: " + err.Error())
		return "", err
	}

	body := strings.Replace(buf.String(), "&lt;", "<", -1)
	body = strings.Replace(body, "&gt;", ">", -1)
	return body, nil
}
