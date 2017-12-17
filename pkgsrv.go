package huker

import (
	"fmt"
	"github.com/go-yaml/yaml"
	"github.com/gorilla/mux"
	"github.com/qiniu/log"
	"html/template"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

const HTML_TMPL = `
	<table border="1" bordercolor="#a0c6e5" style="border-collapse:collapse;" align="left">
		<tr>
			<td>PackageName</td>
			<td>Version</td>
			<td>Date</td>
			<td>MD5 Checksum</td>
			<td>Size</td>
		</tr>

		{{ range . }}
		<tr>
			<td><a href="/{{ .PackageName }}">{{ .PackageName  }}</a></td>
			<td>{{ .Version }}</td>
			<td>{{ .Date }}</td>
			<td>{{ .Md5sum }}</td>
			<td>{{ .Size }}</td>
		</tr>
		{{ end }}
	</table>
	<div style="clear:both">
	{{ len . }} packages in total.
	`

type PackageInfo struct {
	PackageName string
	Version     string `yaml: "version"`
	Date        string `yaml: "date"`
	Md5sum      string `yaml: "md5sum"`
	Size        int64  `yaml: "size"`
	Link        string `yaml: "link"`
}

func (p *PackageInfo) isCorrectPackage(libDir string) (bool, error) {
	fName, err := p.getAbsPath(libDir)
	if err != nil {
		return false, err
	}
	if stat, err := os.Stat(fName); err != nil {
		return false, err
	} else if os.IsNotExist(err) {
		return false, fmt.Errorf("Package %s not found.", fName)
	} else if stat.Size() != p.Size {
		return false, fmt.Errorf("Package size mismatch, %s, %d != %d", fName, stat.Size(), p.Size)
	} else if realCheckSum, err2 := calcFileMD5Sum(fName); err2 != nil {
		return false, err2
	} else if realCheckSum != p.Md5sum {
		return false, fmt.Errorf("Package md5 checksum mismatch, package: %s, %s != %s", fName, realCheckSum, p.Md5sum)
	}
	return true, nil
}

func (p *PackageInfo) getAbsPath(libDir string) (string, error) {
	return filepath.Abs(path.Join(libDir, p.PackageName))
}

func (p *PackageInfo) sync(libDir string) {
	if ok, _ := p.isCorrectPackage(libDir); ok {
		log.Infof("Skip to download package : %s", p.PackageName)
		return
	}
	resp, err := http.Get(p.Link)
	if err != nil {
		log.Errorf("Downloading package failed. package : %s, err: %s", p.Link, err.Error())
		return
	}
	defer resp.Body.Close()

	var fName string
	fName, err = p.getAbsPath(libDir)
	out, err := os.Create(fName)
	if err != nil {
		log.Errorf("Create package file error: %v", err)
		return
	}
	defer out.Close()
	io.Copy(out, resp.Body)

	// Final check
	if ok, err := p.isCorrectPackage(libDir); ok {
		log.Infof("Download package success: %s", p.Link)
	} else {
		log.Infof("Package is still inconrrect after download, err: %v", err)
	}
}

type PackageList []*PackageInfo

func (p PackageList) Len() int {
	return len(p)
}

func (p PackageList) Swap(i, j int) {
	p[i], p[j] = p[j], p[i]
}

func (p PackageList) Less(i, j int) bool {
	return strings.Compare(p[i].PackageName, p[j].PackageName) < 0
}

type PackageServer struct {
	port    int
	pkgRoot string
	pkgConf string
	pkgMap  map[string]*PackageInfo
	httpSrv *http.Server
}

func NewPackageServer(port int, pkgRoot, pkgConf string) (*PackageServer, error) {
	p := &PackageServer{
		port:    port,
		pkgRoot: pkgRoot,
		pkgConf: pkgConf,
		pkgMap:  make(map[string]*PackageInfo),
		httpSrv: &http.Server{
			Addr: fmt.Sprintf(":%d", port),
		},
	}

	f, fErr := os.Open(p.pkgConf)
	if fErr != nil {
		return nil, fErr
	}
	data, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}

	err = yaml.Unmarshal(data, &(p.pkgMap))
	if err != nil {
		return nil, err
	}

	// NOTE: need to update package name.
	for pkgN, pkgInfo := range p.pkgMap {
		pkgInfo.PackageName = pkgN
	}

	return p, nil
}

func (p *PackageServer) hIndex(w http.ResponseWriter, r *http.Request) {
	t, err := template.New("Package Index").Parse(HTML_TMPL)
	if err != nil {
		log.Error("Parse template failed: %v", err)
		return
	}

	var pkgList []*PackageInfo
	for _, pkgInfo := range p.pkgMap {
		pkgList = append(pkgList, pkgInfo)
	}

	sort.Sort(PackageList(pkgList))

	err = t.Execute(w, pkgList)
	if err != nil {
		log.Errorf("Render template error: %v", err)
	}
}

func (p *PackageServer) hDownload(w http.ResponseWriter, r *http.Request) {
	pkgName := mux.Vars(r)["packageName"]
	if _, ok := p.pkgMap[pkgName]; !ok {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(fmt.Sprintf("Package %s not found", pkgName)))
		return
	}
	pkg := p.pkgMap[pkgName]
	if _, err := pkg.isCorrectPackage(p.pkgRoot); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(fmt.Sprintf("check package %s error: %v", pkgName, err)))
	}
	fName, _ := pkg.getAbsPath(p.pkgRoot)
	http.ServeFile(w, r, fName)
}

func (p *PackageServer) sync() {
	for _, pkgInfo := range p.pkgMap {
		if pkgInfo != nil {
			go pkgInfo.sync(p.pkgRoot)
		}
	}
}

func (p *PackageServer) Start() error {
	// Sync package to local lib.
	p.sync()

	// Start Http Server.
	r := mux.NewRouter()
	r.HandleFunc("/", p.hIndex).Methods("GET")
	r.HandleFunc("/{packageName}", p.hDownload).Methods("GET")
	p.httpSrv.Handler = r
	return p.httpSrv.ListenAndServe()
}
