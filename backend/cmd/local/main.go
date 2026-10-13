package main

import (
	"context"
	"github.com/lmc10242/wafangdian-majiang/backend/internal/live"
	"log"
	"net/http"
	"os"
)

func main() {
	s := live.NewServer()
	m := http.NewServeMux()
	m.Handle("/v1/", s.Handler())
	m.Handle("/healthz", s.Handler())
	m.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.Dir("../assets"))))
	m.Handle("/", http.FileServer(http.Dir("web")))
	go s.RunClock(context.Background())
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = "127.0.0.1:8081"
	}
	log.Printf("联机自测入口 http://%s", addr)
	log.Fatal(http.ListenAndServe(addr, m))
}
