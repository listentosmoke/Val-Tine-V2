/*
 * NodePulse Agent - Network Information Module
 *
 * Provides detailed network interface and connection information.
 * Commands:
 *   - interfaces: List all network interfaces
 *   - connections: List active network connections
 *   - routing: Show routing table
 *   - dns: Show DNS configuration
 */

/* On Windows, winsock2.h MUST be included before windows.h (and before any
 * system headers that might transitively include windows.h on MinGW). */
#ifdef _WIN32
#include <winsock2.h>
#include <ws2tcpip.h>
#endif

#include "modules.h"
#include "../platform/platform.h"
#include "../utils/json.h"
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#ifdef _WIN32
#include <windows.h>
#include <iphlpapi.h>
#pragma comment(lib, "iphlpapi.lib")
#pragma comment(lib, "ws2_32.lib")
#else
#include <sys/types.h>
#include <sys/socket.h>
#include <netinet/in.h>
#include <arpa/inet.h>
#include <netdb.h>
#include <ifaddrs.h>
#include <net/if.h>
#ifdef __linux__
#include <linux/if_link.h>
#endif
#endif

/* Include safe_string.h last - its portable strcasestr macro must come after
 * system headers that might interfere with the #define on Windows/MinGW */
#include "../utils/safe_string.h"

/* ============================================================================
 * Platform-Specific Implementation
 * ============================================================================ */

#ifdef _WIN32

