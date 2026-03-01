/*
 * NodePulse Agent - Service Management Module
 *
 * Manages system services (start, stop, restart, status).
 * Commands:
 *   - list: List all services
 *   - status:<service_name>: Get status of a service
 *   - start:<service_name>: Start a service
 *   - stop:<service_name>: Stop a service
 *   - restart:<service_name>: Restart a service
 */

#include "modules.h"
#include "../platform/platform.h"
#include "../utils/safe_string.h"
#include "../utils/json.h"
#include "../utils/validation.h"
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#ifdef _WIN32
#include <windows.h>
#endif

/* ============================================================================
 * Helper Functions
 * ============================================================================ */

/* Validate service name to prevent injection */
static int validate_service_name(const char* name) {
    if (!name || strlen(name) == 0) return 0;
    if (strlen(name) > 256) return 0;

    /* Allow only alphanumeric, dash, underscore, and dot */
    for (const char* p = name; *p; p++) {
        if (!(*p >= 'a' && *p <= 'z') &&
            !(*p >= 'A' && *p <= 'Z') &&
            !(*p >= '0' && *p <= '9') &&
            *p != '-' && *p != '_' && *p != '.') {
            return 0;
        }
    }
    return 1;
}

/* ============================================================================
 * Platform-Specific Implementation
 * ============================================================================ */

#ifdef _WIN32

static int list_services_win32(char** output) {
    SC_HANDLE scManager = OpenSCManager(NULL, NULL, SC_MANAGER_ENUMERATE_SERVICE);
    if (!scManager) {
        *output = safe_strdup("{\"error\":\"Failed to open service manager\"}");
        return -1;
    }

    DWORD bytesNeeded = 0, servicesReturned = 0, resumeHandle = 0;
    EnumServicesStatusExA(scManager, SC_ENUM_PROCESS_INFO, SERVICE_WIN32,
                          SERVICE_STATE_ALL, NULL, 0, &bytesNeeded,
                          &servicesReturned, &resumeHandle, NULL);

    ENUM_SERVICE_STATUS_PROCESSA* services = (ENUM_SERVICE_STATUS_PROCESSA*)malloc(bytesNeeded);
    if (!services) {
        CloseServiceHandle(scManager);
        *output = safe_strdup("{\"error\":\"Memory allocation failed\"}");
        return -1;
    }

    resumeHandle = 0;
    if (!EnumServicesStatusExA(scManager, SC_ENUM_PROCESS_INFO, SERVICE_WIN32,
                               SERVICE_STATE_ALL, (LPBYTE)services, bytesNeeded,
                               &bytesNeeded, &servicesReturned, &resumeHandle, NULL)) {
        free(services);
        CloseServiceHandle(scManager);
        *output = safe_strdup("{\"error\":\"Failed to enumerate services\"}");
        return -1;
    }

    JsonValue* arr = json_array();

    for (DWORD i = 0; i < servicesReturned; i++) {
        JsonValue* svc = json_object();
        json_object_set_string(svc, "name", services[i].lpServiceName);
        json_object_set_string(svc, "display_name", services[i].lpDisplayName);

        const char* state = "unknown";
        switch (services[i].ServiceStatusProcess.dwCurrentState) {
            case SERVICE_RUNNING: state = "running"; break;
            case SERVICE_STOPPED: state = "stopped"; break;
            case SERVICE_PAUSED: state = "paused"; break;
            case SERVICE_START_PENDING: state = "starting"; break;
            case SERVICE_STOP_PENDING: state = "stopping"; break;
            case SERVICE_PAUSE_PENDING: state = "pausing"; break;
            case SERVICE_CONTINUE_PENDING: state = "resuming"; break;
        }
        json_object_set_string(svc, "status", state);
        json_object_set_int(svc, "pid", (int)services[i].ServiceStatusProcess.dwProcessId);

        json_array_append(arr, svc);
    }

    free(services);
    CloseServiceHandle(scManager);

    *output = json_stringify(arr);
    json_free(arr);
    return 0;
}

