/*
 * Verified in decompiled/local/unpacked/Lemonade2.unpacked.exe:
 * - 0x004073C0: builds/submits score upload request.
 * - 0x0045FB70: builds/submits account creation request.
 * - 0x00410030: computes checksumclient from game-state fields.
 * - 0x00418FF0: converts the packed in-game date to the checksum scalar.
 * - 0x00420F10: builds local browser path Lsx\CheckConnection.html.
 *
 * Active host/path contract:
 * - Host: gt.jamdat.ca
 * - Account: GET /createaccount.php?username=...&password=...
 * - Upload:  GET /syncgame.php?game=lemonade2&...
 * - Browser/local: Lsx\CheckConnection.html or
 *   Lsx\CheckConnection.html?username=...
 *
 * Response checks:
 * - account token: "ACCEPT"
 * - upload token: "SUCCESS"
 *
 * The unpacked client sets its post-request flag when the expected token is
 * present, but also when the HTTP helper fails. Only a successful tokenless
 * response leaves the flag clear. Returning the expected token is still the
 * compatible server behavior.
 */
#include "network/lsx_client_protocol.h"

#include <string.h>

/* Matches x86 signed 32-bit overflow in the original client. */
int32_t lt2_lsx_compute_checksum(const Lt2LsxChecksumFields *fields)
{
    int32_t value;

    if (fields == NULL) {
        return 0;
    }

    value =
        fields->gamestartingdate *
        (fields->revenues - fields->stands * 100) *
        fields->lifespan +
        fields->gamegoal * 5 -
        fields->standsassets -
        fields->cupssold +
        fields->gamemode * 7 +
        fields->retainedearnings +
        fields->upgradesassets +
        fields->cashassets;

    return value;
}

/* Fixed 360-day-year, 30-day-month scalar; not a civil timestamp. */
int32_t lt2_lsx_packed_date_scalar(const Lt2PackedDate *date)
{
    int32_t value;

    if (date == NULL) {
        return 0;
    }

    value =
        date->year * (int32_t)0x3df16000 +
        date->month * (int32_t)-0x65813800 +
        (((date->day * 24 + date->hour) * 60 + date->minute) * 60 +
         date->second) * 1000 +
        date->millisecond;

    return value;
}

int lt2_lsx_client_sets_request_flag(int request_result, const char *body,
    const char *token)
{
    if (body != NULL && token != NULL && strstr(body, token) != NULL) {
        return 1;
    }

    return request_result != LT2_LSX_HTTP_RESULT_OK;
}
