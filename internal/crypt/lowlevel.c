/*******************************************************************************
* Copyright 2023 Stefan Majewsky <majewsky@gmx.net>
* SPDX-License-Identifier: GPL-3.0-only
* Refer to the file "LICENSE" for details.
*******************************************************************************/

#include "lowlevel.h"

const char* feature_test() {
#ifndef CRYPT_GENSALT_IMPLEMENTS_DEFAULT_PREFIX
	return "libcrypt does not support CRYPT_GENSALT_IMPLEMENTS_DEFAULT_PREFIX";
#endif
#ifndef CRYPT_GENSALT_IMPLEMENTS_AUTO_ENTROPY
	return "libcrypt does not support CRYPT_GENSALT_IMPLEMENTS_AUTO_ENTROPY";
#endif
	return NULL;
}

char *wrap_crypt_r(const char* phrase, const char* setting) {
	struct crypt_data data;
	memset(&data, 0, sizeof(struct crypt_data));

	char* result = crypt_r(phrase, setting, &data);
	if (result == NULL) {
		return NULL;
	} else {
		return strdup(data.output); //`data` is allocated on the stack
	}
}

char *wrap_crypt_gensalt_rn(const char* prefix) {
	if (strlen(prefix) == 0) {
		prefix = NULL;
	}

	char buf[CRYPT_GENSALT_OUTPUT_SIZE];
	char* result = crypt_gensalt_rn(prefix, 0, NULL, 0, buf, sizeof(buf));
	if (result == NULL) {
		return NULL;
	} else {
		return strndup(buf, CRYPT_GENSALT_OUTPUT_SIZE); //`buf` is allocated on the stack
	}
}