static int control_service_win32(const char* service_name, const char* action, char** output) {
    SC_HANDLE scManager = OpenSCManager(NULL, NULL, SC_MANAGER_ALL_ACCESS);
    if (!scManager) {
        *output = safe_strdup("{\"error\":\"Failed to open service manager\"}");
        return -1;
    }

    SC_HANDLE service = OpenServiceA(scManager, service_name,
                                      SERVICE_START | SERVICE_STOP | SERVICE_QUERY_STATUS);
    if (!service) {
        CloseServiceHandle(scManager);
        *output = safe_strdup("{\"error\":\"Service not found or access denied\"}");
        return -1;
    }

    int result = 0;
    JsonValue* response = json_object();
    json_object_set_string(response, "service", service_name);
    json_object_set_string(response, "action", action);

    if (strcmp(action, "start") == 0) {
        if (StartServiceA(service, 0, NULL)) {
            json_object_set_bool(response, "success", 1);
            json_object_set_string(response, "message", "Service started");
        } else {
            json_object_set_bool(response, "success", 0);
            json_object_set_string(response, "message", "Failed to start service");
            result = -1;
        }
    } else if (strcmp(action, "stop") == 0) {
        SERVICE_STATUS status;
        if (ControlService(service, SERVICE_CONTROL_STOP, &status)) {
            json_object_set_bool(response, "success", 1);
            json_object_set_string(response, "message", "Service stopped");
        } else {
            json_object_set_bool(response, "success", 0);
            json_object_set_string(response, "message", "Failed to stop service");
            result = -1;
        }
    } else if (strcmp(action, "status") == 0) {
        SERVICE_STATUS status;
        if (QueryServiceStatus(service, &status)) {
            const char* state = "unknown";
            switch (status.dwCurrentState) {
                case SERVICE_RUNNING: state = "running"; break;
                case SERVICE_STOPPED: state = "stopped"; break;
                case SERVICE_PAUSED: state = "paused"; break;
                case SERVICE_START_PENDING: state = "starting"; break;
                case SERVICE_STOP_PENDING: state = "stopping"; break;
            }
            json_object_set_bool(response, "success", 1);
            json_object_set_string(response, "status", state);
        } else {
            json_object_set_bool(response, "success", 0);
            json_object_set_string(response, "message", "Failed to query service status");
            result = -1;
        }
    }

    CloseServiceHandle(service);
    CloseServiceHandle(scManager);

    *output = json_stringify(response);
    json_free(response);
    return result;
}

#else /* Linux/macOS */

static int list_services_unix(char** output) {
    char* cmd_output = NULL;
    JsonValue* arr = json_array();

    /* Try systemctl first (systemd) */
    if (run_command("which systemctl >/dev/null 2>&1 && systemctl list-units --type=service --all --no-pager --no-legend 2>/dev/null | head -200", &cmd_output, NULL) == 0 && cmd_output && strlen(cmd_output) > 0) {
        char* line = strtok(cmd_output, "\n");
        while (line) {
            char unit[256] = {0}, load[32] = {0}, active[32] = {0}, sub[32] = {0};
            if (sscanf(line, "%255s %31s %31s %31s", unit, load, active, sub) >= 3) {
                /* Remove .service suffix */
                char* suffix = strstr(unit, ".service");
                if (suffix) *suffix = '\0';

                JsonValue* svc = json_object();
                json_object_set_string(svc, "name", unit);
                json_object_set_string(svc, "status", active);
                json_object_set_string(svc, "sub_status", sub);
                json_object_set_string(svc, "type", "systemd");
                json_array_append(arr, svc);
            }
            line = strtok(NULL, "\n");
        }
        free(cmd_output);
        cmd_output = NULL;
    }
    /* Try service command (SysV init) */
    else if (run_command("which service >/dev/null 2>&1 && service --status-all 2>/dev/null | head -100", &cmd_output, NULL) == 0 && cmd_output) {
        char* line = strtok(cmd_output, "\n");
        while (line) {
            char status_char;
            char service_name[256] = {0};
            /* Format: " [ + ]  service_name" or " [ - ]  service_name" */
            if (sscanf(line, " [ %c ] %255s", &status_char, service_name) == 2) {
                JsonValue* svc = json_object();
                json_object_set_string(svc, "name", service_name);
                json_object_set_string(svc, "status", status_char == '+' ? "running" : "stopped");
                json_object_set_string(svc, "type", "sysv");
                json_array_append(arr, svc);
            }
            line = strtok(NULL, "\n");
        }
        free(cmd_output);
    }

#ifdef __APPLE__
    /* macOS: Use launchctl */
    if (run_command("launchctl list 2>/dev/null | tail -n +2 | head -200", &cmd_output, NULL) == 0 && cmd_output) {
        char* line = strtok(cmd_output, "\n");
        while (line) {
            int pid;
            int status;
            char label[256] = {0};
            if (sscanf(line, "%d %d %255s", &pid, &status, label) >= 3 ||
                sscanf(line, "- %d %255s", &status, label) >= 2) {
                JsonValue* svc = json_object();
                json_object_set_string(svc, "name", label);
                json_object_set_string(svc, "status", pid > 0 ? "running" : "stopped");
                if (pid > 0) json_object_set_int(svc, "pid", pid);
                json_object_set_string(svc, "type", "launchd");
                json_array_append(arr, svc);
            }
            line = strtok(NULL, "\n");
        }
        free(cmd_output);
    }
#endif

    *output = json_stringify(arr);
    json_free(arr);
    return 0;
}

