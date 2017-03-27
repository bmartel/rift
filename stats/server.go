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

type payload struct {
	Message string `json:"message"`
}

type statsServer struct {
	statsStream chan *summary.Stats
	jobStream   chan *summary.JobUpdate
}

func (s *statsServer) UpdateStats(ctx context.Context, stats *summary.Stats) (*summary.Stats, error) {
	s.statsStream <- stats
	return stats, nil
}

func (s *statsServer) UpdateJob(ctx context.Context, job *summary.JobUpdate) (*summary.Job, error) {
	s.jobStream <- job
	return job.Job, nil
}

func setupStatsServer(statsStream chan *summary.Stats, jobStream chan *summary.JobUpdate) {
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
	srv.statsStream = statsStream
	srv.jobStream = jobStream
	summary.RegisterSummaryServer(grpcServer, srv)

	grpcServer.Serve(lis)
}

func socket(statsStream chan *summary.Stats, jobStream chan *summary.JobUpdate) func(*websocket.Conn) {
	return func(ws *websocket.Conn) {
		var p payload
		messageStream := make(chan payload)

		defer func(socketConn *websocket.Conn, message chan payload) {
			socketConn.Close()
			log.Println("Closing socket connection")
			close(message)
		}(ws, messageStream)

		go func(socketConn *websocket.Conn, message chan payload) {
			for {
				if err := websocket.JSON.Receive(socketConn, &p); err != nil {
					log.Println(err)
					log.Println("error reading message")
					message <- payload{"disconnect"}
					return
				}

				message <- p
			}
		}(ws, messageStream)

		for {
			select {
			case msg := <-messageStream:
				switch msg.Message {
				case "disconnect":
					log.Println("Disconnected client")
					return
				}
			case data := <-statsStream:
				if err := websocket.JSON.Send(ws, data); err != nil {
					log.Printf("Socket Error: %v\n", err)
					return
				}
			case data := <-jobStream:
				if err := websocket.JSON.Send(ws, data); err != nil {
					log.Printf("Socket Error: %v\n", err)
					return
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

	statsStream := make(chan *summary.Stats)
	jobStream := make(chan *summary.JobUpdate)

	defer func(stats chan *summary.Stats, job chan *summary.JobUpdate) {
		close(stats)
		close(job)
	}(statsStream, jobStream)

	go setupStatsServer(statsStream, jobStream)

	http.HandleFunc("/", serveTemplate(indexTmpl))

	http.Handle("/ws", websocket.Handler(socket(statsStream, jobStream)))

	box := rice.MustFindBox("static/dist")
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(box.HTTPBox())))

	srvPort := fmt.Sprintf(":%d", *port+1)
	fmt.Println("Listening on port " + srvPort)
	http.ListenAndServe(srvPort, nil)
}
