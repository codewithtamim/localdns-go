# liblocaldns-go

Android native library that routes DNS through an encrypted resolver over a VPN TUN interface. Supports DNS-over-HTTPS (DoH) and DNS-over-TLS (DoT).

Built with Go (CGO) and a thin JNI bridge. Produces `liblocaldns-go.so`.

## How it works

1. The app opens a TUN fd via `VpnService` and passes it to the library.
2. DNS queries from the tunnel are forwarded to the configured DoH/DoT server.
3. Outbound sockets are protected through JNI so they bypass the VPN tunnel.
4. DNS query stats are reported back to Java/Kotlin via callbacks.

## Android integration

Load `liblocaldns-go.so` and call through `org.thebytearray.localdns.tunnel.LocalDnsBackend`:

- `nativeRegisterCallbacks()` — wire up socket protection, resolver discovery, and DNS stats
- `nativeConnectDoH` / `nativeConnectDoT` — start a session
- `nativeSetDoHServer` / `nativeSetDoTServer` — switch resolver at runtime
- `nativeProbeDoH` / `nativeProbeDoT` — test connectivity
- `nativeDisconnect` — tear down a session

## Build

Requires the Android NDK toolchain. From this directory:

```sh
make ANDROID_ARCH_NAME=arm64 \
     TARGET=aarch64-linux-android21 \
     SYSROOT=$NDK/toolchains/llvm/prebuilt/darwin-x86_64/sysroot \
     CC=$NDK/toolchains/llvm/prebuilt/darwin-x86_64/bin/aarch64-linux-android21-clang \
     CFLAGS= LDFLAGS= DESTDIR=./out
```

Output: `out/liblocaldns-go.so`

If Go 1.25.5 is not installed, the Makefile downloads it automatically.