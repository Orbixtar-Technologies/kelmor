package main

import (
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/hosting-panel/panel/internal/objectstore"
)

func main() {
	endpoint := strings.TrimRight(os.Getenv("PANEL_S3_ENDPOINT"), "/")
	listen := os.Getenv("PANEL_S3_LISTEN")
	if listen == "" {
		listen = listenFromEndpoint(endpoint)
	}
	if listen == "" {
		listen = "127.0.0.1:19090"
	}
	root := os.Getenv("PANEL_S3_DATA")
	if root == "" {
		root = "/var/lib/panel/objects"
	}
	if err := os.MkdirAll(root, 0o750); err != nil {
		log.Fatal(err)
	}
	ln, err := net.Listen("tcp", listen)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("object-store %s root=%s", listen, root)
	srv := &objectstore.Server{
		Root:      root,
		AccessKey: os.Getenv("PANEL_S3_ACCESS_KEY"),
		SecretKey: os.Getenv("PANEL_S3_SECRET_KEY"),
		Region:    envDefault("PANEL_S3_REGION", "us-east-1"),
	}
	if err := http.Serve(ln, srv); err != nil {
		log.Fatal(err)
	}
}

func listenFromEndpoint(endpoint string) string {
	if endpoint == "" {
		return ""
	}
	u, err := url.Parse(endpoint)
	if err != nil || u.Host == "" {
		return ""
	}
	return u.Host
}

func envDefault(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
