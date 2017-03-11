package main

import (
	"flag"
	"fmt"
	"html/template"
	"log"
	"net"
	"net/http"

	"golang.org/x/net/context"
	"golang.org/x/net/websocket"

	rice "github.com/GeertJohan/go.rice"
	"github.com/bmartel/rift/summary"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/grpclog"
)

var (
	tls      = flag.Bool("tls", false, "Connection uses TLS if true, else plain TCP")
	certFile = flag.String("cert_file", "", "The TLS cert file")
	keyFile  = flag.String("key_file", "", "The TLS key file")
	port     = flag.Int("port", 9147, "The server port")
)

type statsServer struct {
	stream chan *summary.Stats
}

func (s *statsServer) UpdateStats(ctx context.Context, stats *summary.Stats) (*summary.Stats, error) {
	s.stream <- stats
	return stats, nil
}

func setupStatsServer(stream chan *summary.Stats) {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		grpclog.Fatalf("failed to listen: %v", err)
	}
	var opts []grpc.ServerOption
	if *tls {
		creds, err := credentials.NewServerTLSFromFile(*certFile, *keyFile)
		if err != nil {
			grpclog.Fatalf("Failed to generate credentials %v", err)
		}
		opts = []grpc.ServerOption{grpc.Creds(creds)}
	}
	grpcServer := grpc.NewServer(opts...)
	srv := new(statsServer)
	srv.stream = stream
	summary.RegisterSummaryServer(grpcServer, srv)

	grpcServer.Serve(lis)
}

func socket(stream chan *summary.Stats) func(*websocket.Conn) {
	return func(ws *websocket.Conn) {
		for {
			select {
			case data := <-stream:
				if err := websocket.JSON.Send(ws, data); err != nil {
					log.Println(err)
					break
				}
			}
		}
	}
}

func loadTemplate() *template.Template {
	templateBox, err := rice.FindBox("templates")
	if err != nil {
		log.Fatal(err)
	}
	// get file contents as string
	indexString, err := templateBox.String("index.html")
	if err != nil {
		log.Fatal(err)
	}
	// parse and execute the template
	indexTemplate, err := template.New("index").Parse(indexString)
	if err != nil {
		log.Fatal(err)
	}

	return indexTemplate
}

func serveTemplate(index *template.Template) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		index.ExecuteTemplate(w, "index", nil)
	}
}

func main() {
	flag.Parse()
	indexTmpl := loadTemplate()

	stream := make(chan *summary.Stats)
	go setupStatsServer(stream)

	http.HandleFunc("/", serveTemplate(indexTmpl))

	http.Handle("/ws", websocket.Handler(socket(stream)))

	box := rice.MustFindBox("static/dist")
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(box.HTTPBox())))

	srvPort := fmt.Sprintf(":%d", *port+1)
	fmt.Println("Listening on port " + srvPort)
	http.ListenAndServe(srvPort, nil)
}
