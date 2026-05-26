package android

import (
	"errors"
	"sync/atomic"
	"unsafe"

	"org.thebytearray.localdns/tunnel/liblocaldns-go/app/session"
	"org.thebytearray.localdns/tunnel/liblocaldns-go/features/platform"
	"org.thebytearray.localdns/tunnel/liblocaldns-go/features/stats"
)

/*
#include <stdint.h>
#include <stdlib.h>
extern int localdns_jni_protect(int fd);
extern char* localdns_jni_get_resolvers();
extern void localdns_jni_on_dns_query(const char* hostname, int64_t latencyMs, int status);
*/
import "C"

type jniProtector struct{}

func (p *jniProtector) Protect(socket int32) bool {
	return C.localdns_jni_protect(C.int(socket)) != 0
}

func (p *jniProtector) GetResolvers() string {
	cstr := C.localdns_jni_get_resolvers()
	if cstr == nil {
		return ""
	}
	defer C.free(unsafe.Pointer(cstr))
	return C.GoString(cstr)
}

var globalProtector platform.Protector = &jniProtector{}

type jniListener struct{}

func (l *jniListener) OnTCPSocketClosed(summary *stats.TCPSocketSummary) {}

func (l *jniListener) OnUDPSocketClosed(summary *stats.UDPSocketSummary) {}

type jniDoHListener struct{}

func (l *jniDoHListener) OnQuery(url string) session.DoHQueryToken {
	return session.DoHQueryToken(0)
}

func (l *jniDoHListener) OnResponse(_ session.DoHQueryToken, summary *session.DoHQuerySumary) {
	if summary == nil {
		return
	}
	query := summary.GetQuery()
	hostname := extractHostname(query)
	latency := int64(summary.GetLatency() * 1000)
	status := int(summary.GetStatus())
	cHostname := C.CString(hostname)
	defer C.free(unsafe.Pointer(cHostname))
	C.localdns_jni_on_dns_query(cHostname, C.int64_t(latency), C.int(status))
}

func extractHostname(query []byte) string {
	if len(query) < 13 {
		return ""
	}
	offset := 12
	var labels []string
	for offset < len(query) {
		length := int(query[offset])
		if length == 0 {
			break
		}
		offset++
		if offset+length > len(query) {
			break
		}
		labels = append(labels, string(query[offset:offset+length]))
		offset += length
	}
	if len(labels) == 0 {
		return ""
	}
	result := labels[0]
	for i := 1; i < len(labels); i++ {
		result += "." + labels[i]
	}
	return result
}

var (
	sessions         = map[int32]*session.Session{}
	nextHandle int32 = 1
)

func ConnectDoH(tunFd int, fakeDns, url, bootstrapIPs string) (int32, error) {
	server, err := session.NewDoHServer(url, bootstrapIPs, globalProtector, &jniDoHListener{})
	if err != nil {
		return 0, err
	}
	sess, err := session.ConnectSessionDoH(tunFd, fakeDns, server, globalProtector, &jniListener{})
	if err != nil {
		return 0, err
	}
	handle := atomic.AddInt32(&nextHandle, 1)
	sessions[handle] = sess
	return handle, nil
}

func ConnectDoT(tunFd int, fakeDns, hostname, bootstrapIPs string) (int32, error) {
	server, err := session.NewDoTServer(hostname, bootstrapIPs, globalProtector, &jniDoHListener{})
	if err != nil {
		return 0, err
	}
	sess, err := session.ConnectSessionDoT(tunFd, fakeDns, server, globalProtector, &jniListener{})
	if err != nil {
		return 0, err
	}
	handle := atomic.AddInt32(&nextHandle, 1)
	sessions[handle] = sess
	return handle, nil
}

func Disconnect(handle int32) {
	if sess, ok := sessions[handle]; ok {
		sess.Disconnect()
		delete(sessions, handle)
	}
}

func SetDoHServer(handle int32, url, bootstrapIPs string) error {
	sess, ok := sessions[handle]
	if !ok {
		return errors.New("session not found")
	}
	server, err := session.NewDoHServer(url, bootstrapIPs, globalProtector, &jniDoHListener{})
	if err != nil {
		return err
	}
	sess.SetDoHServer(server)
	return nil
}

func SetDoTServer(handle int32, hostname, bootstrapIPs string) error {
	sess, ok := sessions[handle]
	if !ok {
		return errors.New("session not found")
	}
	server, err := session.NewDoTServer(hostname, bootstrapIPs, globalProtector, &jniDoHListener{})
	if err != nil {
		return err
	}
	sess.SetDoTServer(server)
	return nil
}

func ProbeDoH(url, bootstrapIPs string) error {
	server, err := session.NewDoHServer(url, bootstrapIPs, globalProtector, nil)
	if err != nil {
		return err
	}
	return session.ProbeDoH(server)
}

func ProbeDoT(hostname, bootstrapIPs string) error {
	server, err := session.NewDoTServer(hostname, bootstrapIPs, globalProtector, nil)
	if err != nil {
		return err
	}
	return session.ProbeDoT(server)
}
