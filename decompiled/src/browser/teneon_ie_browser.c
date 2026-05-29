/*
 * TeneonIERelease.dll browser glue.
 *
 * Embedded IE wrapper; browser navigation only, not LSX score upload.
 */
#include "common/win32_recovered_types.h"

typedef int (__thiscall *BrowserVtableCall0)(void *self);
typedef int (__thiscall *BrowserNavigate2)(void *self, char *url,
    void *flags, void *target_frame, void *post_data, void *headers);
typedef int (__thiscall *BrowserPutVisible)(void *self, int visible);

struct IEBrowserContainer {
    void *vtable;                  /* +0x00 */
    u8 unknown_0004[8];
    void *browser;                 /* +0x0C, COM wrapper/vtable owner */
    Lt2StdString *cached_url;      /* +0x10 */
};

static void **browser_vtable(IEBrowserContainer *self)
{
    return self != NULL && self->browser != NULL ? *(void ***)self->browser : NULL;
}

/*
 * Export: ?LoadURL@IEBrowserContainer@@UAE?AW4Results@1@...
 *
 * Navigate2 receives only the URL; flags, target frame, post data, and headers
 * are zero.
 */
Lt2BrowserResult IEBrowserContainer_LoadURL(IEBrowserContainer *self,
    Lt2StdString *url)
{
    void **vt;
    char *url_data;
    int hr;

    if (self == NULL || self->browser == NULL || url == NULL) {
        return LT2_BROWSER_NOT_READY;
    }

    vt = browser_vtable(self);
    if (vt == NULL) {
        return LT2_BROWSER_NOT_READY;
    }

    /* Empty/default URL check against DAT_1004FAC8. */
    if (url->size == 0 || url->data == NULL) {
        return LT2_BROWSER_NOT_READY;
    }

    /*
     * Vtable offsets from Ghidra:
     * - +0x13C: query/ensure browser state
     * - +0x140: set visible/state flag with argument 1 when state query returns 0
     * - +0x0C4: Navigate2(url, 0, 0, 0, 0)
     */
    hr = ((BrowserVtableCall0)vt[0x13c / 4])(self->browser);
    if (hr == 0) {
        ((BrowserPutVisible)vt[0x140 / 4])(self->browser, 1);
    }

    url_data = url->data != NULL ? url->data : "";
    hr = ((BrowserNavigate2)vt[0x0c4 / 4])(self->browser,
        url_data, NULL, NULL, NULL, NULL);

    if (hr == 0) {
        return LT2_BROWSER_OK;
    }
    if (hr == (int)0x80004005) {
        return LT2_BROWSER_NOT_READY;
    }
    if (hr == (int)0x8007000e) {
        return LT2_BROWSER_NAVIGATE_BLOCKED;
    }
    if (hr == (int)0x80070057) {
        return LT2_BROWSER_INVALID_ARGUMENT;
    }
    return LT2_BROWSER_FAILED;
}

/*
 * Export: ?Refresh@IEBrowserContainer@@UAE?AW4Results@1@XZ
 */
Lt2BrowserResult IEBrowserContainer_Refresh(IEBrowserContainer *self)
{
    void **vt;
    void *host_window;
    int hr;

    if (self == NULL || self->browser == NULL) {
        return LT2_BROWSER_NOT_READY;
    }

    host_window = *(void **)((u8 *)self->browser + 0x1C);
    if (!IsWindowVisible((HANDLE)host_window)) {
        return LT2_BROWSER_NOT_READY;
    }

    vt = browser_vtable(self);
    if (vt == NULL) {
        return LT2_BROWSER_NOT_READY;
    }

    /* Vtable offset +0x0C0 is Refresh. */
    hr = ((BrowserVtableCall0)vt[0x0c0 / 4])(self->browser);
    if (hr == (int)0x80004005) {
        return LT2_BROWSER_NOT_READY;
    }
    return hr == 0 ? LT2_BROWSER_OK : LT2_BROWSER_FAILED;
}