static int list_interfaces_win32(char** output) {
    JsonValue* arr = json_array();

    ULONG buf_size = 15000;
    PIP_ADAPTER_ADDRESSES addresses = NULL;
    int iterations = 0;
    ULONG result;

    do {
        addresses = (PIP_ADAPTER_ADDRESSES)malloc(buf_size);
        if (!addresses) {
            *output = safe_strdup("{\"error\":\"Memory allocation failed\"}");
            return -1;
        }

        result = GetAdaptersAddresses(AF_UNSPEC,
            GAA_FLAG_INCLUDE_PREFIX | GAA_FLAG_INCLUDE_GATEWAYS,
            NULL, addresses, &buf_size);

        if (result == ERROR_BUFFER_OVERFLOW) {
            free(addresses);
            addresses = NULL;
        }
        iterations++;
    } while (result == ERROR_BUFFER_OVERFLOW && iterations < 3);

    if (result != NO_ERROR) {
        if (addresses) free(addresses);
        *output = safe_strdup("{\"error\":\"Failed to get adapter info\"}");
        return -1;
    }

    for (PIP_ADAPTER_ADDRESSES addr = addresses; addr != NULL; addr = addr->Next) {
        JsonValue* iface = json_object();

        /* Convert adapter name */
        char name[256] = {0};
        WideCharToMultiByte(CP_UTF8, 0, addr->FriendlyName, -1,
                           name, sizeof(name), NULL, NULL);
        json_object_set_string(iface, "name", name);

        /* Description */
        char desc[256] = {0};
        WideCharToMultiByte(CP_UTF8, 0, addr->Description, -1,
                           desc, sizeof(desc), NULL, NULL);
        json_object_set_string(iface, "description", desc);

        /* MAC address */
        if (addr->PhysicalAddressLength > 0) {
            char mac[32] = {0};
            safe_snprintf(mac, sizeof(mac), "%02X:%02X:%02X:%02X:%02X:%02X",
                         addr->PhysicalAddress[0], addr->PhysicalAddress[1],
                         addr->PhysicalAddress[2], addr->PhysicalAddress[3],
                         addr->PhysicalAddress[4], addr->PhysicalAddress[5]);
            json_object_set_string(iface, "mac", mac);
        }

        /* Status */
        const char* status = "unknown";
        if (addr->OperStatus == IfOperStatusUp) status = "up";
        else if (addr->OperStatus == IfOperStatusDown) status = "down";
        json_object_set_string(iface, "status", status);

        /* Type */
        const char* type = "other";
        if (addr->IfType == IF_TYPE_ETHERNET_CSMACD) type = "ethernet";
        else if (addr->IfType == IF_TYPE_IEEE80211) type = "wifi";
        else if (addr->IfType == IF_TYPE_SOFTWARE_LOOPBACK) type = "loopback";
        else if (addr->IfType == IF_TYPE_TUNNEL) type = "tunnel";
        json_object_set_string(iface, "type", type);

        /* Speed */
        if (addr->TransmitLinkSpeed > 0) {
            json_object_set_int(iface, "speed_mbps", (int)(addr->TransmitLinkSpeed / 1000000));
        }

        /* MTU */
        json_object_set_int(iface, "mtu", (int)addr->Mtu);

        /* IP addresses */
        JsonValue* ips = json_array();
        for (PIP_ADAPTER_UNICAST_ADDRESS ua = addr->FirstUnicastAddress; ua != NULL; ua = ua->Next) {
            char ip[INET6_ADDRSTRLEN] = {0};
            struct sockaddr* sa = ua->Address.lpSockaddr;

            if (sa->sa_family == AF_INET) {
                struct sockaddr_in* sin = (struct sockaddr_in*)sa;
                inet_ntop(AF_INET, &sin->sin_addr, ip, sizeof(ip));
            } else if (sa->sa_family == AF_INET6) {
                struct sockaddr_in6* sin6 = (struct sockaddr_in6*)sa;
                inet_ntop(AF_INET6, &sin6->sin6_addr, ip, sizeof(ip));
            }

            if (strlen(ip) > 0) {
                JsonValue* ip_obj = json_object();
                json_object_set_string(ip_obj, "address", ip);
                json_object_set_string(ip_obj, "family", sa->sa_family == AF_INET ? "ipv4" : "ipv6");
                json_object_set_int(ip_obj, "prefix_length", (int)ua->OnLinkPrefixLength);
                json_array_append(ips, ip_obj);
            }
        }
        json_object_set(iface, "addresses", ips);

        /* Gateway */
        for (PIP_ADAPTER_GATEWAY_ADDRESS gw = addr->FirstGatewayAddress; gw != NULL; gw = gw->Next) {
            char gateway[INET6_ADDRSTRLEN] = {0};
            struct sockaddr* sa = gw->Address.lpSockaddr;

            if (sa->sa_family == AF_INET) {
                struct sockaddr_in* sin = (struct sockaddr_in*)sa;
                inet_ntop(AF_INET, &sin->sin_addr, gateway, sizeof(gateway));
            }

            if (strlen(gateway) > 0) {
                json_object_set_string(iface, "gateway", gateway);
                break;
            }
        }

        /* DNS servers */
        JsonValue* dns_arr = json_array();
        for (PIP_ADAPTER_DNS_SERVER_ADDRESS dns = addr->FirstDnsServerAddress; dns != NULL; dns = dns->Next) {
            char dns_ip[INET6_ADDRSTRLEN] = {0};
            struct sockaddr* sa = dns->Address.lpSockaddr;

            if (sa->sa_family == AF_INET) {
                struct sockaddr_in* sin = (struct sockaddr_in*)sa;
                inet_ntop(AF_INET, &sin->sin_addr, dns_ip, sizeof(dns_ip));
            } else if (sa->sa_family == AF_INET6) {
                struct sockaddr_in6* sin6 = (struct sockaddr_in6*)sa;
                inet_ntop(AF_INET6, &sin6->sin6_addr, dns_ip, sizeof(dns_ip));
            }

            if (strlen(dns_ip) > 0) {
                json_array_append(dns_arr, json_string(dns_ip));
            }
        }
        json_object_set(iface, "dns_servers", dns_arr);

        json_array_append(arr, iface);
    }

    free(addresses);

    *output = json_stringify(arr);
    json_free(arr);
    return 0;
}

