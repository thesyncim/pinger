package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/codahale/hdrhistogram"

	"github.com/dustin/go-humanize"
	"google.golang.org/grpc"
	"io/ioutil"
	"runtime"
	"runtime/debug"
	"runtime/pprof"
	"strconv"
)

var listen = flag.String("l", "localhost:50051", "")
var connections = flag.Int("n", 2, "")
var concurrency = flag.Int("p", 65000, "")
var clientPayload = flag.Int("c", 2, "")
var serverPayload = flag.Int("s", 2, "")
var typ = flag.String("t", "exposed", "")
var duration = flag.Duration("d", time.Second*10, "")
var cpuprof = flag.String("cpuprof", "", "")

/*
                      exposed  conn 10 concurrency 30000 payload 100bytes
_elapsed____ops/s_____MB/s__p50(ms)__p95(ms)__p99(ms)_pMax(ms)
      1s 349517.9     66.7    79.69   150.99   209.72   285.21
      2s 348026.3     66.4    79.69   142.61   176.16   318.77
      3s 357613.5     68.2    88.08   150.99   176.16   234.88
      4s 319309.0     60.9    79.69   159.38   167.77   218.10
      5s 338270.1     64.5    75.50   159.38   226.49   285.21
      6s 378543.6     72.2    75.50   159.38   209.72   285.21
      7s 334797.1     63.9    83.89   159.38   201.33   260.05
      8s 370088.8     70.6    75.50   150.99   176.16   226.49
      9s 378580.8     72.2    79.69   142.61   176.16   209.72
     10s 358173.9     68.3    79.69   150.99   201.33   285.21

_elapsed____ops/s_____MB/s__p50(ms)__p95(ms)__p99(ms)_pMax(ms)
     10s 353091.1     67.3      0.0    151.0    192.9    318.8

                     grpc  conn 10 concurrency 30000 payload 100bytes

_elapsed____ops/s_____MB/s__p50(ms)__p95(ms)__p99(ms)_pMax(ms)
      1s 111151.7     21.2   113.25   570.43   637.53   805.31
      2s 236647.1     45.1   104.86   268.44   285.21   385.88
      3s 219284.7     41.8   109.05   285.21   285.21   369.10
      4s 203171.7     38.8   109.05   285.21   301.99   402.65
      5s 211968.9     40.4   104.86   268.44   285.21   385.88
      6s 245689.6     46.9   109.05   285.21   301.99   369.10
      7s 201664.4     38.5   109.05   285.21   301.99   402.65
      8s 208203.7     39.7   104.86   268.44   285.21   369.10
      9s 245854.9     46.9   109.05   285.21   301.99   402.65
     10s 196955.2     37.6   113.25   285.21   301.99   452.98

_elapsed____ops/s_____MB/s__p50(ms)__p95(ms)__p99(ms)_pMax(ms)
     10s 207356.4     39.6    104.9    285.2    453.0    805.3

 grpc  conn 10 concurrency 30000 payload 1024bytes
_elapsed____ops/s_____MB/s__p50(ms)__p95(ms)__p99(ms)_pMax(ms)
      1s  47126.7     92.0   536.87   738.20   805.31  1207.96
      2s  76681.2    149.8   369.10   570.43   738.20  1610.61
      3s  75967.6    148.4   369.10   520.09   704.64  1275.07
      4s  87341.6    170.6   385.88   469.76   704.64  1140.85
      5s  79218.4    154.7   385.88   536.87   738.20  1140.85
      6s  67142.5    131.1   402.65   503.32   738.20  1409.29
      7s  87561.1    171.0   402.65   520.09   771.75  1207.96
      8s  71758.8    140.2   385.88   520.09   738.20  1342.18
      9s  72513.9    141.6   402.65   503.32   738.20  1140.85
     10s  87058.3    170.0   385.88   603.98   738.20  1140.85

_elapsed____ops/s_____MB/s__p50(ms)__p95(ms)__p99(ms)_pMax(ms)
     10s  74030.7    144.6    369.1    570.4    738.2   1610.6
 exposed  conn 10 concurrency 30000 payload 1024bytes
_elapsed____ops/s_____MB/s__p50(ms)__p95(ms)__p99(ms)_pMax(ms)
      1s  90936.8    177.6   285.21   503.32   570.43   738.20
      2s 147767.2    288.6   209.72   318.77   469.76   603.98
      3s 135500.1    264.6   218.10   251.66   285.21   318.77
      4s 138707.7    270.9   218.10   243.27   260.05   352.32
      5s 126015.8    246.1   218.10   251.66   268.44   318.77
      6s 146063.9    285.3   218.10   251.66   285.21   318.77
      7s 132902.0    259.6   226.49   260.05   285.21   285.21
      8s 136408.7    266.4   226.49   260.05   285.21   318.77
      9s 131948.3    257.7   226.49   251.66   285.21   335.54
     10s 125378.1    244.9   226.49   260.05   285.21   318.77

_elapsed____ops/s_____MB/s__p50(ms)__p95(ms)__p99(ms)_pMax(ms)
     10s 131008.4    255.9      0.0    285.2    419.4    738.2

*/
func doServer(port string) {
	/*if *cpuprof != "" {
		f, err := os.Create(*cpuprof)
		if err != nil {
			log.Fatalf("error creating go cpu file %s", err)
		}
		if err := pprof.StartCPUProfile(f); err != nil {
			log.Fatalf("unable to start cpu profile: %v", err)
		}
		go func() {
			done := make(chan os.Signal, 3)
			signal.Notify(done, syscall.SIGINT, syscall.SIGTERM)
			<-done
			pprof.StopCPUProfile()
			fmt.Printf("profile stopped\n")
			f.Close()
			os.Exit(1)
		}()
	}*/

	switch *typ {
	case "grpc":
		doGrpcServer(port)

	case "gorpc":
		doGorpcServer(port)

	case "exposed":
		doExposedServer(port)

	case "rpcx":
		doRPCXServer(port)

	case "x":
		doXServer(port)

	case "y":
		doYServer(port)

	default:
		log.Fatalf("unknown type: %s", *typ)
	}
}

