/*
 * NodePulse Agent - Webcam Capture Module
 * 
 * Captures a single frame from the default webcam and returns it as base64 JPEG.
 * 
 * Windows: Uses DirectShow via COM
 * macOS: Uses AVFoundation (stub for now)
 * Linux: Uses V4L2 (stub for now)
 */

#include "modules.h"
#include "../platform/platform.h"
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#ifdef _WIN32
#define WIN32_LEAN_AND_MEAN
#include <windows.h>
#include <dshow.h>
#include <ole2.h>

/* COM interface GUIDs */
DEFINE_GUID(CLSID_VideoCapture, 0x17CCA71B, 0xECD7, 0x11D0, 0xB9, 0x08, 0x00, 0xA0, 0xC9, 0x22, 0x31, 0x96);
DEFINE_GUID(CLSID_SampleGrabber, 0xC1F400A0, 0x3F08, 0x11D3, 0x9F, 0x0B, 0x00, 0x60, 0x08, 0x03, 0x9E, 0x37);
DEFINE_GUID(CLSID_NullRenderer, 0xC1F400A4, 0x3F08, 0x11D3, 0x9F, 0x0B, 0x00, 0x60, 0x08, 0x03, 0x9E, 0x37);
DEFINE_GUID(CLSID_FilterGraph, 0xe436ebb3, 0x524f, 0x11ce, 0x9f, 0x53, 0x00, 0x20, 0xaf, 0x0b, 0xa7, 0x70);
DEFINE_GUID(CLSID_SystemDeviceEnum, 0x62BE5D10, 0x60EB, 0x11D0, 0xBD, 0x3B, 0x00, 0xA0, 0xC9, 0x11, 0xCE, 0x86);
DEFINE_GUID(CLSID_VideoInputDeviceCategory, 0x860BB310, 0x5D01, 0x11D0, 0xBD, 0x3B, 0x00, 0xA0, 0xC9, 0x11, 0xCE, 0x86);

static int g_com_initialized = 0;

static int webcam_init(void) {
    if (!g_com_initialized) {
        HRESULT hr = CoInitializeEx(NULL, COINIT_MULTITHREADED);
        if (SUCCEEDED(hr) || hr == S_FALSE) {
            g_com_initialized = 1;
            return 0;
        }
        return -1;
    }
    return 0;
}

static void webcam_cleanup(void) {
    if (g_com_initialized) {
        CoUninitialize();
        g_com_initialized = 0;
    }
}

/* Simple stub implementation - actual DirectShow code is complex */
static int webcam_execute(const char* command, const char* params, char** output) {
    (void)command;
    (void)params;
    
    if (!output) return -1;
    
    /* For now, return an error message indicating webcam capture requires
     * additional setup. A full implementation would use DirectShow or
     * Media Foundation to capture from the webcam.
     */
    
    *output = (char*)malloc(256);
    snprintf(*output, 256, 
        "{\"success\":false,\"error\":\"Webcam capture not yet implemented on this platform\"}");
    
    return 0;
}

#else /* macOS / Linux */

static int webcam_init(void) {
    return 0;
}

static void webcam_cleanup(void) {
}

static int webcam_execute(const char* command, const char* params, char** output) {
    (void)command;
    (void)params;
    
    if (!output) return -1;
    
    *output = (char*)malloc(256);
    snprintf(*output, 256, 
        "{\"success\":false,\"error\":\"Webcam capture not available on this platform\"}");
    
    return 0;
}

#endif

/* Module definition */
Module mod_webcam = {
    .name = "webcam",
    .description = "Capture webcam image",
    .init = webcam_init,
    .execute = webcam_execute,
    .cleanup = webcam_cleanup,
    .initialized = 0
};
