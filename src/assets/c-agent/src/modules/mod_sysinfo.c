/*
 * NodePulse Agent - System Information Module
 * 
 * Gathers system information: hostname, OS, memory, disk, etc.
 */

#include "modules.h"
#include "../platform/platform.h"
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

static int sysinfo_init(void) {
    return 0;
}

static int sysinfo_execute(const char* command, const char* params, char** output) {
    (void)params;
    
    if (!output) return -1;
    
    /* Gather all system information */
    char hostname[256] = {0};
    char username[256] = {0};
    char os_info[256] = {0};
    char arch_info[64] = {0};
    char internal_ip[64] = {0};
    char external_ip[64] = {0};
    char mac_address[32] = {0};
    
    get_hostname(hostname, sizeof(hostname));
    get_username(username, sizeof(username));
    get_os_info(os_info, sizeof(os_info));
    get_arch_info(arch_info, sizeof(arch_info));
    get_internal_ip(internal_ip, sizeof(internal_ip));
    get_external_ip(external_ip, sizeof(external_ip), "https://api.ipify.org");
    get_mac_address(mac_address, sizeof(mac_address));
    
    long uptime = get_system_uptime();
    
    char* memory_info = NULL;
    char* disk_info = NULL;
    get_memory_info(&memory_info);
    get_disk_info(&disk_info);
    
    /* Build JSON response */
    size_t out_len = 2048;
    if (memory_info) out_len += strlen(memory_info);
    if (disk_info) out_len += strlen(disk_info);
    
    *output = (char*)malloc(out_len);
    snprintf(*output, out_len,
             "{"
             "\"hostname\":\"%s\","
             "\"username\":\"%s\","
             "\"os\":\"%s\","
             "\"arch\":\"%s\","
             "\"internal_ip\":\"%s\","
             "\"external_ip\":\"%s\","
             "\"mac_address\":\"%s\","
             "\"uptime\":%ld,"
             "\"memory\":%s,"
             "\"disk\":%s"
             "}",
             hostname,
             username,
             os_info,
             arch_info,
             internal_ip,
             external_ip,
             mac_address,
             uptime,
             memory_info ? memory_info : "{}",
             disk_info ? disk_info : "{}"
    );
    
    if (memory_info) free(memory_info);
    if (disk_info) free(disk_info);
    
    return 0;
}

static void sysinfo_cleanup(void) {
    /* Nothing to clean up */
}

/* Module definition */
Module mod_sysinfo = {
    .name = "systemInfo",
    .description = "Gather system information",
    .init = sysinfo_init,
    .execute = sysinfo_execute,
    .cleanup = sysinfo_cleanup,
    .initialized = 0
};
