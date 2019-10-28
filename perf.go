package main

/*
#cgo CFLAGS: -I/root/dk/spdk/include
#cgo LDFLAGS: -L/root/dk/spdk/build/lib -lspdk -lspdk_env_dpdk
#cgo LDFLAGS: -L/root/dk/spdk/dpdk/build/lib -lrte_eal -lrte_mempool -lrte_ring -lrte_bus_pci

#include "spdk/stdinc.h"
#include "spdk/env.h"
#include "spdk/nvme.h"

bool probe_callback(void *cb_ctx,
	const struct spdk_nvme_transport_id *trid,
	struct spdk_nvme_ctrlr_opts *opts);
void attach_callback(void *cb_ctx, 
	const struct spdk_nvme_transport_id *trid,
	struct spdk_nvme_ctrlr *ctrlr,
	const struct spdk_nvme_ctrlr_opts *opts);
*/
import "C"

import (
	"fmt"
	"math"
	"unsafe"
)

type workerThread struct {
	namespaceContext *C.struct_ns_worker_ctx
	next             *workerThread
	lcore            uint
}

var g_workers *workerThread
var g_num_workers = 0

func registerWorkers() {
	var i C.uint32_t
	g_workers = nil
	for i = C.spdk_env_get_first_core(); i < math.MaxUint32; i = C.spdk_env_get_next_core(i) {
		worker := &workerThread{}
		worker.lcore = uint(i)
		worker.next = g_workers
		g_workers = worker
		g_num_workers += 1
	}
}

//export probeCallback
func probeCallback(callbackContext unsafe.Pointer, 
	transportID *C.struct_spdk_nvme_transport_id,
	opts *C.struct_spdk_nvme_ctrlr_opts) bool {

	traddr := C.GoBytes(unsafe.Pointer(&transportID.traddr), 257)
	fmt.Println("Attaching to NVMe controller at", string(traddr))
	opts.io_queue_size = 65535
	opts.header_digest = false
	opts.data_digest = false
	opts.keep_alive_timeout_ms = 0
	return true
}

func registerController(controller *C.struct_spdk_nvme_ctrlr) {

}

//export attachCallback
func attachCallback(callbackContext unsafe.Pointer, 
	transportID *C.struct_spdk_nvme_transport_id,
	controller *C.struct_spdk_nvme_ctrlr, opts *C.struct_spdk_nvme_ctrlr_opts) {

	pci_addr := &C.struct_spdk_pci_addr{};
	returnValue, _ := C.spdk_pci_addr_parse(pci_addr, &transportID.traddr[0])
	if returnValue != 0 {
		fmt.Println("spdk_pci_addr_parse error")
		return
	}
	pci_dev, _ := C.spdk_nvme_ctrlr_get_pci_device(controller)
	if pci_dev == nil {
		fmt.Println("spdk_nvme_ctrlr_get_pci_device not found")
		return
	}
	pci_id, _ := C.spdk_pci_device_get_id(pci_dev)

	traddr := C.GoBytes(unsafe.Pointer(&transportID.traddr), 257)
	fmt.Printf("Attached to NVMe controller at %s [%04x:%04x]\n",
		string(traddr), pci_id.vendor_id, pci_id.device_id)
}

func registerConterollers() int {
	fmt.Println("Initializing NVMe Controllers")
	returnValue, _ := C.spdk_nvme_probe(nil, nil,
		C.spdk_nvme_probe_cb(C.probe_callback), 
		C.spdk_nvme_attach_cb(C.attach_callback), nil)
	if returnValue != 0 {
		fmt.Println("spdk_nvme_probe failed")
		return -1
	}
	return 0
}

func main() {
	opts := C.struct_spdk_env_opts{}
	C.spdk_env_opts_init(&opts)
	opts.name = C.CString("perf")
	opts.shm_id = -1
	fmt.Println("env_opts", opts)
	returnValue, _ := C.spdk_env_init(&opts)
	if returnValue < 0 {
		fmt.Println("Unable to initialize SPDK env:", returnValue)
		return
	}

	// g_tsc_rate := C.spdk_get_ticks_hz()

	registerWorkers()

	registerConterollers()

}
