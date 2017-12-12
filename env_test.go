package huker

import "testing"

func TestEnvVariables_RenderTemplate(t *testing.T) {
	cases := [][]string{
		{"{{.JavaHome}}, {{.ConfRootDir}}", "/usr/bin/java, ../"},
		{"{,{{.PkgLogDir}}", "{,/log"},
		{"{{.PkgDataDir}}", "/data"},
	}

	e := &EnvVariables{
		JavaHome:     "/usr/bin/java",
		ConfRootDir:  "../",
		PkgRootDir:   "/",
		PkgConfDir:   "/conf",
		PkgDataDir:   "/data",
		PkgLogDir:    "/log",
		PkgStdoutDir: "/stdout",
	}

	for _, cas := range cases {
		if ret, err := e.RenderTemplate(cas[0]); err != nil {
			t.Errorf("%v", err)
		} else if ret != cas[1] {
			t.Errorf("%s != %s", ret, cas[1])
		}
	}
}
