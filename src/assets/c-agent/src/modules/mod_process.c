/*
 * NodePulse Agent - Process Management Module
 *
 * Provides detailed process information and management.
 * Commands:
 *   - list: List all processes with details
 *   - info:<pid>: Get detailed info about a specific process
 *   - kill:<pid>: Kill a process by PID
 *   - search:<name>: Search for processes by name
 */

#include "modules.h"
#include "../platform/platform.h"
#include "../utils/json.h"
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#ifdef _WIN32
#include <windows.h>
#include <tlhelp32.h>
#include <psapi.h>
#pragma comment(lib, "psapi.lib")
#else
#include <dirent.h>
#include <ctype.h>
#include <sys/types.h>
#include <signal.h>
#endif

/* Include safe_string.h last - its portable strcasestr macro must come after
 * system headers that might interfere with the #define on Windows/MinGW */
#include "../utils/safe_string.h"

/* ============================================================================
 * Platform-Specific Implementation
 * ============================================================================ */

#ifdef _WIN32

static int list_processes_win32(char** output, const char* filter) {
    HANDLE snapshot = CreateToolhelp32Snapshot(TH32CS_SNAPPROCESS, 0);
    if (snapshot == INVALID_HANDLE_VALUE) {
        *output = safe_strdup("{\"error\":\"Failed to create process snapshot\"}");
        return -1;
    }

    JsonValue* arr = json_array();
    PROCESSENTRY32 pe;
    pe.dwSize = sizeof(pe);

    if (Process32First(snapshot, &pe)) {
        do {
            /* Apply filter if provided */
            if (filter && strlen(filter) > 0) {
                if (!strcasestr(pe.szExeFile, filter)) {
                    continue;
                }
            }

            JsonValue* proc = json_object();
            json_object_set_int(proc, "pid", (int)pe.th32ProcessID);
            json_object_set_int(proc, "ppid", (int)pe.th32ParentProcessID);
            json_object_set_string(proc, "name", pe.szExeFile);
            json_object_set_int(proc, "threads", (int)pe.cntThreads);

            /* Try to get more info */
            HANDLE hProcess = OpenProcess(PROCESS_QUERY_INFORMATION | PROCESS_VM_READ,
                                          FALSE, pe.th32ProcessID);
            if (hProcess) {
                /* Get memory info */
                PROCESS_MEMORY_COUNTERS pmc;
                if (GetProcessMemoryInfo(hProcess, &pmc, sizeof(pmc))) {
                    json_object_set_int(proc, "memory_kb", (int)(pmc.WorkingSetSize / 1024));
                }

                /* Get executable path */
                char path[MAX_PATH] = {0};
                if (GetModuleFileNameExA(hProcess, NULL, path, MAX_PATH)) {
                    json_object_set_string(proc, "path", path);
                }

                CloseHandle(hProcess);
            }

            json_array_append(arr, proc);
        } while (Process32Next(snapshot, &pe));
    }

    CloseHandle(snapshot);

    *output = json_stringify(arr);
    json_free(arr);
    return 0;
}

static int get_process_info_win32(int pid, char** output) {
    HANDLE hProcess = OpenProcess(PROCESS_QUERY_INFORMATION | PROCESS_VM_READ,
                                   FALSE, (DWORD)pid);
    if (!hProcess) {
        *output = safe_strdup("{\"error\":\"Process not found or access denied\"}");
        return -1;
    }

    JsonValue* proc = json_object();
    json_object_set_int(proc, "pid", pid);

    /* Get executable path */
    char path[MAX_PATH] = {0};
    if (GetModuleFileNameExA(hProcess, NULL, path, MAX_PATH)) {
        json_object_set_string(proc, "path", path);

        /* Extract name from path */
        char* name = strrchr(path, '\\');
        json_object_set_string(proc, "name", name ? name + 1 : path);
    }

    /* Get memory info */
    PROCESS_MEMORY_COUNTERS pmc;
    if (GetProcessMemoryInfo(hProcess, &pmc, sizeof(pmc))) {
        json_object_set_int(proc, "memory_kb", (int)(pmc.WorkingSetSize / 1024));
        json_object_set_int(proc, "peak_memory_kb", (int)(pmc.PeakWorkingSetSize / 1024));
        json_object_set_int(proc, "page_faults", (int)pmc.PageFaultCount);
    }

    /* Get creation time */
    FILETIME creation, exit_time, kernel, user;
    if (GetProcessTimes(hProcess, &creation, &exit_time, &kernel, &user)) {
        SYSTEMTIME st;
        FileTimeToSystemTime(&creation, &st);
        char time_str[64];
        safe_snprintf(time_str, sizeof(time_str), "%04d-%02d-%02d %02d:%02d:%02d",
                     st.wYear, st.wMonth, st.wDay, st.wHour, st.wMinute, st.wSecond);
        json_object_set_string(proc, "start_time", time_str);

        /* Calculate CPU time */
        ULARGE_INTEGER k, u;
        k.LowPart = kernel.dwLowDateTime;
        k.HighPart = kernel.dwHighDateTime;
        u.LowPart = user.dwLowDateTime;
        u.HighPart = user.dwHighDateTime;
        json_object_set_int(proc, "cpu_time_ms", (int)((k.QuadPart + u.QuadPart) / 10000));
    }

    /* Get priority */
    DWORD priority = GetPriorityClass(hProcess);
    const char* priority_str = "normal";
    if (priority == HIGH_PRIORITY_CLASS) priority_str = "high";
    else if (priority == REALTIME_PRIORITY_CLASS) priority_str = "realtime";
    else if (priority == IDLE_PRIORITY_CLASS) priority_str = "idle";
    else if (priority == BELOW_NORMAL_PRIORITY_CLASS) priority_str = "below_normal";
    else if (priority == ABOVE_NORMAL_PRIORITY_CLASS) priority_str = "above_normal";
    json_object_set_string(proc, "priority", priority_str);

    CloseHandle(hProcess);

    *output = json_stringify(proc);
    json_free(proc);
    return 0;
}

