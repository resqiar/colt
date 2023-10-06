package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/schollz/progressbar/v3"
	"github.com/urfave/cli/v2"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

func main() {
	app := &cli.App{
		Name:    "Colt",
		Usage:   "Load Testing Tool",
		Version: "v6.9 (GOTTEM!)",
		Flags: []cli.Flag{
			&cli.IntFlag{
				Name:    "workers",
				Aliases: []string{"w"},
				Usage:   "Number of conccurent workers connection used",
				Value:   1,
			},
			&cli.IntFlag{
				Name:    "duration",
				Aliases: []string{"d"},
				Usage:   "How long in seconds the job run",
				Value:   10,
			},
			&cli.IntFlag{
				Name:    "amount",
				Aliases: []string{"a"},
				Usage:   "How much jobs must be completed",
				Value:   100,
			},
			&cli.StringFlag{
				Name:     "url",
				Aliases:  []string{"u"},
				Usage:    "Target URL",
				Required: true,
			},
		},
		Action: func(ctx *cli.Context) error {
			amount := ctx.String("amount")
			workers := ctx.String("workers")
			url := ctx.String("url")

			if amount != "" {
				parsed_amount, _ := strconv.Atoi(amount)
				parsed_workers, _ := strconv.Atoi(workers)

				var bar = progressbar.NewOptions(parsed_amount,
					progressbar.OptionEnableColorCodes(true),
					progressbar.OptionSetWidth(30),
					progressbar.OptionSetTheme(progressbar.Theme{
						Saucer:        "[green]=[reset]",
						SaucerHead:    "[green]>[reset]",
						SaucerPadding: " ",
						BarStart:      "[",
						BarEnd:        "]",
					}))

				begin(url, parsed_amount, parsed_workers, bar)
			}

			return nil
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(app)
	}
}

var http_pool = &http.Client{
	Timeout: 1 * time.Second,
}

func worker(URL string, jobs <-chan int, worker_result chan<- bool, worker_done chan<- bool, worker_time chan<- float64, bar *progressbar.ProgressBar) {
	for range jobs {
		startTime := time.Now().UnixMilli()
		res, err := http_pool.Get(URL)
		endTime := time.Now().UnixMilli()

		if err != nil || res.StatusCode != 200 {
			worker_result <- false
		} else {
			worker_result <- true
		}

		defer res.Body.Close()
		worker_time <- float64(endTime - startTime)
		bar.Add(1)
	}

	worker_done <- true
}

func begin(URL string, amount int, workers int, bar *progressbar.ProgressBar) {
	var success int32
	var failed int32

	var number_of_jobs = make(chan int, amount)
	var worker_time = make(chan float64, amount)
	var worker_result = make(chan bool, amount)
	var worker_done = make(chan bool)

	startTime := time.Now()
	for i := 0; i < workers; i++ {
		go worker(URL, number_of_jobs, worker_result, worker_done, worker_time, bar)
	}

	for i := 0; i < amount; i++ {
		number_of_jobs <- i
	}
	close(number_of_jobs)

	for i := 0; i < workers; i++ {
		<-worker_done
	}
	close(worker_result)
	close(worker_time)
	endTime := time.Now()

	// listen for worker_result
	for result := range worker_result {
		if result {
			atomic.AddInt32(&success, 1)
		} else {
			atomic.AddInt32(&failed, 1)
		}
	}

	var times []float64
	var total_sum float64
	for result := range worker_time {
		times = append(times, result)
		total_sum += result
	}

	fmt.Printf("\n")
	message := message.NewPrinter(language.English)
	fmt.Printf("| Jobs completed in %s\n", endTime.Sub(startTime))
	message.Printf("| Total number of success: %d \n", success)
	message.Printf("| Total number of failed : %d \n", failed)
	message.Printf("| Average time taken per request : %fms \n", total_sum/float64(len(times)))
}