static int list_connections_win32(char** output) {
    JsonValue* arr = json_array();

    /* TCP connections */
    PMIB_TCPTABLE2 tcp_table = NULL;
    ULONG tcp_size = 0;

    GetTcpTable2(NULL, &tcp_size, TRUE);
    tcp_table = (PMIB_TCPTABLE2)malloc(tcp_size);
    if (tcp_table && GetTcpTable2(tcp_table, &tcp_size, TRUE) == NO_ERROR) {
        for (DWORD i = 0; i < tcp_table->dwNumEntries; i++) {
            MIB_TCPROW2* row = &tcp_table->table[i];
            JsonValue* conn = json_object();

            json_object_set_string(conn, "protocol", "tcp");

            /* Local address */
            struct in_addr local_addr;
            local_addr.S_un.S_addr = row->dwLocalAddr;
            char local_ip[INET_ADDRSTRLEN];
            inet_ntop(AF_INET, &local_addr, local_ip, sizeof(local_ip));
            json_object_set_string(conn, "local_address", local_ip);
            json_object_set_int(conn, "local_port", ntohs((u_short)row->dwLocalPort));

            /* Remote address */
            struct in_addr remote_addr;
            remote_addr.S_un.S_addr = row->dwRemoteAddr;
            char remote_ip[INET_ADDRSTRLEN];
            inet_ntop(AF_INET, &remote_addr, remote_ip, sizeof(remote_ip));
            json_object_set_string(conn, "remote_address", remote_ip);
            json_object_set_int(conn, "remote_port", ntohs((u_short)row->dwRemotePort));

            /* State */
            const char* state = "unknown";
            switch (row->dwState) {
                case MIB_TCP_STATE_CLOSED: state = "closed"; break;
                case MIB_TCP_STATE_LISTEN: state = "listen"; break;
                case MIB_TCP_STATE_SYN_SENT: state = "syn_sent"; break;
                case MIB_TCP_STATE_SYN_RCVD: state = "syn_rcvd"; break;
                case MIB_TCP_STATE_ESTAB: state = "established"; break;
                case MIB_TCP_STATE_FIN_WAIT1: state = "fin_wait1"; break;
                case MIB_TCP_STATE_FIN_WAIT2: state = "fin_wait2"; break;
                case MIB_TCP_STATE_CLOSE_WAIT: state = "close_wait"; break;
                case MIB_TCP_STATE_CLOSING: state = "closing"; break;
                case MIB_TCP_STATE_LAST_ACK: state = "last_ack"; break;
                case MIB_TCP_STATE_TIME_WAIT: state = "time_wait"; break;
            }
            json_object_set_string(conn, "state", state);

            /* PID */
            json_object_set_int(conn, "pid", (int)row->dwOwningPid);

            json_array_append(arr, conn);
        }
    }
    if (tcp_table) free(tcp_table);

    /* UDP endpoints */
    PMIB_UDPTABLE_OWNER_PID udp_table = NULL;
    ULONG udp_size = 0;

    GetExtendedUdpTable(NULL, &udp_size, TRUE, AF_INET, UDP_TABLE_OWNER_PID, 0);
    udp_table = (PMIB_UDPTABLE_OWNER_PID)malloc(udp_size);
    if (udp_table && GetExtendedUdpTable(udp_table, &udp_size, TRUE, AF_INET, UDP_TABLE_OWNER_PID, 0) == NO_ERROR) {
        for (DWORD i = 0; i < udp_table->dwNumEntries; i++) {
            MIB_UDPROW_OWNER_PID* row = &udp_table->table[i];
            JsonValue* conn = json_object();

            json_object_set_string(conn, "protocol", "udp");

            struct in_addr local_addr;
            local_addr.S_un.S_addr = row->dwLocalAddr;
            char local_ip[INET_ADDRSTRLEN];
            inet_ntop(AF_INET, &local_addr, local_ip, sizeof(local_ip));
            json_object_set_string(conn, "local_address", local_ip);
            json_object_set_int(conn, "local_port", ntohs((u_short)row->dwLocalPort));

            json_object_set_string(conn, "state", "listening");
            json_object_set_int(conn, "pid", (int)row->dwOwningPid);

            json_array_append(arr, conn);
        }
    }
    if (udp_table) free(udp_table);

    *output = json_stringify(arr);
    json_free(arr);
    return 0;
}

