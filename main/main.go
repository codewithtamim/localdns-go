package main

// #cgo LDFLAGS: -llog
// #include <android/log.h>
// #include <stdlib.h>
import "C"

import (
	"fmt"
	"unsafe"

	"org.thebytearray.localdns/tunnel/liblocaldns-go/infra/android"
)

func cstring(s string) *C.char {
	return C.CString(s)
}

func goString(str *C.char) string {
	if str == nil {
		return ""
	}
	return C.GoString(str)
}

//export localdnsConnectDoH
func localdnsConnectDoH(tunFd C.int, fakeDns *C.char, url *C.char, bootstrapIPs *C.char, handleOut *C.int) C.int {
	handle, err := android.ConnectDoH(int(tunFd), goString(fakeDns), goString(url), goString(bootstrapIPs))
	if err != nil {
		logError("localdnsConnectDoH: %v", err)
		return -1
	}
	*handleOut = C.int(handle)
	return 0
}

//export localdnsConnectDoT
func localdnsConnectDoT(tunFd C.int, fakeDns *C.char, hostname *C.char, bootstrapIPs *C.char, handleOut *C.int) C.int {
	handle, err := android.ConnectDoT(int(tunFd), goString(fakeDns), goString(hostname), goString(bootstrapIPs))
	if err != nil {
		logError("localdnsConnectDoT: %v", err)
		return -1
	}
	*handleOut = C.int(handle)
	return 0
}

//export localdnsDisconnect
func localdnsDisconnect(handle C.int) {
	android.Disconnect(int32(handle))
}

//export localdnsSetDoHServer
func localdnsSetDoHServer(handle C.int, url *C.char, bootstrapIPs *C.char) C.int {
	if err := android.SetDoHServer(int32(handle), goString(url), goString(bootstrapIPs)); err != nil {
		logError("localdnsSetDoHServer: %v", err)
		return -1
	}
	return 0
}

//export localdnsSetDoTServer
func localdnsSetDoTServer(handle C.int, hostname *C.char, bootstrapIPs *C.char) C.int {
	if err := android.SetDoTServer(int32(handle), goString(hostname), goString(bootstrapIPs)); err != nil {
		logError("localdnsSetDoTServer: %v", err)
		return -1
	}
	return 0
}

//export localdnsProbeDoH
func localdnsProbeDoH(url *C.char, bootstrapIPs *C.char) C.int {
	if err := android.ProbeDoH(goString(url), goString(bootstrapIPs)); err != nil {
		return -1
	}
	return 0
}

//export localdnsProbeDoT
func localdnsProbeDoT(hostname *C.char, bootstrapIPs *C.char) C.int {
	if err := android.ProbeDoT(goString(hostname), goString(bootstrapIPs)); err != nil {
		return -1
	}
	return 0
}

//export localdnsVersion
func localdnsVersion() *C.char {
	return cstring("LocalDNS 1.0.0")
}

func logError(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	tag := cstring("LocalDNS")
	defer C.free(unsafe.Pointer(tag))
	cmsg := cstring(msg)
	defer C.free(unsafe.Pointer(cmsg))
	C.__android_log_write(C.ANDROID_LOG_ERROR, tag, cmsg)
}

func main() {}
