package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/hajimehoshi/oto"

	"github.com/hajimehoshi/go-mp3"
)

func main() {
	key := flag.String("key", "", "api key")
	obl := flag.Int("oblast", 1, "Oblast")
	trevoga := flag.String("trevoga", "Sub.mp3", "File name alarm on")
	vidbiy := flag.String("vidbiy", "Sub.mp3", "File name alarm off")
	flag.Parse()
	if *key == "" {
		log.Fatalln("No API key")
	}
	addr, err := net.ResolveTCPAddr("tcp", "tcp.alerts.com.ua:1024")
	if err != nil {
		log.Fatalln(err)
	}
	conn, err := net.DialTCP("tcp", nil, addr)
	if err != nil {
		log.Fatalln(err)
	}
	defer conn.Close()
	ctx, cancel := context.WithCancel(context.Background())
	r := bufio.NewReader(conn)
	w := bufio.NewWriter(conn)
	stat := make(chan string)
	signal_chan := make(chan os.Signal, 1)
	signal.Notify(
		signal_chan,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT,
		syscall.SIGABRT,
	)
	go func() {
		s := <-signal_chan
		log.Println(s.String())
		cancel()
	}()
	go func(ctx context.Context, stat chan string) {
		t := time.Now()
		for {
			select {
			case <-ctx.Done():
				close(stat)
				return
			default:
				msg, err := r.ReadString('\n')
				if err != nil {
					if err != io.EOF {
						log.Println(err)
					}
					continue
				}
				fs := strings.Split(msg, ":")
				if len(fs) < 2 {
					continue
				}
				switch fs[0] {
				case "a":
					if fs[1] != "ok" {
						log.Println(fs[1])
						close(stat)
						return
					}
				case "p":
					t = time.Now()
					log.Println(msg)
				case "s":
					stat <- fs[1]
				}
			}
			if time.Since(t).Seconds() > 20 {
				log.Println("Timeout ping")
				close(stat)
				return
			}
		}
	}(ctx, stat)
	_, err = w.WriteString(*key + "\n")
	if err != nil {
		log.Println(err)
	}
	err = w.Flush()
	if err != nil {
		log.Println(err)
	}
	for st := range stat {
		o := strings.Split(st, "=")
		if len(o) < 2 {
			continue
		}
		ob, err := strconv.Atoi(o[0])
		if err != nil {
			continue
		}
		al, err := strconv.ParseBool(o[1])
		if err != nil {
			continue
		}
		fmt.Println(ob, al)
		if ob == int(*obl) {
			if al {
				go play(*trevoga)
			} else {
				go play(*vidbiy)
			}
		}
	}
	os.Exit(0)
}
func play(file string) {
	f, err := os.Open(file)
	if err != nil {
		log.Println(err)
		return
	}
	defer f.Close()
	d, err := mp3.NewDecoder(f)
	if err != nil {
		log.Println(err)
		return
	}
	c, err := oto.NewContext(d.SampleRate(), 2, 2, 8192)
	if err != nil {
		log.Println(err)
		return
	}
	defer c.Close()
	p := c.NewPlayer()
	defer p.Close()
	fmt.Printf("Length: %d[bytes]\n", d.Length())
	if _, err := io.Copy(p, d); err != nil {
		log.Println(err)
		return
	}
}