static int show_routing_win32(char** output) {
    JsonValue* arr = json_array();

    PMIB_IPFORWARDTABLE route_table = NULL;
    ULONG route_size = 0;

    GetIpForwardTable(NULL, &route_size, TRUE);
    route_table = (PMIB_IPFORWARDTABLE)malloc(route_size);

    if (route_table && GetIpForwardTable(route_table, &route_size, TRUE) == NO_ERROR) {
        for (DWORD i = 0; i < route_table->dwNumEntries; i++) {
            MIB_IPFORWARDROW* row = &route_table->table[i];
            JsonValue* route = json_object();

            struct in_addr dest, mask, gateway;
            dest.S_un.S_addr = row->dwForwardDest;
            mask.S_un.S_addr = row->dwForwardMask;
            gateway.S_un.S_addr = row->dwForwardNextHop;

            char dest_ip[INET_ADDRSTRLEN];
            char mask_ip[INET_ADDRSTRLEN];
            char gw_ip[INET_ADDRSTRLEN];

            inet_ntop(AF_INET, &dest, dest_ip, sizeof(dest_ip));
            inet_ntop(AF_INET, &mask, mask_ip, sizeof(mask_ip));
            inet_ntop(AF_INET, &gateway, gw_ip, sizeof(gw_ip));

            json_object_set_string(route, "destination", dest_ip);
            json_object_set_string(route, "netmask", mask_ip);
            json_object_set_string(route, "gateway", gw_ip);
            json_object_set_int(route, "metric", (int)row->dwForwardMetric1);
            json_object_set_int(route, "interface_index", (int)row->dwForwardIfIndex);

            const char* type = "other";
            if (row->dwForwardType == MIB_IPROUTE_TYPE_DIRECT) type = "direct";
            else if (row->dwForwardType == MIB_IPROUTE_TYPE_INDIRECT) type = "indirect";
            json_object_set_string(route, "type", type);

            json_array_append(arr, route);
        }
    }
    if (route_table) free(route_table);

    *output = json_stringify(arr);
    json_free(arr);
    return 0;
}

#else /* Linux/macOS */

static int list_interfaces_unix(char** output) {
    JsonValue* arr = json_array();

    struct ifaddrs* ifaddr;
    if (getifaddrs(&ifaddr) == -1) {
        *output = safe_strdup("{\"error\":\"Failed to get interfaces\"}");
        return -1;
    }

    /* Track which interfaces we've added */
    char added[32][64] = {0};
    int added_count = 0;

    for (struct ifaddrs* ifa = ifaddr; ifa != NULL; ifa = ifa->ifa_next) {
        if (ifa->ifa_name == NULL) continue;

        /* Check if we've already created an object for this interface */
        int found = 0;
        for (int i = 0; i < added_count; i++) {
            if (strcmp(added[i], ifa->ifa_name) == 0) {
                found = 1;
                break;
            }
        }
        if (found) continue;

        /* Add to tracking */
        if (added_count < 32) {
            safe_strcpy(added[added_count], sizeof(added[0]), ifa->ifa_name);
            added_count++;
        }

        JsonValue* iface = json_object();
        json_object_set_string(iface, "name", ifa->ifa_name);

        /* Status */
        json_object_set_string(iface, "status", (ifa->ifa_flags & IFF_UP) ? "up" : "down");

        /* Type based on name pattern */
        const char* type = "other";
        if (strncmp(ifa->ifa_name, "lo", 2) == 0) type = "loopback";
        else if (strncmp(ifa->ifa_name, "eth", 3) == 0 || strncmp(ifa->ifa_name, "en", 2) == 0) type = "ethernet";
        else if (strncmp(ifa->ifa_name, "wlan", 4) == 0 || strncmp(ifa->ifa_name, "wl", 2) == 0) type = "wifi";
        else if (strncmp(ifa->ifa_name, "tun", 3) == 0 || strncmp(ifa->ifa_name, "tap", 3) == 0) type = "tunnel";
        else if (strncmp(ifa->ifa_name, "docker", 6) == 0 || strncmp(ifa->ifa_name, "br-", 3) == 0) type = "bridge";
        json_object_set_string(iface, "type", type);

        /* Collect all addresses for this interface */
        JsonValue* ips = json_array();
        for (struct ifaddrs* addr = ifaddr; addr != NULL; addr = addr->ifa_next) {
            if (addr->ifa_addr == NULL || strcmp(addr->ifa_name, ifa->ifa_name) != 0) continue;

            if (addr->ifa_addr->sa_family == AF_INET) {
                char ip[INET_ADDRSTRLEN];
                struct sockaddr_in* sin = (struct sockaddr_in*)addr->ifa_addr;
                inet_ntop(AF_INET, &sin->sin_addr, ip, sizeof(ip));

                JsonValue* ip_obj = json_object();
                json_object_set_string(ip_obj, "address", ip);
                json_object_set_string(ip_obj, "family", "ipv4");

                if (addr->ifa_netmask) {
                    struct sockaddr_in* mask = (struct sockaddr_in*)addr->ifa_netmask;
                    uint32_t m = ntohl(mask->sin_addr.s_addr);
                    int prefix = 0;
                    while (m) {
                        prefix += (m & 1);
                        m >>= 1;
                    }
                    json_object_set_int(ip_obj, "prefix_length", prefix);
                }

                json_array_append(ips, ip_obj);
            } else if (addr->ifa_addr->sa_family == AF_INET6) {
                char ip[INET6_ADDRSTRLEN];
                struct sockaddr_in6* sin6 = (struct sockaddr_in6*)addr->ifa_addr;
                inet_ntop(AF_INET6, &sin6->sin6_addr, ip, sizeof(ip));

                JsonValue* ip_obj = json_object();
                json_object_set_string(ip_obj, "address", ip);
                json_object_set_string(ip_obj, "family", "ipv6");
                json_array_append(ips, ip_obj);
            }
#ifdef AF_PACKET
            else if (addr->ifa_addr->sa_family == AF_PACKET) {
                struct sockaddr_ll* sll = (struct sockaddr_ll*)addr->ifa_addr;
                if (sll->sll_halen == 6) {
                    char mac[32];
                    safe_snprintf(mac, sizeof(mac), "%02X:%02X:%02X:%02X:%02X:%02X",
                                 sll->sll_addr[0], sll->sll_addr[1], sll->sll_addr[2],
                                 sll->sll_addr[3], sll->sll_addr[4], sll->sll_addr[5]);
                    json_object_set_string(iface, "mac", mac);
                }
            }
#endif
        }
        json_object_set(iface, "addresses", ips);

        json_array_append(arr, iface);
    }

    freeifaddrs(ifaddr);

    *output = json_stringify(arr);
    json_free(arr);
    return 0;
}

