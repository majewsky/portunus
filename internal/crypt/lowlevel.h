/*******************************************************************************
* Copyright 2023 Stefan Majewsky <majewsky@gmx.net>
* SPDX-License-Identifier: GPL-3.0-only
* Refer to the file "LICENSE" for details.
*******************************************************************************/

#ifndef PORTUNUS_CRYPT_LOWLEVEL_H
#define PORTUNUS_CRYPT_LOWLEVEL_H

#define _GNU_SOURCE
#include <crypt.h>
#include <stdlib.h>
#include <string.h>

// Tests for libxcrypt features that we require.
// On success, returns NULL. On error, returns an error message.
const char* feature_test();

// Wrapper for crypt_r(). Used on the Go side in llCrypt().
char *wrap_crypt_r(const char* phrase, const char* setting);

// Wrapper for crypt_gensalt_rn(). Used on the Go side in llGenerateSalt().
char *wrap_crypt_gensalt_rn(const char* prefix);

#endif
