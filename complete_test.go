package rpcng

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/iqoption/rpcng/codec/json"
	"github.com/iqoption/rpcng/plugin/keepalive"
)

type EchoStrHandler struct{}

func (*EchoStrHandler) Methods() map[string]string {
	return map[string]string{
		"Echo": "Echo",
	}
}

func (*EchoStrHandler) Echo(str string) (string, error) {
	return str, nil
}

type EchoStructHandler struct{}

func (*EchoStructHandler) Methods() map[string]string {
	return map[string]string{
		"Echo": "Echo",
	}
}

func (*EchoStructHandler) Echo(val CodecStruct) (*CodecStruct, error) {
	return &val, nil
}

func BenchmarkRPCNG_String(b *testing.B) {
	var (
		err    error
		addr   = ":9876"
		codec  = json.New()
		server *Server
	)

	if server, err = NewTCPServer(addr, codec); err != nil {
		b.Fatalf("Can't create server: %s", err)
	}

	if err := server.Handler(&EchoStrHandler{}); err != nil {
		b.Fatalf("Can't register handler: %s", err)
	}

	server.Plugin(
		keepalive.NewServer(0),
	)

	if err := server.Start(); err != nil {
		b.Fatalf("Can't start server: %s", err)
	}
	defer server.Stop()

	var client = NewTCPClient(addr, codec)
	if err = client.Start(); err != nil {
		b.Fatalf("Can't start client: %s", err)
	}
	defer client.Stop()

	var request = "Hueraga"

	b.SetParallelism(250)
	b.ReportAllocs()
	b.ResetTimer()
	b.SetBytes(1)
	b.RunParallel(func(pb *testing.PB) {
		for i := 0; pb.Next(); i++ {
			var reply string

			if err := client.Request("Echo").Args(request).Reply(&reply).Call(context.Background()); err != nil {
				b.Fatalf(`Unexpected error when execute call with request "%v". Error: %s"`, request, err)
			}

			if reply != request {
				b.Fatalf("Unexpected response\n%#v\nExpected\n%#v\n", reply, request)
			}
		}
	})
}

func BenchmarkRPCNG_Struct(b *testing.B) {
	var (
		err    error
		addr   = ":9876"
		codec  = json.New()
		server *Server
	)

	if server, err = NewTCPServer(addr, codec); err != nil {
		b.Fatalf("Can't create server: %s", err)
	}

	if err := server.Handler(&EchoStructHandler{}); err != nil {
		b.Fatalf("Can't register handler: %s", err)
	}

	server.Plugin(
		keepalive.NewServer(0),
	)

	if err := server.Start(); err != nil {
		b.Fatalf("Can't start server: %s", err)
	}
	defer server.Stop()

	var client = NewTCPClient(addr, codec)
	if err = client.Start(); err != nil {
		b.Fatalf("Can't start client: %s", err)
	}
	defer client.Stop()

	b.SetParallelism(250)
	b.ReportAllocs()
	b.ResetTimer()
	b.SetBytes(1)
	b.RunParallel(func(pb *testing.PB) {
		for i := 0; pb.Next(); i++ {
			var reply CodecStruct

			if err := client.Request("Echo").Args(&sample).Reply(&reply).Call(context.Background()); err != nil {
				b.Fatalf(`Unexpected error when execute call with request "%v". Error: %s"`, sample, err)
			}

			if !reflect.DeepEqual(reply, sample) {
				b.Fatalf("Unexpected response\n%#v\nExpected\n%#v\n", reply, sample)
			}
		}
	})
}

type SlowHandler struct{}

func (*SlowHandler) Methods() map[string]string {
	return map[string]string{
		"Slow": "Slow",
	}
}

func (*SlowHandler) Slow() (int, error) {
	fmt.Println("execute request 30 sec")
	time.Sleep(30 * time.Second)
	fmt.Println("respond")
	return 907856, nil
}

func TestGracefulShutdown(t *testing.T) {
	var (
		err    error
		addr   = ":9876"
		codec  = json.New()
		server *Server
	)

	if server, err = NewTCPServer(addr, codec); err != nil {
		t.Fatalf("Can't create server: %s", err)
	}

	if err := server.Handler(&SlowHandler{}); err != nil {
		t.Fatalf("Can't register handler: %s", err)
	}

	server.Plugin(
		keepalive.NewServer(0),
	)

	if err := server.Start(); err != nil {
		t.Fatalf("Can't start server: %s", err)
	}
	defer server.Stop()

	var client = NewTCPClient(addr, codec)
	if err := client.Start(); err != nil {
		t.Fatalf("Can't start client: %s", err)
	}
	defer client.Stop()

	done := make(chan bool)

	go func() {
		fmt.Println("Start request")
		var v int
		if err := client.Request("Slow").Args().Reply(&v).Call(context.Background()); err != nil {
			t.Fatalf(`Unexpected error when execute call. Error: %s"`, err)
		}
		fmt.Println("Request done")
		if v != 907856 {
			t.Fatal("Unexpected responce")
		}
		close(done)
	}()

	fmt.Println("Await 10 sec")
	time.Sleep(10 * time.Second)

	fmt.Println("Stopping server")
	server.Stop()

	fmt.Println("Server stopped")

	<-done
	fmt.Println("Test completed")
}
