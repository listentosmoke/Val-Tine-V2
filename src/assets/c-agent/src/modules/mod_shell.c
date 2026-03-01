/*
 * NodePulse Agent - Shell/Terminal Module
 * 
 * Executes shell commands and returns output.
 */

#include "modules.h"
#include "../platform/platform.h"
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

static int shell_init(void) {
    /* No initialization needed */
    return 0;
}

static int shell_execute(const char* command, const char* params, char** output) {
    if (!command || !output) return -1;
    
    /* The command itself is what we execute */
    char* cmd_output = NULL;
    size_t cmd_output_len = 0;
    
    int exit_code = run_command(command, &cmd_output, &cmd_output_len);
    
    /* Build JSON response */
    size_t json_capacity = cmd_output_len + 256;
    *output = (char*)malloc(json_capacity);
    
    /* Escape the output for JSON */
    char* escaped = (char*)malloc(cmd_output_len * 2 + 1);
    char* p = escaped;
    for (size_t i = 0; i < cmd_output_len && cmd_output[i]; i++) {
        char c = cmd_output[i];
        switch (c) {
            case '"':  *p++ = '\\'; *p++ = '"'; break;
            case '\\': *p++ = '\\'; *p++ = '\\'; break;
            case '\n': *p++ = '\\'; *p++ = 'n'; break;
            case '\r': *p++ = '\\'; *p++ = 'r'; break;
            case '\t': *p++ = '\\'; *p++ = 't'; break;
            default:
                if (c >= 32 && c < 127) {
                    *p++ = c;
                }
                break;
        }
    }
    *p = '\0';
    
    snprintf(*output, json_capacity,
             "{\"exit_code\":%d,\"output\":\"%s\"}",
             exit_code, escaped);
    
    free(escaped);
    if (cmd_output) free(cmd_output);
    
    return 0;
}

static void shell_cleanup(void) {
    /* Nothing to clean up */
}

/* Module definition */
Module mod_shell = {
    .name = "terminal",
    .description = "Execute shell commands",
    .init = shell_init,
    .execute = shell_execute,
    .cleanup = shell_cleanup,
    .initialized = 0
};
