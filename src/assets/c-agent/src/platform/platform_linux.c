/*
 * NodePulse Agent - Linux Platform Implementation
 *
 * Refactored with:
 * - Proper SSL certificate verification (configurable)
 * - Safe string operations
 * - Improved error handling
 */

#ifdef __linux__

#define _GNU_SOURCE

#include "platform.h"
#include "../utils/safe_string.h"
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
#include <sys/sysinfo.h>
#include <sys/statvfs.h>
#include <sys/utsname.h>
#include <sys/wait.h>
#include <netinet/in.h>
#include <arpa/inet.h>
#include <netdb.h>
#include <ifaddrs.h>
#include <net/if.h>
#include <errno.h>
#include <curl/curl.h>

/* SSL verification configuration - can be enabled at compile time */
#ifndef NODEPULSE_SSL_VERIFY
#define NODEPULSE_SSL_VERIFY 1  /* Enable SSL verification by default */
#endif

/* Connection timeout in seconds */
#define HTTP_CONNECT_TIMEOUT 30
#define HTTP_TIMEOUT 60

/* ============================================================================
 * Global State
 * ============================================================================ */

static CURL* g_curl = NULL;

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
 * CURL Helpers
 * ============================================================================ */

struct curl_buffer {
    char* data;
    size_t size;
};

static size_t curl_write_callback(void* contents, size_t size, size_t nmemb, void* userp) {
    size_t realsize = size * nmemb;
    struct curl_buffer* buf = (struct curl_buffer*)userp;
    
    buf->data = realloc(buf->data, buf->size + realsize + 1);
    memcpy(buf->data + buf->size, contents, realsize);
    buf->size += realsize;
    buf->data[buf->size] = '\0';
    
    return realsize;
}

/* ============================================================================
 * Initialization
 * ============================================================================ */

int platform_init(void) {
    curl_global_init(CURL_GLOBAL_ALL);
    g_curl = curl_easy_init();
    if (!g_curl) return -1;
    
    srand(time(NULL) ^ getpid());
    return 0;
}

void platform_cleanup(void) {
    if (g_curl) {
        curl_easy_cleanup(g_curl);
        g_curl = NULL;
    }
    curl_global_cleanup();
}

/* ============================================================================
 * HTTP Transport
 * ============================================================================ */

int http_post(const char* url, const char* data, size_t data_len,
              const char* content_type, HttpResponse* response) {
    if (!g_curl || !response) return -1;
    
    CURL* curl = curl_easy_duphandle(g_curl);
    if (!curl) return -1;
    
    struct curl_buffer buf = {NULL, 0};
    
    struct curl_slist* headers = NULL;
    char ct_header[256];
    snprintf(ct_header, sizeof(ct_header), "Content-Type: %s", content_type);
    headers = curl_slist_append(headers, ct_header);
    
    curl_easy_setopt(curl, CURLOPT_URL, url);
    curl_easy_setopt(curl, CURLOPT_HTTPHEADER, headers);
    curl_easy_setopt(curl, CURLOPT_POSTFIELDS, data);
    curl_easy_setopt(curl, CURLOPT_POSTFIELDSIZE, (long)data_len);
    curl_easy_setopt(curl, CURLOPT_WRITEFUNCTION, curl_write_callback);
    curl_easy_setopt(curl, CURLOPT_WRITEDATA, &buf);
    curl_easy_setopt(curl, CURLOPT_CONNECTTIMEOUT, (long)HTTP_CONNECT_TIMEOUT);
    curl_easy_setopt(curl, CURLOPT_TIMEOUT, (long)HTTP_TIMEOUT);
    curl_easy_setopt(curl, CURLOPT_USERAGENT, "Mozilla/5.0 (X11; Linux x86_64)");

    /* SSL verification - enabled by default for security */
#if NODEPULSE_SSL_VERIFY
    curl_easy_setopt(curl, CURLOPT_SSL_VERIFYPEER, 1L);
    curl_easy_setopt(curl, CURLOPT_SSL_VERIFYHOST, 2L);
#else
    curl_easy_setopt(curl, CURLOPT_SSL_VERIFYPEER, 0L);
    curl_easy_setopt(curl, CURLOPT_SSL_VERIFYHOST, 0L);
#endif
    
    CURLcode res = curl_easy_perform(curl);
    
    long http_code = 0;
    curl_easy_getinfo(curl, CURLINFO_RESPONSE_CODE, &http_code);
    
    response->data = buf.data;
    response->size = buf.size;
    response->status_code = (int)http_code;
    
    curl_slist_free_all(headers);
    curl_easy_cleanup(curl);
    
    return (res == CURLE_OK) ? 0 : -1;
}

