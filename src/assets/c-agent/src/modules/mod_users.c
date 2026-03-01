/*
 * NodePulse Agent - User Accounts Module
 *
 * Provides user account listing and information.
 * Commands:
 *   - list: List all user accounts
 *   - current: Get current logged-in user info
 *   - info:<username>: Get specific user information
 */

#include "modules.h"
#include "../utils/safe_string.h"
#include "../utils/json.h"
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#ifdef _WIN32
#include <windows.h>
#include <lm.h>
#include <sddl.h>
#pragma comment(lib, "netapi32.lib")
#pragma comment(lib, "advapi32.lib")
#else
#include <pwd.h>
#include <grp.h>
#include <unistd.h>
#include <sys/types.h>
#endif

/* ============================================================================
 * Platform-Specific Implementation
 * ============================================================================ */

#ifdef _WIN32

static int list_users_win32(char** output) {
    JsonValue* arr = json_array();

    /* Enumerate local users */
    LPUSER_INFO_2 user_buf = NULL;
    DWORD entries_read = 0;
    DWORD total_entries = 0;
    DWORD resume_handle = 0;
    NET_API_STATUS status;

    status = NetUserEnum(NULL, 2, FILTER_NORMAL_ACCOUNT,
                        (LPBYTE*)&user_buf, MAX_PREFERRED_LENGTH,
                        &entries_read, &total_entries, &resume_handle);

    if (status == NERR_Success || status == ERROR_MORE_DATA) {
        for (DWORD i = 0; i < entries_read; i++) {
            JsonValue* user = json_object();

            /* Convert wide string to char */
            char username[256] = {0};
            char fullname[256] = {0};
            char comment[512] = {0};
            char home_dir[512] = {0};

            WideCharToMultiByte(CP_UTF8, 0, user_buf[i].usri2_name, -1,
                               username, sizeof(username), NULL, NULL);
            WideCharToMultiByte(CP_UTF8, 0, user_buf[i].usri2_full_name, -1,
                               fullname, sizeof(fullname), NULL, NULL);
            WideCharToMultiByte(CP_UTF8, 0, user_buf[i].usri2_comment, -1,
                               comment, sizeof(comment), NULL, NULL);
            WideCharToMultiByte(CP_UTF8, 0, user_buf[i].usri2_home_dir, -1,
                               home_dir, sizeof(home_dir), NULL, NULL);

            json_object_set_string(user, "username", username);
            json_object_set_string(user, "full_name", fullname);
            json_object_set_string(user, "comment", comment);
            json_object_set_string(user, "home_dir", home_dir);

            /* Account flags */
            DWORD flags = user_buf[i].usri2_flags;
            json_object_set_bool(user, "disabled", (flags & UF_ACCOUNTDISABLE) ? 1 : 0);
            json_object_set_bool(user, "locked", (flags & UF_LOCKOUT) ? 1 : 0);
            json_object_set_bool(user, "password_never_expires",
                                (flags & UF_DONT_EXPIRE_PASSWD) ? 1 : 0);

            /* Privilege level */
            const char* priv = "user";
            if (user_buf[i].usri2_priv == USER_PRIV_ADMIN) priv = "admin";
            else if (user_buf[i].usri2_priv == USER_PRIV_GUEST) priv = "guest";
            json_object_set_string(user, "privilege", priv);

            /* Last logon */
            if (user_buf[i].usri2_last_logon > 0) {
                json_object_set_int(user, "last_logon", (int)user_buf[i].usri2_last_logon);
            }

            json_array_append(arr, user);
        }
        NetApiBufferFree(user_buf);
    }

    *output = json_stringify(arr);
    json_free(arr);
    return 0;
}

static int get_current_user_win32(char** output) {
    JsonValue* user = json_object();

    /* Get username */
    char username[256] = {0};
    DWORD size = sizeof(username);
    if (GetUserNameA(username, &size)) {
        json_object_set_string(user, "username", username);
    }

    /* Get computer name */
    char computer[256] = {0};
    size = sizeof(computer);
    if (GetComputerNameA(computer, &size)) {
        json_object_set_string(user, "computer", computer);
    }

    /* Get user SID */
    HANDLE token;
    if (OpenProcessToken(GetCurrentProcess(), TOKEN_QUERY, &token)) {
        DWORD token_info_len;
        GetTokenInformation(token, TokenUser, NULL, 0, &token_info_len);

        TOKEN_USER* token_user = (TOKEN_USER*)malloc(token_info_len);
        if (token_user) {
            if (GetTokenInformation(token, TokenUser, token_user, token_info_len, &token_info_len)) {
                LPSTR sid_string = NULL;
                if (ConvertSidToStringSidA(token_user->User.Sid, &sid_string)) {
                    json_object_set_string(user, "sid", sid_string);
                    LocalFree(sid_string);
                }
            }
            free(token_user);
        }

        /* Check if admin */
        TOKEN_ELEVATION elevation;
        GetTokenInformation(token, TokenElevation, &elevation, sizeof(elevation), &token_info_len);
        json_object_set_bool(user, "elevated", elevation.TokenIsElevated ? 1 : 0);

        CloseHandle(token);
    }

    /* Get user profile path */
    char profile_path[MAX_PATH] = {0};
    DWORD profile_size = sizeof(profile_path);
    if (GetEnvironmentVariableA("USERPROFILE", profile_path, profile_size)) {
        json_object_set_string(user, "profile_path", profile_path);
    }

    *output = json_stringify(user);
    json_free(user);
    return 0;
}

