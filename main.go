package main

import (
	"fmt"
	"github.com/urfave/cli"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

var (
	txAvg = int64(0) //平均相应时长
	txMax = int64(0) //最大相应时长
	txMin = int64(0) //最小相应时长

	txTotal   = int64(0) //总请求
	txSTotal  = int64(0) //成功请求
	txETotal  = int64(0) //错误请求
	reqsPool  = sync.Pool{}
	reqCilent = sync.Pool{}
)

func main() {

	reqsPool.New = func() interface{} {
		return &http.Request{}
	}

	reqCilent.New = func() interface{} {
		return &http.Client{}
	}

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

	tm := time.Now()

	ts := c.Int("limit")
	ur := c.String("url")
	m := string("m")
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
	once := sync.Once{}
	ta := time.After(time.Duration(int64(ts)) * time.Second)
	for {
		select {
		case <-ta:
			goto end
		default:
			wg := sync.WaitGroup{}

			wg.Add(concurrency)
			for i := 0; i < concurrency; i++ {
				go func() {
					defer func() {
						if r := recover(); r != nil {
							log.Println("timeLimit recover:", r)
						}
					}()

					client := &http.Client{}
					req := reqsPool.Get().(*http.Request)
					req.Method = m
					req.URL = u

					req.Header = make(map[string][]string)
					req.Header.Set("Content-Type", "text/plain")
					if T != "" {
						req.Header.Set("Content-Type", T)
					}
					if p != "" {
						req.Body = bodyReader
					}

					tm := time.Now()
					resp, err := client.Do(req)

					since := int64(time.Since(tm))
					once.Do(func() { //对 min 进行一次初始化
						atomic.SwapInt64(&txMin, since)
					})
					if txMax < since {
						atomic.SwapInt64(&txMax, since)
					} else if txMin > since {
						atomic.SwapInt64(&txMin, since)
					}

					atomic.AddInt64(&txAvg, since)
					atomic.AddInt64(&txTotal, 1)
					if err != nil {
						atomic.AddInt64(&txETotal, 1)
					} else {
						if resp.StatusCode == 200 {
							atomic.AddInt64(&txSTotal, 1)
						}
					}
					wg.Done()
				}()
			}
			wg.Wait()
		}
	}
end:
	log.Println("since:", time.Since(tm))
	fmt.Println(fmt.Sprintf("txTotal:\t%d\tSuccess:\t%d\tError:\t%d\ttx/s:%.2f", txTotal, txSTotal, txETotal, float64(txSTotal)/float64(ts)))
	fmt.Println(fmt.Sprintf("avg:\t%s\ttMax\t%s\ttMin\t%s", time.Duration(txAvg/txTotal).String(), time.Duration(txMax).String(), time.Duration(txMin).String()))
}
