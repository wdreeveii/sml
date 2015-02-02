package main

import (
	"fmt"
	"io/ioutil"
	"sml/parse"
)

func main() {
	data, err := ioutil.ReadFile("example.sml")
	if err != nil {
		fmt.Println(err)
		return
	}
	t, err := parse.Parse("hello", string(data))
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(t["hello"].Root)
	newroot, err := t["hello"].Root.Reduce()
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(t["hello"].Root)
	fmt.Println(newroot)
}
