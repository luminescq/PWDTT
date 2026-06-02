//go:build linux

package backend

/*
#cgo pkg-config: ayatana-appindicator3-0.1
#include "tray_linux.h"
#include <stdlib.h>
*/
import "C"
import (
	"bytes"
	"image"
	"image/png"
	_ "image/png"
	"os"
	"path/filepath"
	"unsafe"

	"golang.org/x/image/draw"
)

var trayShowFn func()
var trayQuitFn func()

//export onShowClicked
func onShowClicked() {
	if trayShowFn != nil {
		trayShowFn()
	}
}

//export onQuitClicked
func onQuitClicked() {
	if trayQuitFn != nil {
		trayQuitFn()
	}
}

func startTray(iconData []byte, onShow, onQuit func()) {
	trayShowFn = onShow
	trayQuitFn = onQuit

	tmp := filepath.Join(os.TempDir(), "wdtt-tray-icon.png")

	// Resize to 22x22 for AppIndicator
	if src, _, err := image.Decode(bytes.NewReader(iconData)); err == nil {
		dst := image.NewRGBA(image.Rect(0, 0, 22, 22))
		draw.BiLinear.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Over, nil)
		var buf bytes.Buffer
		if err := png.Encode(&buf, dst); err == nil {
			iconData = buf.Bytes()
		}
	}

	_ = os.WriteFile(tmp, iconData, 0644)

	cPath := C.CString(tmp)
	defer C.free(unsafe.Pointer(cPath))
	C.wdtt_tray_init(cPath)

	go C.wdtt_gtk_main()
}

func setTrayVisible(v bool) {
	vis := C.int(0)
	if v {
		vis = C.int(1)
	}
	C.wdtt_tray_set_visible(vis)
}