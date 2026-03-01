/*
 * NodePulse Agent - Screenshot Module
 * 
 * Captures screen and returns base64-encoded image.
 */

#include "modules.h"
#include "../platform/platform.h"
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

static int screenshot_init(void) {
    return 0;
}

static int screenshot_execute(const char* command, const char* params, char** output) {
    (void)command;
    (void)params;
    
    if (!output) return -1;
    
    char* base64_data = NULL;
    
    if (capture_screenshot(&base64_data) == 0 && base64_data) {
        size_t b64_len = strlen(base64_data);
        size_t out_len = b64_len + 128;
        *output = (char*)malloc(out_len);
        snprintf(*output, out_len, 
                 "{\"success\":true,\"format\":\"bmp\",\"size\":%zu,\"data\":\"%s\"}",
                 b64_len, base64_data);
        free(base64_data);
        return 0;
    } else {
        *output = strdup("{\"success\":false,\"error\":\"Failed to capture screenshot\"}");
        return -1;
    }
}

static void screenshot_cleanup(void) {
    /* Nothing to clean up */
}

/* Module definition */
Module mod_screenshot = {
    .name = "screenshot",
    .description = "Capture screen content",
    .init = screenshot_init,
    .execute = screenshot_execute,
    .cleanup = screenshot_cleanup,
    .initialized = 0
};
