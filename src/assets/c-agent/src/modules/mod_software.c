/*
 * NodePulse Agent - Software Inventory Module
 *
 * Lists installed applications on the system.
 * Commands:
 *   - list: List all installed software
 *   - search:<query>: Search for software by name
 */

#include "modules.h"
#include "../platform/platform.h"
#include "../utils/json.h"
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#ifdef _WIN32
#include <windows.h>
#endif

/* Include safe_string.h last - its portable strcasestr macro must come after
 * system headers that might interfere with the #define on Windows/MinGW */
#include "../utils/safe_string.h"

/* ============================================================================
 * Platform-Specific Implementation
 * ============================================================================ */

#ifdef _WIN32

static int get_installed_software_win32(char** output) {
    StringBuffer sb;
    if (strbuf_init(&sb, 8192) != 0) {
        return -1;
    }

    strbuf_append(&sb, "[");
    int first = 1;

    /* Query both 64-bit and 32-bit registry keys */
    const char* reg_paths[] = {
        "SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\Uninstall",
        "SOFTWARE\\WOW6432Node\\Microsoft\\Windows\\CurrentVersion\\Uninstall"
    };

    for (int path_idx = 0; path_idx < 2; path_idx++) {
        HKEY hKey;
        LONG result = RegOpenKeyExA(HKEY_LOCAL_MACHINE, reg_paths[path_idx],
                                     0, KEY_READ | KEY_WOW64_64KEY, &hKey);
        if (result != ERROR_SUCCESS) continue;

        DWORD index = 0;
        char subkey_name[256];
        DWORD subkey_len = sizeof(subkey_name);

        while (RegEnumKeyExA(hKey, index++, subkey_name, &subkey_len,
                             NULL, NULL, NULL, NULL) == ERROR_SUCCESS) {
            HKEY hSubKey;
            char full_path[512];
            safe_snprintf(full_path, sizeof(full_path), "%s\\%s",
                         reg_paths[path_idx], subkey_name);

            if (RegOpenKeyExA(HKEY_LOCAL_MACHINE, full_path, 0,
                             KEY_READ | KEY_WOW64_64KEY, &hSubKey) == ERROR_SUCCESS) {
                char display_name[256] = {0};
                char display_version[64] = {0};
                char publisher[256] = {0};
                char install_date[32] = {0};
                DWORD size;

                size = sizeof(display_name);
                RegQueryValueExA(hSubKey, "DisplayName", NULL, NULL,
                                (LPBYTE)display_name, &size);

                if (strlen(display_name) > 0) {
                    size = sizeof(display_version);
                    RegQueryValueExA(hSubKey, "DisplayVersion", NULL, NULL,
                                    (LPBYTE)display_version, &size);

                    size = sizeof(publisher);
                    RegQueryValueExA(hSubKey, "Publisher", NULL, NULL,
                                    (LPBYTE)publisher, &size);

                    size = sizeof(install_date);
                    RegQueryValueExA(hSubKey, "InstallDate", NULL, NULL,
                                    (LPBYTE)install_date, &size);

                    /* Build JSON entry */
                    JsonValue* entry = json_object();
                    json_object_set_string(entry, "name", display_name);
                    json_object_set_string(entry, "version", display_version);
                    json_object_set_string(entry, "publisher", publisher);
                    json_object_set_string(entry, "install_date", install_date);

                    char* entry_str = json_stringify(entry);
                    if (entry_str) {
                        if (!first) strbuf_append(&sb, ",");
                        first = 0;
                        strbuf_append(&sb, entry_str);
                        free(entry_str);
                    }
                    json_free(entry);
                }

                RegCloseKey(hSubKey);
            }

            subkey_len = sizeof(subkey_name);
        }

        RegCloseKey(hKey);
    }

    strbuf_append(&sb, "]");
    *output = strbuf_detach(&sb);
    return 0;
}

#else /* Linux/macOS */