static int list_connections_unix(char** output) {
    JsonValue* arr = json_array();

#ifdef __linux__
    /* Read /proc/net/tcp and /proc/net/udp */
    FILE* f = fopen("/proc/net/tcp", "r");
    if (f) {
        char line[512];
        fgets(line, sizeof(line), f); /* Skip header */

        while (fgets(line, sizeof(line), f)) {
            unsigned int local_addr, remote_addr;
            unsigned int local_port, remote_port;
            int state;
            unsigned int inode;

            if (sscanf(line, "%*d: %X:%X %X:%X %X %*X:%*X %*X:%*X %*X %*d %*d %u",
                      &local_addr, &local_port, &remote_addr, &remote_port, &state, &inode) >= 5) {
                JsonValue* conn = json_object();

                json_object_set_string(conn, "protocol", "tcp");

                struct in_addr la, ra;
                la.s_addr = local_addr;
                ra.s_addr = remote_addr;

                char local_ip[INET_ADDRSTRLEN];
                char remote_ip[INET_ADDRSTRLEN];
                inet_ntop(AF_INET, &la, local_ip, sizeof(local_ip));
                inet_ntop(AF_INET, &ra, remote_ip, sizeof(remote_ip));

                json_object_set_string(conn, "local_address", local_ip);
                json_object_set_int(conn, "local_port", (int)local_port);
                json_object_set_string(conn, "remote_address", remote_ip);
                json_object_set_int(conn, "remote_port", (int)remote_port);

                const char* state_str = "unknown";
                switch (state) {
                    case 1: state_str = "established"; break;
                    case 2: state_str = "syn_sent"; break;
                    case 3: state_str = "syn_rcvd"; break;
                    case 4: state_str = "fin_wait1"; break;
                    case 5: state_str = "fin_wait2"; break;
                    case 6: state_str = "time_wait"; break;
                    case 7: state_str = "closed"; break;
                    case 8: state_str = "close_wait"; break;
                    case 9: state_str = "last_ack"; break;
                    case 10: state_str = "listen"; break;
                    case 11: state_str = "closing"; break;
                }
                json_object_set_string(conn, "state", state_str);
                json_object_set_int(conn, "inode", (int)inode);

                json_array_append(arr, conn);
            }
        }
        fclose(f);
    }

    f = fopen("/proc/net/udp", "r");
    if (f) {
        char line[512];
        fgets(line, sizeof(line), f);

        while (fgets(line, sizeof(line), f)) {
            unsigned int local_addr;
            unsigned int local_port;
            unsigned int inode;

            if (sscanf(line, "%*d: %X:%X %*X:%*X %*X %*X:%*X %*X:%*X %*X %*d %*d %u",
                      &local_addr, &local_port, &inode) >= 2) {
                JsonValue* conn = json_object();

                json_object_set_string(conn, "protocol", "udp");

                struct in_addr la;
                la.s_addr = local_addr;
                char local_ip[INET_ADDRSTRLEN];
                inet_ntop(AF_INET, &la, local_ip, sizeof(local_ip));

                json_object_set_string(conn, "local_address", local_ip);
                json_object_set_int(conn, "local_port", (int)local_port);
                json_object_set_string(conn, "state", "listening");
                json_object_set_int(conn, "inode", (int)inode);

                json_array_append(arr, conn);
            }
        }
        fclose(f);
    }
#else
    /* macOS: Use netstat command */
    char* cmd_output = NULL;
    if (run_command("netstat -an 2>/dev/null | grep -E '^(tcp|udp)' | head -100", &cmd_output, NULL) == 0 && cmd_output) {
        char* line = strtok(cmd_output, "\n");
        while (line) {
            char proto[8], local[64], remote[64], state[32];
            if (sscanf(line, "%7s %*s %*s %63s %63s %31s", proto, local, remote, state) >= 3) {
                JsonValue* conn = json_object();

                json_object_set_string(conn, "protocol", proto[0] == 't' ? "tcp" : "udp");

                /* Parse local address:port */
                char* port_sep = strrchr(local, '.');
                if (port_sep) {
                    *port_sep = '\0';
                    json_object_set_string(conn, "local_address", local);
                    json_object_set_int(conn, "local_port", atoi(port_sep + 1));
                }

                /* Parse remote address:port */
                port_sep = strrchr(remote, '.');
                if (port_sep && strcmp(remote, "*.*") != 0) {
                    *port_sep = '\0';
                    json_object_set_string(conn, "remote_address", remote);
                    json_object_set_int(conn, "remote_port", atoi(port_sep + 1));
                }

                json_object_set_string(conn, "state", state);

                json_array_append(arr, conn);
            }
            line = strtok(NULL, "\n");
        }
        free(cmd_output);
    }
#endif

    *output = json_stringify(arr);
    json_free(arr);
    return 0;
}

