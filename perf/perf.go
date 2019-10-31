package main

/*
#cgo CFLAGS: -I/root/dk/spdk/include
#cgo LDFLAGS: -L/root/dk/spdk/build/lib -lspdk -lspdk_env_dpdk
#cgo LDFLAGS: -L/root/dk/spdk/dpdk/build/lib -lrte_eal -lrte_mempool -lrte_ring -lrte_bus_pci

#include "spdk/stdinc.h"
#include "spdk/env.h"
#include "spdk/nvme.h"
#include "spdk/nvme_intel.h"

bool probe_callback(void *cb_ctx,
	const struct spdk_nvme_transport_id *trid,
	struct spdk_nvme_ctrlr_opts *opts);
void attach_callback(void *cb_ctx,
	const struct spdk_nvme_transport_id *trid,
	struct spdk_nvme_ctrlr *ctrlr,
	const struct spdk_nvme_ctrlr_opts *opts);
void io_complete_callback(void *cb_ctx,
	const struct spdk_nvme_cpl *complete);

bool get_pi_loc(struct spdk_nvme_ns *ns);
void memory_set(void *s, int c, size_t n);
bool complete_is_error(struct spdk_nvme_cpl *cpl);
*/
import "C"

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"unsafe"
)

const ioSizeBytes = 4096
const queueDepth = 128
const ioAlign = 0x200

var maxMetadataIoSize C.uint32_t = 0
var maxIoSizeBlocks C.uint32_t = 0

type workerThread struct {
	namespaceContext *C.struct_ns_worker_ctx
	next             *workerThread
	lcore            uint
}

var workers = make(map[uint]*workerThread)