const (
	minLatency = 10 * time.Microsecond
	maxLatency = 10 * time.Second
)

func clampLatency(d, min, max time.Duration) time.Duration {
	if d < min {
		return min
	}
	if d > max {
		return max
	}
	return d
}

var stats struct {
	sync.Mutex
	latency *hdrhistogram.WindowedHistogram
	ops     uint64
	bytes   uint64
}

func doClient() {
	addr := "localhost:50051"
	if args := flag.Args(); len(args) > 0 {
		addr = flag.Arg(0)
	}

	stats.latency = hdrhistogram.NewWindowed(1,
		minLatency.Nanoseconds(), maxLatency.Nanoseconds(), 1)

	switch *typ {
	case "grpc":
		doGrpcClient(addr)

	case "gorpc":
		doGorpcClient(addr)

	case "rpcx":
		doRPCXClient(addr)

	case "exposed":
		doexposedClient(addr)

	case "x":
		doXClient(addr)

	case "y":
		doYClient(addr)

	default:
		log.Fatalf("unknown type: %s", *typ)
	}

	cumLatency := hdrhistogram.New(minLatency.Nanoseconds(), maxLatency.Nanoseconds(), 1)
	tick := time.Tick(time.Second)
	done := make(chan os.Signal, 3)
	signal.Notify(done, syscall.SIGINT, syscall.SIGTERM)
	start := time.Now()
	lastNow := start
	var lastOps uint64
	var lastBytes uint64

	if *duration > 0 {
		go func() {
			time.Sleep(*duration)

			done <- syscall.Signal(0)
		}()
	}

	if *cpuprof != "" {
		f, err := os.Create(*cpuprof)
		if err != nil {
			log.Fatalf("error creating go cpu file %s", err)
		}
		if err := pprof.StartCPUProfile(f); err != nil {
			log.Fatalf("unable to start cpu profile: %v", err)
		}
		defer func() {
			pprof.StopCPUProfile()
			f.Close()
		}()
	}

	for i := 0; ; {
		select {
		case <-tick:
			stats.Lock()
			ops := stats.ops
			bytes := stats.bytes
			h := stats.latency.Merge()
			stats.latency.Rotate()
			stats.Unlock()

			cumLatency.Merge(h)
			p50 := h.ValueAtQuantile(50)
			p95 := h.ValueAtQuantile(95)
			p99 := h.ValueAtQuantile(99)
			pMax := h.ValueAtQuantile(100)
			now := time.Now()
			elapsed := now.Sub(lastNow).Seconds()

			if i%20 == 0 {
				//fmt.Println("_elapsed____ops/s_____MB/s__p50(ms)__p95(ms)__p99(ms)_pMax(ms)")
			}
			i++
			fmt.Fprintf(ioutil.Discard, " %8.1f %8.1f %8.2f %8.2f %8.2f %8.2f\n",
				float64(ops-lastOps)/elapsed,
				float64(bytes-lastBytes)/(1024*1024*elapsed),
				time.Duration(p50).Seconds()*1000,
				time.Duration(p95).Seconds()*1000,
				time.Duration(p99).Seconds()*1000,
				time.Duration(pMax).Seconds()*1000)

			lastNow = now
			lastOps = ops
			lastBytes = bytes

		case <-done:
			stats.Lock()
			ops := stats.ops
			bytes := stats.bytes
			h := stats.latency.Merge()
			stats.Unlock()

			cumLatency.Merge(h)
			p50 := h.ValueAtQuantile(50)
			p95 := cumLatency.ValueAtQuantile(95)
			p99 := cumLatency.ValueAtQuantile(99)
			pMax := cumLatency.ValueAtQuantile(100)
			elapsed := time.Since(start).Seconds()
			var garC debug.GCStats
			debug.ReadGCStats(&garC)

			var m runtime.MemStats
			runtime.ReadMemStats(&m)

			fmt.Printf(*typ+"|"+strconv.Itoa(*clientPayload)+"|"+strconv.Itoa(*connections)+"|"+strconv.Itoa(*concurrency)+" | %8.1f | %8.1f | %8.1f | %8.1f | %8.1f |%8.1f | %d | %s\n",
				float64(ops)/elapsed,
				float64(bytes)/(1024*1024*elapsed),
				time.Duration(p50).Seconds()*1000,
				time.Duration(p95).Seconds()*1000,
				time.Duration(p99).Seconds()*1000,
				time.Duration(pMax).Seconds()*1000,
				garC.NumGC,
				humanize.Bytes(m.TotalAlloc))

			return
		}
	}
}

func main() {
	grpc.EnableTracing = false
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	flag.Parse()

	go func() {
		doServer(*listen)
		return

	}()

	doClient()
}