static int kill_process_win32(int pid, char** output) {
    HANDLE hProcess = OpenProcess(PROCESS_TERMINATE, FALSE, (DWORD)pid);
    if (!hProcess) {
        *output = safe_strdup("{\"error\":\"Process not found or access denied\"}");
        return -1;
    }

    JsonValue* result = json_object();
    json_object_set_int(result, "pid", pid);

    if (TerminateProcess(hProcess, 1)) {
        json_object_set_bool(result, "success", 1);
        json_object_set_string(result, "message", "Process terminated");
    } else {
        json_object_set_bool(result, "success", 0);
        json_object_set_string(result, "message", "Failed to terminate process");
    }

    CloseHandle(hProcess);

    *output = json_stringify(result);
    json_free(result);
    return 0;
}

#else /* Linux/macOS */

static int list_processes_unix(char** output, const char* filter) {
    JsonValue* arr = json_array();

#ifdef __linux__
    DIR* proc_dir = opendir("/proc");
    if (!proc_dir) {
        *output = safe_strdup("{\"error\":\"Failed to open /proc\"}");
        return -1;
    }

    struct dirent* entry;
    while ((entry = readdir(proc_dir)) != NULL) {
        /* Check if entry is a PID (all digits) */
        int is_pid = 1;
        for (char* p = entry->d_name; *p; p++) {
            if (!isdigit((unsigned char)*p)) {
                is_pid = 0;
                break;
            }
        }
        if (!is_pid) continue;

        int pid = atoi(entry->d_name);

        /* Read process info */
        char status_path[256];
        safe_snprintf(status_path, sizeof(status_path), "/proc/%d/status", pid);

        FILE* status_file = fopen(status_path, "r");
        if (!status_file) continue;

        char name[256] = {0};
        int ppid = 0;
        long vm_rss = 0;
        int threads = 0;
        char state = '?';

        char line[256];
        while (fgets(line, sizeof(line), status_file)) {
            if (strncmp(line, "Name:", 5) == 0) {
                sscanf(line, "Name:\t%255s", name);
            } else if (strncmp(line, "PPid:", 5) == 0) {
                sscanf(line, "PPid:\t%d", &ppid);
            } else if (strncmp(line, "VmRSS:", 6) == 0) {
                sscanf(line, "VmRSS:\t%ld", &vm_rss);
            } else if (strncmp(line, "Threads:", 8) == 0) {
                sscanf(line, "Threads:\t%d", &threads);
            } else if (strncmp(line, "State:", 6) == 0) {
                sscanf(line, "State:\t%c", &state);
            }
        }
        fclose(status_file);

        /* Apply filter */
        if (filter && strlen(filter) > 0) {
            if (!strcasestr(name, filter)) {
                continue;
            }
        }

        /* Get command line */
        char cmdline_path[256];
        safe_snprintf(cmdline_path, sizeof(cmdline_path), "/proc/%d/cmdline", pid);
        char cmdline[512] = {0};
        FILE* cmdline_file = fopen(cmdline_path, "r");
        if (cmdline_file) {
            size_t len = fread(cmdline, 1, sizeof(cmdline) - 1, cmdline_file);
            /* Replace null chars with spaces */
            for (size_t i = 0; i < len; i++) {
                if (cmdline[i] == '\0') cmdline[i] = ' ';
            }
            fclose(cmdline_file);
        }

        JsonValue* proc = json_object();
        json_object_set_int(proc, "pid", pid);
        json_object_set_int(proc, "ppid", ppid);
        json_object_set_string(proc, "name", name);
        json_object_set_int(proc, "memory_kb", (int)vm_rss);
        json_object_set_int(proc, "threads", threads);

        char state_str[2] = {state, '\0'};
        json_object_set_string(proc, "state", state_str);

        if (strlen(cmdline) > 0) {
            json_object_set_string(proc, "cmdline", str_trim(cmdline));
        }

        json_array_append(arr, proc);
    }

    closedir(proc_dir);
#else
    /* macOS: Use ps command */
    char* ps_output = NULL;
    const char* cmd = filter && strlen(filter) > 0
        ? "ps -axo pid,ppid,rss,state,comm 2>/dev/null"
        : "ps -axo pid,ppid,rss,state,comm 2>/dev/null | head -500";

    if (run_command(cmd, &ps_output, NULL) == 0 && ps_output) {
        char* line = strtok(ps_output, "\n");
        line = strtok(NULL, "\n"); /* Skip header */

        while (line) {
            int pid, ppid, rss;
            char state[16], name[256];

            if (sscanf(line, "%d %d %d %15s %255s", &pid, &ppid, &rss, state, name) >= 4) {
                if (!filter || strlen(filter) == 0 || strcasestr(name, filter)) {
                    JsonValue* proc = json_object();
                    json_object_set_int(proc, "pid", pid);
                    json_object_set_int(proc, "ppid", ppid);
                    json_object_set_string(proc, "name", name);
                    json_object_set_int(proc, "memory_kb", rss);
                    json_object_set_string(proc, "state", state);
                    json_array_append(arr, proc);
                }
            }
            line = strtok(NULL, "\n");
        }
        free(ps_output);
    }
#endif

    *output = json_stringify(arr);
    json_free(arr);
    return 0;
}

