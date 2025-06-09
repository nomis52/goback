package main

import "fmt"

func main() {
	ctrl := NewIPMIController("10.0.0.245", "ADMIN", "ADMIN")
	s, err := ctrl.Status()

	fmt.Println(s)
	fmt.Println(err)
}
