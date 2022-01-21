package cmd

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var l *log.Logger

type restyLogger struct {
	*log.Logger
}

var (
	// flags
	requests int
	workers  int

	// config
	url     string
	header  map[string]string
	body    string
	method  string
	retry   int
	logPath string
)

// testCmd represents the test command
var testCmd = &cobra.Command{
	Use: "test",
	Run: test,
}

func init() {
	rootCmd.AddCommand(testCmd)

	viper.AddConfigPath(".")
	viper.SetConfigType("json")
	viper.SetConfigName("config")
	if err := viper.ReadInConfig(); err != nil {
		panic(err)
	}

	url = viper.GetString("url")
	header = viper.GetStringMapString("header")
	body = viper.GetString("body")
	method = viper.GetString("method")
	retry = viper.GetInt("retry")
	logPath = viper.GetString("log")

	defaultRequests := 100
	defaultWorkers := 10
	testCmd.Flags().IntVarP(&requests, "request", "r", defaultRequests, "number of requests")
	testCmd.Flags().IntVarP(&workers, "worker", "w", defaultWorkers, "number of workers")
}

func test(cmd *cobra.Command, args []string) {
	l = log.Default()
	if logPath != "" {
		logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0664)
		if err != nil {
			l.Println(err)
		} else {
			l.SetOutput(logFile)
		}
		defer logFile.Close()
	}

	var wg sync.WaitGroup
	jobs := make(chan int, requests)

	for w := 1; w <= workers; w++ {
		go func(w int) {
			worker(w, jobs, &wg)
		}(w)
	}

	start := time.Now()
	for j := 1; j <= requests; j++ {
		wg.Add(1)
		jobs <- j
	}
	close(jobs)
	wg.Wait()

	stop := time.Now()
	total := stop.Sub(start)
	avg := total / time.Duration(requests)
	fmt.Println("Total time:", total.String(), "Avg:", avg.String())
}

func worker(id int, jobs <-chan int, wg *sync.WaitGroup) {
	apiClient := resty.New()
	apiClient.SetTimeout(5 * time.Second)
	apiClient.SetRetryCount(retry)
	rl := restyLogger{l}
	apiClient.SetLogger(rl)
	apiClient.AddRetryCondition(func(r *resty.Response, err error) bool {
		return r.StatusCode() != http.StatusOK
	})

	for j := range jobs {
		req := apiClient.NewRequest()
		req.SetHeaders(header)
		req.SetBody(body)

		resp, err := req.Execute(method, url)
		if err != nil {
			log.Println("worker", id, "started job", j, "\n", err)
		} else {
			log.Print("worker", id, "started job", j, "\n", resp)
		}
		wg.Done()
	}
}

func (l restyLogger) Errorf(format string, v ...interface{}) {
	l.Printf(format, v...)
}
func (l restyLogger) Warnf(format string, v ...interface{}) {
	l.Printf(format, v...)
}
func (l restyLogger) Debugf(format string, v ...interface{}) {
	l.Printf(format, v...)
}
