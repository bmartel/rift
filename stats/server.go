package main

import (
	"flag"
	"fmt"
	"html/template"
	"log"
	"net"
	"net/http"
	"sync"

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

type client struct {
	connections map[*websocket.Conn]bool
	mtx         sync.RWMutex
}

func (c *client) add(conn *websocket.Conn) {
	c.mtx.Lock()
	c.connections[conn] = true
	c.mtx.Unlock()
}

func (c *client) exists(conn *websocket.Conn) bool {
	c.mtx.RLock()
	_, ok := c.connections[conn]
	c.mtx.RUnlock()
	return ok
}

func (c *client) remove(conn *websocket.Conn) {
	c.mtx.Lock()
	if _, ok := c.connections[conn]; ok {
		delete(c.connections, conn)
	}
	c.mtx.Unlock()
}

func (c *client) broadcast(data *summary.JobUpdate) {
	c.mtx.Lock()
	for cl := range c.connections {
		if err := websocket.JSON.Send(cl, data); err != nil {
			log.Printf("Socket Error: %v\n", err)
		}
	}
	c.mtx.Unlock()
}

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

func socket(clients *client) func(*websocket.Conn) {
	return func(ws *websocket.Conn) {
		var p payload

		clients.add(ws)

		defer func(socketConn *websocket.Conn) {
			clients.remove(ws)
			socketConn.Close()
			log.Println("Closing socket connection")
		}(ws)

		for {
			if err := websocket.JSON.Receive(ws, &p); err != nil {
				log.Println(err)
				log.Println("error reading message")
				return
			}
			switch p.Message {
			case "disconnect":
				log.Println("Disconnected client")
				return
			}
		}
	}
}

func socketStream(statsStream chan *summary.Stats, jobStream chan *summary.JobUpdate, clients *client) {
	for {
		select {
		case data := <-jobStream:
			clients.broadcast(data)
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
	clients := &client{
		connections: make(map[*websocket.Conn]bool, 0),
		mtx:         sync.RWMutex{},
	}
	defer func(stats chan *summary.Stats, job chan *summary.JobUpdate, cl *client) {
		close(stats)
		close(job)
	}(statsStream, jobStream, clients)

	go setupStatsServer(statsStream, jobStream)
	go socketStream(statsStream, jobStream, clients)

	http.HandleFunc("/", serveTemplate(indexTmpl))

	http.Handle("/ws", websocket.Handler(socket(clients)))

	box := rice.MustFindBox("static/dist")
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(box.HTTPBox())))

	srvPort := fmt.Sprintf(":%d", *port+1)
	fmt.Println("Listening on port " + srvPort)
	http.ListenAndServe(srvPort, nil)
}
