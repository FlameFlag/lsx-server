#include "lt2rb/error.h"

#include <stdarg.h>
#include <stdio.h>

static char g_error[1024];

const char *lt2rb_error(void)
{
    return g_error[0] == '\0' ? "failed" : g_error;
}

void lt2rb_clear_error(void)
{
    g_error[0] = '\0';
}

int lt2rb_set_error(const char *fmt, ...)
{
    va_list args;

    va_start(args, fmt);
    vsnprintf(g_error, sizeof(g_error), fmt, args);
    va_end(args);
    return 0;
}
