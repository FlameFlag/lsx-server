/*
 * Recovered LSX client/server compatibility contract.
 *
 * This file is a C translation of the recovered behavior documented in
 * web/static/project/findings/content.md and implemented in this Go server.
 */
#ifndef LT2_LSX_CLIENT_PROTOCOL_H
#define LT2_LSX_CLIENT_PROTOCOL_H

#include <stdint.h>

#define LT2_LSX_HTTP_RESULT_OK 1

typedef struct Lt2LsxChecksumFields {
    int32_t gamestartingdate;
    int32_t revenues;
    int32_t stands;
    int32_t lifespan;
    int32_t gamegoal;
    int32_t standsassets;
    int32_t cupssold;
    int32_t gamemode;
    int32_t retainedearnings;
    int32_t upgradesassets;
    int32_t cashassets;
} Lt2LsxChecksumFields;

typedef struct Lt2PackedDate {
    int16_t year;
    int16_t month;
    int16_t day;
    int16_t hour;
    int16_t unused_0008;
    int16_t minute;
    int16_t second;
    int16_t millisecond;
} Lt2PackedDate;

int32_t lt2_lsx_compute_checksum(const Lt2LsxChecksumFields *fields);
int32_t lt2_lsx_packed_date_scalar(const Lt2PackedDate *date);
int lt2_lsx_client_sets_request_flag(int request_result, const char *body,
    const char *token);

#endif /* LT2_LSX_CLIENT_PROTOCOL_H */
