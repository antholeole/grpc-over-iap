package main

import (
	"context"
	"fmt"
	"github.com/antholeole/grpc-over-iap"
	"log"
	"net"
	"net/http"
	"os"

	pb "example/v1"
	grpchttp1server "golang.stackrox.io/grpc-http1/server"

	"google.golang.org/grpc"
)

const port = 443

// server
type server struct {
	pb.UnimplementedPingServiceServer
}

func (s *server) Ping(ctx context.Context, in *pb.PingRequest) (*pb.PingResponse, error) {
	log.Printf("Received ping: %v", in.GetPing())
	return &pb.PingResponse{Pong: in.GetPing() + 1}, nil
}

// mainly from here https://github.com/stackrox/go-grpc-http1/blob/main/_integration-tests/echo_service_test.go#L810C1-L837C2
func runServer() error {
	grpcSrv := grpc.NewServer()

	pb.RegisterPingServiceServer(grpcSrv, &server{})

	lis, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", port))
	if err != nil {
		log.Fatalf("err listening to tcp port %d: %s", port, err)
	}

	opts := []grpchttp1server.Option{grpchttp1server.PreferGRPCWeb(false)}

	downgradingSrv := &http.Server{}

	httpMux := http.NewServeMux()
	httpMux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	downgradingSrv.Handler = grpchttp1server.CreateDowngradingHandler(grpcSrv, httpMux, opts...)

	log.Printf("running on %s", lis.Addr().String())

	go grpcSrv.Serve(lis)
	err = downgradingSrv.Serve(lis)

	if err != nil {
		log.Printf("terminating grpc server: %s", err)
	}

	return nil
}

// runClient
func runClient() error {
	cc, reqCtx, err := grpcoveriap.BuildClient(context.Background(), grpcoveriap.BuildClientOpts{
		// replace with your IAP url. Don't include the port
		Url:            "localhost:50051",
		ServiceAccount: "[SERVICE_ACCOUNT_NAME]@[PROJECT_ID].iam.gserviceaccount.com",
	})

	if err != nil {
		log.Fatalf("could not build client: %v", err)
	}

	c := pb.NewPingServiceClient(cc)

	r, err := c.Ping(reqCtx, &pb.PingRequest{Ping: 1})
	if err != nil {
		log.Fatalf("could not ping: %v", err)
	}
	log.Printf("Server Response: %d", r.GetPong())

	return nil
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: ./program <server|client>")
		os.Exit(1)
	}

	command := os.Args[1]

	var err error
	switch command {
	case "server":
		err = runServer()
	case "client":
		err = runClient()
	default:
		fmt.Printf("Unknown command: %s\n", command)
		fmt.Println("Usage: ./program <server|client>")
		os.Exit(1)
	}

	if err != nil {
		log.Fatalf("err running %s: %s", command, err.Error())
	}
}