static int control_service_unix(const char* service_name, const char* action, char** output) {
    char cmd[512];
    char* cmd_output = NULL;
    int result = 0;

    JsonValue* response = json_object();
    json_object_set_string(response, "service", service_name);
    json_object_set_string(response, "action", action);

    /* Check for systemctl */
    if (run_command("which systemctl >/dev/null 2>&1", &cmd_output, NULL) == 0) {
        free(cmd_output);
        cmd_output = NULL;

        if (strcmp(action, "status") == 0) {
            safe_snprintf(cmd, sizeof(cmd), "systemctl is-active '%s' 2>/dev/null", service_name);
        } else {
            safe_snprintf(cmd, sizeof(cmd), "systemctl %s '%s' 2>&1", action, service_name);
        }
    }
#ifdef __APPLE__
    /* macOS launchctl */
    else if (strcmp(action, "start") == 0) {
        safe_snprintf(cmd, sizeof(cmd), "launchctl start '%s' 2>&1", service_name);
    } else if (strcmp(action, "stop") == 0) {
        safe_snprintf(cmd, sizeof(cmd), "launchctl stop '%s' 2>&1", service_name);
    } else if (strcmp(action, "status") == 0) {
        safe_snprintf(cmd, sizeof(cmd), "launchctl list '%s' 2>&1", service_name);
    }
#else
    /* SysV init fallback */
    else {
        safe_snprintf(cmd, sizeof(cmd), "service '%s' %s 2>&1", service_name, action);
    }
#endif
    else {
        safe_snprintf(cmd, sizeof(cmd), "service '%s' %s 2>&1", service_name, action);
    }

    result = run_command(cmd, &cmd_output, NULL);

    if (strcmp(action, "status") == 0) {
        json_object_set_bool(response, "success", 1);
        if (cmd_output) {
            /* Trim whitespace */
            char* status = str_trim(cmd_output);
            json_object_set_string(response, "status", status);
        }
    } else {
        json_object_set_bool(response, "success", result == 0);
        if (cmd_output && strlen(cmd_output) > 0) {
            json_object_set_string(response, "message", str_trim(cmd_output));
        } else {
            json_object_set_string(response, "message",
                result == 0 ? "Command executed successfully" : "Command failed");
        }
    }

    if (cmd_output) free(cmd_output);

    *output = json_stringify(response);
    json_free(response);
    return result == 0 ? 0 : -1;
}

#endif

/* ============================================================================
 * Module Interface Implementation
 * ============================================================================ */

static int services_init(void) {
    return 0;
}

static int services_execute(const char* command, const char* params, char** output) {
    (void)params;

    if (!output) return -1;

    if (!command || strcmp(command, "list") == 0) {
#ifdef _WIN32
        return list_services_win32(output);
#else
        return list_services_unix(output);
#endif
    }

    /* Parse command:service_name format */
    const char* actions[] = {"status:", "start:", "stop:", "restart:"};
    for (int i = 0; i < 4; i++) {
        size_t action_len = strlen(actions[i]);
        if (strncmp(command, actions[i], action_len) == 0) {
            const char* service_name = command + action_len;

            /* Validate service name */
            if (!validate_service_name(service_name)) {
                *output = safe_strdup("{\"error\":\"Invalid service name\"}");
                return -1;
            }

            const char* action = actions[i];
            char action_name[16];
            safe_strcpy(action_name, action, sizeof(action_name));
            action_name[strlen(action_name) - 1] = '\0';  /* Remove trailing colon */

            if (strcmp(action_name, "restart") == 0) {
                /* Restart = stop + start */
                char* stop_output = NULL;
#ifdef _WIN32
                control_service_win32(service_name, "stop", &stop_output);
                platform_sleep(1000);
                int result = control_service_win32(service_name, "start", output);
#else
                control_service_unix(service_name, "stop", &stop_output);
                platform_sleep(1000);
                int result = control_service_unix(service_name, "start", output);
#endif
                if (stop_output) free(stop_output);
                return result;
            }

#ifdef _WIN32
            return control_service_win32(service_name, action_name, output);
#else
            return control_service_unix(service_name, action_name, output);
#endif
        }
    }

    *output = safe_strdup("{\"error\":\"Unknown command. Use: list, status:<name>, start:<name>, stop:<name>, restart:<name>\"}");
    return -1;
}

static void services_cleanup(void) {
    /* Nothing to clean up */
}

/* Module definition */
Module mod_services = {
    .name = "services",
    .description = "Service management",
    .init = services_init,
    .execute = services_execute,
    .cleanup = services_cleanup,
    .initialized = 0
};
