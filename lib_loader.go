package qtsyms

import (
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/kitech/gopp"
	"github.com/kitech/gopp/cgopp"
)

var qtlibpaths = map[string]string{}
var mainqtmods = []string{"Core", "Gui", "Widgets", "Network", "Qml", "Quick", "QuickControls2", "QuickWidgets"}

func filterQtsoimages(soimgs []string) (rets []string) {
	gopp.Mapdo(soimgs, func(vx any) any {
		v := vx.(string)
		bname := filepath.Base(v)
		if strings.HasPrefix(bname, "Qt") { // macos
			rets = append(rets, v)
		} else if strings.HasPrefix(bname, "libQt") {
			rets = append(rets, v)
		}
		return nil
	})
	return
}

// libQt6Core.so => Core
func qtlibname2mod(nameorpath string) string {
	bname := qtlibname2link(nameorpath)
	if strings.HasPrefix(bname, "Qt") {
		if bname[2] >= '0' && bname[2] <= '9' {
			return bname[3:]
		}
		return bname[2:]
	}
	return bname
}

// libQt6Core.so => Qt6Core
func qtlibname2link(nameorpath string) string {
	// QtCore // mac
	// libQtCore.dylib // mac
	// libQtCore.so // linux/unix
	// libQt5Core.so // linux/unix
	// libQt6Core.so // linux/unix
	// libQtCore.dll // win
	// libQt5Core.dll // win
	// libQt6Core.dll // win
	bname := nameorpath
	bname = filepath.Base(bname)
	pos := strings.Index(bname, ".")
	if pos > 0 {
		bname = bname[:pos]
	}
	if strings.HasPrefix(bname, "lib") {
		bname = bname[3:]
	}
	return bname
}

func FindAllQtlibs() (rets []string) {
	nowt := time.Now()
	// about 143.562879ms !!! not good!!!
	defer func() { log.Println(gopp.MyFuncName(), time.Since(nowt), len(rets)) }()
	if runtime.GOOS == "android" {
		return androidFindAllQtlibs()
	}
	return desktopFindAllQtlibs()
}
func desktopFindAllQtlibs() (rets []string) {
	var names []string
	for _, qtmod := range mainqtmods {
		v := qtmod2rclibnames(qtmod, true)
		names = append(names, v...)
	}
	libdirs := getsyslibdirs()
	for i, libdir := range libdirs {
		_ = i
		for j, name := range names {
			_ = j
			path := filepath.Join(libdir, name)
			if gopp.FileExist2(path) {
				rets = append(rets, path)
			}
		}
	}
	if len(libdirs) == 0 {
		log.Println("allqtlibs", len(libdirs), libdirs, len(libdirs))
	}
	return
}
func androidFindAllQtlibs() (rets []string) {
	argatfn := cgopp.Dlsym0("qtapp_argat")
	argptr := cgopp.FfiCall[voidptr](argatfn, 0)
	androidqtentrylibfile := cgopp.GoString(argptr)
	cgopp.Cfreepg(argptr)

	// /data/~~~***/lib/arm64
	applibdir := filepath.Dir(androidqtentrylibfile)
	libfiles, err := filepath.Glob(applibdir + "/*")
	gopp.ErrPrint(err, androidqtentrylibfile, applibdir)

	libfiles = filterQtsoimages(libfiles)
	return libfiles
}

// nameorpath
func qtmod2rclibnames(nameorpath string, incinline bool) (rets []string) {
	mod := qtlibname2mod(nameorpath)
	rets = gopp.Sliceof("Qt"+mod, "Qt6"+mod,
		"libQt"+mod+".so", "libQt"+mod+".dylib", "libQt"+mod+".dll",
		"libQt6"+mod+".so", "libQt6"+mod+".dylib", "libQt6"+mod+".dll",
	)
	if incinline {
		rets = append(rets,
			"libQt"+mod+"Inline.so", "libQt"+mod+"Inline.dylib", "libQt"+mod+"Inline.dll",
		)
	}
	return
}

// func FindModule(modname string) (string, error) {
// 	modname = "Core"
// 	dlh, err := purego.Dlopen(modname, purego.RTLD_LAZY)
// 	gopp.ErrPrint(err, modname)
// 	log.Println(dlh)

// 	return modname, nil
// }

// basename like libQtCore.so
// search in libdirs
func findmoduleBylibname(libname string) string {
	libdirs := getsyslibdirs()
	for _, libdir := range libdirs {
		libfile := filepath.Join(libdir, libname)
		if gopp.FileExist2(libfile) {
			return libfile
		}
	}
	return ""
}

func getsyslibdirs() []string {
	libdirs := []string{"", "./", "/opt/qt/lib/", "/usr/lib/", "/usr/lib64/", "/usr/local/lib/", "/usr/local/opt/qt/lib/", gopp.Mustify1(os.UserHomeDir()) + "/.nix-profile/lib/"}

	for _, envname := range []string{"LD_LIBRARY_PATH", "DYLD_LIBRARY_PATH"} {
		envldpath := os.Getenv(envname)
		if len(envldpath) == 0 {
			continue
		}
		fld := strings.Split(envldpath, ":")
		libdirs = append(libdirs, fld...)
	}

	qmakepath, err := Which("qmake")
	// log.Println(qmakepath, err)
	if err == nil {
		qmakedir := filepath.Dir(qmakepath)
		qmakelibdir1 := filepath.Join(qmakedir, "../lib")
		qmakelibdir2 := filepath.Join(qmakedir, "../lib64")
		libdirs = append(libdirs, qmakelibdir1, qmakelibdir2)

		rets, err := filepath.Glob(qmakelibdir1 + "/Qt*.framework")
		if err == nil {
			libdirs = append(libdirs, rets...)
		}
		// log.Println(rets)
		rets, err = filepath.Glob(qmakelibdir2 + "/Qt*.framework")
		if err == nil {
			libdirs = append(libdirs, rets...)
		}
		// log.Println(rets)
	}

	return libdirs
}

func Which(name string) (string, error) {
	lines, err := gopp.RunCmd(".", "which", name)
	if err != nil {
		return "", err
	}
	if len(lines) == 0 {
		return "", os.ErrNotExist
	}
	return gopp.FirstofGv(lines), nil
}
