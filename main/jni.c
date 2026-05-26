#include <jni.h>
#include <stdlib.h>
#include <string.h>
#include <stdint.h>

extern int localdnsConnectDoH(int tun_fd, char* fake_dns, char* url, char* bootstrap_ips, int* handle_out);
extern int localdnsConnectDoT(int tun_fd, char* fake_dns, char* hostname, char* bootstrap_ips, int* handle_out);
extern void localdnsDisconnect(int handle);
extern int localdnsSetDoHServer(int handle, char* url, char* bootstrap_ips);
extern int localdnsSetDoTServer(int handle, char* hostname, char* bootstrap_ips);
extern int localdnsProbeDoH(char* url, char* bootstrap_ips);
extern int localdnsProbeDoT(char* hostname, char* bootstrap_ips);
extern char* localdnsVersion(void);

static JavaVM *g_jvm = NULL;
static jobject g_backend = NULL;
static jmethodID g_protect_method = NULL;
static jmethodID g_get_resolvers_method = NULL;
static jmethodID g_on_dns_query_method = NULL;

static JNIEnv* get_env(int *needs_detach) {
    JNIEnv *env = NULL;
    *needs_detach = 0;
    if ((*g_jvm)->GetEnv(g_jvm, (void**)&env, JNI_VERSION_1_6) != JNI_OK) {
        if ((*g_jvm)->AttachCurrentThread(g_jvm, &env, NULL) != 0) {
            return NULL;
        }
        *needs_detach = 1;
    }
    return env;
}

int localdns_jni_protect(int fd) {
    if (g_jvm == NULL || g_backend == NULL || g_protect_method == NULL) {
        return 0;
    }
    int needs_detach = 0;
    JNIEnv *env = get_env(&needs_detach);
    if (env == NULL) {
        return 0;
    }
    jboolean result = (*env)->CallBooleanMethod(env, g_backend, g_protect_method, fd);
    if ((*env)->ExceptionCheck(env)) {
        (*env)->ExceptionClear(env);
        result = JNI_FALSE;
    }
    if (needs_detach) {
        (*g_jvm)->DetachCurrentThread(g_jvm);
    }
    return result ? 1 : 0;
}

char* localdns_jni_get_resolvers(void) {
    if (g_jvm == NULL || g_backend == NULL || g_get_resolvers_method == NULL) {
        return strdup("");
    }
    int needs_detach = 0;
    JNIEnv *env = get_env(&needs_detach);
    if (env == NULL) {
        return strdup("");
    }
    jstring jresolvers = (jstring)(*env)->CallObjectMethod(env, g_backend, g_get_resolvers_method);
    if ((*env)->ExceptionCheck(env)) {
        (*env)->ExceptionClear(env);
        if (needs_detach) {
            (*g_jvm)->DetachCurrentThread(g_jvm);
        }
        return strdup("");
    }
    const char *resolvers = (*env)->GetStringUTFChars(env, jresolvers, NULL);
    char *copy = strdup(resolvers);
    (*env)->ReleaseStringUTFChars(env, jresolvers, resolvers);
    (*env)->DeleteLocalRef(env, jresolvers);
    if (needs_detach) {
        (*g_jvm)->DetachCurrentThread(g_jvm);
    }
    return copy;
}

void localdns_jni_on_dns_query(const char* hostname, int64_t latency_ms, int status) {
    if (g_jvm == NULL || g_backend == NULL || g_on_dns_query_method == NULL) {
        return;
    }
    int needs_detach = 0;
    JNIEnv *env = get_env(&needs_detach);
    if (env == NULL) {
        return;
    }
    jstring jhostname = (*env)->NewStringUTF(env, hostname != NULL ? hostname : "");
    (*env)->CallVoidMethod(env, g_backend, g_on_dns_query_method, jhostname, (jlong)latency_ms, status);
    if ((*env)->ExceptionCheck(env)) {
        (*env)->ExceptionClear(env);
    }
    (*env)->DeleteLocalRef(env, jhostname);
    if (needs_detach) {
        (*g_jvm)->DetachCurrentThread(g_jvm);
    }
}

static char* jstring_to_c(JNIEnv *env, jstring str) {
    if (str == NULL) {
        return strdup("");
    }
    const char *utf = (*env)->GetStringUTFChars(env, str, NULL);
    char *copy = strdup(utf);
    (*env)->ReleaseStringUTFChars(env, str, utf);
    return copy;
}

JNIEXPORT void JNICALL
Java_org_thebytearray_localdns_tunnel_LocalDnsBackend_nativeRegisterCallbacks(JNIEnv *env, jobject thiz) {
    (*env)->GetJavaVM(env, &g_jvm);
    if (g_backend != NULL) {
        (*env)->DeleteGlobalRef(env, g_backend);
    }
    g_backend = (*env)->NewGlobalRef(env, thiz);
    jclass clazz = (*env)->GetObjectClass(env, thiz);
    g_protect_method = (*env)->GetMethodID(env, clazz, "protectSocket", "(I)Z");
    g_get_resolvers_method = (*env)->GetMethodID(env, clazz, "getSystemResolvers", "()Ljava/lang/String;");
    g_on_dns_query_method = (*env)->GetMethodID(env, clazz, "onDnsQuery", "(Ljava/lang/String;JI)V");
}

