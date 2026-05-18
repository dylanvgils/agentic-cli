package output

import "fmt"

func Step(name string) {
	fmt.Printf("=> %s\n", name)
}

func Stepf(format string, args ...any) {
	fmt.Printf("=> "+format+"\n", args...)
}
