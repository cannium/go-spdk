package main

/*
#include "spdk/nvme.h"

bool get_pi_loc(struct spdk_nvme_ns *ns) {
	return spdk_nvme_ns_get_data(ns)->dps.md_start;
}

void memory_set(void *s, int c, size_t n) {
	memset(s, c, n);
}

bool complete_is_error(struct spdk_nvme_cpl *cpl) {
	return spdk_nvme_cpl_is_error(cpl);
}
*/
import "C"
