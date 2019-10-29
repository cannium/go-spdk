package main

/*
#include "spdk/nvme.h"

bool get_pi_loc(struct spdk_nvme_ns *ns) {
	return spdk_nvme_ns_get_data(ns)->dps.md_start;
}
*/
import "C"
