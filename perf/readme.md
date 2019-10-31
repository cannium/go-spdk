Original version:
```
$ ./perf -q 128 -o 4096 -n 1 -w randwrite -t 300 -c 1
Starting SPDK v19.10-pre / DPDK 19.08.0 initialization...
[ DPDK EAL parameters: perf --no-shconf -c 1 --log-level=lib.eal:6 --log-level=lib.cryptodev:5 --log-level=user1:6 --base-virtaddr=0x200000000000 --match-allocations --file-prefix=spdk_pid3186001 ]
EAL: No available hugepages reported in hugepages-1048576kB
Initializing NVMe Controllers
Attaching to NVMe Controller at 0000:5f:00.0
Attached to NVMe Controller at 0000:5f:00.0 [1c5f:0557]
Associating PCIE (0000:5f:00.0) with lcore 0
Initialization complete. Launching workers.
Starting thread on core 0
========================================================
                                                                    Latency(us)
Device Information             :       IOPS      MiB/s    Average        min        max
PCIE (0000:5f:00.0) from core 0:  601306.18    2348.85     212.85       7.11  100492.35
========================================================
Total                          :  601306.18    2348.85     212.85       7.11  100492.35
```

Go version:
```
$ ./perf
env_opts {0x197a2c0 0x7fbb97fa7449 -1 -1 -1 -1 false false false 0 <nil> <nil> <nil> <nil>}
Starting SPDK v19.10-pre / DPDK 19.08.0 initialization...
[ DPDK EAL parameters: perf --no-shconf -c 0x1 --log-level=lib.eal:6 --log-level=lib.cryptodev:5 --log-level=user1:6 --base-virtaddr=0x200000000000 --match-allocations --file-prefix=spdk_pid3182588 ]
EAL: No available hugepages reported in hugepages-1048576kB
Initializing NVMe Controllers
Attaching to NVMe controller at 0000:5f:00.0
Attached to NVMe controller at 0000:5f:00.0 [1c5f:0557]
workers: map[0:0xc00000e080]
controllers: map[PCIE (0000:5f:00.0):0xc0000e8000]
namespaces:
PCIE (0000:5f:00.0) &{controller:0x2000003d50c0 namespace:0x20000098cd40 next:<nil> ioSizeBlocks:8 numIoRequests:384 sizeInIos:781404246 blockSize:512 metadataSize:0 metadataInterleave:false piLoc:false piType:0 ioFlags:0 name:PCIE (0000:5f:00.0)}Associating PCIE (0000:5f:00.0) with lcore 0
Initialization complete. Launching workers.
Starting thread on core 0
Total IOs: 179687263 IOPS: 598957.5433333333
Min latency(ns): 1458 max latency(ns): 146084790
```

Performance penalty less than 1%.