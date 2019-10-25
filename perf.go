package main

/*
#cgo CFLAGS: -I/root/dk/spdk/include
#cgo LDFLAGS: -L/root/dk/spdk/build/lib -lspdk -lspdk_env_dpdk
#cgo LDFLAGS: -L/root/dk/spdk/dpdk/build/lib -lrte_eal -lrte_mempool -lrte_ring -lrte_bus_pci

#include "spdk/stdinc.h"
#include "spdk/env.h"
*/
import "C"

import (
	"fmt"
)

func main() {
	opts := C.struct_spdk_env_opts{}
	C.spdk_env_opts_init(&opts)
	opts.name = C.CString("perf")
	opts.shm_id = -1
	fmt.Println(opts)
	returnValue, _ := C.spdk_env_init(&opts)
	if returnValue < 0 {
		fmt.Println("Unable to initialize SPDK env:", returnValue)
		return
	}
}