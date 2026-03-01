/*
 * NodePulse Agent - File Operations Module
 * 
 * Handles file browsing, reading, writing, and deletion.
 */

#include "modules.h"
#include "../platform/platform.h"
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

static int files_init(void) {
    return 0;
}

/*
 * Commands:
 *   list:<path>     - List directory contents
 *   read:<path>     - Read file contents (base64)
 *   write:<path>    - Write base64 content to file (params = base64 data)
 *   delete:<path>   - Delete a file
 *   exists:<path>   - Check if file exists
 *   mkdir:<path>    - Create directory
 */
static int files_execute(const char* command, const char* params, char** output) {
    if (!command || !output) return -1;
    
    /* Parse command:path format */
    char cmd_type[32] = {0};
    char path[1024] = {0};
    
    const char* colon = strchr(command, ':');
    if (!colon) {
        *output = strdup("{\"error\":\"Invalid command format, expected cmd:path\"}");
        return -1;
    }
    
    size_t cmd_len = colon - command;
    if (cmd_len >= sizeof(cmd_type)) cmd_len = sizeof(cmd_type) - 1;
    strncpy(cmd_type, command, cmd_len);
    strncpy(path, colon + 1, sizeof(path) - 1);
    
    /* Handle commands */
    if (strcmp(cmd_type, "list") == 0) {
        char* listing = NULL;
        if (file_list(path, &listing) == 0) {
            size_t out_len = strlen(listing) + 64;
            *output = (char*)malloc(out_len);
            snprintf(*output, out_len, "{\"files\":%s}", listing);
            free(listing);
            return 0;
        } else {
            *output = strdup("{\"error\":\"Failed to list directory\"}");
            return -1;
        }
    }
    else if (strcmp(cmd_type, "read") == 0) {
        char* content = NULL;
        size_t content_len = 0;
        
        if (file_read(path, &content, &content_len) == 0) {
            /* Base64 encode the content */
            char* b64 = base64_encode((unsigned char*)content, content_len);
            if (b64) {
                size_t out_len = strlen(b64) + 64;
                *output = (char*)malloc(out_len);
                snprintf(*output, out_len, "{\"content\":\"%s\",\"size\":%zu}", b64, content_len);
                free(b64);
            } else {
                *output = strdup("{\"error\":\"Failed to encode file\"}");
            }
            free(content);
            return 0;
        } else {
            *output = strdup("{\"error\":\"Failed to read file\"}");
            return -1;
        }
    }
    else if (strcmp(cmd_type, "write") == 0) {
        if (!params || strlen(params) == 0) {
            *output = strdup("{\"error\":\"No data provided\"}");
            return -1;
        }
        
        /* Decode base64 params */
        size_t decoded_len = 0;
        unsigned char* decoded = base64_decode(params, &decoded_len);
        
        if (decoded && decoded_len > 0) {
            if (file_write(path, (char*)decoded, decoded_len) == 0) {
                *output = (char*)malloc(128);
                snprintf(*output, 128, "{\"success\":true,\"bytes_written\":%zu}", decoded_len);
            } else {
                *output = strdup("{\"error\":\"Failed to write file\"}");
            }
            free(decoded);
        } else {
            /* If base64 decode not implemented, write raw */
            size_t params_len = strlen(params);
            if (file_write(path, params, params_len) == 0) {
                *output = (char*)malloc(128);
                snprintf(*output, 128, "{\"success\":true,\"bytes_written\":%zu}", params_len);
            } else {
                *output = strdup("{\"error\":\"Failed to write file\"}");
            }
        }
        return 0;
    }
    else if (strcmp(cmd_type, "delete") == 0) {
        if (file_delete(path) == 0) {
            *output = strdup("{\"success\":true}");
            return 0;
        } else {
            *output = strdup("{\"error\":\"Failed to delete file\"}");
            return -1;
        }
    }
    else if (strcmp(cmd_type, "exists") == 0) {
        int exists = file_exists(path);
        long size = exists ? file_size(path) : 0;
        *output = (char*)malloc(128);
        snprintf(*output, 128, "{\"exists\":%s,\"size\":%ld}", 
                 exists ? "true" : "false", size);
        return 0;
    }
    else if (strcmp(cmd_type, "mkdir") == 0) {
        if (dir_create(path) == 0) {
            *output = strdup("{\"success\":true}");
            return 0;
        } else {
            *output = strdup("{\"error\":\"Failed to create directory\"}");
            return -1;
        }
    }
    else if (strcmp(cmd_type, "tree") == 0) {
        /* tree command: path format is "actualPath:depth" e.g. "C:\:3" */
        char actual_path[1024] = {0};
        int depth = 3; /* default depth */
        
        /* Find the last colon to split path:depth */
        char* last_colon = strrchr(path, ':');
        if (last_colon && last_colon != path) {
            /* Check if after colon is a number */
            char* endptr;
            long parsed_depth = strtol(last_colon + 1, &endptr, 10);
            if (*endptr == '\0' && parsed_depth > 0 && parsed_depth <= 5) {
                depth = (int)parsed_depth;
                size_t path_len = (size_t)(last_colon - path);
                if (path_len < sizeof(actual_path)) {
                    strncpy(actual_path, path, path_len);
                    actual_path[path_len] = '\0';
                }
            } else {
                /* No valid depth, use full path */
                strncpy(actual_path, path, sizeof(actual_path) - 1);
            }
        } else {
            strncpy(actual_path, path, sizeof(actual_path) - 1);
        }
        
        char* tree_output = NULL;
        if (file_list_recursive(actual_path, depth, &tree_output) == 0) {
            *output = tree_output;
            return 0;
        } else {
            *output = strdup("{\"error\":\"Failed to list directory tree\"}");
            return -1;
        }
    }
    else {
        *output = (char*)malloc(128);
        snprintf(*output, 128, "{\"error\":\"Unknown command: %s\"}", cmd_type);
        return -1;
    }
}

static void files_cleanup(void) {
    /* Nothing to clean up */
}

/* Module definition */
Module mod_files = {
    .name = "files",
    .description = "File operations (read, write, list, delete)",
    .init = files_init,
    .execute = files_execute,
    .cleanup = files_cleanup,
    .initialized = 0
};
