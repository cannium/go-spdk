package main

/*
#include "spdk/nvme.h"
#include <stdio.h>

bool probe_callback(void *cb_ctx,
	const struct spdk_nvme_transport_id *trid,
	struct spdk_nvme_ctrlr_opts *opts) {

	bool probeCallback(void*,
		const struct spdk_nvme_transport_id*,
		struct spdk_nvme_ctrlr_opts*);
    return probeCallback(cb_ctx, trid, opts);
}

void attach_callback(void *cb_ctx,
	const struct spdk_nvme_transport_id *trid,
	struct spdk_nvme_ctrlr *ctrlr,
	const struct spdk_nvme_ctrlr_opts *opts) {

	void attachCallback(void*,
		const struct spdk_nvme_transport_id*,
		struct spdk_nvme_ctrlr*,
		const struct spdk_nvme_ctrlr_opts*);
    attachCallback(cb_ctx, trid, ctrlr, opts);
}

void io_complete_callback(void *cb_ctx,
	const struct spdk_nvme_cpl *complete) {

    int *task_id = cb_ctx;
	void ioCompleteCallback(int,
		const struct spdk_nvme_cpl*);

    ioCompleteCallback(*task_id, complete);
}
*/
import "C"
