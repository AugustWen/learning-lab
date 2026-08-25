package main

import (
	"context"
	"fmt"
	"time"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	select {
	// 此处时长影响输出结果，实际是比哪个更快返回,但是并不能保证一定更快
	case <-time.After(3001 * time.Millisecond):
		fmt.Println("overslept")
	case <-ctx.Done():
		fmt.Println("context done", ctx.Err())
	}
}
