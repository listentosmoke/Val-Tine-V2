/*
 * NodePulse Agent - Environment Variables Module
 *
 * Provides environment variable listing and management.
 * Commands:
 *   - list: List all environment variables
 *   - get:<name>: Get specific environment variable
 *   - search:<pattern>: Search environment variables by name/value
 */

#include "modules.h"
#include "../utils/json.h"
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#ifdef _WIN32
#include <windows.h>
#else
extern char **environ;
#include <ctype.h>
#endif

/* Include safe_string.h last - its portable strcasestr macro must come after
 * system headers that might interfere with the #define on Windows/MinGW */
#include "../utils/safe_string.h"

/* ============================================================================
 * Platform-Specific Implementation
 * ============================================================================ */

#ifdef _WIN32

static int list_environment_win32(char** output, const char* filter) {
    LPCH env_block = GetEnvironmentStrings();
    if (!env_block) {
        *output = safe_strdup("{\"error\":\"Failed to get environment variables\"}");
        return -1;
    }

    JsonValue* arr = json_array();
    LPCH current = env_block;

    while (*current) {
        /* Skip if starts with '=' (hidden variables) */
        if (*current != '=') {
            /* Find the '=' separator */
            char* eq = strchr(current, '=');
            if (eq) {
                size_t name_len = eq - current;
                char* name = (char*)malloc(name_len + 1);
                if (name) {
                    memcpy(name, current, name_len);
                    name[name_len] = '\0';
                    char* value = eq + 1;

                    /* Apply filter if provided */
                    int include = 1;
                    if (filter && strlen(filter) > 0) {
                        include = strcasestr(name, filter) != NULL ||
                                  strcasestr(value, filter) != NULL;
                    }

                    if (include) {
                        JsonValue* env = json_object();
                        json_object_set_string(env, "name", name);
                        json_object_set_string(env, "value", value);
                        json_array_append(arr, env);
                    }

                    free(name);
                }
            }
        }
        /* Move to next variable (null-terminated strings) */
        current += strlen(current) + 1;
    }

    FreeEnvironmentStrings(env_block);

    *output = json_stringify(arr);
    json_free(arr);
    return 0;
}

static int get_environment_win32(const char* name, char** output) {
    char buffer[32767]; /* Max environment variable size on Windows */
    DWORD result = GetEnvironmentVariableA(name, buffer, sizeof(buffer));

    JsonValue* env = json_object();
    json_object_set_string(env, "name", name);

    if (result > 0 && result < sizeof(buffer)) {
        json_object_set_string(env, "value", buffer);
        json_object_set_bool(env, "exists", 1);
    } else {
        json_object_set_string(env, "value", "");
        json_object_set_bool(env, "exists", 0);
    }

    *output = json_stringify(env);
    json_free(env);
    return 0;
}

#else /* Linux/macOS */

static int list_environment_unix(char** output, const char* filter) {
    JsonValue* arr = json_array();

    if (environ) {
        for (char** env = environ; *env != NULL; env++) {
            char* eq = strchr(*env, '=');
            if (eq) {
                size_t name_len = eq - *env;
                char* name = (char*)malloc(name_len + 1);
                if (name) {
                    memcpy(name, *env, name_len);
                    name[name_len] = '\0';
                    char* value = eq + 1;

                    /* Apply filter if provided */
                    int include = 1;
                    if (filter && strlen(filter) > 0) {
                        include = strcasestr(name, filter) != NULL ||
                                  strcasestr(value, filter) != NULL;
                    }

                    if (include) {
                        JsonValue* env_obj = json_object();
                        json_object_set_string(env_obj, "name", name);
                        json_object_set_string(env_obj, "value", value);
                        json_array_append(arr, env_obj);
                    }

                    free(name);
                }
            }
        }
    }

    *output = json_stringify(arr);
    json_free(arr);
    return 0;
}

static int get_environment_unix(const char* name, char** output) {
    const char* value = getenv(name);

    JsonValue* env = json_object();
    json_object_set_string(env, "name", name);

    if (value) {
        json_object_set_string(env, "value", value);
        json_object_set_bool(env, "exists", 1);
    } else {
        json_object_set_string(env, "value", "");
        json_object_set_bool(env, "exists", 0);
    }

    *output = json_stringify(env);
    json_free(env);
    return 0;
}

#endif

/* ============================================================================
 * Module Interface Implementation
 * ============================================================================ */

static int environment_init(void) {
    return 0;
}

static int environment_execute(const char* command, const char* params, char** output) {
    (void)params;

    if (!output) return -1;

    if (!command || strcmp(command, "list") == 0) {
#ifdef _WIN32
        return list_environment_win32(output, NULL);
#else
        return list_environment_unix(output, NULL);
#endif
    }

    if (strncmp(command, "get:", 4) == 0) {
        const char* name = command + 4;
        if (!name || strlen(name) == 0) {
            *output = safe_strdup("{\"error\":\"Variable name required\"}");
            return -1;
        }
#ifdef _WIN32
        return get_environment_win32(name, output);
#else
        return get_environment_unix(name, output);
#endif
    }

    if (strncmp(command, "search:", 7) == 0) {
        const char* filter = command + 7;
#ifdef _WIN32
        return list_environment_win32(output, filter);
#else
        return list_environment_unix(output, filter);
#endif
    }

    *output = safe_strdup("{\"error\":\"Unknown command. Use: list, get:<name>, search:<pattern>\"}");
    return -1;
}

static void environment_cleanup(void) {
    /* Nothing to clean up */
}

/* Module definition */
Module mod_environment = {
    .name = "environment",
    .description = "Environment variable management",
    .init = environment_init,
    .execute = environment_execute,
    .cleanup = environment_cleanup,
    .initialized = 0
};
