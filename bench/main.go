package main

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

func main() {
	var numberclients = []int{1, 5, 10}
	var payloadSize = []int{8, 128, 1024, 32 << 10, 64 << 10}
	var frameworks = []string{"grpc", "exposed"}
	var concurrencies = []int{1, 50, 500, 5000, 50000}

	for _, payload := range payloadSize {
		for _, nc := range numberclients {
			for _, concurrency := range concurrencies {
				for _, framework := range frameworks {
					cmd := exec.Command("pinger",
						"-c", strconv.Itoa(payload), "-s", strconv.Itoa(payload),
						"-n", strconv.Itoa(nc),
						"-p", strconv.Itoa(concurrency),
						"-d", "5s",
						"-t", framework)
					out, err := cmd.CombinedOutput()
					if err != nil {
						panic(err)
					}
					fmt.Println(strings.TrimSpace(string(out)))
				}
			}
		}
	}
}