JNIEXPORT void JNICALL
Java_org_thebytearray_localdns_tunnel_LocalDnsBackend_nativeUnregisterCallbacks(JNIEnv *env, jobject thiz) {
    if (g_backend != NULL) {
        (*env)->DeleteGlobalRef(env, g_backend);
        g_backend = NULL;
    }
}

JNIEXPORT jint JNICALL
Java_org_thebytearray_localdns_tunnel_LocalDnsBackend_nativeConnectDoH(
        JNIEnv *env, jobject thiz, jint tun_fd, jstring fake_dns, jstring url, jstring bootstrap_ips) {
    char *fake_dns_c = jstring_to_c(env, fake_dns);
    char *url_c = jstring_to_c(env, url);
    char *bootstrap_c = jstring_to_c(env, bootstrap_ips);
    int handle = 0;
    int result = localdnsConnectDoH(tun_fd, fake_dns_c, url_c, bootstrap_c, &handle);
    free(fake_dns_c);
    free(url_c);
    free(bootstrap_c);
    if (result != 0) {
        return -1;
    }
    return handle;
}

JNIEXPORT jint JNICALL
Java_org_thebytearray_localdns_tunnel_LocalDnsBackend_nativeConnectDoT(
        JNIEnv *env, jobject thiz, jint tun_fd, jstring fake_dns, jstring hostname, jstring bootstrap_ips) {
    char *fake_dns_c = jstring_to_c(env, fake_dns);
    char *hostname_c = jstring_to_c(env, hostname);
    char *bootstrap_c = jstring_to_c(env, bootstrap_ips);
    int handle = 0;
    int result = localdnsConnectDoT(tun_fd, fake_dns_c, hostname_c, bootstrap_c, &handle);
    free(fake_dns_c);
    free(hostname_c);
    free(bootstrap_c);
    if (result != 0) {
        return -1;
    }
    return handle;
}

JNIEXPORT void JNICALL
Java_org_thebytearray_localdns_tunnel_LocalDnsBackend_nativeDisconnect(JNIEnv *env, jobject thiz, jint handle) {
    localdnsDisconnect(handle);
}

JNIEXPORT jint JNICALL
Java_org_thebytearray_localdns_tunnel_LocalDnsBackend_nativeSetDoHServer(
        JNIEnv *env, jobject thiz, jint handle, jstring url, jstring bootstrap_ips) {
    char *url_c = jstring_to_c(env, url);
    char *bootstrap_c = jstring_to_c(env, bootstrap_ips);
    int result = localdnsSetDoHServer(handle, url_c, bootstrap_c);
    free(url_c);
    free(bootstrap_c);
    return result;
}

JNIEXPORT jint JNICALL
Java_org_thebytearray_localdns_tunnel_LocalDnsBackend_nativeSetDoTServer(
        JNIEnv *env, jobject thiz, jint handle, jstring hostname, jstring bootstrap_ips) {
    char *hostname_c = jstring_to_c(env, hostname);
    char *bootstrap_c = jstring_to_c(env, bootstrap_ips);
    int result = localdnsSetDoTServer(handle, hostname_c, bootstrap_c);
    free(hostname_c);
    free(bootstrap_c);
    return result;
}

JNIEXPORT jboolean JNICALL
Java_org_thebytearray_localdns_tunnel_LocalDnsBackend_nativeProbeDoH(
        JNIEnv *env, jobject thiz, jstring url, jstring bootstrap_ips) {
    char *url_c = jstring_to_c(env, url);
    char *bootstrap_c = jstring_to_c(env, bootstrap_ips);
    int result = localdnsProbeDoH(url_c, bootstrap_c);
    free(url_c);
    free(bootstrap_c);
    return result == 0 ? JNI_TRUE : JNI_FALSE;
}

JNIEXPORT jboolean JNICALL
Java_org_thebytearray_localdns_tunnel_LocalDnsBackend_nativeProbeDoT(
        JNIEnv *env, jobject thiz, jstring hostname, jstring bootstrap_ips) {
    char *hostname_c = jstring_to_c(env, hostname);
    char *bootstrap_c = jstring_to_c(env, bootstrap_ips);
    int result = localdnsProbeDoT(hostname_c, bootstrap_c);
    free(hostname_c);
    free(bootstrap_c);
    return result == 0 ? JNI_TRUE : JNI_FALSE;
}

JNIEXPORT jstring JNICALL
Java_org_thebytearray_localdns_tunnel_LocalDnsBackend_nativeVersion(JNIEnv *env, jobject thiz) {
    char *version = localdnsVersion();
    if (version == NULL) {
        return NULL;
    }
    jstring result = (*env)->NewStringUTF(env, version);
    free(version);
    return result;
}