/* HTTP POST with custom headers */
int http_post_with_headers(const char* url, const char* data, size_t data_len,
                           const char* content_type, const char** headers,
                           int header_count, HttpResponse* response) {
    if (!g_curl || !response) return -1;
    
    CURL* curl = curl_easy_duphandle(g_curl);
    if (!curl) return -1;
    
    struct curl_buffer buf = {NULL, 0};
    
    struct curl_slist* header_list = NULL;
    char ct_header[256];
    snprintf(ct_header, sizeof(ct_header), "Content-Type: %s", content_type);
    header_list = curl_slist_append(header_list, ct_header);
    
    /* Add custom headers (key-value pairs) */
    for (int i = 0; i + 1 < header_count; i += 2) {
        char custom_header[512];
        snprintf(custom_header, sizeof(custom_header), "%s: %s", headers[i], headers[i + 1]);
        header_list = curl_slist_append(header_list, custom_header);
    }
    
    curl_easy_setopt(curl, CURLOPT_URL, url);
    curl_easy_setopt(curl, CURLOPT_HTTPHEADER, header_list);
    curl_easy_setopt(curl, CURLOPT_POSTFIELDS, data);
    curl_easy_setopt(curl, CURLOPT_POSTFIELDSIZE, (long)data_len);
    curl_easy_setopt(curl, CURLOPT_WRITEFUNCTION, curl_write_callback);
    curl_easy_setopt(curl, CURLOPT_WRITEDATA, &buf);
    curl_easy_setopt(curl, CURLOPT_CONNECTTIMEOUT, (long)HTTP_CONNECT_TIMEOUT);
    curl_easy_setopt(curl, CURLOPT_TIMEOUT, (long)HTTP_TIMEOUT);
    curl_easy_setopt(curl, CURLOPT_USERAGENT, "Mozilla/5.0 (X11; Linux x86_64)");

    /* SSL verification - enabled by default for security */
#if NODEPULSE_SSL_VERIFY
    curl_easy_setopt(curl, CURLOPT_SSL_VERIFYPEER, 1L);
    curl_easy_setopt(curl, CURLOPT_SSL_VERIFYHOST, 2L);
#else
    curl_easy_setopt(curl, CURLOPT_SSL_VERIFYPEER, 0L);
    curl_easy_setopt(curl, CURLOPT_SSL_VERIFYHOST, 0L);
#endif

    CURLcode res = curl_easy_perform(curl);

    long http_code = 0;
    curl_easy_getinfo(curl, CURLINFO_RESPONSE_CODE, &http_code);

    response->data = buf.data;
    response->size = buf.size;
    response->status_code = (int)http_code;

    curl_slist_free_all(header_list);
    curl_easy_cleanup(curl);

    return (res == CURLE_OK) ? 0 : -1;
}

int http_get(const char* url, HttpResponse* response) {
    if (!g_curl || !response) return -1;

    CURL* curl = curl_easy_duphandle(g_curl);
    if (!curl) return -1;

    struct curl_buffer buf = {NULL, 0};

    curl_easy_setopt(curl, CURLOPT_URL, url);
    curl_easy_setopt(curl, CURLOPT_WRITEFUNCTION, curl_write_callback);
    curl_easy_setopt(curl, CURLOPT_WRITEDATA, &buf);
    curl_easy_setopt(curl, CURLOPT_CONNECTTIMEOUT, (long)HTTP_CONNECT_TIMEOUT);
    curl_easy_setopt(curl, CURLOPT_TIMEOUT, (long)HTTP_TIMEOUT);
    curl_easy_setopt(curl, CURLOPT_USERAGENT, "Mozilla/5.0 (X11; Linux x86_64)");

    /* SSL verification - enabled by default for security */
#if NODEPULSE_SSL_VERIFY
    curl_easy_setopt(curl, CURLOPT_SSL_VERIFYPEER, 1L);
    curl_easy_setopt(curl, CURLOPT_SSL_VERIFYHOST, 2L);
#else
    curl_easy_setopt(curl, CURLOPT_SSL_VERIFYPEER, 0L);
    curl_easy_setopt(curl, CURLOPT_SSL_VERIFYHOST, 0L);
#endif

    CURLcode res = curl_easy_perform(curl);

    long http_code = 0;
    curl_easy_getinfo(curl, CURLINFO_RESPONSE_CODE, &http_code);

    response->data = buf.data;
    response->size = buf.size;
    response->status_code = (int)http_code;

    curl_easy_cleanup(curl);

    return (res == CURLE_OK) ? 0 : -1;
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
    if (!*output) {
        pclose(fp);
        return -1;
    }
    size_t total = 0;

    char buffer[4096];
    while (fgets(buffer, sizeof(buffer), fp)) {
        size_t len = strlen(buffer);
        if (total + len >= capacity) {
            capacity *= 2;
            char* new_output = (char*)realloc(*output, capacity);
            if (!new_output) {
                free(*output);
                *output = NULL;
                pclose(fp);
                return -1;
            }
            *output = new_output;
        }
        /* Use safe memcpy instead of strcpy */
        memcpy(*output + total, buffer, len);
        total += len;
    }
    (*output)[total] = '\0';
    if (output_len) *output_len = total;

    int status = pclose(fp);
    return WIFEXITED(status) ? WEXITSTATUS(status) : -1;
}

