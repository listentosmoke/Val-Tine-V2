/*
 * NodePulse Agent - Network Scanning Module
 * 
 * Network discovery and port scanning capabilities.
 */

#include "modules.h"
#include "../platform/platform.h"
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

static int network_init(void) {
    return 0;
}

/*
 * Commands:
 *   interfaces       - List network interfaces
 *   hosts:<subnet>   - Scan subnet for active hosts (e.g., hosts:192.168.1)
 *   ports:<host>     - Scan common ports on host
 */
static int network_execute(const char* command, const char* params, char** output) {
    (void)params;
    
    if (!command || !output) return -1;
    
    /* Check for interfaces command */
    if (strcmp(command, "interfaces") == 0) {
        char* iface_info = NULL;
        if (get_network_interfaces(&iface_info) == 0 && iface_info) {
            /* Escape for JSON */
            size_t len = strlen(iface_info);
            char* escaped = (char*)malloc(len * 2 + 1);
            char* p = escaped;
            for (size_t i = 0; i < len; i++) {
                char c = iface_info[i];
                switch (c) {
                    case '"':  *p++ = '\\'; *p++ = '"'; break;
                    case '\\': *p++ = '\\'; *p++ = '\\'; break;
                    case '\n': *p++ = '\\'; *p++ = 'n'; break;
                    case '\r': *p++ = '\\'; *p++ = 'r'; break;
                    case '\t': *p++ = '\\'; *p++ = 't'; break;
                    default: *p++ = c; break;
                }
            }
            *p = '\0';
            
            *output = (char*)malloc(strlen(escaped) + 64);
            snprintf(*output, strlen(escaped) + 64, "{\"interfaces\":\"%s\"}", escaped);
            
            free(escaped);
            free(iface_info);
            return 0;
        } else {
            *output = strdup("{\"error\":\"Failed to get interfaces\"}");
            return -1;
        }
    }
    
    /* Parse command:argument format */
    char cmd_type[32] = {0};
    char argument[256] = {0};
    
    const char* colon = strchr(command, ':');
    if (colon) {
        size_t cmd_len = colon - command;
        if (cmd_len >= sizeof(cmd_type)) cmd_len = sizeof(cmd_type) - 1;
        strncpy(cmd_type, command, cmd_len);
        strncpy(argument, colon + 1, sizeof(argument) - 1);
    } else {
        strncpy(cmd_type, command, sizeof(cmd_type) - 1);
    }
    
    if (strcmp(cmd_type, "hosts") == 0) {
        if (strlen(argument) == 0) {
            *output = strdup("{\"error\":\"Subnet required, e.g., hosts:192.168.1\"}");
            return -1;
        }
        
        char* hosts = NULL;
        if (network_scan_hosts(argument, &hosts) == 0 && hosts) {
            /* Parse hosts into JSON array */
            size_t out_len = strlen(hosts) + 128;
            *output = (char*)malloc(out_len);
            
            /* Convert newline-separated hosts to JSON array */
            char* json_hosts = (char*)malloc(strlen(hosts) + 256);
            char* jp = json_hosts;
            *jp++ = '[';
            
            int first = 1;
            char* line = strtok(hosts, "\n");
            while (line) {
                /* Trim whitespace */
                while (*line == ' ') line++;
                if (strlen(line) > 0) {
                    if (!first) *jp++ = ',';
                    first = 0;
                    jp += sprintf(jp, "\"%s\"", line);
                }
                line = strtok(NULL, "\n");
            }
            *jp++ = ']';
            *jp = '\0';
            
            snprintf(*output, out_len, "{\"subnet\":\"%s\",\"hosts\":%s}", argument, json_hosts);
            
            free(json_hosts);
            free(hosts);
            return 0;
        } else {
            *output = strdup("{\"error\":\"Failed to scan hosts\"}");
            return -1;
        }
    }
    else if (strcmp(cmd_type, "ports") == 0) {
        if (strlen(argument) == 0) {
            *output = strdup("{\"error\":\"Host required, e.g., ports:192.168.1.1\"}");
            return -1;
        }
        
        /* Common ports to scan */
        int common_ports[] = {21, 22, 23, 25, 53, 80, 110, 135, 139, 143, 
                              443, 445, 993, 995, 1433, 1521, 3306, 3389, 
                              5432, 5900, 8080, 8443};
        int port_count = sizeof(common_ports) / sizeof(common_ports[0]);
        
        char* open_ports = NULL;
        if (network_scan_ports(argument, common_ports, port_count, &open_ports) == 0 && open_ports) {
            size_t out_len = strlen(open_ports) + 128;
            *output = (char*)malloc(out_len);
            snprintf(*output, out_len, "{\"host\":\"%s\",\"open_ports\":%s}", argument, open_ports);
            free(open_ports);
            return 0;
        } else {
            *output = strdup("{\"error\":\"Failed to scan ports\"}");
            return -1;
        }
    }
    else {
        *output = (char*)malloc(128);
        snprintf(*output, 128, "{\"error\":\"Unknown command: %s\"}", cmd_type);
        return -1;
    }
}

static void network_cleanup(void) {
    /* Nothing to clean up */
}

/* Module definition */
Module mod_network = {
    .name = "networkScan",
    .description = "Network discovery and port scanning",
    .init = network_init,
    .execute = network_execute,
    .cleanup = network_cleanup,
    .initialized = 0
};