static int get_process_info_unix(int pid, char** output) {
    JsonValue* proc = json_object();
    json_object_set_int(proc, "pid", pid);

#ifdef __linux__
    char path[256];

    /* Read status file */
    safe_snprintf(path, sizeof(path), "/proc/%d/status", pid);
    FILE* f = fopen(path, "r");
    if (!f) {
        json_free(proc);
        *output = safe_strdup("{\"error\":\"Process not found\"}");
        return -1;
    }

    char line[256];
    while (fgets(line, sizeof(line), f)) {
        char key[64], value[192];
        if (sscanf(line, "%63[^:]:\t%191[^\n]", key, value) == 2) {
            if (strcmp(key, "Name") == 0) json_object_set_string(proc, "name", value);
            else if (strcmp(key, "PPid") == 0) json_object_set_int(proc, "ppid", atoi(value));
            else if (strcmp(key, "Uid") == 0) {
                int uid;
                sscanf(value, "%d", &uid);
                json_object_set_int(proc, "uid", uid);
            }
            else if (strcmp(key, "VmRSS") == 0) {
                long rss;
                sscanf(value, "%ld", &rss);
                json_object_set_int(proc, "memory_kb", (int)rss);
            }
            else if (strcmp(key, "VmPeak") == 0) {
                long peak;
                sscanf(value, "%ld", &peak);
                json_object_set_int(proc, "peak_memory_kb", (int)peak);
            }
            else if (strcmp(key, "Threads") == 0) json_object_set_int(proc, "threads", atoi(value));
            else if (strcmp(key, "State") == 0) {
                char state[2] = {value[0], '\0'};
                json_object_set_string(proc, "state", state);
            }
        }
    }
    fclose(f);

    /* Read exe link */
    safe_snprintf(path, sizeof(path), "/proc/%d/exe", pid);
    char exe_path[512] = {0};
    ssize_t len = readlink(path, exe_path, sizeof(exe_path) - 1);
    if (len > 0) {
        exe_path[len] = '\0';
        json_object_set_string(proc, "path", exe_path);
    }

    /* Read command line */
    safe_snprintf(path, sizeof(path), "/proc/%d/cmdline", pid);
    f = fopen(path, "r");
    if (f) {
        char cmdline[1024] = {0};
        size_t cmdlen = fread(cmdline, 1, sizeof(cmdline) - 1, f);
        for (size_t i = 0; i < cmdlen; i++) {
            if (cmdline[i] == '\0') cmdline[i] = ' ';
        }
        if (cmdlen > 0) {
            json_object_set_string(proc, "cmdline", str_trim(cmdline));
        }
        fclose(f);
    }

    /* Read start time from stat */
    safe_snprintf(path, sizeof(path), "/proc/%d/stat", pid);
    f = fopen(path, "r");
    if (f) {
        char stat_line[1024];
        if (fgets(stat_line, sizeof(stat_line), f)) {
            /* Find starttime (field 22) */
            char* p = stat_line;
            int field = 0;
            unsigned long long starttime = 0;

            /* Skip past the command name (in parentheses) */
            p = strchr(p, ')');
            if (p) {
                p += 2;
                field = 2;
                char* token;
                while ((token = strsep(&p, " ")) != NULL) {
                    field++;
                    if (field == 22) {
                        starttime = strtoull(token, NULL, 10);
                        break;
                    }
                }
            }

            if (starttime > 0) {
                /* Convert to seconds since boot */
                long hz = sysconf(_SC_CLK_TCK);
                json_object_set_int(proc, "start_time_ticks", (int)(starttime / hz));
            }
        }
        fclose(f);
    }
#else
    /* macOS: Use ps */
    char cmd[128];
    safe_snprintf(cmd, sizeof(cmd), "ps -p %d -o pid,ppid,rss,state,user,comm,args 2>/dev/null", pid);
    char* ps_output = NULL;
    if (run_command(cmd, &ps_output, NULL) == 0 && ps_output) {
        char* line = strtok(ps_output, "\n");
        line = strtok(NULL, "\n"); /* Skip header */
        if (line) {
            int ppid, rss;
            char state[16], user[64], name[256], args[512];
            if (sscanf(line, "%*d %d %d %15s %63s %255s %511[^\n]",
                      &ppid, &rss, state, user, name, args) >= 4) {
                json_object_set_int(proc, "ppid", ppid);
                json_object_set_int(proc, "memory_kb", rss);
                json_object_set_string(proc, "state", state);
                json_object_set_string(proc, "user", user);
                json_object_set_string(proc, "name", name);
                if (strlen(args) > 0) {
                    json_object_set_string(proc, "cmdline", args);
                }
            }
        }
        free(ps_output);
    } else {
        json_free(proc);
        *output = safe_strdup("{\"error\":\"Process not found\"}");
        return -1;
    }
#endif

    *output = json_stringify(proc);
    json_free(proc);
    return 0;
}

