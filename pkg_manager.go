package huker

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"github.com/go-yaml/yaml"
	"github.com/qiniu/log"
	"html/template"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"
)

const HTML_TMPL = `
	<table border="1" bordercolor="#a0c6e5" style="border-collapse:collapse;" align="left">
		<tr>
			<td>PackageName</td>
			<td>Date</td>
			<td>MD5 Checksum</td>
			<td>Size</td>
		</tr>

		{{ range $key, $value := . }}
		<tr>
			<td><a href="/{{ $key }}">{{ $key }}</a></td>
			<td>{{ index $value "date" }}</td>
			<td>{{ index $value "md5sum" }}</td>
			<td>{{ index $value "size" }}</td>
		</tr>
		{{ end }}
	</table>
	<div style="clear:both">
	{{ len . }} packages in total.
	`

type PackageInfo struct {
	pkgName string
	md5sum  string
	size    int64
	link    string
}

func parsePackageInfo(pkgName string, m map[interface{}]interface{}) *PackageInfo {
	pkgAddr := m["link"].(string)
	md5sum := m["md5sum"].(string)
	size := int64(m["size"].(int))
	return &PackageInfo{
		pkgName: pkgName,
		md5sum:  md5sum,
		size:    size,
		link:    pkgAddr,
	}
}

func calcFileMD5Sum(fileName string) string {
	f, err := os.Open(fileName)
	if err != nil {
		log.Fatal("Open file error: %v", err)
		return ""
	}
	defer f.Close()
	hashReader := md5.New()
	if _, err := io.Copy(hashReader, f); err != nil {
		log.Fatal("Calcuate md5 sum error: %v", err)
		return ""
	}
	return hex.EncodeToString(hashReader.Sum(nil))

}

func isPackageCorrect(pkgInfo *PackageInfo, pkgRootDir string) bool {
	pkgFileName := fmt.Sprintf("%s/%s", pkgRootDir, pkgInfo.pkgName)
	stat, err := os.Stat(pkgFileName)
	if err != nil {
		log.Errorf("Check the existence error, package: %s, error: %v", pkgFileName, err)
		return false
	} else if calcFileMD5Sum(pkgFileName) == pkgInfo.md5sum && stat.Size() == pkgInfo.size {
		return true
	} else {
		return false
	}
}

func verifyAndSyncPackage(pkgInfo *PackageInfo, pkgRootDir string) {
	if _, err := os.Stat(pkgRootDir); os.IsNotExist(err) {
		os.MkdirAll(pkgRootDir, 755)
	}

	pkgFileName := fmt.Sprintf("%s/%s", pkgRootDir, pkgInfo.pkgName)
	if isPackageCorrect(pkgInfo, pkgRootDir) {
		log.Debugf("Package exists, skip to download from mirror: %s", pkgFileName)
		return
	}

	log.Infof("Downloading package: %s", pkgInfo.link)
	resp, err := http.Get(pkgInfo.link)
	if err != nil {
		log.Errorf("Downloading package failed. package : %s, err: %s", pkgInfo.link, err.Error())
		return
	}
	defer resp.Body.Close()

	out, err := os.Create(pkgFileName)
	if err != nil {
		log.Errorf("Create package file error: %v", err)
		return
	}

	defer out.Close()
	io.Copy(out, resp.Body)
	log.Infof("Download package success: %s", pkgInfo.link)
}

func syncPackage(meta map[interface{}]interface{}, pkgRootDir string) {
	for pkgName := range meta {
		pkgInfo := parsePackageInfo(pkgName.(string), meta[pkgName].(map[interface{}]interface{}))
		go verifyAndSyncPackage(pkgInfo, pkgRootDir)
	}
}

func StartPkgManager(listenAddress, pkgConf, pkgRootDir string) {
	data, err := ioutil.ReadFile(pkgConf)
	if err != nil {
		log.Errorf("Read pkg.yaml failed: %v", err)
		return
	}

	m := make(map[interface{}]interface{})
	err = yaml.Unmarshal(data, &m)
	if err != nil {
		log.Errorf("Deserilize yaml config failed: %v", err)
		return
	}

	t := template.New("Package Index")
	t, err = t.Parse(HTML_TMPL)
	if err != nil {
		log.Error("Parse template failed: %v", err)
		return
	}

	syncPackage(m, pkgRootDir)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		path := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/"), "/")

		if m[path] != nil {
			pkgInfo := parsePackageInfo(path, m[path].(map[interface{}]interface{}))
			if isPackageCorrect(pkgInfo, pkgRootDir) {
				pkgFileName := fmt.Sprintf("%s/%s", pkgRootDir, pkgInfo.pkgName)
				http.ServeFile(w, r, pkgFileName)
				log.Infof("GET /%s - %d %s %s", path, 200, time.Since(start), r.RemoteAddr)
			} else {
				w.WriteHeader(http.StatusNotFound)
				fmt.Fprint(w, "Not Found")
				log.Infof("GET /%s - %d %s %s", path, 404, time.Since(start), r.RemoteAddr)
			}
		} else if path != "" {
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprint(w, "Not Found")
			log.Infof("GET /%s - %d %s %s", path, 404, time.Since(start), r.RemoteAddr)
		} else {
			t.Execute(w, m)
			log.Infof("GET /%s - %d %s %s", path, 200, time.Since(start), r.RemoteAddr)
		}
	})

	http.ListenAndServe(listenAddress, nil)
}
