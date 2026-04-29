//go:build darwin && cgo

// wifi-app-mac-helper is an *optional* helper binary that retrieves raw
// 802.11 Information Element bytes from the system WiFi cache on macOS via
// Apple's private Apple80211.framework. It exists because Apple's public
// CoreWLAN API does not expose the IEs needed to populate fields like
// BSSColor, BSSLoad, 802.11k/v/r support, DTIM period, WPS, MaxPhyRate, etc.
//
// Contract:
//
//	stdin:  unused
//	stdout: JSON object {"records":[{"bssid":"aa:bb:cc:dd:ee:ff","ssid":"...","ie_hex":"45..."}, ...]}
//	stderr: human-readable error log
//	exit:   0 on success, non-zero on error (with stderr explaining)
//
// Loading is via dlopen so the main app is not statically linked against
// private symbols — if Apple removes or changes them, the helper just fails
// gracefully and the main app continues with its CoreWLAN-only data.
//
// Untested in this repo (no macOS CI). Treat the dlopen calls as a starting
// point; expect to refine once you can run on a real Sonoma/Sequoia install.
package main

/*
#cgo LDFLAGS: -framework CoreFoundation -framework Foundation -ldl
#cgo CFLAGS: -x objective-c -fobjc-arc

#import <Foundation/Foundation.h>
#import <CoreFoundation/CoreFoundation.h>
#import <dlfcn.h>
#import <stdlib.h>
#import <string.h>

// Reverse-engineered Apple80211 framework signatures. Public references:
//   - https://opensource.apple.com (intermittent; many Apple80211 headers
//     have appeared and disappeared across releases)
//   - third-party headers shipped with WirelessDiagnostics analyses
//
// These calls are ABI-unstable. Each macOS release tweaks something. We
// dlopen rather than link so a runtime miss is recoverable.
typedef int (*apple80211_open_fn)(void **ctx);
typedef int (*apple80211_close_fn)(void *ctx);
typedef int (*apple80211_bind_fn)(void *ctx, CFStringRef ifaceName);
typedef int (*apple80211_scan_fn)(void *ctx, CFArrayRef *results, CFDictionaryRef params);
typedef int (*apple80211_get_cached_fn)(void *ctx, CFArrayRef *results);

static void *g_handle = NULL;
static apple80211_open_fn g_open = NULL;
static apple80211_close_fn g_close = NULL;
static apple80211_bind_fn g_bind = NULL;
static apple80211_scan_fn g_scan = NULL;
static apple80211_get_cached_fn g_cached = NULL;

// helper_load attempts to dlopen the Apple80211 private framework and resolve
// the symbols we need. Returns 1 on success, 0 on failure (with a reason
// written to stderr by the caller).
static int helper_load(void) {
	if (g_handle) {
		return 1;
	}
	g_handle = dlopen("/System/Library/PrivateFrameworks/Apple80211.framework/Apple80211", RTLD_LAZY);
	if (!g_handle) {
		return 0;
	}
	g_open    = (apple80211_open_fn)       dlsym(g_handle, "Apple80211Open");
	g_close   = (apple80211_close_fn)      dlsym(g_handle, "Apple80211Close");
	g_bind    = (apple80211_bind_fn)       dlsym(g_handle, "Apple80211BindToInterface");
	g_scan    = (apple80211_scan_fn)       dlsym(g_handle, "Apple80211Scan");
	g_cached  = (apple80211_get_cached_fn) dlsym(g_handle, "Apple80211GetCachedScanResults");
	if (!g_open || !g_close || !g_bind) {
		return 0;
	}
	return 1;
}

// helper_collect_json drives one scan, walks the resulting CFArray of
// CFDictionary entries, and serialises {BSSID, SSID_STR, IE} into JSON. The
// CFData under "IE" is the concatenated TLV stream of beacon information
// elements — exactly what parseInformationElements in the main app expects.
//
// Returns a newly-allocated UTF-8 string the caller must free with
// helper_free_string. NULL on failure.
static char *helper_collect_json(const char *iface, char *err, size_t errLen) {
	@autoreleasepool {
		if (!helper_load()) {
			snprintf(err, errLen, "Apple80211 framework unavailable: %s", dlerror());
			return NULL;
		}

		void *ctx = NULL;
		int rc = g_open(&ctx);
		if (rc != 0 || ctx == NULL) {
			snprintf(err, errLen, "Apple80211Open failed (rc=%d)", rc);
			return NULL;
		}

		CFStringRef ifaceCF = CFStringCreateWithCString(NULL, iface, kCFStringEncodingUTF8);
		rc = g_bind(ctx, ifaceCF);
		CFRelease(ifaceCF);
		if (rc != 0) {
			g_close(ctx);
			snprintf(err, errLen, "Apple80211BindToInterface failed (rc=%d)", rc);
			return NULL;
		}

		CFArrayRef results = NULL;
		// Try a fresh scan first; fall back to the cache if the explicit scan
		// call is missing on this macOS version.
		if (g_scan) {
			rc = g_scan(ctx, &results, NULL);
		} else if (g_cached) {
			rc = g_cached(ctx, &results);
		} else {
			rc = -1;
		}
		if (rc != 0 || results == NULL) {
			g_close(ctx);
			snprintf(err, errLen, "Apple80211Scan/GetCachedScanResults failed (rc=%d)", rc);
			return NULL;
		}

		NSMutableArray *records = [NSMutableArray array];
		CFIndex count = CFArrayGetCount(results);
		for (CFIndex i = 0; i < count; i++) {
			CFDictionaryRef entry = (CFDictionaryRef)CFArrayGetValueAtIndex(results, i);
			if (!entry || CFGetTypeID(entry) != CFDictionaryGetTypeID()) {
				continue;
			}
			NSDictionary *dict = (__bridge NSDictionary *)entry;

			NSString *ssid = dict[@"SSID_STR"];
			if (!ssid) ssid = dict[@"SSID"];
			id bssidObj = dict[@"BSSID"];
			NSString *bssid = nil;
			if ([bssidObj isKindOfClass:[NSString class]]) {
				bssid = (NSString *)bssidObj;
			} else if ([bssidObj isKindOfClass:[NSData class]]) {
				NSData *d = (NSData *)bssidObj;
				if (d.length == 6) {
					const uint8_t *b = d.bytes;
					bssid = [NSString stringWithFormat:@"%02x:%02x:%02x:%02x:%02x:%02x",
					         b[0], b[1], b[2], b[3], b[4], b[5]];
				}
			}
			if (!bssid) {
				continue;
			}

			NSData *ieData = dict[@"IE"];
			NSString *ieHex = @"";
			if ([ieData isKindOfClass:[NSData class]] && ieData.length > 0) {
				NSMutableString *s = [NSMutableString stringWithCapacity:ieData.length * 2];
				const uint8_t *b = ieData.bytes;
				for (NSUInteger j = 0; j < ieData.length; j++) {
					[s appendFormat:@"%02x", b[j]];
				}
				ieHex = s;
			}

			NSMutableDictionary *out = [NSMutableDictionary dictionary];
			out[@"bssid"]  = [bssid lowercaseString];
			out[@"ssid"]   = ssid ? ssid : @"";
			out[@"ie_hex"] = ieHex;
			[records addObject:out];
		}

		CFRelease(results);
		g_close(ctx);

		NSDictionary *root = @{@"records": records};
		NSError *jsonErr = nil;
		NSData *json = [NSJSONSerialization dataWithJSONObject:root options:0 error:&jsonErr];
		if (!json) {
			snprintf(err, errLen, "JSON serialisation failed: %s",
			         jsonErr.localizedDescription.UTF8String);
			return NULL;
		}
		NSString *out = [[NSString alloc] initWithData:json encoding:NSUTF8StringEncoding];
		return strdup([out UTF8String]);
	}
}

static void helper_free_string(char *s) {
	if (s) free(s);
}
*/
import "C"

import (
	"flag"
	"fmt"
	"os"
	"unsafe"
)

func main() {
	var iface string
	flag.StringVar(&iface, "iface", "en0", "WiFi interface to bind to")
	flag.Parse()

	cIface := C.CString(iface)
	defer C.free(unsafe.Pointer(cIface))

	errBuf := make([]byte, 256)
	cErr := (*C.char)(unsafe.Pointer(&errBuf[0]))
	cJSON := C.helper_collect_json(cIface, cErr, C.size_t(len(errBuf)))
	if cJSON == nil {
		msg := C.GoString(cErr)
		if msg == "" {
			msg = "unknown error"
		}
		fmt.Fprintln(os.Stderr, msg)
		os.Exit(1)
	}
	defer C.helper_free_string(cJSON)

	fmt.Println(C.GoString(cJSON))
}