static int show_routing_unix(char** output) {
    JsonValue* arr = json_array();

#ifdef __linux__
    FILE* f = fopen("/proc/net/route", "r");
    if (f) {
        char line[256];
        fgets(line, sizeof(line), f); /* Skip header */

        while (fgets(line, sizeof(line), f)) {
            char iface[32];
            unsigned int dest, gateway, mask;
            int flags, metric;

            if (sscanf(line, "%31s %X %X %d %*d %*d %d %X",
                      iface, &dest, &gateway, &flags, &metric, &mask) >= 5) {
                JsonValue* route = json_object();

                struct in_addr d, g, m;
                d.s_addr = dest;
                g.s_addr = gateway;
                m.s_addr = mask;

                char dest_ip[INET_ADDRSTRLEN];
                char gw_ip[INET_ADDRSTRLEN];
                char mask_ip[INET_ADDRSTRLEN];

                inet_ntop(AF_INET, &d, dest_ip, sizeof(dest_ip));
                inet_ntop(AF_INET, &g, gw_ip, sizeof(gw_ip));
                inet_ntop(AF_INET, &m, mask_ip, sizeof(mask_ip));

                json_object_set_string(route, "destination", dest_ip);
                json_object_set_string(route, "gateway", gw_ip);
                json_object_set_string(route, "netmask", mask_ip);
                json_object_set_string(route, "interface", iface);
                json_object_set_int(route, "metric", metric);
                json_object_set_int(route, "flags", flags);

                json_array_append(arr, route);
            }
        }
        fclose(f);
    }
#else
    /* macOS: Use netstat -rn */
    char* cmd_output = NULL;
    if (run_command("netstat -rn 2>/dev/null | grep -v '^Routing'", &cmd_output, NULL) == 0 && cmd_output) {
        char* line = strtok(cmd_output, "\n");
        int skip_header = 1;

        while (line) {
            if (skip_header) {
                skip_header = 0;
                line = strtok(NULL, "\n");
                continue;
            }

            char dest[64], gateway[64], flags[16], iface[32];
            if (sscanf(line, "%63s %63s %15s %*s %*s %31s", dest, gateway, flags, iface) >= 4) {
                JsonValue* route = json_object();

                json_object_set_string(route, "destination", dest);
                json_object_set_string(route, "gateway", gateway);
                json_object_set_string(route, "flags", flags);
                json_object_set_string(route, "interface", iface);

                json_array_append(arr, route);
            }
            line = strtok(NULL, "\n");
        }
        free(cmd_output);
    }
#endif

    *output = json_stringify(arr);
    json_free(arr);
    return 0;
}