static int get_installed_software_unix(char** output) {
    StringBuffer sb;
    if (strbuf_init(&sb, 8192) != 0) {
        return -1;
    }

    strbuf_append(&sb, "[");

    /* Try different package managers */
    char* pkg_output = NULL;
    int found = 0;

    /* Try dpkg (Debian/Ubuntu) */
    if (run_command("which dpkg >/dev/null 2>&1 && dpkg-query -W -f='${Package}|${Version}|${Status}\\n' 2>/dev/null | head -500", &pkg_output, NULL) == 0 && pkg_output) {
        char* line = strtok(pkg_output, "\n");
        int first = 1;
        while (line) {
            char name[256] = {0}, version[64] = {0}, status[128] = {0};
            if (sscanf(line, "%255[^|]|%63[^|]|%127[^\n]", name, version, status) >= 2) {
                /* Only show installed packages */
                if (strstr(status, "installed") || strlen(status) == 0) {
                    if (!first) strbuf_append(&sb, ",");
                    first = 0;
                    strbuf_appendf(&sb, "{\"name\":\"%s\",\"version\":\"%s\",\"type\":\"dpkg\"}", name, version);
                    found = 1;
                }
            }
            line = strtok(NULL, "\n");
        }
        free(pkg_output);
        pkg_output = NULL;
    }

    /* Try rpm (RHEL/CentOS/Fedora) */
    if (!found && run_command("which rpm >/dev/null 2>&1 && rpm -qa --queryformat '%{NAME}|%{VERSION}|%{VENDOR}\\n' 2>/dev/null | head -500", &pkg_output, NULL) == 0 && pkg_output) {
        char* line = strtok(pkg_output, "\n");
        int first = !found;
        while (line) {
            char name[256] = {0}, version[64] = {0}, vendor[256] = {0};
            if (sscanf(line, "%255[^|]|%63[^|]|%255[^\n]", name, version, vendor) >= 2) {
                if (!first) strbuf_append(&sb, ",");
                first = 0;
                strbuf_appendf(&sb, "{\"name\":\"%s\",\"version\":\"%s\",\"publisher\":\"%s\",\"type\":\"rpm\"}", name, version, vendor);
                found = 1;
            }
            line = strtok(NULL, "\n");
        }
        free(pkg_output);
        pkg_output = NULL;
    }

    /* Try pacman (Arch Linux) */
    if (!found && run_command("which pacman >/dev/null 2>&1 && pacman -Q 2>/dev/null | head -500", &pkg_output, NULL) == 0 && pkg_output) {
        char* line = strtok(pkg_output, "\n");
        int first = !found;
        while (line) {
            char name[256] = {0}, version[64] = {0};
            if (sscanf(line, "%255s %63s", name, version) == 2) {
                if (!first) strbuf_append(&sb, ",");
                first = 0;
                strbuf_appendf(&sb, "{\"name\":\"%s\",\"version\":\"%s\",\"type\":\"pacman\"}", name, version);
                found = 1;
            }
            line = strtok(NULL, "\n");
        }
        free(pkg_output);
        pkg_output = NULL;
    }

#ifdef __APPLE__
    /* macOS: Try brew */
    if (run_command("which brew >/dev/null 2>&1 && brew list --versions 2>/dev/null | head -200", &pkg_output, NULL) == 0 && pkg_output) {
        char* line = strtok(pkg_output, "\n");
        int first = !found;
        while (line) {
            char name[256] = {0}, version[64] = {0};
            if (sscanf(line, "%255s %63s", name, version) >= 1) {
                if (!first) strbuf_append(&sb, ",");
                first = 0;
                strbuf_appendf(&sb, "{\"name\":\"%s\",\"version\":\"%s\",\"type\":\"brew\"}", name, version);
                found = 1;
            }
            line = strtok(NULL, "\n");
        }
        free(pkg_output);
    }
#endif

    strbuf_append(&sb, "]");
    *output = strbuf_detach(&sb);
    return 0;
}

#endif

/* ============================================================================
 * Module Interface Implementation
 * ============================================================================ */

static int software_init(void) {
    return 0;
}

static int software_execute(const char* command, const char* params, char** output) {
    (void)params;

    if (!output) return -1;

    if (!command || strcmp(command, "list") == 0) {
        /* List all installed software */
#ifdef _WIN32
        return get_installed_software_win32(output);
#else
        return get_installed_software_unix(output);
#endif
    }

    if (strncmp(command, "search:", 7) == 0) {
        /* Search for specific software */
        const char* query = command + 7;

        char* all_software = NULL;
#ifdef _WIN32
        if (get_installed_software_win32(&all_software) != 0) {
#else
        if (get_installed_software_unix(&all_software) != 0) {
#endif
            *output = safe_strdup("{\"error\":\"Failed to get software list\"}");
            return -1;
        }

        /* Parse and filter */
        const char* error = NULL;
        JsonValue* software_list = json_parse(all_software, &error);
        free(all_software);

        if (!software_list) {
            *output = safe_strdup("{\"error\":\"Failed to parse software list\"}");
            return -1;
        }

        JsonValue* filtered = json_array();
        size_t count = json_array_length(software_list);

        for (size_t i = 0; i < count; i++) {
            JsonValue* item = json_array_get(software_list, i);
            const char* name = json_object_get_string(item, "name");
            if (name && strcasestr(name, query)) {
                json_array_append(filtered, json_clone(item));
            }
        }

        *output = json_stringify(filtered);
        json_free(filtered);
        json_free(software_list);
        return 0;
    }

    *output = safe_strdup("{\"error\":\"Unknown command. Use: list, search:<query>\"}");
    return -1;
}

static void software_cleanup(void) {
    /* Nothing to clean up */
}

/* Module definition */
Module mod_software = {
    .name = "software",
    .description = "Software inventory management",
    .init = software_init,
    .execute = software_execute,
    .cleanup = software_cleanup,
    .initialized = 0
};
