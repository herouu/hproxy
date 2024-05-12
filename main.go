package main

import (
	"context"
	"fmt"
	"github.com/bogdanovich/dns_resolver"
	"github.com/kardianos/service"
	"github.com/urfave/cli/v2"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"time"
)

type SystemService struct {
	reverseProxy string
	ip           string
	bind         string
	serviceName  string
}

func (ss *SystemService) Start(s service.Service) error {
	log.Println("coming Start.......")
	go ss.run()
	return nil
}

func (ss *SystemService) run() {
	srv := &http.Server{}
	srv.Addr = ss.bind
	srv.Handler = ss
	log.Printf("Listening on %s, forwarding to %s", ss.bind, ss.reverseProxy)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalln("ListenAndServe: ", err)
	}
}

func (ss *SystemService) Stop(s service.Service) error {
	log.Println("coming Stop.......")
	return nil
}

func main() {
	app := cli.NewApp()
	app.Name = "hproxy"
	app.Usage = "how to use hproxy"
	app.Commands = []*cli.Command{
		{
			Name:   "install",
			Action: ctrlAction,
		},
		{
			Name:   "uninstall",
			Action: ctrlAction,
		},
		{
			Name:   "start",
			Action: ctrlAction,
		},
		{
			Name:   "restart",
			Action: ctrlAction,
		},
		{
			Name:   "stop",
			Action: ctrlAction,
		},
	}
	app.Flags = []cli.Flag{
		&cli.StringFlag{
			Name:    "remote",
			Aliases: []string{"r"},
			Value:   "https://jrebel.qekang.com:443",
			Usage:   "reverse proxy addr",
		},
		&cli.StringFlag{
			Name:    "bind",
			Aliases: []string{"p"},
			Value:   "0.0.0.0:8888",
			Usage:   "listen on port",
		},
		&cli.StringFlag{
			Name:  "ip",
			Value: "",
			Usage: "reverse proxy addr server ip",
		},
		&cli.StringFlag{
			Name:    "name",
			Aliases: []string{"n"},
			Value:   "jrebel-proxy",
			Usage:   "daemon service name",
		},
	}

	app.Action = startAction
	app.Version = "1.0.0"
	err := app.Run(os.Args)
	if err != nil {
		return
	}
}

func createSystemService(c *cli.Context) (service.Service, error) {
	ss := &SystemService{reverseProxy: c.String("remote"), ip: c.String("ip"),
		bind: c.String("bind"), serviceName: c.String("name")}

	svcConfig := &service.Config{
		Name:        ss.serviceName,
		DisplayName: ss.serviceName,
		Description: fmt.Sprintf("this is %s service.", ss.serviceName),
		Arguments:   []string{"--bind", ss.bind, "--remote", ss.reverseProxy, "--ip", ss.ip},
	}

	s, err := service.New(ss, svcConfig)
	if err != nil {
		return nil, fmt.Errorf("service New failed, err: %v\n", err)
	}
	return s, nil
}

func ctrlAction(c *cli.Context) error {
	s, err := createSystemService(c)
	if err != nil {
		log.Printf("createSystemService failed, err: %v\n", err)
		return err
	}
	err = service.Control(s, c.Command.Name)
	log.Printf("ctrl action is %s", c.Command.Name)
	if err != nil {
		log.Printf("service Run %s failed, err: %v\n", c.String("name"), err)
		return err
	}
	return nil
}

func startAction(c *cli.Context) error {
	s, err := createSystemService(c)
	if err != nil {
		log.Printf("createSystemService failed, err: %v\n", err)
		return err
	}
	// 默认 运行 Run
	err = s.Run()
	if err != nil {
		log.Printf("service Run failed, err: %v\n", err)
		return err
	}

	return nil
}

func (h *SystemService) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Println(r.RemoteAddr + " " + r.Method + " " + r.URL.String() + " " + r.Proto + " " + r.UserAgent())
	remote, err := url.Parse(h.reverseProxy)
	if err != nil {
		log.Fatalln(err)
	}
	dialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	http.DefaultTransport.(*http.Transport).DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		remote := strings.Split(addr, ":")
		if h.ip == "" {
			resolver := dns_resolver.New([]string{"114.114.114.114", "114.114.115.115", "119.29.29.29", "223.5.5.5", "8.8.8.8", "208.67.222.222", "208.67.220.220"})
			resolver.RetryTimes = 5
			ip, err := resolver.LookupHost(remote[0])
			if err != nil {
				log.Println(err)
			}
			h.ip = ip[0].String()
		}
		addr = h.ip + ":" + remote[1]
		return dialer.DialContext(ctx, network, addr)
	}
	proxy := httputil.NewSingleHostReverseProxy(remote)
	r.Host = remote.Host
	proxy.ServeHTTP(w, r)
}

func init() {
	logFile, err := os.OpenFile("./hproxy.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		fmt.Println("open log file failed, err:", err)
		return
	}
	log.SetOutput(logFile)
	log.SetFlags(log.Lshortfile | log.Lmicroseconds | log.Ldate)

}
