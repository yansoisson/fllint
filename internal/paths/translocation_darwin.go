//go:build darwin

package paths

/*
#cgo LDFLAGS: -framework CoreFoundation -ldl
#include <CoreFoundation/CoreFoundation.h>
#include <dlfcn.h>
#include <stdlib.h>

// SecTranslocate functions are in the Security framework but not in public headers.
// We load them dynamically to avoid build issues across SDK versions.
typedef Boolean (*IsTranslocatedFunc)(CFURLRef, bool*, CFErrorRef*);
typedef CFURLRef (*OriginalPathFunc)(CFURLRef, CFErrorRef*);

static char* resolveTranslocation(const char* path) {
	// Dynamically load the Security framework
	void* handle = dlopen("/System/Library/Frameworks/Security.framework/Security", RTLD_LAZY);
	if (!handle) return NULL;

	IsTranslocatedFunc isTranslocated =
		(IsTranslocatedFunc)dlsym(handle, "SecTranslocateIsTranslocatedURL");
	OriginalPathFunc originalPath =
		(OriginalPathFunc)dlsym(handle, "SecTranslocateCreateOriginalPathForURL");

	if (!isTranslocated || !originalPath) {
		dlclose(handle);
		return NULL;
	}

	CFStringRef cfPath = CFStringCreateWithCString(NULL, path, kCFStringEncodingUTF8);
	if (!cfPath) { dlclose(handle); return NULL; }

	CFURLRef url = CFURLCreateWithFileSystemPath(NULL, cfPath, kCFURLPOSIXPathStyle, true);
	CFRelease(cfPath);
	if (!url) { dlclose(handle); return NULL; }

	bool translocated = false;
	CFErrorRef error = NULL;

	if (!isTranslocated(url, &translocated, &error) || !translocated) {
		CFRelease(url);
		if (error) CFRelease(error);
		dlclose(handle);
		return NULL;
	}

	error = NULL;
	CFURLRef originalURL = originalPath(url, &error);
	CFRelease(url);

	if (!originalURL) {
		if (error) CFRelease(error);
		dlclose(handle);
		return NULL;
	}

	CFStringRef origStr = CFURLCopyFileSystemPath(originalURL, kCFURLPOSIXPathStyle);
	CFRelease(originalURL);
	if (!origStr) { dlclose(handle); return NULL; }

	CFIndex len = CFStringGetMaximumSizeForEncoding(CFStringGetLength(origStr), kCFStringEncodingUTF8) + 1;
	char* result = (char*)malloc(len);
	if (!CFStringGetCString(origStr, result, len, kCFStringEncodingUTF8)) {
		free(result);
		CFRelease(origStr);
		dlclose(handle);
		return NULL;
	}

	CFRelease(origStr);
	dlclose(handle);
	return result;
}
*/
import "C"
import "unsafe"

// resolveTranslocatedPath returns the original path of a macOS translocated .app.
// When macOS runs a quarantined app, it copies it to a temporary read-only path
// (App Translocation). This function resolves back to the original location
// so that the Data/ folder can be found alongside the .app on disk.
// Returns empty string if the path is not translocated.
func resolveTranslocatedPath(appPath string) string {
	cPath := C.CString(appPath)
	defer C.free(unsafe.Pointer(cPath))

	result := C.resolveTranslocation(cPath)
	if result == nil {
		return ""
	}
	defer C.free(unsafe.Pointer(result))
	return C.GoString(result)
}
