package main

//
// start the master process, which is implemented
// in ../mr/master.go
//
// go run mrmaster.go pg*.txt
//
// Please do not change this file.
//

import (
	"../mr"
	"io/ioutil"
	"log"
	"regexp"
)
import "time"
import "os"
import "fmt"

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: mrmaster inputfiles...\n")
		os.Exit(1)
	}

	//清空mr-mid mr-out
	files, err := ioutil.ReadDir("./")
	if err != nil {
		log.Fatal(err)
	}
	for _, f := range files {
		if match, _ := regexp.MatchString("mr-mid-*", f.Name()); match {
			os.Remove(f.Name())
		}
		if match, _ := regexp.MatchString("mr-out-*", f.Name()); match {
			os.Remove(f.Name())
		}
	}

	m := mr.MakeMaster(os.Args[1:], 10)
	for m.Done() == false {
		time.Sleep(time.Second)
	}

	time.Sleep(time.Second)
}
