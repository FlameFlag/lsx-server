#pragma once

#include <cstddef>
#include <cstdint>

namespace keyvis {

void init_crc_table();
uint32_t crc_step(uint32_t crc, uint8_t byte);
uint32_t crc(const char *s, size_t n);
uint32_t hex_crc_group(uint32_t value, int group_size);

} // namespace keyvis
