package main

import (
	"crypto/rand"
	"github.com/thesyncim/exposed"
	"github.com/thesyncim/exposed/encoding/codec/proto"
	"log"
	"net"
	"time"
)

func doExposedServer(port string) {
	payload := make([]byte, *serverPayload)
	_, _ = rand.Read(payload)
	ln, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("unable to listen: %v", err)
	}
	s := exposed.NewServer(
		exposed.ServerCodec(proto.CodecName),
	)
	s.HandleFunc("echo",
		func(ctx *exposed.Context, req exposed.Message, resp exposed.Message) (err error) {
			resp.(*PingResponse).Payload = payload
			return nil
		},
		&exposed.OperationTypes{
			ArgsType: func() exposed.Message {
				return new(PingRequest)
			},
			ReplyType: func() exposed.Message {
				return new(PingResponse)
			},
		})
	if err := s.Serve(ln); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

func exposedWorker(c *exposed.Client) {
	payload := make([]byte, *clientPayload)
	_, _ = rand.Read(payload)

	for {
		start := time.Now()
		req := &PingRequest{Payload: payload}
		var pingResp PingResponse
		err := c.Call("echo", req, &pingResp)
		if err != nil {
			log.Fatal(err)
		}
		elapsed := clampLatency(time.Since(start), minLatency, maxLatency)
		stats.Lock()
		if err := stats.latency.Current.RecordValue(elapsed.Nanoseconds()); err != nil {
			log.Fatal(err)
		}
		stats.ops++
		stats.bytes += uint64(len(payload) + len(pingResp.Payload))
		stats.Unlock()
	}
}

func doexposedClient(addr string) {
	clients := make([]*exposed.Client, *connections)
	for i := 0; i < len(clients); i++ {
		clients[i] = exposed.NewClient(addr,
			exposed.PipelineRequests(true),
			//	exposed.ClientCodec(echocodec{}.Name()),
			exposed.ClientCodec(proto.CodecName),
		)
		clients[i].Call("echo", &PingRequest{}, &PingResponse{})
	}

	for i := 0; i < *concurrency; i++ {
		go exposedWorker(clients[i%len(clients)])
	}
}