int get_process_list(char** output) {
    return run_command("ps aux --no-headers", output, NULL);
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
    struct utsname uname_data;
    if (uname(&uname_data) != 0) return -1;
    snprintf(buf, len, "%s %s", uname_data.sysname, uname_data.release);
    return 0;
}

int get_arch_info(char* buf, size_t len) {
    struct utsname uname_data;
    if (uname(&uname_data) != 0) return -1;
    strncpy(buf, uname_data.machine, len - 1);
    buf[len - 1] = '\0';
    return 0;
}

int get_internal_ip(char* buf, size_t len) {
    struct ifaddrs *ifaddr, *ifa;
    
    if (getifaddrs(&ifaddr) == -1) return -1;
    
    for (ifa = ifaddr; ifa != NULL; ifa = ifa->ifa_next) {
        if (ifa->ifa_addr == NULL) continue;
        if (ifa->ifa_addr->sa_family != AF_INET) continue;
        if (strcmp(ifa->ifa_name, "lo") == 0) continue;
        
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
        if (strcmp(ifa->ifa_name, "lo") == 0) continue;
        
        char path[256];
        snprintf(path, sizeof(path), "/sys/class/net/%s/address", ifa->ifa_name);
        
        FILE* f = fopen(path, "r");
        if (f) {
            if (fgets(buf, len, f)) {
                /* Trim newline */
                char* nl = strchr(buf, '\n');
                if (nl) *nl = '\0';
                fclose(f);
                freeifaddrs(ifaddr);
                return 0;
            }
            fclose(f);
        }
    }
    
    freeifaddrs(ifaddr);
    return -1;
}

long get_system_uptime(void) {
    struct sysinfo info;
    if (sysinfo(&info) != 0) return -1;
    return info.uptime;
}

int get_memory_info(char** output) {
    struct sysinfo info;
    if (sysinfo(&info) != 0) return -1;
    
    unsigned long total = info.totalram * info.mem_unit;
    unsigned long free_mem = info.freeram * info.mem_unit;
    unsigned long used = total - free_mem;
    int percent = (int)((used * 100) / total);
    
    *output = (char*)malloc(256);
    snprintf(*output, 256,
             "{\"total\":%lu,\"available\":%lu,\"used\":%lu,\"percent\":%d}",
             total, free_mem, used, percent);
    return 0;
}

int get_disk_info(char** output) {
    struct statvfs stat;
    if (statvfs("/", &stat) != 0) return -1;
    
    unsigned long total = stat.f_blocks * stat.f_frsize;
    unsigned long free_space = stat.f_bfree * stat.f_frsize;
    unsigned long used = total - free_space;
    
    *output = (char*)malloc(256);
    snprintf(*output, 256,
             "{\"total\":%lu,\"free\":%lu,\"used\":%lu}",
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
        snprintf(item, sizeof(item), "%s{\"name\":\"%s\",\"is_dir\":%s,\"size\":%ld}",
                 first ? "" : ",", entry->d_name,
                 S_ISDIR(st.st_mode) ? "true" : "false",
                 (long)st.st_size);
        first = 0;
        
        size_t item_len = strlen(item);
        if (pos + item_len + 2 >= capacity) {
            capacity *= 2;
            char* new_output = (char*)realloc(*output, capacity);
            if (!new_output) {
                closedir(dir);
                free(*output);
                *output = NULL;
                return -1;
            }
            *output = new_output;
        }
        memcpy(*output + pos, item, item_len);
        pos += item_len;
    }

    closedir(dir);

    (*output)[pos++] = ']';
    (*output)[pos] = '\0';
    return 0;
}