#endif

/* DNS configuration (cross-platform) */
static int show_dns(char** output) {
    JsonValue* dns = json_object();
    JsonValue* servers = json_array();

#ifdef _WIN32
    /* Windows: Get DNS from network adapters */
    ULONG buf_size = 15000;
    PIP_ADAPTER_ADDRESSES addresses = (PIP_ADAPTER_ADDRESSES)malloc(buf_size);

    if (addresses && GetAdaptersAddresses(AF_UNSPEC, GAA_FLAG_INCLUDE_PREFIX, NULL, addresses, &buf_size) == NO_ERROR) {
        for (PIP_ADAPTER_ADDRESSES addr = addresses; addr != NULL; addr = addr->Next) {
            for (PIP_ADAPTER_DNS_SERVER_ADDRESS dns_addr = addr->FirstDnsServerAddress; dns_addr != NULL; dns_addr = dns_addr->Next) {
                char dns_ip[INET6_ADDRSTRLEN] = {0};
                struct sockaddr* sa = dns_addr->Address.lpSockaddr;

                if (sa->sa_family == AF_INET) {
                    struct sockaddr_in* sin = (struct sockaddr_in*)sa;
                    inet_ntop(AF_INET, &sin->sin_addr, dns_ip, sizeof(dns_ip));
                } else if (sa->sa_family == AF_INET6) {
                    struct sockaddr_in6* sin6 = (struct sockaddr_in6*)sa;
                    inet_ntop(AF_INET6, &sin6->sin6_addr, dns_ip, sizeof(dns_ip));
                }

                if (strlen(dns_ip) > 0) {
                    /* Check for duplicates */
                    int found = 0;
                    /* Just add it - we'll handle dedup in frontend if needed */
                    json_array_append(servers, json_string(dns_ip));
                }
            }
        }
    }
    if (addresses) free(addresses);
#else
    /* Unix: Read /etc/resolv.conf */
    FILE* f = fopen("/etc/resolv.conf", "r");
    if (f) {
        char line[256];
        while (fgets(line, sizeof(line), f)) {
            if (strncmp(line, "nameserver", 10) == 0) {
                char server[64];
                if (sscanf(line, "nameserver %63s", server) == 1) {
                    json_array_append(servers, json_string(server));
                }
            } else if (strncmp(line, "search", 6) == 0) {
                char domain[256];
                if (sscanf(line, "search %255s", domain) == 1) {
                    json_object_set_string(dns, "search_domain", domain);
                }
            }
        }
        fclose(f);
    }
#endif

    json_object_set(dns, "servers", servers);

    *output = json_stringify(dns);
    json_free(dns);
    return 0;
}

/* ============================================================================
 * Module Interface Implementation
 * ============================================================================ */

static int netinfo_init(void) {
#ifdef _WIN32
    WSADATA wsa;
    WSAStartup(MAKEWORD(2, 2), &wsa);
#endif
    return 0;
}

static int netinfo_execute(const char* command, const char* params, char** output) {
    (void)params;

    if (!output) return -1;

    if (!command || strcmp(command, "interfaces") == 0) {
#ifdef _WIN32
        return list_interfaces_win32(output);
#else
        return list_interfaces_unix(output);
#endif
    }

    if (strcmp(command, "connections") == 0) {
#ifdef _WIN32
        return list_connections_win32(output);
#else
        return list_connections_unix(output);
#endif
    }

    if (strcmp(command, "routing") == 0) {
#ifdef _WIN32
        return show_routing_win32(output);
#else
        return show_routing_unix(output);
#endif
    }

    if (strcmp(command, "dns") == 0) {
        return show_dns(output);
    }

    *output = safe_strdup("{\"error\":\"Unknown command. Use: interfaces, connections, routing, dns\"}");
    return -1;
}

static void netinfo_cleanup(void) {
#ifdef _WIN32
    WSACleanup();
#endif
}

/* Module definition */
Module mod_netinfo = {
    .name = "netinfo",
    .description = "Network information",
    .init = netinfo_init,
    .execute = netinfo_execute,
    .cleanup = netinfo_cleanup,
    .initialized = 0
};
