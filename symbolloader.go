package qtsyms

import (
	"bytes"
	"cmd/goinct"
	"compress/gzip"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/ebitengine/purego"
	"github.com/kitech/gopp"
	"github.com/kitech/gopp/cgopp"
)

// 为什么是两个变量
var qtsymbolsloaded = false
var InitLoaded = false

// TODO 这个非常耗时
// 返回匹配的值
// 这个需要客户端手工调用，以后可能以某种方式自动调用
func LoadAllQtSymbols() []string {
	log.Println(qtlibpaths)
	if qtsymbolsloaded {
		log.Println("Already loaded???", len(QtSymbols))
		return nil
	}
	qtsymbolsloaded = true
	InitLoaded = true

	log.Println(qtsymbolsgobgz == nil, len(qtsymbolsgobgz))
	defer func() { qtsymbolsgobgz = nil }()
	defer func() {
		Qtsymsts.Symmemsz = gopp.DeepSizeof(QtSymbols, 0)
		Qtsymsts.ClassCnt = len(QtSymbols)
	}()

	var nowt = time.Now()
	// loadcacheok := loadsymbolsjson()
	loadcacheok := loadsymbolsgob()
	Qtsymsts.Loadsymdur = time.Since(nowt)

	if !loadcacheok && runtime.GOOS == "android" {
		nowt = time.Now()
		loadcacheok = loadsymbolsembedgob()
		// loadcacheok = loadsymbolsembedjson()
		Qtsymsts.Loadsymdur = time.Since(nowt)
	}

	if loadcacheok {
		return nil
	} else {
		var rets []string
		nowt = time.Now()
		if true {
			rets = implLoadAllQtSymbolsByGonm()
		} else {
			rets = implLoadAllQtSymbolsByCmdnm()
		}
		Qtsymsts.Loadsymdur = time.Since(nowt)
		nowt = time.Now()
		savesymbolsjson()
		savesymbolsgob()
		Qtsymsts.Savesymdur = time.Since(nowt)
		return rets
	}
}

// by search, by cmdline nm
// should deprecated because we hav by gonm version
func implLoadAllQtSymbolsByCmdnm() []string {
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

			Addsymrawline(filepath.Base(vx.(string)), line)
		}
		return
	})
	log.Println(gopp.Lenof(signtx), "clz", len(QtSymbols), "all", Qtsymsts.TotalCnt, "Weaks", Qtsymsts.WeakCnt, time.Since(nowt)) // about 1.1s
	signts := gopp.IV2Strings(signtx.([]any))

	// qtsymbolsraw = signts
	return signts
}

func implLoadAllQtSymbolsByGonm() []string {
	shlibs := cgopp.DyldImagesSelf()
	qtshlibs := filterQtsoimages(shlibs)
	if len(qtshlibs) == 0 {
		qtshlibs = FindAllQtlibs()
	}

	for _, shlib := range qtshlibs {
		log.Println("run NMget...", shlib)
		rawsyms, err := goinct.NMget(shlib)
		if err != nil {
			Qtsymsts.Errors = append(Qtsymsts.Errors, err)
			continue
		}
		for _, rawsym := range rawsyms {
			// Addsymrawline("", rawsym.Name)
			addqtsym("", rawsym.Name, string(rawsym.Code))
		}
	}

	return nil
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
	log.Println("gobdec", time.Since(nowt), qtsymcachenamegob, gopp.FileSize(qtsymcachenamegob))

	return err == nil
}

func loadsymbolsembedgob() bool {
	var ebdata = qtsymbolsgobgz
	gzr, err := gzip.NewReader(bytes.NewBuffer(ebdata))
	gopp.ErrPrint(err, len(ebdata))
	if err != nil {
		Qtsymsts.Errors = append(Qtsymsts.Errors, err)
		return false
	}
	defer gzr.Close()

	deco := gob.NewDecoder(gzr)
	err = deco.Decode(&QtSymbols)
	gopp.ErrPrint(err, len(ebdata))
	if err != nil {
		Qtsymsts.Errors = append(Qtsymsts.Errors, err)
		return false
	}

	return true
}

// func loadsymbolsembedjson() bool {
// 	var ebdata = qtsymbolsjsongz
// 	gzr, err := gzip.NewReader(bytes.NewBuffer(ebdata))
// 	gopp.ErrPrint(err, len(ebdata))
// 	if err != nil {
// 		Qtsymsts.Errors = append(Qtsymsts.Errors, err)
// 		return false
// 	}
// 	defer gzr.Close()

// 	deco := json.NewDecoder(gzr)
// 	err = deco.Decode(&QtSymbols)
// 	gopp.ErrPrint(err, len(ebdata))
// 	if err != nil {
// 		Qtsymsts.Errors = append(Qtsymsts.Errors, err)
// 		return false
// 	}

// 	return true
// }
