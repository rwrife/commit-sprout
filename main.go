// Command commit-sprout is a terminal pet that grows an ASCII plant from your
// real git commits. This file is wiring only; all behavior lives in cmd/.
package main

import "github.com/rwrife/commit-sprout/cmd"

func main() {
	cmd.Execute()
}