/* Recursive directory listing for tree command */
static int file_list_recursive_internal(const char* path, int depth, char** output, size_t* capacity, size_t* pos, int* first) {
    if (depth <= 0) return 0;
    
    DIR* dir = opendir(path);
    if (!dir) return -1;
    
    struct dirent* entry;
    while ((entry = readdir(dir)) != NULL) {
        if (strcmp(entry->d_name, ".") == 0 || strcmp(entry->d_name, "..") == 0) continue;
        
        char full_path[PATH_MAX];
        snprintf(full_path, sizeof(full_path), "%s/%s", path, entry->d_name);
        
        struct stat st;
        if (stat(full_path, &st) != 0) continue;
        
        int isDir = S_ISDIR(st.st_mode);
        
        /* Escape path for JSON (replace backslash with double backslash if any) */
        char item[2048];
        if (isDir && depth > 1) {
            /* For directories with remaining depth, recurse and include children */
            size_t child_capacity = 4096;
            char* children = (char*)malloc(child_capacity);
            size_t child_pos = 1;
            children[0] = '[';
            int child_first = 1;
            
            file_list_recursive_internal(full_path, depth - 1, &children, &child_capacity, &child_pos, &child_first);
            
            children[child_pos++] = ']';
            children[child_pos] = '\0';
            
            snprintf(item, sizeof(item), "%s{\"name\":\"%s\",\"path\":\"%s\",\"type\":\"directory\",\"children\":%s}",
                     *first ? "" : ",", entry->d_name, full_path, children);
            free(children);
        } else if (isDir) {
            snprintf(item, sizeof(item), "%s{\"name\":\"%s\",\"path\":\"%s\",\"type\":\"directory\"}",
                     *first ? "" : ",", entry->d_name, full_path);
        } else {
            snprintf(item, sizeof(item), "%s{\"name\":\"%s\",\"path\":\"%s\",\"type\":\"file\",\"size\":%ld}",
                     *first ? "" : ",", entry->d_name, full_path, (long)st.st_size);
        }
        *first = 0;
        
        size_t item_len = strlen(item);
        while (*pos + item_len + 2 >= *capacity) {
            *capacity *= 2;
            char* new_output = (char*)realloc(*output, *capacity);
            if (!new_output) {
                closedir(dir);
                return -1;
            }
            *output = new_output;
        }
        memcpy(*output + *pos, item, item_len);
        *pos += item_len;
    }

    closedir(dir);
    return 0;
}

int file_list_recursive(const char* path, int depth, char** output) {
    size_t capacity = 16384; /* Start with larger buffer for tree */
    *output = (char*)malloc(capacity);
    size_t pos = 1;
    (*output)[0] = '[';
    int first = 1;
    
    file_list_recursive_internal(path, depth, output, &capacity, &pos, &first);
    
    (*output)[pos++] = ']';
    (*output)[pos] = '\0';
    return 0;
}

int dir_create(const char* path) {
    return mkdir(path, 0755);
}

/* ============================================================================
 * Screenshot (Linux - X11)
 * ============================================================================ */

int capture_screenshot(char** base64_data) {
    /* Use scrot or import for screenshot */
    char* output = NULL;
    int result = run_command("scrot -o /tmp/.screenshot.png && base64 -w0 /tmp/.screenshot.png && rm /tmp/.screenshot.png", &output, NULL);
    
    if (result == 0 && output) {
        *base64_data = output;
        return 0;
    }
    
    if (output) free(output);
    *base64_data = NULL;
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
        
        /* Set non-blocking */
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
        
        struct timeval tv = {0, 500000}; /* 500ms timeout */
        int result = select(sock + 1, NULL, &writefds, NULL, &tv);
        
        if (result > 0) {
            char entry[64];
            snprintf(entry, sizeof(entry), "%s%d", first ? "" : ",", ports[i]);
            first = 0;

            size_t entry_len = strlen(entry);
            if (pos + entry_len + 2 >= capacity) {
                capacity *= 2;
                char* new_output = (char*)realloc(*output, capacity);
                if (!new_output) {
                    close(sock);
                    free(*output);
                    *output = NULL;
                    return -1;
                }
                *output = new_output;
            }
            memcpy(*output + pos, entry, entry_len);
            pos += entry_len;
        }

        close(sock);
    }

    (*output)[pos++] = ']';
    (*output)[pos] = '\0';
    return 0;
}

int get_network_interfaces(char** output) {
    return run_command("ip addr", output, NULL);
}

/* ============================================================================
 * Keylogger (Not supported on Linux without X11 hooks)
 * ============================================================================ */

int keylogger_start(void) {
    return -1; /* Not implemented */
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
    int fd = open("/dev/urandom", O_RDONLY);
    if (fd < 0) {
        for (size_t i = 0; i < len; i++) {
            buf[i] = (unsigned char)(rand() & 0xFF);
        }
        return 0;
    }
    read(fd, buf, len);
    close(fd);
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

#endif /* __linux__ */