static int kill_process_unix(int pid, char** output) {
    JsonValue* result = json_object();
    json_object_set_int(result, "pid", pid);

    if (kill(pid, SIGKILL) == 0) {
        json_object_set_bool(result, "success", 1);
        json_object_set_string(result, "message", "Process terminated");
    } else {
        json_object_set_bool(result, "success", 0);
        json_object_set_string(result, "message", "Failed to terminate process");
    }

    *output = json_stringify(result);
    json_free(result);
    return 0;
}

#endif

/* ============================================================================
 * Module Interface Implementation
 * ============================================================================ */

static int process_init(void) {
    return 0;
}

static int process_execute(const char* command, const char* params, char** output) {
    (void)params;

    if (!output) return -1;

    if (!command || strcmp(command, "list") == 0) {
#ifdef _WIN32
        return list_processes_win32(output, NULL);
#else
        return list_processes_unix(output, NULL);
#endif
    }

    if (strncmp(command, "info:", 5) == 0) {
        int pid = atoi(command + 5);
        if (pid <= 0) {
            *output = safe_strdup("{\"error\":\"Invalid PID\"}");
            return -1;
        }
#ifdef _WIN32
        return get_process_info_win32(pid, output);
#else
        return get_process_info_unix(pid, output);
#endif
    }

    if (strncmp(command, "kill:", 5) == 0) {
        int pid = atoi(command + 5);
        if (pid <= 0) {
            *output = safe_strdup("{\"error\":\"Invalid PID\"}");
            return -1;
        }
#ifdef _WIN32
        return kill_process_win32(pid, output);
#else
        return kill_process_unix(pid, output);
#endif
    }

    if (strncmp(command, "search:", 7) == 0) {
        const char* filter = command + 7;
#ifdef _WIN32
        return list_processes_win32(output, filter);
#else
        return list_processes_unix(output, filter);
#endif
    }

    *output = safe_strdup("{\"error\":\"Unknown command. Use: list, info:<pid>, kill:<pid>, search:<name>\"}");
    return -1;
}

static void process_cleanup(void) {
    /* Nothing to clean up */
}

/* Module definition */
Module mod_process = {
    .name = "process",
    .description = "Process management",
    .init = process_init,
    .execute = process_execute,
    .cleanup = process_cleanup,
    .initialized = 0
};
