// Hook Echo is a simply utility used for testing the Webhook package.

package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

func main() {
	if len(os.Args) > 1 {
		fmt.Printf("arg: %s\n", strings.Join(os.Args[1:], " "))
	}

	var env []string
	for _, v := range os.Environ() {
		if strings.HasPrefix(v, "HOOK_") {
			env = append(env, v)
		}
	}

	if len(env) > 0 {
		fmt.Printf("env: %s\n", strings.Join(env, " "))
	}

	if (len(os.Args) > 1) && (strings.HasPrefix(os.Args[1], "exit=")) {
		exitCodeStr := os.Args[1][5:]
		exitCode, err := strconv.Atoi(exitCodeStr)
		if err != nil {
			fmt.Printf("Exit code %s not an int!", exitCodeStr)
			os.Exit(-1)
		}
		os.Exit(exitCode)
	}
}
