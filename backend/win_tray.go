//go:build windows

package backend

import (
	"bytes"
	"image"
	stdraw "image/draw"
	_ "image/png"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"unsafe"

	"github.com/lxn/win"
	xdraw "golang.org/x/image/draw"
)

const (
	wmTrayMsg = win.WM_USER + 1
	idShow    = 1
	idQuit    = 2
)

var (
	trayOnce sync.Once
	trayHwnd win.HWND
	trayNID  win.NOTIFYICONDATA

	user32          = syscall.NewLazyDLL("user32.dll")
	procAppendMenuW = user32.NewProc("AppendMenuW")
)

func appendMenu(hMenu win.HMENU, flags uint32, id uintptr, text string) {
	p, _ := syscall.UTF16PtrFromString(text)
	procAppendMenuW.Call(uintptr(hMenu), uintptr(flags), id, uintptr(unsafe.Pointer(p)))
}

func startTray(iconData []byte, onShow, onQuit func()) {
	trayOnce.Do(func() {
		go runTrayLoop(iconData, onShow, onQuit)
	})
}

func setTrayVisible(v bool) {
	if trayHwnd == 0 {
		return
	}
	nid := trayNID
	nid.UFlags = win.NIF_STATE
	if v {
		nid.DwState = 0
	} else {
		nid.DwState = win.NIS_HIDDEN
	}
	nid.DwStateMask = win.NIS_HIDDEN
	win.Shell_NotifyIcon(win.NIM_MODIFY, &nid)
}

func pngToHICON(data []byte, size int) win.HICON {
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return 0
	}
	dst := image.NewNRGBA(image.Rect(0, 0, size, size))
	xdraw.BiLinear.Scale(dst, dst.Bounds(), src, src.Bounds(), stdraw.Over, nil)

	// Build BGRA pixel buffer (Windows DIB is bottom-up)
	buf := make([]byte, size*size*4)
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			c := dst.NRGBAAt(x, size-1-y) // flip vertically
			i := (y*size + x) * 4
			buf[i+0] = c.B
			buf[i+1] = c.G
			buf[i+2] = c.R
			buf[i+3] = c.A
		}
	}

	hdc := win.GetDC(0)
	defer win.ReleaseDC(0, hdc)

	bmi := win.BITMAPINFOHEADER{
		BiSize:        uint32(unsafe.Sizeof(win.BITMAPINFOHEADER{})),
		BiWidth:       int32(size),
		BiHeight:      int32(size),
		BiPlanes:      1,
		BiBitCount:    32,
		BiCompression: win.BI_RGB,
	}
	var bits unsafe.Pointer
	hbmColor := win.CreateDIBSection(hdc, &bmi, win.DIB_RGB_COLORS, &bits, 0, 0)
	if hbmColor == 0 || bits == nil {
		return 0
	}
	copy((*[1 << 24]byte)(bits)[:size*size*4], buf)

	// Build 1-bit AND mask from alpha: transparent pixels (A<128) → 1 (show through), opaque → 0
	rowBytes := (size + 15) / 16 * 2 // WORD-aligned
	maskBuf := make([]byte, rowBytes*size)
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			c := dst.NRGBAAt(x, size-1-y)
			if c.A < 128 {
				maskBuf[y*rowBytes+x/8] |= 1 << uint(7-x%8)
			}
		}
	}
	hbmMask := win.CreateBitmap(int32(size), int32(size), 1, 1, unsafe.Pointer(&maskBuf[0]))

	ii := win.ICONINFO{
		FIcon:    win.TRUE,
		HbmColor: hbmColor,
		HbmMask:  hbmMask,
	}
	hIcon := win.CreateIconIndirect(&ii)
	win.DeleteObject(win.HGDIOBJ(hbmColor))
	win.DeleteObject(win.HGDIOBJ(hbmMask))
	return hIcon
}

func runTrayLoop(iconData []byte, onShow, onQuit func()) {
	hIcon := pngToHICON(iconData, 16)

	// Fallback: write PNG to temp and try LoadImage (won't work for PNG but harmless)
	if hIcon == 0 {
		iconPath := filepath.Join(os.TempDir(), "pwdtt-tray.png")
		_ = os.WriteFile(iconPath, iconData, 0644)
		iconPathW, _ := syscall.UTF16PtrFromString(iconPath)
		hIcon = win.HICON(win.LoadImage(0, iconPathW, win.IMAGE_ICON, 16, 16, win.LR_LOADFROMFILE))
	}

	className, _ := syscall.UTF16PtrFromString("pwdtt_tray")
	wc := win.WNDCLASSEX{
		CbSize:      uint32(unsafe.Sizeof(win.WNDCLASSEX{})),
		LpszClassName: className,
		LpfnWndProc: syscall.NewCallback(func(hwnd win.HWND, msg uint32, wp, lp uintptr) uintptr {
			switch msg {
			case wmTrayMsg:
				ev := lp & 0xFFFF
				if ev == win.WM_RBUTTONUP || ev == win.WM_LBUTTONUP {
					showTrayMenu(hwnd, onShow, onQuit)
				}
			case win.WM_COMMAND:
				switch win.LOWORD(uint32(wp)) {
				case idShow:
					if onShow != nil {
						onShow()
					}
				case idQuit:
					if onQuit != nil {
						onQuit()
					}
				}
			}
			return win.DefWindowProc(hwnd, msg, wp, lp)
		}),
	}
	win.RegisterClassEx(&wc)

	hwnd := win.CreateWindowEx(0, className, className, 0, 0, 0, 0, 0,
		win.HWND_MESSAGE, 0, 0, nil)
	trayHwnd = hwnd

	tip, _ := syscall.UTF16FromString("PWDTT")
	nid := win.NOTIFYICONDATA{
		HWnd:             hwnd,
		UID:              1,
		UFlags:           win.NIF_ICON | win.NIF_MESSAGE | win.NIF_TIP,
		UCallbackMessage: wmTrayMsg,
		HIcon:            hIcon,
	}
	nid.CbSize = uint32(unsafe.Sizeof(nid))
	copy(nid.SzTip[:], tip)
	trayNID = nid
	win.Shell_NotifyIcon(win.NIM_ADD, &nid)

	var msg win.MSG
	for win.GetMessage(&msg, 0, 0, 0) > 0 {
		win.TranslateMessage(&msg)
		win.DispatchMessage(&msg)
	}
}

func showTrayMenu(hwnd win.HWND, onShow, onQuit func()) {
	hMenu := win.CreatePopupMenu()
	appendMenu(hMenu, win.MF_STRING, idShow, "Открыть")
	appendMenu(hMenu, win.MF_SEPARATOR, 0, "")
	appendMenu(hMenu, win.MF_STRING, idQuit, "Выход")

	var pt win.POINT
	win.GetCursorPos(&pt)
	win.SetForegroundWindow(hwnd)
	win.TrackPopupMenu(hMenu, win.TPM_BOTTOMALIGN|win.TPM_LEFTALIGN, pt.X, pt.Y, 0, hwnd, nil)
	win.DestroyMenu(hMenu)
}
