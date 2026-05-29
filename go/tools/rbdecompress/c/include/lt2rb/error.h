#ifndef LT2RB_ERROR_H
#define LT2RB_ERROR_H

const char *lt2rb_error(void);
void lt2rb_clear_error(void);
int lt2rb_set_error(const char *fmt, ...);

#endif
