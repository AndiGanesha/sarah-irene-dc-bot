package main

import (
	"context"
	"log"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/AndiGanesha/sarah-irene-dc-bot/application"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	app, err := application.NewApp(ctx, cancel)
	if err != nil {
		log.Fatal(err)
	}
	defer app.Close()

	<-ctx.Done()
	log.Println("shutting down...")
}
