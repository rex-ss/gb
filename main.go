package main

import (
	"fmt"
	"github.com/panjf2000/ants/v2"
	"github.com/urfave/cli"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"sync/atomic"
	"time"
)

var (
	txAvg = int64(0) //平均相应时长
	txMax = int64(0) //最大相应时长
	txMin = int64(0) //最小相应时长

	txTotal  = int64(0) //总请求
	txSTotal = int64(0) //成功请求
	txETotal = int64(0) //错误请求

	client = &http.Client{}
)

func main() {

	runtime.GOMAXPROCS(runtime.NumCPU() * 2)

	app := &cli.App{
		Name: "glr",

		UsageText: "Usage: glr [options] [http[s]://]hostname[:port]/path...",
		Authors:   []cli.Author{{Name: "Rex", Email: "rex@163.com"}},
		Version:   "0.0.1",
		Commands: []cli.Command{
			TimeLimit,
		},
	}
	app.Run(os.Args)

}

var TimeLimit = cli.Command{
	Name:        "t",
	Usage:       "timelimit  Seconds to max. to spend on benchmarking",
	Description: "Seconds to max. to spend on benchmarking",
	//Action:timeLimit,
	Flags: []cli.Flag{
		&cli.IntFlag{
			Name:  "limit",
			Value: 1,
			Usage: "first number",
		},
		&cli.StringFlag{
			Name:  "url",
			Value: "",
			Usage: "Url",
		},
		&cli.StringFlag{
			Name:  "p",
			Value: "",
			Usage: "指定要POST/PUT的文件，同时要设置-T参数",
		},
		&cli.StringFlag{
			Name:  "T",
			Value: "text/plain",
			Usage: "指定使用POST或PUT上传文本时的文本类型",
		},
		&cli.StringFlag{
			Name:  "m",
			Value: "GET",
			Usage: "指定使用method",
		},
		&cli.StringFlag{
			Name:  "c",
			Value: "10",
			Usage: "concurrency Number of multiple requests to make at a time",
		},
	},
	Action: timeLimit,
}

func timeLimit(c *cli.Context) {

	ts := c.Int("limit")
	ur := c.String("url")
	m := c.String("m")
	T := c.String("T")
	p := c.String("p")
	concurrency := c.Int("c")
	var bodyReader io.ReadCloser
	if p != "" {
		if body, err := ioutil.ReadFile(p); err != nil {
			log.Fatal("ReadFile:", err.Error())
		} else {
			if _, err = bodyReader.Read(body); err != nil {
				log.Fatal("bodyReader.Read:", err.Error())
			}
		}

	}

	u, err := url.Parse(ur)
	if err != nil {
		log.Fatal("url.Parse error:", err.Error())
	}

	ta := time.After(time.Duration(int64(ts)) * time.Second)
	req1 := &http.Request{}

	req1.Method = m
	req1.URL = u

	req1.Header = make(map[string][]string)
	req1.Header.Set("Content-Type", "text/plain")
	req1.Header.Set("Connection", "keep-alive")
	if T != "" {
		req1.Header.Set("Content-Type", T)
	}
	if p != "" {
		req1.Body = bodyReader
	}
	tm := time.Now()
	client.Do(req1)
	since := int64(time.Since(tm))
	atomic.SwapInt64(&txMin, since)
	pool, _ := ants.NewPool(10000)
	defer pool.Release()
	for i := 0; i < concurrency; i++ {
		go func() {
			req := &http.Request{}
			req.Method = m
			req.URL = u
			req.Header = make(map[string][]string)
			req.Header.Set("Content-Type", "text/plain")
			req.Header.Set("Connection", "keep-alive")
			if T != "" {
				req.Header.Set("Content-Type", T)
			}
			if p != "" {
				req.Body = bodyReader
			}
			for {
				tm := time.Now()
				resp, err := client.Do(req)
				since := int64(time.Since(tm))
				atomic.AddInt64(&txAvg, since)
				if txMax < since {
					atomic.SwapInt64(&txMax, since)
				} else if txMin > since {
					atomic.SwapInt64(&txMin, since)
				}

				if err == nil {
					if resp.StatusCode == 200 {
						atomic.AddInt64(&txSTotal, 1)
					}
				} else {
					fmt.Println("err:", err.Error())
					atomic.AddInt64(&txETotal, 1)
				}
			}
		}()

	}
	<-ta
	log.Println("go number:", runtime.NumGoroutine())
	fmt.Println(fmt.Sprintf("txTotal:\t%d\tSuccess:\t%d\tError:\t%d\ttx/s:%.2f", txSTotal+txETotal, txSTotal, txETotal, float64(txSTotal)/float64(ts)))
	fmt.Println(fmt.Sprintf("avg:\t%s\ttMax\t%s\ttMin\t%s", time.Duration(txAvg/(txSTotal+txETotal)), time.Duration(txMax).String(), time.Duration(txMin).String()))
}
