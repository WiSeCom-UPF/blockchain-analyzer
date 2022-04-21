package main

import (
	"fmt"
	"math/big"
	// "reflect"
)

func main() {
	s := "e71f04b5ef63e34"
	i := new(big.Int)
	i.SetString(s, 16)

	s1 := "e71f04b5ef63e35"
	i1 := new(big.Int)
	i1.SetString(s1, 16)

	if i.CmpAbs(i1) == -1{
		fmt.Println("hanji")
	}
	// fmt.Println(temp) // 10
	// fmt.Println(reflect.TypeOf(i))
}