func registerWorkers() {
	var i C.uint32_t
	for i = C.spdk_env_get_first_core(); i < math.MaxUint32; i = C.spdk_env_get_next_core(i) {
		worker := &workerThread{}
		worker.lcore = uint(i)
		workers[worker.lcore] = worker
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

type namespaceEntry struct {
	controller         *C.struct_spdk_nvme_ctrlr
	namespace          *C.struct_spdk_nvme_ns
	next               *namespaceEntry
	ioSizeBlocks       C.uint32_t
	numIoRequests      C.uint32_t
	sizeInIos          C.uint64_t
	blockSize          C.uint32_t
	metadataSize       C.uint32_t
	metadataInterleave C.bool
	piLoc              C.bool
	piType             C.enum_spdk_nvme_pi_type
	ioFlags            C.uint32_t
	name               string
}

var namespaces = make(map[string]*namespaceEntry)

func registerNamespace(controller *C.struct_spdk_nvme_ctrlr,
	namespace *C.struct_spdk_nvme_ns) {

	if !C.spdk_nvme_ns_is_active(namespace) {
		fmt.Println("Skipping inactive namespace")
		return
	}

	ns_size := C.spdk_nvme_ns_get_size(namespace)
	sector_size := C.spdk_nvme_ns_get_sector_size(namespace)
	if ns_size < ioSizeBytes || sector_size > ioSizeBytes {
		fmt.Println("invalid ns_size/sector_size for IO size")
		return
	}

	max_xfer_size := C.spdk_nvme_ns_get_max_io_xfer_size(namespace)
	ioQpairOpts := &C.struct_spdk_nvme_io_qpair_opts{}
	C.spdk_nvme_ctrlr_get_default_io_qpair_opts(controller, ioQpairOpts,
		C.sizeof_struct_spdk_nvme_io_qpair_opts)
	entryCount := (ioSizeBytes-1)/max_xfer_size + 2
	if (queueDepth * entryCount) > ioQpairOpts.io_queue_size {
		fmt.Println("Warn: Consider using lower queue depth or small IO size because " +
			"IO requests may be queued at the NVMe driver")
	}
	entryCount += 1

	entry := &namespaceEntry{
		controller:         controller,
		namespace:          namespace,
		numIoRequests:      queueDepth * entryCount,
		sizeInIos:          ns_size / ioSizeBytes,
		ioSizeBlocks:       ioSizeBytes / sector_size,
		blockSize:          C.spdk_nvme_ns_get_extended_sector_size(namespace),
		metadataSize:       C.spdk_nvme_ns_get_md_size(namespace),
		metadataInterleave: C.spdk_nvme_ns_supports_extended_lba(namespace),
		piLoc:              C.get_pi_loc(namespace), // no way to access bit field, use a polyfill function
		piType:             C.spdk_nvme_ns_get_pi_type(namespace),
	}
	if maxMetadataIoSize < entry.metadataSize {
		maxMetadataIoSize = entry.metadataSize
	}
	if maxIoSizeBlocks < entry.ioSizeBlocks {
		maxIoSizeBlocks = entry.metadataSize
	}
	transportID := C.spdk_nvme_ctrlr_get_transport_id(controller)
	traddr := C.GoBytes(unsafe.Pointer(&transportID.traddr), 257)
	entry.name = fmt.Sprintf("PCIE (%s)", string(traddr))

	namespaces[entry.name] = entry
}

type controllerEntry struct {
	nvmeController *C.struct_spdk_nvme_ctrlr
	transportType  C.enum_spdk_nvme_transport_type
	latencyPage    *C.struct_spdk_nvme_intel_rw_latency_page
	qPairs         []*C.struct_spdk_nvme_qpair
	next           *controllerEntry
	name           string
}

var controllers = make(map[string]*controllerEntry)

func registerController(controller *C.struct_spdk_nvme_ctrlr) {
	entry := &controllerEntry{}

	entry.latencyPage = (*C.struct_spdk_nvme_intel_rw_latency_page)(
		C.spdk_dma_zmalloc(C.sizeof_struct_spdk_nvme_intel_rw_latency_page,
			4096, nil))
	if entry.latencyPage == nil {
		fmt.Println("Allocation error (latency page)")
		os.Exit(1)
	}

	transportID := C.spdk_nvme_ctrlr_get_transport_id(controller)
	traddr := C.GoBytes(unsafe.Pointer(&transportID.traddr), 257)
	entry.name = fmt.Sprintf("PCIE (%s)", string(traddr))
	controllers[entry.name] = entry

	entry.nvmeController = controller
	entry.transportType = 256 // PCIE

	var namespaceID C.uint32_t
	for namespaceID = C.spdk_nvme_ctrlr_get_first_active_ns(controller); namespaceID != 0; namespaceID = C.spdk_nvme_ctrlr_get_next_active_ns(controller, namespaceID) {
		namespace := C.spdk_nvme_ctrlr_get_ns(controller, namespaceID)
		if namespace == nil {
			continue
		}
		registerNamespace(controller, namespace)
	}
}

//export attachCallback
func attachCallback(callbackContext unsafe.Pointer,
	transportID *C.struct_spdk_nvme_transport_id,
	controller *C.struct_spdk_nvme_ctrlr, opts *C.struct_spdk_nvme_ctrlr_opts) {

	pci_addr := &C.struct_spdk_pci_addr{}
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

	registerController(controller)
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

type namespaceWorkerContext struct {
	worker            *workerThread
	namespace         *namespaceEntry
	queuePair         *C.struct_spdk_nvme_qpair
	currentQueueDepth int
	ioCompleted       int64
	isDraining        bool
}

// we only support one core - namespace association
var associateContext namespaceWorkerContext

func associateWorkersWithNamespace() {
	if len(workers) != 1 || len(namespaces) != 1 {
		fmt.Println("unhandled case for go-spdk-perf")
		os.Exit(-1)
	}
	for lcore, worker := range workers {
		for name, space := range namespaces {
			fmt.Printf("Associating %s with lcore %d\n", name, lcore)
			associateContext = namespaceWorkerContext{
				worker:    worker,
				namespace: space,
			}
			return
		}
	}
}

func nvmeInitNamespaceWorkerContext() int {
	associateContext.queuePair = &C.struct_spdk_nvme_qpair{}
	qPairOptions := C.struct_spdk_nvme_io_qpair_opts{}

	C.spdk_nvme_ctrlr_get_default_io_qpair_opts(associateContext.namespace.controller,
		&qPairOptions,
		C.sizeof_struct_spdk_nvme_io_qpair_opts)
	if qPairOptions.io_queue_requests < associateContext.namespace.numIoRequests {
		qPairOptions.io_queue_requests = associateContext.namespace.numIoRequests
	}
	qPairOptions.delay_pcie_doorbell = true

	associateContext.queuePair = C.spdk_nvme_ctrlr_alloc_io_qpair(
		associateContext.namespace.controller,
		&qPairOptions,
		C.sizeof_struct_spdk_nvme_io_qpair_opts)
	if associateContext.queuePair == nil {
		fmt.Println("ERROR: spdk_nvme_ctrlr_alloc_io_qpair failed")
		return -1
	}
	return 0
}

type perfTask struct {
	ioVector  C.struct_iovec
	submitTsc C.uint64_t
	isRead    bool
	context   *namespaceWorkerContext
}

func allocateTask(payloadPattern int) perfTask {
	task := perfTask{
		context: &associateContext,
	}
	task.ioVector.iov_base = C.spdk_dma_zmalloc(ioSizeBytes, ioAlign, nil)
	if task.ioVector.iov_base == nil {
		fmt.Println("task.ioVector.iov_base spdk_dma_zmalloc failed")
		os.Exit(1)
	}
	task.ioVector.iov_len = ioSizeBytes
	pattern := payloadPattern%8 + 1
	C.memory_set(task.ioVector.iov_base, C.int(pattern), ioSizeBytes)
	return task
}

//export ioCompleteCallback
func ioCompleteCallback(callbackContext unsafe.Pointer,
	complete *C.struct_spdk_nvme_cpl) {

	if C.complete_is_error(complete) {
		fmt.Println("IO error")
	}

	associateContext.currentQueueDepth -= 1
	associateContext.ioCompleted += 1
	if associateContext.isDraining {
		return
	}
	task := tasks[rand.Int()%len(tasks)]
	submitTask(task)
}

func nvmeSubmitIO(task *perfTask, offsetInIos int64) int {
	var lba uint64 = uint64(offsetInIos * int64(task.context.namespace.ioSizeBlocks))
	rc := C.spdk_nvme_ns_cmd_write(task.context.namespace.namespace, task.context.queuePair,
		task.ioVector.iov_base, C.ulong(lba),
		task.context.namespace.ioSizeBlocks,
		C.spdk_nvme_cmd_cb(C.io_complete_callback), nil,
		task.context.namespace.ioFlags)
	return int(rc)
}

func submitTask(task *perfTask) {
	offset := rand.Int63() % int64(task.context.namespace.sizeInIos)
	task.submitTsc = C.spdk_get_ticks()
	task.isRead = false
	rc := nvmeSubmitIO(task, offset)
	if rc != 0 {
		fmt.Println("starting I/O failed")
	} else {
		task.context.currentQueueDepth += 1
	}
}

var tasks []*perfTask

func submitIO() {
	for i := queueDepth; i > 0; i-- {
		task := allocateTask(int(i))
		tasks = append(tasks, &task)
		submitTask(&task)
	}
}

func workerFunction(worker *workerThread) int {
	fmt.Println("Starting thread on core", worker.lcore)
	v := nvmeInitNamespaceWorkerContext()
	if v != 0 {
		fmt.Println("ERROR: init_ns_worker_ctx() failed")
		return 1
	}

	tsc_end := C.spdk_get_ticks() + C.uint64_t(g_time_in_sec)*g_tsc_rate

	submitIO()

	for {
		rc := C.spdk_nvme_qpair_process_completions(associateContext.queuePair, 0)
		if rc < 0 {
			fmt.Println("NVMe io qpair process completion error")
			os.Exit(1)
		}
		if C.spdk_get_ticks() > tsc_end {
			break
		}
	}

	// draining remaining tasks
	associateContext.isDraining = true
	for associateContext.currentQueueDepth > 0 {
		rc := C.spdk_nvme_qpair_process_completions(associateContext.queuePair, 0)
		if rc < 0 {
			fmt.Println("NVMe io qpair process completion error")
			os.Exit(1)
		}
	}
	return 0
}

var g_tsc_rate C.uint64_t
var g_time_in_sec C.int = 300

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

	g_tsc_rate = C.spdk_get_ticks_hz()

	registerWorkers()

	registerConterollers()

	fmt.Println("workers:", workers)
	fmt.Println("controllers:", controllers)
	fmt.Println("namespaces:")
	for name, space := range namespaces {
		fmt.Printf("%s %+v", name, space)
	}

	if len(namespaces) == 0 {
		fmt.Println("No valid NVMe controllers found")
		return
	}

	associateWorkersWithNamespace()

	fmt.Println("Initialization complete. Launching workers.")

	masterCore := C.spdk_env_get_current_core()
	var masterWorker *workerThread
	for lcore, worker := range workers {
		if lcore != uint(masterCore) {
			fmt.Println("lcore != masterCore", lcore, masterCore)
		} else {
			masterWorker = worker
		}
	}
	if masterWorker == nil {
		fmt.Println("master worker is nil")
		return
	}

	rc := workerFunction(masterWorker)

	C.spdk_env_thread_wait_all()

	printStats()
	os.Exit(rc)
}

func printStats() {
	fmt.Println("total IOs:", associateContext.ioCompleted,
		"IOPS:", float64(associateContext.ioCompleted)/float64(g_time_in_sec))
}
