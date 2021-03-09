package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/go-redis/redis/v8"
)

const (
	prefixalias = "redir:alias:"
	prefixip    = "redir:ip:"
)

func main() {
	opt, err := redis.ParseURL("redis://0.0.0.0:6379/9")
	if err != nil {
		log.Fatalf("failed to create opt store: %v", err)
	}
	cli := redis.NewClient(opt)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	fmt.Println("start...")

	// dump aliases
	aliases, err := cli.Keys(ctx, prefixalias+"*").Result()
	for _, a := range aliases {
		data, err := cli.Get(ctx, a).Result()
		if err != nil {
			log.Fatalf("failed to read the alias: %v", err)
		}
		err = os.WriteFile("alias/"+a+".yml", []byte(data), os.ModePerm)
		if err != nil {
			log.Fatalf("failed to write data: %v", err)
		}
	}
	fmt.Println("finish alias: ", len(aliases))

	// dump ips
	ips, err := cli.Keys(ctx, prefixip+"*").Result()
	for _, ip := range ips {
		data, err := cli.Get(ctx, ip).Result()
		if err != nil {
			log.Fatalf("failed to read the alias: %v", err)
		}
		err = os.WriteFile("ips/"+ip+".yml", []byte(data), os.ModePerm)
		if err != nil {
			log.Fatalf("failed to write data: %v", err)
		}
	}

	fmt.Println("finish ips: ", len(ips))
}
