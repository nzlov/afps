// +build log

package main

import "fmt"

func log(s ...interface{}) {
	fmt.Println(s...)
}
