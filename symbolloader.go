package qtsyms

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ebitengine/purego"
	"github.com/kitech/gopp"
)

var qtsymbolsloaded = false

// var qtsymbolsraw []string

// TODO 这个非常耗时
// 返回匹配的值
func LoadAllQtSymbols(stub string) []string {
	log.Println(qtlibpaths)
	if qtsymbolsloaded {
		log.Println("Already loaded???", len(QtSymbols))
		return nil
	}
	qtsymbolsloaded = true

	// loadcacheok := loadsymbolsjson()
	loadcacheok := loadsymbolsgob()

	if loadcacheok {
		return nil
	} else {
		rets := implLoadAllQtSymbols(stub)
		savesymbolsjson()
		savesymbolsgob()
		return rets
	}
}

func implLoadAllQtSymbols(stub string) []string {
	// log.Println(qtlibpaths)
	var nowt = time.Now()

	// todo 还要查找inline动态库
	libpfx := gopp.Mustify(os.UserHomeDir())[0].Str() + "/.nix-profile/lib"
	globtmpl := fmt.Sprintf("%s/Qt*.framework/Qt*", libpfx)
	libs, err := filepath.Glob(globtmpl)
	gopp.ErrPrint(err, libs)
	libnames := gopp.Mapdo(libs, func(vx any) any {
		return filepath.Base(vx.(string))
	})
	log.Println(gopp.FirstofGv(libs), libnames, len(libs))
	// inlineds := []string{}
	gopp.Mapdo(libs, func(idx int, v string) {
		libnames := qtmod2rclibnames(v, true)

		for _, libname := range libnames {
			dlh, err := purego.Dlopen(libname, purego.RTLD_LAZY)
			if err != nil {
				continue
			}
			purego.Dlclose(dlh)
			// log.Println(v, libname)

			libfile := findmoduleBylibname(libname)
			if gopp.Empty(libfile) {
				continue
			}

			lines, err := gopp.RunCmd(".", "nm", libfile)
			gopp.ErrPrint(err, v)
			// log.Println(idx, libname, len(lines), libfile)
			for _, line := range lines {
				Addsymrawline(filepath.Base(v), line)
			}
		}
	})
	log.Println("Maybe use about little secs...")
	signtx := gopp.Mapdo(libs, func(idx int, vx any) (rets []any) {
		// log.Println(idx, vx, gopp.Bytes2Humz(gopp.FileSize(vx.(string))))
		lines, err := gopp.RunCmd(".", "nm", vx.(string))
		gopp.ErrPrint(err, vx)
		// log.Println(idx, vx, len(lines))
		for _, line := range lines {
			if strings.Contains(line, "Private") {
				continue
			}

			if strings.Contains(line, stub) {
				// log.Println(line)
				name := gopp.Lastof(strings.Split(line, " ")).Str()
				signt, ok := Demangle(name)
				log.Println(name, ok, signt)
				rets = append(rets, name, signt)
			}
			Addsymrawline(filepath.Base(vx.(string)), line)
		}
		return
	})
	log.Println(gopp.Lenof(signtx), len(QtSymbols), time.Since(nowt)) // about 1.1s
	signts := gopp.IV2Strings(signtx.([]any))

	// qtsymbolsraw = signts
	return signts
}

// /// structured symbols cache
const qtsymcachenamejson = "qtsymbols.json"
const qtsymcachenamegob = "qtsymbols.gob"

func savesymbolsjson() {
	nowt := time.Now()
	bcc, err := json.Marshal(QtSymbols)
	gopp.ErrPrint(err)
	gopp.SafeWriteFile(qtsymcachenamejson, bcc, 0644)
	bcc = nil
	// jsonenc 106.696382ms
	log.Println("jsonenc", time.Since(nowt), qtsymcachenamejson)
}
func loadsymbolsjson() bool {
	if !gopp.FileExist2(qtsymcachenamejson) {
		return false
	}
	QtSymbols = nil

	nowt := time.Now()
	bcc, err := os.ReadFile(qtsymcachenamejson)
	gopp.ErrPrint(err)

	err = json.Unmarshal(bcc, &QtSymbols)
	gopp.ErrPrint(err)
	// about 400ms
	log.Println("decode big json", time.Since(nowt), qtsymcachenamejson)
	bcc = nil

	return err == nil
}

func savesymbolsgob() {
	nowt := time.Now()
	var buf = bytes.NewBuffer(nil)
	enco := gob.NewEncoder(buf)
	err := enco.Encode(QtSymbols)
	gopp.ErrPrint(err)
	gopp.SafeWriteFile(qtsymcachenamegob, buf.Bytes(), 0644)
	// gobenc 75.741979ms
	log.Println("gobenc", time.Since(nowt), qtsymcachenamegob)
}
func loadsymbolsgob() bool {
	if !gopp.FileExist2(qtsymcachenamegob) {
		return false
	}

	QtSymbols = nil

	nowt := time.Now()
	fo, err := os.Open(qtsymcachenamegob)
	gopp.ErrPrint(err)
	if err != nil {
		return false
	}
	defer fo.Close()

	deco := gob.NewDecoder(fo)
	err = deco.Decode(&QtSymbols)
	gopp.ErrPrint(err)
	// 37.778846ms - 45.944927ms
	log.Println("gobdec", time.Since(nowt), qtsymcachenamegob)

	return err == nil
}