static int get_user_info_win32(const char* username, char** output) {
    JsonValue* user = json_object();

    /* Convert username to wide string */
    WCHAR wusername[256];
    MultiByteToWideChar(CP_UTF8, 0, username, -1, wusername, 256);

    LPUSER_INFO_4 user_info = NULL;
    NET_API_STATUS status = NetUserGetInfo(NULL, wusername, 4, (LPBYTE*)&user_info);

    if (status == NERR_Success && user_info) {
        char fullname[256] = {0};
        char comment[512] = {0};
        char home_dir[512] = {0};
        char script_path[512] = {0};

        WideCharToMultiByte(CP_UTF8, 0, user_info->usri4_full_name, -1,
                           fullname, sizeof(fullname), NULL, NULL);
        WideCharToMultiByte(CP_UTF8, 0, user_info->usri4_comment, -1,
                           comment, sizeof(comment), NULL, NULL);
        WideCharToMultiByte(CP_UTF8, 0, user_info->usri4_home_dir, -1,
                           home_dir, sizeof(home_dir), NULL, NULL);
        WideCharToMultiByte(CP_UTF8, 0, user_info->usri4_script_path, -1,
                           script_path, sizeof(script_path), NULL, NULL);

        json_object_set_string(user, "username", username);
        json_object_set_string(user, "full_name", fullname);
        json_object_set_string(user, "comment", comment);
        json_object_set_string(user, "home_dir", home_dir);
        json_object_set_string(user, "script_path", script_path);

        DWORD flags = user_info->usri4_flags;
        json_object_set_bool(user, "disabled", (flags & UF_ACCOUNTDISABLE) ? 1 : 0);
        json_object_set_bool(user, "locked", (flags & UF_LOCKOUT) ? 1 : 0);
        json_object_set_bool(user, "password_never_expires",
                            (flags & UF_DONT_EXPIRE_PASSWD) ? 1 : 0);

        const char* priv = "user";
        if (user_info->usri4_priv == USER_PRIV_ADMIN) priv = "admin";
        else if (user_info->usri4_priv == USER_PRIV_GUEST) priv = "guest";
        json_object_set_string(user, "privilege", priv);

        json_object_set_int(user, "last_logon", (int)user_info->usri4_last_logon);
        json_object_set_int(user, "num_logons", (int)user_info->usri4_num_logons);
        json_object_set_int(user, "bad_pw_count", (int)user_info->usri4_bad_pw_count);

        /* Get user SID */
        if (user_info->usri4_user_sid) {
            LPSTR sid_string = NULL;
            if (ConvertSidToStringSidA(user_info->usri4_user_sid, &sid_string)) {
                json_object_set_string(user, "sid", sid_string);
                LocalFree(sid_string);
            }
        }

        NetApiBufferFree(user_info);
    } else {
        json_object_set_string(user, "error", "User not found");
    }

    *output = json_stringify(user);
    json_free(user);
    return 0;
}

#else /* Linux/macOS */

static int list_users_unix(char** output) {
    JsonValue* arr = json_array();

    struct passwd* pwd;
    setpwent();

    while ((pwd = getpwent()) != NULL) {
        JsonValue* user = json_object();

        json_object_set_string(user, "username", pwd->pw_name);
        json_object_set_int(user, "uid", (int)pwd->pw_uid);
        json_object_set_int(user, "gid", (int)pwd->pw_gid);
        json_object_set_string(user, "home_dir", pwd->pw_dir);
        json_object_set_string(user, "shell", pwd->pw_shell);

        /* Get GECOS field (usually full name) */
        if (pwd->pw_gecos && strlen(pwd->pw_gecos) > 0) {
            /* Extract just the name part (before first comma) */
            char gecos[256];
            safe_strcpy(gecos, sizeof(gecos), pwd->pw_gecos);
            char* comma = strchr(gecos, ',');
            if (comma) *comma = '\0';
            json_object_set_string(user, "full_name", gecos);
        }

        /* Determine if system account (UID < 1000 on most systems) */
        json_object_set_bool(user, "system_account", pwd->pw_uid < 1000 ? 1 : 0);

        /* Get primary group name */
        struct group* grp = getgrgid(pwd->pw_gid);
        if (grp) {
            json_object_set_string(user, "primary_group", grp->gr_name);
        }

        json_array_append(arr, user);
    }

    endpwent();

    *output = json_stringify(arr);
    json_free(arr);
    return 0;
}

