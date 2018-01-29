package huker

import (
	"context"
	"fmt"
	"github.com/go-yaml/yaml"
	"github.com/gorilla/mux"
	"github.com/qiniu/log"
	"html/template"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

const htmlTempl = `
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

type packageInfo struct {
	PackageName string
	Version     string `yaml:"version"`
	Date        string `yaml:"date"`
	Md5sum      string `yaml:"md5sum"`
	Size        int64  `yaml:"size"`
	Link        string `yaml:"link"`
}

func (p *packageInfo) isCorrectPackage(libDir string) (bool, error) {
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

func (p *packageInfo) getAbsPath(libDir string) (string, error) {
	return filepath.Abs(path.Join(libDir, p.PackageName))
}

func (p *packageInfo) sync(libDir string, wg *sync.WaitGroup) {
	defer wg.Done()
	if ok, _ := p.isCorrectPackage(libDir); ok {
		log.Infof("Skip to download package : %s", p.PackageName)
		return
	}
	abspath, err := p.getAbsPath(libDir)
	if err != nil {
		log.Errorf("%v", err)
		return
	}
	if err := WebGetToLocal(p.Link, abspath); err != nil {
		log.Errorf("%v", err)
		return
	}
	// Final check
	if ok, err := p.isCorrectPackage(libDir); ok {
		log.Infof("Download package success: %s", p.Link)
	} else {
		log.Infof("Package is still inconrrect after download, err: %v", err)
	}
}

type packageList []*packageInfo

func (p packageList) Len() int {
	return len(p)
}

func (p packageList) Swap(i, j int) {
	p[i], p[j] = p[j], p[i]
}

func (p packageList) Less(i, j int) bool {
	return strings.Compare(p[i].PackageName, p[j].PackageName) < 0
}

// The package server is the package manager of huker, all supervisor agent will send a HTTP request
// to package server for downloading the specific package.
type PackageServer struct {
	port    int
	pkgRoot string
	pkgConf string
	pkgMap  map[string]*packageInfo
	httpSrv *http.Server
}

// Create a new package server
func NewPackageServer(port int, pkgRoot, pkgConf string) (*PackageServer, error) {
	p := &PackageServer{
		port:    port,
		pkgRoot: pkgRoot,
		pkgConf: pkgConf,
		pkgMap:  make(map[string]*packageInfo),
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
	t, err := template.New("Package Index").Parse(htmlTempl)
	if err != nil {
		log.Error("Parse template failed: %v", err)
		return
	}

	var pkgList []*packageInfo
	for _, pkgInfo := range p.pkgMap {
		pkgList = append(pkgList, pkgInfo)
	}

	sort.Sort(packageList(pkgList))

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
	wg := &sync.WaitGroup{}
	wg.Add(len(p.pkgMap))
	for _, pkgInfo := range p.pkgMap {
		if pkgInfo != nil {
			go pkgInfo.sync(p.pkgRoot, wg)
		}
	}
	wg.Wait()
	log.Infof("Finished to sync the release packages.")
}

// Start the package server by listening the specific HTTP port.
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

// Shutdown the package server.
func (p *PackageServer) Shutdown() error {
	return p.httpSrv.Shutdown(context.Background())
}