static int get_current_user_unix(char** output) {
    JsonValue* user = json_object();

    uid_t uid = getuid();
    uid_t euid = geteuid();
    gid_t gid = getgid();

    struct passwd* pwd = getpwuid(uid);
    if (pwd) {
        json_object_set_string(user, "username", pwd->pw_name);
        json_object_set_string(user, "home_dir", pwd->pw_dir);
        json_object_set_string(user, "shell", pwd->pw_shell);

        if (pwd->pw_gecos && strlen(pwd->pw_gecos) > 0) {
            char gecos[256];
            safe_strcpy(gecos, sizeof(gecos), pwd->pw_gecos);
            char* comma = strchr(gecos, ',');
            if (comma) *comma = '\0';
            json_object_set_string(user, "full_name", gecos);
        }
    }

    json_object_set_int(user, "uid", (int)uid);
    json_object_set_int(user, "euid", (int)euid);
    json_object_set_int(user, "gid", (int)gid);

    /* Check if running as root */
    json_object_set_bool(user, "elevated", euid == 0 ? 1 : 0);

    /* Get hostname */
    char hostname[256] = {0};
    if (gethostname(hostname, sizeof(hostname)) == 0) {
        json_object_set_string(user, "hostname", hostname);
    }

    /* Get primary group name */
    struct group* grp = getgrgid(gid);
    if (grp) {
        json_object_set_string(user, "primary_group", grp->gr_name);
    }

    /* Get supplementary groups */
    int ngroups = 32;
    gid_t groups[32];
#ifdef __APPLE__
    int result = getgroups(ngroups, groups);
#else
    int result = getgroups(ngroups, groups);
#endif
    if (result > 0) {
        JsonValue* groups_arr = json_array();
        for (int i = 0; i < result; i++) {
            struct group* g = getgrgid(groups[i]);
            if (g) {
                JsonValue* grp_name = json_string(g->gr_name);
                json_array_append(groups_arr, grp_name);
            }
        }
        json_object_set(user, "groups", groups_arr);
    }

    *output = json_stringify(user);
    json_free(user);
    return 0;
}

static int get_user_info_unix(const char* username, char** output) {
    JsonValue* user = json_object();

    struct passwd* pwd = getpwnam(username);
    if (!pwd) {
        json_object_set_string(user, "error", "User not found");
        *output = json_stringify(user);
        json_free(user);
        return -1;
    }

    json_object_set_string(user, "username", pwd->pw_name);
    json_object_set_int(user, "uid", (int)pwd->pw_uid);
    json_object_set_int(user, "gid", (int)pwd->pw_gid);
    json_object_set_string(user, "home_dir", pwd->pw_dir);
    json_object_set_string(user, "shell", pwd->pw_shell);

    if (pwd->pw_gecos && strlen(pwd->pw_gecos) > 0) {
        char gecos[256];
        safe_strcpy(gecos, sizeof(gecos), pwd->pw_gecos);
        char* comma = strchr(gecos, ',');
        if (comma) *comma = '\0';
        json_object_set_string(user, "full_name", gecos);
    }

    json_object_set_bool(user, "system_account", pwd->pw_uid < 1000 ? 1 : 0);

    /* Get primary group name */
    struct group* grp = getgrgid(pwd->pw_gid);
    if (grp) {
        json_object_set_string(user, "primary_group", grp->gr_name);
    }

    /* Get all groups this user belongs to */
    JsonValue* groups_arr = json_array();
    setgrent();
    struct group* g;
    while ((g = getgrent()) != NULL) {
        for (char** member = g->gr_mem; *member != NULL; member++) {
            if (strcmp(*member, username) == 0) {
                JsonValue* grp_name = json_string(g->gr_name);
                json_array_append(groups_arr, grp_name);
                break;
            }
        }
    }
    endgrent();
    json_object_set(user, "groups", groups_arr);

    *output = json_stringify(user);
    json_free(user);
    return 0;
}

#endif

/* ============================================================================
 * Module Interface Implementation
 * ============================================================================ */

static int users_init(void) {
    return 0;
}

static int users_execute(const char* command, const char* params, char** output) {
    (void)params;

    if (!output) return -1;

    if (!command || strcmp(command, "list") == 0) {
#ifdef _WIN32
        return list_users_win32(output);
#else
        return list_users_unix(output);
#endif
    }

    if (strcmp(command, "current") == 0) {
#ifdef _WIN32
        return get_current_user_win32(output);
#else
        return get_current_user_unix(output);
#endif
    }

    if (strncmp(command, "info:", 5) == 0) {
        const char* username = command + 5;
        if (!username || strlen(username) == 0) {
            *output = safe_strdup("{\"error\":\"Username required\"}");
            return -1;
        }
#ifdef _WIN32
        return get_user_info_win32(username, output);
#else
        return get_user_info_unix(username, output);
#endif
    }

    *output = safe_strdup("{\"error\":\"Unknown command. Use: list, current, info:<username>\"}");
    return -1;
}

static void users_cleanup(void) {
    /* Nothing to clean up */
}

/* Module definition */
Module mod_users = {
    .name = "users",
    .description = "User account management",
    .init = users_init,
    .execute = users_execute,
    .cleanup = users_cleanup,
    .initialized = 0
};
